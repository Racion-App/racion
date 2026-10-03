package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/messenger"
	"racion/internal/planner"
)

// Bots — боты «Рациона» в Telegram и MAX. Неделю подключают по ссылке со страницы плана, и бот
// присылает список покупок по отделам: в магазине идут от молочного к овощам, а не листают
// полсотни строк. Нажал на товар — он отмечен купленным и на сайте. Мессенджеры спрятаны за
// messenger.Client, логика одна на оба. В Telegram сайт ещё и открывается внутри мессенджера
// мини-приложением, а привязанный аккаунт видит в боте свои недели без отдельной ссылки.
type Bots struct {
	clients  map[messenger.Platform]messenger.Client
	repo     MessengerRepo
	plans    BotPlans
	accounts BotAccounts
	taste    BotTaste    // «как было?»: ответ учит планировщик аккаунта
	journal  UserJournal // журнал веб-пуша: что уже напомнили аккаунту
	baseURL  string
	log      *zap.Logger
	pause    time.Duration // между напоминаниями: Telegram пускает около 30 сообщений в секунду
}

// BotDeps — что ботам нужно от остального приложения.
type BotDeps struct {
	Repo     MessengerRepo
	Plans    BotPlans
	Accounts BotAccounts
	Taste    BotTaste
	Journal  UserJournal
}

// BotPlans — недели для бота: список, отметки, свои покупки, недели аккаунта.
type BotPlans interface {
	Get(ctx context.Context, id string, lang i18n.Lang) (planner.Plan, error)
	Original(ctx context.Context, id string) (planner.Plan, i18n.Lang, error)
	Extras(ctx context.Context, planID string) ([]domain.Extra, error)
	Checks(ctx context.Context, planID string) ([]string, error)
	SetCheck(ctx context.Context, planID string, in domain.CheckInput, viewer *domain.User) error
	Mine(ctx context.Context, userID string) ([]domain.PlanSummary, error)
}

// BotTaste — ответ «как было?» после ужина.
type BotTaste interface {
	Feedback(ctx context.Context, userID, recipeID string, liked bool) error
}

// UserJournal — журнал отправленных веб-пушей аккаунта.
type UserJournal interface {
	MarkSent(ctx context.Context, userID, key string) (bool, error)
}

type MessengerRepo interface {
	SaveChat(ctx context.Context, platform, chatID, lang string) (bool, error)
	SetLang(ctx context.Context, platform, chatID, lang string) error
	SetUser(ctx context.Context, platform, chatID string, userID *string) error
	ForgetUser(ctx context.Context, platform, userID string) error
	CreateLink(ctx context.Context, tokenHash, userID string, expires time.Time) error
	TakeLink(ctx context.Context, tokenHash string) (string, error)
	ReminderPlans(ctx context.Context, platform, chatID string, userID *string) ([]domain.PlanReminderInfo, error)
	Chat(ctx context.Context, platform, chatID string) (domain.MessengerChat, error)
	SetBlocked(ctx context.Context, platform, chatID string, blocked bool) error
	SetSettings(ctx context.Context, platform, chatID string, s domain.NotifySettings) error
	LinkPlan(ctx context.Context, platform, chatID, planID string) error
	Plans(ctx context.Context, platform, chatID string) ([]domain.ChatPlan, error)
	MarkSent(ctx context.Context, platform, chatID, key string) (bool, error)
	Active(ctx context.Context) ([]domain.MessengerChat, error)
}

// BotAccounts — аккаунты сайта для бота: кто привязал мессенджер.
type BotAccounts interface {
	ByLink(ctx context.Context, provider, id string) (domain.User, error)
	Link(ctx context.Context, userID, provider, id string, move bool) error
}

// LinkTTL — сколько живёт ссылка «Привязать Telegram» из кабинета.
const LinkTTL = 15 * time.Minute

