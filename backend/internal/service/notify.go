package service

import (
	"io"
	"go.uber.org/zap"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"racion/internal/domain"
	"racion/internal/i18n"
)

// Напоминания через Web Push. Ключи VAPID создаются один раз и хранятся в settings.
// Tick вызывается раз в 10–15 минут: для каждого подписанного считает местное время и шлёт то,
// что пора: «в магазин» в выбранный день и час, «завтра готовим …» вечером, «собрать неделю» в воскресенье.

type PushRepo interface {
	Save(ctx context.Context, userID string, s domain.PushSubscription) error
	Delete(ctx context.Context, endpoint string) error
	ByUser(ctx context.Context, userID string) ([]domain.PushSubscription, error)
	Users(ctx context.Context) ([]domain.NotifyUser, error)
	Settings(ctx context.Context, userID string) (domain.NotifySettings, error)
	SetSettings(ctx context.Context, userID string, s domain.NotifySettings) error
	MarkSent(ctx context.Context, userID, key string) (bool, error)
	PlansForReminders(ctx context.Context, userID string) ([]domain.PlanReminderInfo, error)
	// устройства без аккаунта: подписка привязана не к человеку, а к неделям, открытым на устройстве
	SaveDevice(ctx context.Context, s domain.PushSubscription, settings domain.NotifySettings, planID string) error
	DeleteDevice(ctx context.Context, endpoint string) error
	Devices(ctx context.Context) ([]domain.PushDevice, error)
	DevicePlans(ctx context.Context, endpoint string) ([]domain.PlanReminderInfo, error)
	MarkSentDevice(ctx context.Context, endpoint, key string) (bool, error)
}

type SettingsRepo interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

type Notifications struct {
	repo       PushRepo
	settings   SettingsRepo
	subscriber string // mailto: для VAPID
	baseURL    string
	pub, priv  string
	send       func(ctx context.Context, sub domain.PushSubscription, n domain.Notification) (int, error)
	digest     func(ctx context.Context) (recipes, collections int) // что нового за неделю; nil — дайджест не шлём
	log        *zap.Logger
}

func NewNotifications(repo PushRepo, settings SettingsRepo, subscriber, baseURL string) *Notifications {
	// webpush-go сам добавляет «mailto:» ко всему, что не https-URL; с готовым «mailto:» выходит «mailto:mailto:…»,
	// FCM это терпит, а Apple отвечает 403 BadJwtToken
	n := &Notifications{repo: repo, settings: settings, subscriber: strings.TrimPrefix(subscriber, "mailto:"), baseURL: strings.TrimRight(baseURL, "/")}
	n.send = n.webpush
	n.log = zap.L().Named("push")
	return n
}

// Init загружает или создаёт ключи VAPID.
func (n *Notifications) Init(ctx context.Context) error {
	pub, err1 := n.settings.Get(ctx, "vapid_public")
	priv, err2 := n.settings.Get(ctx, "vapid_private")
	if err1 == nil && err2 == nil && pub != "" && priv != "" {
		n.pub, n.priv = pub, priv
		return nil
	}
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return err
	}
	if err := n.settings.Set(ctx, "vapid_public", pub); err != nil {
		return err
	}
	if err := n.settings.Set(ctx, "vapid_private", priv); err != nil {
		return err
	}
	n.pub, n.priv = pub, priv
	return nil
}

func (n *Notifications) PublicKey() string { return n.pub }

func (n *Notifications) Subscribe(ctx context.Context, userID string, s domain.PushSubscription) error {
	if !validSub(&s) {
		return domain.ErrBadInput
	}
	return n.repo.Save(ctx, userID, s)
}

func (n *Notifications) Unsubscribe(ctx context.Context, endpoint string) error {
	return n.repo.Delete(ctx, endpoint)
}

