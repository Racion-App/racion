package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
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
// messenger.Client, логика одна на оба.
type Bots struct {
	clients map[messenger.Platform]messenger.Client
	repo    MessengerRepo
	plans   *Plans
	baseURL string
	log     *zap.Logger
}

type MessengerRepo interface {
	SaveChat(ctx context.Context, platform, chatID, lang string) error
	SetLang(ctx context.Context, platform, chatID, lang string) error
	Chat(ctx context.Context, platform, chatID string) (domain.MessengerChat, error)
	SetBlocked(ctx context.Context, platform, chatID string, blocked bool) error
	SetSettings(ctx context.Context, platform, chatID string, s domain.NotifySettings) error
	LinkPlan(ctx context.Context, platform, chatID, planID string) error
	Plans(ctx context.Context, platform, chatID string) ([]string, error)
	MarkSent(ctx context.Context, platform, chatID, key string) (bool, error)
	Active(ctx context.Context) ([]domain.MessengerChat, error)
}

func NewBots(repo MessengerRepo, plans *Plans, baseURL string, log *zap.Logger) *Bots {
	return &Bots{clients: map[messenger.Platform]messenger.Client{}, repo: repo, plans: plans, baseURL: strings.TrimRight(baseURL, "/"), log: log}
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
	}
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

// Handle — одно обновление из мессенджера.
func (b *Bots) Handle(ctx context.Context, u messenger.Update) error {
	c, ok := b.clients[u.Platform]
	if !ok || u.ChatID == "" {
		return nil
	}
	first := "" // язык мессенджера — только для нового чата; нажатие кнопки его обычно не сообщает
	if l, ok := i18n.Valid(u.Lang); ok {
		first = string(l)
	}
	if err := b.repo.SaveChat(ctx, string(u.Platform), u.ChatID, first); err != nil {
		return err
	}
	chat, err := b.repo.Chat(ctx, string(u.Platform), u.ChatID)
	if err != nil {
		return err
	}
	lang := langOr(chat.Lang, i18n.RU)
	switch {
	case u.Callback != "":
		return b.callback(ctx, c, u, lang)
	case u.Start && strings.HasPrefix(u.Payload, "p"):
		return b.connect(ctx, c, u.ChatID, strings.TrimPrefix(u.Payload, "p"), lang)
	default:
		return b.send(ctx, c, u.ChatID, b.intro(c, lang))
	}
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
	}
	if err := b.repo.LinkPlan(ctx, string(c.Platform()), chatID, id); err != nil {
		return err
	}
	welcome := messenger.Message{
		Text: "<b>" + html.EscapeString(planHeading(plan, lang)) + "</b>\n" + html.EscapeString(i18n.T(lang, "bot.connected")),
		Rows: [][]messenger.Button{{{Text: i18n.T(lang, "bot.open"), URL: b.baseURL + "/plan/" + id}}},
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
	// без ответа на нажатие у человека крутится индикатор на кнопке
	defer func() { _ = c.Answer(context.WithoutCancel(ctx), u.Callback, "") }()
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
	open := []messenger.Button{{Text: i18n.T(lang, "bot.open"), URL: b.baseURL + "/plan/" + plan.ID + "?mode=shop"}}
	if len(pages) == 0 {
		return messenger.Message{Text: html.EscapeString(i18n.T(lang, "bot.empty")), Rows: [][]messenger.Button{open}}
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
	head.WriteString("<b>" + html.EscapeString(g.Label) + "</b>")
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
		Rows: [][]messenger.Button{{{Text: i18n.T(lang, "bot.build"), URL: b.baseURL + "/"}}},
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
func (b *Bots) blocked(ctx context.Context, c messenger.Client, chatID string, err error) error {
	if errors.Is(err, messenger.ErrBlocked) {
		_ = b.repo.SetBlocked(ctx, string(c.Platform()), chatID, true)
		return nil
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