func NewBots(d BotDeps, baseURL string, log *zap.Logger) *Bots {
	return &Bots{clients: map[messenger.Platform]messenger.Client{}, repo: d.Repo, plans: d.Plans, accounts: d.Accounts, taste: d.Taste, journal: d.Journal,
		baseURL: strings.TrimRight(baseURL, "/"), log: log, pause: 40 * time.Millisecond}
}

// Add подключает мессенджер; без токена бота мессенджер просто не добавляют.
func (b *Bots) Add(c messenger.Client) { b.clients[c.Platform()] = c }

func (b *Bots) Client(p string) (messenger.Client, bool) {
	if b == nil {
		return nil, false
	}
	c, ok := b.clients[messenger.Platform(p)]
	return c, ok
}

func (b *Bots) All() []messenger.Client {
	if b == nil {
		return nil
	}
	out := make([]messenger.Client, 0, len(b.clients))
	for _, p := range []messenger.Platform{messenger.Telegram, messenger.Max} {
		if c, ok := b.clients[p]; ok {
			out = append(out, c)
		}
	}
	return out
}

// Names — имена ботов по мессенджерам: сайт показывает по ним карточку привязки и ссылки.
func (b *Bots) Names() map[string]string {
	if b == nil || len(b.clients) == 0 {
		return nil
	}
	out := make(map[string]string, len(b.clients))
	for p, c := range b.clients {
		out[string(p)] = c.Username()
	}
	return out
}

// Hook ставит вебхуки на baseURL/api/bots/<мессенджер>. Обновления Telegram и MAX шлют только
// на https, поэтому на локальном стенде без публичного адреса бот молчит.
func (b *Bots) Hook(ctx context.Context) {
	if b == nil || len(b.clients) == 0 {
		return
	}
	if !strings.HasPrefix(b.baseURL, "https://") {
		b.log.Warn("bots: no https BASE_URL, webhooks not set", zap.String("base", b.baseURL))
		return
	}
	for _, c := range b.All() {
		hook := b.baseURL + "/api/bots/" + string(c.Platform())
		var err error
		for try := 1; try <= 3; try++ {
			if err = c.SetWebhook(ctx, hook); err == nil {
				break
			}
			select { // API мессенджера мог быть недоступен в момент запуска
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(try) * 20 * time.Second):
			}
		}
		if err != nil {
			b.log.Warn("bots: webhook", zap.String("platform", string(c.Platform())), zap.Error(err))
			continue
		}
		b.log.Info("bots: webhook", zap.String("platform", string(c.Platform())), zap.String("bot", c.Username()))
		if err := c.SetProfile(ctx, profiles(c)); err != nil {
			b.log.Warn("bots: profile", zap.String("platform", string(c.Platform())), zap.Error(err))
		}
		// кнопка меню по умолчанию — по-русски: чат на другом языке получит свою при первом /start
		b.menu(ctx, c, "", i18n.RU)
	}
}

// profiles — описание бота и команда /list на всех языках сайта; без языка — английский.
func profiles(c messenger.Client) map[string]messenger.Profile {
	out := make(map[string]messenger.Profile, len(i18n.Langs)+1)
	for _, l := range append([]i18n.Lang{""}, i18n.Langs...) {
		tl := l
		if l == "" {
			tl = i18n.EN
		}
		about := i18n.T(tl, "bot.about")
		out[string(l)] = messenger.Profile{
			About:       about,
			Description: about + "\n\n" + i18n.T(tl, "bot.intro", i18n.T(tl, "plan.bot."+string(c.Platform()))),
			Commands: []messenger.Command{
				{Name: "list", Description: i18n.T(tl, "bot.cmd.list")},
				{Name: "settings", Description: i18n.T(tl, "bot.cmd.settings")},
			},
		}
	}
	return out
}