// validSub проверяет подписку браузера; незнакомый язык становится русским.
func validSub(s *domain.PushSubscription) bool {
	if s.Endpoint == "" || len(s.Endpoint) > 1000 || s.P256dh == "" || s.Auth == "" || !strings.HasPrefix(s.Endpoint, "https://") {
		return false
	}
	if _, ok := i18n.Valid(s.Lang); !ok {
		s.Lang = "ru"
	}
	return true
}

// SubscribeDevice — напоминания без регистрации: подписка этого устройства и неделя, по которой напоминать.
// Повторный вызов с другой неделей добавляет её к устройству. today — утреннее «что готовим сегодня»;
// остальное как у всех: список в день покупок, в воскресенье — собрать следующую неделю.
func (n *Notifications) SubscribeDevice(ctx context.Context, s domain.PushSubscription, planID string, tz int, today bool) error {
	if !validSub(&s) || planID == "" || tz < -14*60 || tz > 14*60 {
		return domain.ErrBadInput
	}
	set := domain.DeviceNotify()
	set.Today = today
	set.Tz = tz
	return n.repo.SaveDevice(ctx, s, set, planID)
}

func (n *Notifications) UnsubscribeDevice(ctx context.Context, endpoint string) error {
	return n.repo.DeleteDevice(ctx, endpoint)
}

func (n *Notifications) Settings(ctx context.Context, userID string) (domain.NotifySettings, int, error) {
	s, err := n.repo.Settings(ctx, userID)
	if err != nil {
		return s, 0, err
	}
	subs, _ := n.repo.ByUser(ctx, userID)
	return s, len(subs), nil
}

func (n *Notifications) SetSettings(ctx context.Context, userID string, s domain.NotifySettings) error {
	if s.ShopDay < 0 || s.ShopDay > 6 || s.ShopHour < -1 || s.ShopHour > 23 || s.Tz < -14*60 || s.Tz > 14*60 || s.TodayHour < 0 || s.TodayHour > 23 || s.PrepHour < 0 || s.PrepHour > 23 {
		return domain.ErrBadInput
	}
	return n.repo.SetSettings(ctx, userID, s)
}

// Test шлёт проверочное сообщение на все устройства пользователя.
func (n *Notifications) Test(ctx context.Context, userID string, lang i18n.Lang) error {
	return n.deliver(ctx, userID, domain.Notification{Title: i18n.T(lang, "push.test.title"), Body: i18n.T(lang, "push.test.body"), URL: n.baseURL + "/me", Tag: "test"})
}

// Tick — один проход по подписчикам.
func (n *Notifications) Tick(ctx context.Context, now time.Time) (sent int, err error) {
	users, err := n.repo.Users(ctx)
	if err != nil {
		return 0, err
	}
	for _, u := range users {
		sent += n.tickUser(ctx, u, now)
	}
	devices, err := n.repo.Devices(ctx)
	if err != nil {
		return sent, err
	}
	for _, d := range devices {
		sent += n.tickDevice(ctx, d, now)
	}
	return sent, nil
}

// tickDevice — напоминания устройству без аккаунта по его неделям. Подписка у него одна, поэтому
// отказ push-сервиса (подписки больше нет) сразу удаляет устройство вместе с его неделями.
func (n *Notifications) tickDevice(ctx context.Context, d domain.PushDevice, now time.Time) int {
	plans, err := n.repo.DevicePlans(ctx, d.Sub.Endpoint)
	if err != nil || len(plans) == 0 {
		return 0
	}
	lang := i18n.Lang(d.Sub.Lang)
	local := now.UTC().Add(time.Duration(d.Settings.Tz) * time.Minute)
	sent := 0
	for _, r := range dueReminders(local, d.Settings, plans) {
		// «как было?» записывается в аккаунт, дайджест новинок — для тех, кто его включил в кабинете
		if r.Kind == "ask" || r.Kind == "digest" {
			continue
		}
		if ok, _ := n.repo.MarkSentDevice(ctx, d.Sub.Endpoint, r.Key); !ok {
			continue
		}
		msg, ok := n.notification(ctx, r, lang)
		if !ok {
			continue
		}
		code, err := n.send(ctx, d.Sub, msg)
		if code == http.StatusNotFound || code == http.StatusGone || code == http.StatusForbidden {
			n.log.Warn("push device dropped", zap.String("endpoint", short(d.Sub.Endpoint)), zap.Int("status", code), zap.Error(err))
			_ = n.repo.Delete(ctx, d.Sub.Endpoint)
			return sent
		}
		if err != nil {
			n.log.Warn("push device", zap.String("kind", r.Kind), zap.Error(err))
			continue
		}
		sent++
	}
	return sent
}

