package service

import (
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
}

func NewNotifications(repo PushRepo, settings SettingsRepo, subscriber, baseURL string) *Notifications {
	n := &Notifications{repo: repo, settings: settings, subscriber: subscriber, baseURL: strings.TrimRight(baseURL, "/")}
	n.send = n.webpush
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
	if s.Endpoint == "" || len(s.Endpoint) > 1000 || s.P256dh == "" || s.Auth == "" || !strings.HasPrefix(s.Endpoint, "https://") {
		return domain.ErrBadInput
	}
	if _, ok := i18n.Valid(s.Lang); !ok {
		s.Lang = "ru"
	}
	return n.repo.Save(ctx, userID, s)
}

func (n *Notifications) Unsubscribe(ctx context.Context, endpoint string) error {
	return n.repo.Delete(ctx, endpoint)
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
	if s.ShopDay < 0 || s.ShopDay > 6 || s.ShopHour < 0 || s.ShopHour > 23 || s.Tz < -14*60 || s.Tz > 14*60 {
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
	return sent, nil
}

func (n *Notifications) tickUser(ctx context.Context, u domain.NotifyUser, now time.Time) int {
	local := now.UTC().Add(time.Duration(u.Settings.Tz) * time.Minute)
	today := local.Format("2006-01-02")
	tomorrow := local.AddDate(0, 0, 1).Format("2006-01-02")
	subs, err := n.repo.ByUser(ctx, u.UserID)
	if err != nil || len(subs) == 0 {
		return 0
	}
	lang := i18n.Lang(subs[0].Lang)
	plans, err := n.repo.PlansForReminders(ctx, u.UserID)
	if err != nil {
		return 0
	}
	sent := 0
	// магазин: в выбранный день и час, для ближайшей недели, где ещё есть что купить
	if int(local.Weekday()) == u.Settings.ShopDay && local.Hour() == u.Settings.ShopHour {
		for _, p := range plans {
			if p.StartDate < local.AddDate(0, 0, -6).Format("2006-01-02") || p.StartDate > local.AddDate(0, 0, 7).Format("2006-01-02") {
				continue
			}
			left := p.Items - p.Checked
			if left <= 0 {
				continue
			}
			if ok, _ := n.repo.MarkSent(ctx, u.UserID, "shop:"+p.ID+":"+today); ok {
				n.deliver(ctx, u.UserID, domain.Notification{Title: i18n.T(lang, "push.shop.title"), Body: i18n.T(lang, "push.shop.body", left), URL: n.baseURL + "/plan/" + p.ID + "?mode=shop", Tag: "shop"})
				sent++
			}
			break
		}
	}
	// вечером: что готовим завтра
	if u.Settings.Prep && local.Hour() == 19 {
		for _, p := range plans {
			dishes := p.Dishes[tomorrow]
			if len(dishes) == 0 {
				continue
			}
			if ok, _ := n.repo.MarkSent(ctx, u.UserID, "prep:"+p.ID+":"+tomorrow); ok {
				n.deliver(ctx, u.UserID, domain.Notification{Title: i18n.T(lang, "push.prep.title"), Body: strings.Join(dishes, " · "), URL: n.baseURL + "/plan/" + p.ID, Tag: "prep"})
				sent++
			}
			break
		}
	}
	// после ужина: «как было?» с двумя кнопками — ответ учит планировщик
	if !u.Settings.NoAsk && local.Hour() == 20 {
		for _, p := range plans {
			d, ok := p.Dinner[today]
			if !ok || strings.HasPrefix(d.RecipeID, "u_") {
				continue
			}
			if ok, _ := n.repo.MarkSent(ctx, u.UserID, "ask:"+p.ID+":"+today); ok {
				n.deliver(ctx, u.UserID, domain.Notification{Title: i18n.T(lang, "push.ask.title", d.Title), Body: i18n.T(lang, "push.ask.body"), URL: n.baseURL + "/plan/" + p.ID, Tag: "ask", Recipe: d.RecipeID,
					Actions: []domain.NotificationAction{{Action: "like", Title: i18n.T(lang, "feedback.like")}, {Action: "meh", Title: i18n.T(lang, "feedback.meh")}}})
				sent++
			}
			break
		}
	}
	// воскресенье в полдень: на следующую неделю плана нет
	if u.Settings.Week && local.Weekday() == time.Sunday && local.Hour() == 12 {
		monday := local.AddDate(0, 0, 1).Format("2006-01-02")
		has := false
		for _, p := range plans {
			if p.StartDate == monday {
				has = true
			}
		}
		if !has {
			if ok, _ := n.repo.MarkSent(ctx, u.UserID, "week:"+monday); ok {
				n.deliver(ctx, u.UserID, domain.Notification{Title: i18n.T(lang, "push.week.title"), Body: i18n.T(lang, "push.week.body"), URL: n.baseURL + "/", Tag: "week"})
				sent++
			}
		}
	}
	return sent
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
	for _, s := range subs {
		code, err := n.send(ctx, s, msg)
		if code == http.StatusNotFound || code == http.StatusGone {
			_ = n.repo.Delete(ctx, s.Endpoint)
			continue
		}
		if err != nil {
			last = err
		}
	}
	return last
}

func (n *Notifications) webpush(ctx context.Context, s domain.PushSubscription, msg domain.Notification) (int, error) {
	body, _ := json.Marshal(msg)
	res, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{Endpoint: s.Endpoint, Keys: webpush.Keys{P256dh: s.P256dh, Auth: s.Auth}},
		&webpush.Options{Subscriber: n.subscriber, VAPIDPublicKey: n.pub, VAPIDPrivateKey: n.priv, TTL: 6 * 3600, Urgency: webpush.UrgencyNormal})
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return res.StatusCode, fmt.Errorf("push: http %d", res.StatusCode)
	}
	return res.StatusCode, nil
}