// PlanLinks — ссылки «подключить неделю» для страницы плана: только мессенджеры, у которых есть бот.
func (b *Bots) PlanLinks(planID string) map[string]string {
	if b == nil || len(b.clients) == 0 {
		return nil
	}
	key, ok := planKey(planID)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(b.clients))
	for p, c := range b.clients {
		out[string(p)] = c.StartLink("p" + key)
	}
	return out
}

// Handle — одно обновление из мессенджера. Человек заблокировал бота — не ошибка: чат отмечен,
// писать ему больше нечего.
func (b *Bots) Handle(ctx context.Context, u messenger.Update) error {
	if err := b.handle(ctx, u); err != nil && !errors.Is(err, messenger.ErrBlocked) {
		return err
	}
	return nil
}

func (b *Bots) handle(ctx context.Context, u messenger.Update) error {
	c, ok := b.clients[u.Platform]
	if !ok || u.ChatID == "" {
		return nil
	}
	chat, created, err := b.chat(ctx, c, u.ChatID, u.Lang)
	if err != nil {
		return err
	}
	lang := langOr(chat.Lang, i18n.RU)
	if created {
		// часовой пояс угадываем по языку мессенджера; мини-приложение потом поставит точный из браузера
		if tz := defaultTz(u.Lang); tz != chat.Settings.Tz {
			s := chat.Settings
			s.Tz = tz
			if err := b.repo.SetSettings(ctx, string(u.Platform), u.ChatID, s); err != nil {
				return err
			}
			chat.Settings = s
		}
	}
	if created || u.Start {
		b.menu(ctx, c, u.ChatID, lang)
	}
	if chat.UserID == nil && u.UserID != "" {
		// мессенджер могли привязать к аккаунту в мини-приложении: чат узнаёт аккаунт с первого сообщения
		if usr, err := b.accounts.ByLink(ctx, string(u.Platform), u.UserID); err == nil {
			if err := b.repo.SetUser(ctx, string(u.Platform), u.ChatID, &usr.ID); err != nil {
				return err
			}
			chat.UserID = &usr.ID
		}
	}
	if u.Callback != "" {
		// без ответа на нажатие у человека крутится индикатор на кнопке
		defer func() { _ = c.Answer(context.WithoutCancel(ctx), u.Callback, "") }()
	}
	switch {
	case strings.HasPrefix(u.Data, "n:"):
		return b.settingsTap(ctx, c, u, chat, lang)
	case strings.HasPrefix(u.Data, "f:"):
		return b.feedbackTap(ctx, c, u, chat, lang)
	case u.Callback != "":
		return b.callback(ctx, c, u, lang)
	case u.Start && strings.HasPrefix(u.Payload, "p"):
		return b.connect(ctx, c, u.ChatID, strings.TrimPrefix(u.Payload, "p"), lang)
	case u.Start && strings.HasPrefix(u.Payload, "u"):
		return b.link(ctx, c, u, strings.TrimPrefix(u.Payload, "u"), lang)
	case command(u.Text) == "/settings":
		return b.send(ctx, c, u.ChatID, b.settingsMessage(chat, lang))
	default:
		return b.latest(ctx, c, chat, lang)
	}
}

// command — «/settings@racionappbot что-то» → «/settings».
func command(text string) string {
	f := strings.Fields(text)
	if len(f) == 0 || !strings.HasPrefix(f[0], "/") {
		return ""
	}
	cmd, _, _ := strings.Cut(strings.ToLower(f[0]), "@")
	return cmd
}

// defaultTz — часовой пояс нового чата по языку мессенджера, в минутах к UTC.
func defaultTz(messengerLang string) int {
	switch messengerLang {
	case "uk":
		return 120
	case "kk":
		return 300
	case "de", "fr", "it", "es", "nl", "pl", "cs":
		return 60
	case "en", "pt":
		return 0
	case "zh":
		return 480
	case "ja":
		return 540
	}
	return 180 // русский, турецкий и всё неизвестное — Москва
}