func (n *Notifications) tickUser(ctx context.Context, u domain.NotifyUser, now time.Time) int {
	subs, err := n.repo.ByUser(ctx, u.UserID)
	if err != nil || len(subs) == 0 {
		return 0
	}
	lang := i18n.Lang(subs[0].Lang)
	plans, err := n.repo.PlansForReminders(ctx, u.UserID)
	if err != nil {
		return 0
	}
	local := now.UTC().Add(time.Duration(u.Settings.Tz) * time.Minute)
	sent := 0
	for _, r := range dueReminders(local, u.Settings, plans) {
		if r.Kind == "digest" && n.digest == nil {
			continue
		}
		// журнал общий с ботом: если напоминание уже ушло в Telegram, здесь оно отмечено и не повторится
		if ok, _ := n.repo.MarkSent(ctx, u.UserID, r.Key); !ok {
			continue
		}
		msg, ok := n.notification(ctx, r, lang)
		if !ok {
			continue
		}
		n.deliver(ctx, u.UserID, msg)
		sent++
	}
	return sent
}

// notification — напоминание в виде веб-пуша; false — сказать нечего (в дайджесте пусто).
func (n *Notifications) notification(ctx context.Context, r Reminder, lang i18n.Lang) (domain.Notification, bool) {
	plan := n.baseURL + "/plan/" + r.PlanID
	switch r.Kind {
	case "shop":
		return domain.Notification{Title: i18n.T(lang, "push.shop.title"), Body: i18n.T(lang, "push.shop.body", r.Left), URL: plan + "?mode=shop", Tag: "shop"}, true
	case "today":
		return domain.Notification{Title: i18n.T(lang, "push.today.title"), Body: strings.Join(r.Dishes, " · "), URL: plan + "?day=" + r.Date, Tag: "today"}, true
	case "prepday-eve":
		return domain.Notification{Title: i18n.T(lang, "push.prepday.eve.title"), Body: i18n.T(lang, "push.prepday.body", r.Prep.Items, minutesLabel(lang, r.Prep.TotalMin)), URL: plan + "?mode=shop", Tag: "prepday"}, true
	case "prepday":
		return domain.Notification{Title: i18n.T(lang, "push.prepday.title"), Body: i18n.T(lang, "push.prepday.body", r.Prep.Items, minutesLabel(lang, r.Prep.TotalMin)), URL: plan + "#prep", Tag: "prepday"}, true
	case "prep":
		return domain.Notification{Title: i18n.T(lang, "push.prep.title"), Body: strings.Join(r.Dishes, " · "), URL: plan, Tag: "prep"}, true
	case "ask":
		return domain.Notification{Title: i18n.T(lang, "push.ask.title", r.Dinner.Title), Body: i18n.T(lang, "push.ask.body"), URL: plan, Tag: "ask", Recipe: r.Dinner.RecipeID,
			Actions: []domain.NotificationAction{{Action: "like", Title: i18n.T(lang, "feedback.like")}, {Action: "meh", Title: i18n.T(lang, "feedback.meh")}}}, true
	case "digest":
		// только если что-то появилось
		if recipes, cols := n.digest(ctx); recipes+cols > 0 {
			return domain.Notification{Title: i18n.T(lang, "push.digest.title"), Body: i18n.T(lang, "push.digest.body", recipes, cols), URL: n.baseURL + "/collections", Tag: "digest"}, true
		}
	case "week":
		body := i18n.T(lang, "push.week.body")
		if len(r.Weaning) > 0 && r.Weaning[0].Next != "" {
			body = i18n.T(lang, "push.week.wean", r.Weaning[0].Next)
		}
		return domain.Notification{Title: i18n.T(lang, "push.week.title"), Body: body, URL: n.baseURL + "/", Tag: "week"}, true
	case "wean":
		return domain.Notification{Title: i18n.T(lang, "push.wean.title"), Body: strings.Join(weanLines(lang, r.Weaning), "\n") + "\n" + i18n.T(lang, "push.wean.how"), URL: plan + "#weaning", Tag: "wean"}, true
	case "wean-diary":
		return domain.Notification{Title: i18n.T(lang, "push.weandiary.title", r.Weaning[0].Food), Body: i18n.T(lang, "push.weandiary.body"), URL: plan + "#weaning", Tag: "wean-diary"}, true
	}
	return domain.Notification{}, false
}

// deliver шлёт на все устройства; мёртвые подписки (404/410) удаляет.
func (n *Notifications) deliver(ctx context.Context, userID string, msg domain.Notification) error {
	subs, err := n.repo.ByUser(ctx, userID)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return domain.ErrNotFound
	}
	var last error
	alive := 0
	for _, s := range subs {
		code, err := n.send(ctx, s, msg)
		// 404/410 — подписки больше нет; 403 — подписка на чужие ключи VAPID (старый сервер): тоже мёртвая
		if code == http.StatusNotFound || code == http.StatusGone || code == http.StatusForbidden {
			n.log.Warn("push subscription dropped", zap.String("endpoint", short(s.Endpoint)), zap.Int("status", code), zap.Error(err))
			_ = n.repo.Delete(ctx, s.Endpoint)
			continue
		}
		if err != nil {
			last = err
			continue
		}
		alive++
	}
	// все подписки оказались мёртвыми: для клиента это «устройств нет», он переподпишется и повторит
	if alive == 0 && last == nil {
		return domain.ErrNotFound
	}
	return last
}

func short(endpoint string) string { return endpoint[:min(len(endpoint), 48)] }

func (n *Notifications) webpush(ctx context.Context, s domain.PushSubscription, msg domain.Notification) (int, error) {
	body, _ := json.Marshal(msg)
	res, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{Endpoint: s.Endpoint, Keys: webpush.Keys{P256dh: s.P256dh, Auth: s.Auth}},
		&webpush.Options{Subscriber: n.subscriber, VAPIDPublicKey: n.pub, VAPIDPrivateKey: n.priv, TTL: 6 * 3600, Urgency: webpush.UrgencyNormal})
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		// push-сервисы объясняют отказ в теле (Apple: {"reason":"BadJwtToken"}), оно нужно в логе
		body, _ := io.ReadAll(io.LimitReader(res.Body, 300))
		return res.StatusCode, fmt.Errorf("push: http %d %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	if msg.Tag == "test" {
		n.log.Info("push test sent", zap.String("endpoint", short(s.Endpoint)), zap.Int("status", res.StatusCode))
	}
	return res.StatusCode, nil
}

// SetDigest подключает подсчёт новинок за неделю для пятничного дайджеста.
func (n *Notifications) SetDigest(f func(ctx context.Context) (int, int)) { n.digest = f }

// minutesLabel — «4 ч 10 мин» на языке уведомления.
func minutesLabel(l i18n.Lang, m int) string {
	if m < 60 {
		return i18n.T(l, "push.min", m)
	}
	if m%60 == 0 {
		return i18n.T(l, "push.hours", m/60)
	}
	return i18n.T(l, "push.hoursMin", m/60, m%60)
}