// chat заводит чат или находит его. Язык мессенджера берём только для нового чата: нажатие кнопки
// язык обычно не сообщает, а дальше чат говорит на языке недели.
func (b *Bots) chat(ctx context.Context, c messenger.Client, chatID, messengerLang string) (domain.MessengerChat, bool, error) {
	first := ""
	if l, ok := i18n.Valid(messengerLang); ok {
		first = string(l)
	}
	created, err := b.repo.SaveChat(ctx, string(c.Platform()), chatID, first)
	if err != nil {
		return domain.MessengerChat{}, false, err
	}
	chat, err := b.repo.Chat(ctx, string(c.Platform()), chatID)
	return chat, created, err
}

// menu — кнопка слева от поля ввода открывает сайт на языке чата. Не вышло — не беда: остаются
// кнопки под списком.
func (b *Bots) menu(ctx context.Context, c messenger.Client, chatID string, lang i18n.Lang) {
	if err := c.SetMenu(ctx, chatID, i18n.T(lang, "page.brand"), b.appURL(lang, "/")); err != nil {
		b.log.Warn("bots: menu", zap.String("platform", string(c.Platform())), zap.Error(err))
	}
}

// appURL — адрес страницы сайта на языке чата: язык в префиксе, для русского — параметром,
// иначе телефон с английским интерфейсом открыл бы русскую неделю по-английски.
func (b *Bots) appURL(lang i18n.Lang, path string) string {
	if lang == i18n.RU {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		return b.baseURL + path + sep + "lang=ru"
	}
	return b.baseURL + "/" + string(lang) + path
}

// latest — ответ на /list, «Начать» и любые слова: список последней недели, чтобы в магазине не
// листать чат. Берём самую свежую из подключённых к чату и, если аккаунт привязан, из недель
// аккаунта: собрал неделю на сайте — она уже в боте. Недель нет — подсказка, где взять ссылку.
func (b *Bots) latest(ctx context.Context, c messenger.Client, chat domain.MessengerChat, lang i18n.Lang) error {
	connected, err := b.repo.Plans(ctx, chat.Platform, chat.ChatID)
	if err != nil {
		return err
	}
	if chat.UserID != nil {
		if mine, err := b.plans.Mine(ctx, *chat.UserID); err == nil && len(mine) > 0 {
			if at, err := time.Parse(time.RFC3339, mine[0].CreatedAt); err == nil {
				connected = append(connected, domain.ChatPlan{ID: mine[0].ID, At: at})
			}
		}
	}
	sort.SliceStable(connected, func(i, j int) bool { return connected[i].At.After(connected[j].At) })
	for _, p := range connected {
		plan, err := b.plans.Get(ctx, p.ID, lang)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		key, _ := planKey(p.ID)
		pages, checked := b.list(ctx, plan, lang)
		return b.send(ctx, c, chat.ChatID, b.listMessage(plan, pages, checked, 0, key, lang))
	}
	return b.send(ctx, c, chat.ChatID, b.intro(c, lang))
}

// link — человек пришёл по ссылке «Привязать Telegram» из кабинета. Мессенджер привязывается к
// аккаунту, даже если раньше был привязан к другому: человек подтвердил это, войдя на сайте.
func (b *Bots) link(ctx context.Context, c messenger.Client, u messenger.Update, token string, lang i18n.Lang) error {
	userID, err := b.repo.TakeLink(ctx, tokenHash(token))
	if errors.Is(err, domain.ErrNotFound) || (err == nil && u.UserID == "") {
		return b.send(ctx, c, u.ChatID, messenger.Message{Text: html.EscapeString(i18n.T(lang, "bot.link.expired"))})
	}
	if err != nil {
		return err
	}
	if err := b.accounts.Link(ctx, userID, string(c.Platform()), u.UserID, true); err != nil {
		return err
	}
	if err := b.repo.SetUser(ctx, string(c.Platform()), u.ChatID, &userID); err != nil {
		return err
	}
	return b.send(ctx, c, u.ChatID, messenger.Message{
		Text: html.EscapeString(i18n.T(lang, "bot.linked")),
		Rows: [][]messenger.Button{{{Text: i18n.T(lang, "bot.open"), URL: b.appURL(lang, "/me"), App: true}}},
	})
}

// LinkURL — одноразовая ссылка на бота для кабинета: по ней бот привяжет мессенджер к аккаунту.
func (b *Bots) LinkURL(ctx context.Context, platform, userID string) (string, error) {
	c, ok := b.Client(platform)
	if !ok {
		return "", domain.ErrNotFound
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := b.repo.CreateLink(ctx, tokenHash(token), userID, time.Now().Add(LinkTTL)); err != nil {
		return "", err
	}
	return c.StartLink("u" + token), nil
}

// Unlinked — аккаунт отвязал мессенджер: его чаты больше не знают аккаунт.
func (b *Bots) Unlinked(ctx context.Context, platform, userID string) error {
	if b == nil {
		return nil
	}
	return b.repo.ForgetUser(ctx, platform, userID)
}

// WebApp проверяет подпись данных мини-приложения. Чужая или старая подпись — ErrUnauthorized.
func (b *Bots) WebApp(platform, initData string) (messenger.WebAppUser, error) {
	c, ok := b.Client(platform)
	if !ok {
		return messenger.WebAppUser{}, domain.ErrNotFound
	}
	app, ok := c.(messenger.MiniApp)
	if !ok {
		return messenger.WebAppUser{}, domain.ErrNotFound
	}
	u, err := app.VerifyInitData(initData, time.Now())
	if err != nil {
		return u, domain.ErrUnauthorized
	}
	return u, nil
}

// Known — мини-приложение узнало аккаунт: личный чат с ботом тоже его узнаёт. В Telegram id личного
// чата совпадает с id человека; чата ещё нет — ничего не меняется.
func (b *Bots) Known(ctx context.Context, platform, messengerUserID, userID string) error {
	return b.repo.SetUser(ctx, platform, messengerUserID, &userID)
}

// StartPath — куда вести мини-приложение по параметру запуска: «p<неделя>» — на страницу недели.
func (b *Bots) StartPath(param string) string {
	if id, ok := planIDFromKey(strings.TrimPrefix(param, "p")); ok && strings.HasPrefix(param, "p") {
		return "/plan/" + id
	}
	return ""
}

// SendList — «Список в Telegram» внутри мини-приложения: бот сразу присылает список в чат, по ссылке
// уходить не нужно. Человек ещё не начинал чат с ботом — ErrBlocked, тогда сайт откроет ссылку.
func (b *Bots) SendList(ctx context.Context, platform string, wu messenger.WebAppUser, planID string) error {
	c, ok := b.Client(platform)
	if !ok {
		return domain.ErrNotFound
	}
	key, ok := planKey(planID)
	if !ok {
		return domain.ErrNotFound
	}
	chat, _, err := b.chat(ctx, c, wu.ID, wu.Lang)
	if err != nil {
		return err
	}
	return b.connect(ctx, c, wu.ID, key, langOr(chat.Lang, i18n.RU))
}

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// connect подключает неделю по ссылке со страницы плана: приветствие и сразу список покупок.
// Язык чата становится языком недели: собирали её по-русски — и список нужен по-русски, даже
// если сам мессенджер у человека на английском.
func (b *Bots) connect(ctx context.Context, c messenger.Client, chatID, key string, lang i18n.Lang) error {
	id, ok := planIDFromKey(key)
	if !ok {
		return b.send(ctx, c, chatID, messenger.Message{Text: html.EscapeString(i18n.T(lang, "bot.notfound"))})
	}
	plan, planLang, err := b.plans.Original(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return b.send(ctx, c, chatID, messenger.Message{Text: html.EscapeString(i18n.T(lang, "bot.notfound"))})
	}
	if err != nil {
		return err
	}
	if planLang != lang {
		lang = planLang
		if err := b.repo.SetLang(ctx, string(c.Platform()), chatID, string(lang)); err != nil {
			return err
		}
		b.menu(ctx, c, chatID, lang)
	}
	if err := b.repo.LinkPlan(ctx, string(c.Platform()), chatID, id); err != nil {
		return err
	}
	// неделю называет сам список ниже, здесь — что произошло и что будет дальше
	welcome := messenger.Message{
		Text: html.EscapeString(i18n.T(lang, "bot.connected")) + "\n\n" + html.EscapeString(i18n.T(lang, "bot.remind.hint")),
		Rows: [][]messenger.Button{{{Text: i18n.T(lang, "bot.open"), URL: b.appURL(lang, "/plan/"+id), App: true}}},
	}
	if err := b.send(ctx, c, chatID, welcome); err != nil {
		return err
	}
	pages, checked := b.list(ctx, plan, lang)
	return b.send(ctx, c, chatID, b.listMessage(plan, pages, checked, 0, key, lang))
}

// list — отделы списка и отметки «куплено» недели: те же, что видит сайт.
func (b *Bots) list(ctx context.Context, plan planner.Plan, lang i18n.Lang) ([]botPage, map[string]bool) {
	extras, _ := b.plans.Extras(ctx, plan.ID)
	checks, _ := b.plans.Checks(ctx, plan.ID)
	return shopPages(plan, extras, lang), checkSet(checks)
}

// callback — нажатие кнопки под списком: «l:<неделя>:<отдел>» — листать отделы,
// «t:<неделя>:<отдел>:<продукт>» — отметить купленным или вернуть в список.
func (b *Bots) callback(ctx context.Context, c messenger.Client, u messenger.Update, lang i18n.Lang) error {
	// в id своей покупки есть двоеточие: «extra:12»
	parts := strings.SplitN(u.Data, ":", 4)
	if len(parts) < 3 || (parts[0] != "l" && parts[0] != "t") {
		return nil
	}
	id, ok := planIDFromKey(parts[1])
	if !ok {
		return nil
	}
	page, _ := strconv.Atoi(parts[2])
	plan, err := b.plans.Get(ctx, id, lang)
	if err != nil {
		return b.edit(ctx, c, u.ChatID, u.MessageID, messenger.Message{Text: html.EscapeString(i18n.T(lang, "bot.notfound"))})
	}
	pages, checked := b.list(ctx, plan, lang)
	if parts[0] == "t" && len(parts) == 4 {
		if it, ok := findItem(pages, page, parts[3]); ok {
			was := checked[it.ID]
			in := domain.CheckInput{ItemID: it.ID, Checked: !was, Name: it.Name, Qty: it.Qty, Cost: it.Cost}
			if err := b.plans.SetCheck(ctx, id, in, nil); err != nil {
				return err
			}
			checked[it.ID] = !was
		}
	}
	return b.edit(ctx, c, u.ChatID, u.MessageID, b.listMessage(plan, pages, checked, page, parts[1], lang))
}

// listMessage — один отдел списка: что осталось купить, кнопка на каждый продукт, листание отделов.
func (b *Bots) listMessage(plan planner.Plan, pages []botPage, checked map[string]bool, page int, key string, lang i18n.Lang) messenger.Message {
	open := []messenger.Button{{Text: i18n.T(lang, "bot.open"), URL: b.appURL(lang, "/plan/"+plan.ID+"?mode=shop"), App: true}}
	// какая это неделя: к списку возвращаются через несколько дней, а недель в чате бывает несколько
	week := "<i>" + html.EscapeString(planHeading(plan, lang)) + "</i>\n"
	if len(pages) == 0 {
		return messenger.Message{Text: week + html.EscapeString(i18n.T(lang, "bot.empty")), Rows: [][]messenger.Button{open}}
	}
	page = max(0, min(page, len(pages)-1))
	total, bought := 0, 0
	var left float64
	for _, g := range pages {
		for _, it := range g.Items {
			total++
			if checked[it.ID] {
				bought++
			} else {
				left += it.Cost
			}
		}
	}
	g := pages[page]
	var head strings.Builder
	head.WriteString(week + "<b>" + html.EscapeString(g.Label) + "</b>")
	if len(pages) > 1 {
		head.WriteString(" · " + html.EscapeString(i18n.T(lang, "bot.dept", page+1, len(pages))))
	}
	head.WriteString("\n")
	if bought == total {
		head.WriteString(html.EscapeString(i18n.T(lang, "bot.done")))
	} else {
		head.WriteString(html.EscapeString(i18n.T(lang, "bot.left", total-bought, total)))
		if left > 0 { // у своих покупок цены нет: когда остались только они, «≈ 0 ₽» читалось бы как ошибка
			head.WriteString(" · ≈ " + html.EscapeString(plan.Country.Money(left)))
		}
	}
	rows := make([][]messenger.Button, 0, len(g.Items)+2)
	for i, it := range g.Items {
		text := it.Name
		if it.Qty != "" {
			text += " · " + it.Qty
		}
		if checked[it.ID] {
			text = "✓ " + text
		}
		rows = append(rows, []messenger.Button{{Text: text, Data: toggleData(key, page, it.ID, i)}})
	}
	var nav []messenger.Button
	if page > 0 {
		nav = append(nav, messenger.Button{Text: "‹ " + pages[page-1].Label, Data: fmt.Sprintf("l:%s:%d", key, page-1)})
	}
	if page < len(pages)-1 {
		nav = append(nav, messenger.Button{Text: pages[page+1].Label + " ›", Data: fmt.Sprintf("l:%s:%d", key, page+1)})
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, open)
	return messenger.Message{Text: head.String(), Rows: rows}
}

// intro — ответ на «Начать» без недели и на любые слова: где взять ссылку.
func (b *Bots) intro(c messenger.Client, lang i18n.Lang) messenger.Message {
	// надпись кнопки берём из того же ключа, что и сайт: так текст в боте и на кнопке не разойдутся
	button := i18n.T(lang, "plan.bot."+string(c.Platform()))
	return messenger.Message{
		Text: html.EscapeString(i18n.T(lang, "bot.intro", button)),
		Rows: [][]messenger.Button{{{Text: i18n.T(lang, "bot.build"), URL: b.appURL(lang, "/"), App: true}}},
	}
}

func (b *Bots) send(ctx context.Context, c messenger.Client, chatID string, m messenger.Message) error {
	_, err := c.Send(ctx, chatID, m)
	return b.blocked(ctx, c, chatID, err)
}

func (b *Bots) edit(ctx context.Context, c messenger.Client, chatID, messageID string, m messenger.Message) error {
	return b.blocked(ctx, c, chatID, c.Edit(ctx, chatID, messageID, m))
}

// blocked — человек заблокировал бота: отмечаем и больше не пишем, пока сам не вернётся.
// Ошибку отдаём дальше: мини-приложению по ней понятно, что надо открыть ссылку на бота.
func (b *Bots) blocked(ctx context.Context, c messenger.Client, chatID string, err error) error {
	if errors.Is(err, messenger.ErrBlocked) {
		_ = b.repo.SetBlocked(ctx, string(c.Platform()), chatID, true)
	}
	return err
}

// botItem — строка списка в боте: продукт недели или своя покупка вроде «молоко для кофе».
type botItem struct {
	ID   string // id продукта или «extra:<id>»: те же ключи, что у отметок на сайте
	Name string
	Qty  string
	Cost float64
}

type botPage struct {
	Label string
	Items []botItem
}

// shopPages — отделы списка с тем, что надо купить. Домашнее (соль, масло) и то, что уже есть дома,
// не показываем; свои покупки — последним отделом, как на сайте.
func shopPages(plan planner.Plan, extras []domain.Extra, lang i18n.Lang) []botPage {
	var out []botPage
	for _, g := range plan.Shopping {
		var items []botItem
		for _, it := range g.Items {
			if !it.Pantry && !it.AtHome {
				items = append(items, botItem{ID: it.IngredientID, Name: it.Name, Qty: itemQty(it, lang), Cost: it.Cost})
			}
		}
		if len(items) > 0 {
			out = append(out, botPage{Label: g.Label, Items: items})
		}
	}
	if len(extras) > 0 {
		own := botPage{Label: i18n.T(lang, "list.extras")}
		for _, e := range extras {
			own.Items = append(own.Items, botItem{ID: "extra:" + strconv.FormatInt(e.ID, 10), Name: e.Name, Qty: e.Qty})
		}
		out = append(out, own)
	}
	return out
}

// toggleData — данные кнопки продукта. У Telegram на них 64 байта: если id продукта не влезает,
// вместо него номер строки в отделе.
func toggleData(key string, page int, itemID string, idx int) string {
	if d := fmt.Sprintf("t:%s:%d:%s", key, page, itemID); len(d) <= 64 {
		return d
	}
	return fmt.Sprintf("t:%s:%d:#%d", key, page, idx)
}

// findItem ищет продукт по id во всём списке: пока список был открыт в чате, на сайте могли
// заменить блюдо, и отделы сдвинулись. Номер строки («#3») — только в своём отделе.
func findItem(pages []botPage, page int, ref string) (botItem, bool) {
	if strings.HasPrefix(ref, "#") {
		i, err := strconv.Atoi(ref[1:])
		if err == nil && page >= 0 && page < len(pages) && i >= 0 && i < len(pages[page].Items) {
			return pages[page].Items[i], true
		}
		return botItem{}, false
	}
	for _, g := range pages {
		for _, it := range g.Items {
			if it.ID == ref {
				return it, true
			}
		}
	}
	return botItem{}, false
}

// itemQty — сколько купить: «2 × 930 мл» для упаковок, «400 г» для весового и штучного.
func itemQty(it planner.ShopItem, lang i18n.Lang) string {
	if it.Loose || it.Unit == "pcs" || it.Packs <= 1 {
		return planner.FormatQty(lang, it.Buy, it.Unit)
	}
	return fmt.Sprintf("%d × %s", it.Packs, planner.FormatQty(lang, it.Pack, it.Unit))
}

// planHeading — «Неделя с 6 октября 2026» или название праздничного стола.
func planHeading(plan planner.Plan, lang i18n.Lang) string {
	if plan.Occasion != nil {
		return plan.Occasion.Title
	}
	if len(plan.Days) > 0 {
		if t, err := time.Parse("2006-01-02", plan.Days[0].Date); err == nil {
			return i18n.T(lang, "bot.week", i18n.DayMonthYear(lang, t.Day(), int(t.Month()), t.Year()))
		}
	}
	return i18n.T(lang, "page.brand")
}

func checkSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func langOr(s string, def i18n.Lang) i18n.Lang {
	if l, ok := i18n.Valid(s); ok {
		return l
	}
	return def
}

// planKey — id недели в 22 символа base64url: ссылка запуска Telegram вмещает 64 символа из
// A-Z, a-z, 0-9, «_» и «-», а uuid с дефисами вместе с данными кнопки туда не влезает.
func planKey(id string) (string, bool) {
	raw, err := hex.DecodeString(strings.ReplaceAll(id, "-", ""))
	if err != nil || len(raw) != 16 {
		return "", false
	}
	return base64.RawURLEncoding.EncodeToString(raw), true
}

func planIDFromKey(key string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil || len(raw) != 16 {
		return "", false
	}
	h := hex.EncodeToString(raw)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], true
}
