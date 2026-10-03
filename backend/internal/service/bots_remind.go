package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/messenger"
)

// Напоминания и их настройки в боте. Что и когда напоминать, считает dueReminders — та же функция, что
// у веб-пуша; здесь — как это выглядит в чате: «пора в магазин» приходит сразу списком, «что готовим» —
// блюдами дня, «как было?» — кнопками ответа. Настройки — сообщение с кнопками, которое правится на месте.

// Tick — напоминания во все чаты; main зовёт его раз в 10 минут, перед веб-пушем. Привязанному аккаунту
// отправленное отмечается и в журнале веб-пуша: одно напоминание приходит в одно место, в Telegram.
func (b *Bots) Tick(ctx context.Context, now time.Time) (int, error) {
	if b == nil || len(b.clients) == 0 {
		return 0, nil
	}
	chats, err := b.repo.Active(ctx)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, chat := range chats {
		c, ok := b.clients[messenger.Platform(chat.Platform)]
		if !ok {
			continue
		}
		plans, err := b.repo.ReminderPlans(ctx, chat.Platform, chat.ChatID, chat.UserID)
		if err != nil {
			b.log.Warn("bots: reminder plans", zap.String("platform", chat.Platform), zap.Error(err))
			continue
		}
		lang := langOr(chat.Lang, i18n.RU)
		local := now.UTC().Add(time.Duration(chat.Settings.Tz) * time.Minute)
		for _, r := range dueReminders(local, chat.Settings, plans) {
			// дайджест новинок — про сайт, ему место в веб-пуше; «как было?» учит планировщик аккаунта,
			// без привязки ответ записать некуда
			if r.Kind == "digest" || (r.Kind == "ask" && chat.UserID == nil) {
				continue
			}
			if ok, err := b.repo.MarkSent(ctx, chat.Platform, chat.ChatID, r.Key); err != nil || !ok {
				continue
			}
			m, ok := b.reminder(ctx, r, lang)
			if !ok {
				continue
			}
			if err := b.send(ctx, c, chat.ChatID, m); err != nil {
				if !errors.Is(err, messenger.ErrBlocked) {
					b.log.Warn("bots: reminder", zap.String("platform", chat.Platform), zap.String("kind", r.Kind), zap.Error(err))
				}
				break // бот заблокирован или мессенджер не принял: этому чату сейчас больше не пишем
			}
			sent++
			if chat.UserID != nil && b.journal != nil {
				_, _ = b.journal.MarkSent(ctx, *chat.UserID, r.Key)
			}
			if b.pause > 0 {
				select {
				case <-ctx.Done():
					return sent, ctx.Err()
				case <-time.After(b.pause):
				}
			}
		}
	}
	return sent, nil
}

// reminder — напоминание сообщением в чат; false — сказать нечего (неделю успели удалить).
func (b *Bots) reminder(ctx context.Context, r Reminder, lang i18n.Lang) (messenger.Message, bool) {
	key, _ := planKey(r.PlanID)
	title := func(k string, args ...any) string {
		return "<b>" + html.EscapeString(i18n.T(lang, k, args...)) + "</b>\n"
	}
	open := func(path string) []messenger.Button {
		return []messenger.Button{{Text: i18n.T(lang, "bot.open"), URL: b.appURL(lang, path), App: true}}
	}
	plan := "/plan/" + r.PlanID
	switch r.Kind {
	case "shop":
		// напоминание сразу и есть список: с ним и идут в магазин
		p, err := b.plans.Get(ctx, r.PlanID, lang)
		if err != nil {
			return messenger.Message{}, false
		}
		pages, checked := b.list(ctx, p, lang)
		m := b.listMessage(p, pages, checked, 0, key, lang)
		m.Text = title("push.shop.title") + m.Text
		return m, true
	case "today":
		return messenger.Message{Text: title("push.today.title") + bullets(r.Dishes), Rows: [][]messenger.Button{open(plan + "?day=" + r.Date)}}, true
	case "prep":
		return messenger.Message{Text: title("push.prep.title") + bullets(r.Dishes), Rows: [][]messenger.Button{open(plan + "?day=" + r.Date)}}, true
	case "prepday-eve", "prepday":
		k := "push.prepday.title"
		if r.Kind == "prepday-eve" {
			k = "push.prepday.eve.title"
		}
		return messenger.Message{
			Text: title(k) + html.EscapeString(i18n.T(lang, "push.prepday.body", r.Prep.Items, minutesLabel(lang, r.Prep.TotalMin))),
			Rows: [][]messenger.Button{{{Text: i18n.T(lang, "bot.btn.list"), Data: fmt.Sprintf("l:%s:0", key)}}, open(plan + "?prep=1")},
		}, true
	case "ask":
		data := "f:" + r.Dinner.RecipeID + ":"
		if len(data)+1 > 64 { // у Telegram на данные кнопки 64 байта
			return messenger.Message{}, false
		}
		return messenger.Message{
			Text: title("push.ask.title", r.Dinner.Title) + html.EscapeString(i18n.T(lang, "push.ask.body")),
			Rows: [][]messenger.Button{
				{{Text: i18n.T(lang, "feedback.like"), Data: data + "1"}, {Text: i18n.T(lang, "feedback.meh"), Data: data + "0"}},
				{{Text: i18n.T(lang, "bot.ask.never"), Data: data + "x"}},
			},
		}, true
	case "week":
		return messenger.Message{
			Text: title("push.week.title") + html.EscapeString(i18n.T(lang, "bot.week.body")),
			Rows: [][]messenger.Button{{{Text: i18n.T(lang, "bot.build"), URL: b.appURL(lang, "/"), App: true}}},
		}, true
	}
	return messenger.Message{}, false
}

// feedbackTap — ответ на «как было?»: «f:<рецепт>:1» понравилось, «0» не зашло, «x» больше не спрашивать.
func (b *Bots) feedbackTap(ctx context.Context, c messenger.Client, u messenger.Update, chat domain.MessengerChat, lang i18n.Lang) error {
	if len(u.Data) < 5 || u.Data[len(u.Data)-2] != ':' {
		return nil
	}
	recipe, answer := u.Data[2:len(u.Data)-2], u.Data[len(u.Data)-1:]
	var text string
	switch answer {
	case "x":
		s := chat.Settings
		s.NoAsk = true
		if err := b.repo.SetSettings(ctx, chat.Platform, chat.ChatID, s); err != nil {
			return err
		}
		text = i18n.T(lang, "bot.ask.off")
	case "1", "0":
		if chat.UserID != nil && b.taste != nil {
			if err := b.taste.Feedback(ctx, *chat.UserID, recipe, answer == "1"); err != nil && !errors.Is(err, domain.ErrNotFound) {
				return err
			}
		}
		text = i18n.T(lang, "bot.ask.thanks")
	default:
		return nil
	}
	return b.edit(ctx, c, u.ChatID, u.MessageID, messenger.Message{Text: html.EscapeString(text)})
}

// settingsMessage — напоминания кнопками: строка включает и выключает, под ней — день и час.
func (b *Bots) settingsMessage(chat domain.MessengerChat, lang i18n.Lang) messenger.Message {
	s := chat.Settings
	toggle := func(key string, on bool, data string) []messenger.Button {
		state := "bot.set.off"
		if on {
			state = "bot.set.on"
		}
		return []messenger.Button{{Text: i18n.T(lang, key) + " · " + i18n.T(lang, state), Data: data}}
	}
	at := func(h int, data string) messenger.Button {
		return messenger.Button{Text: i18n.T(lang, "bot.set.at", clock(h)), Data: data}
	}
	rows := [][]messenger.Button{toggle("bot.set.shop", s.ShopHour >= 0, "n:t:shop")}
	if s.ShopHour >= 0 {
		rows = append(rows, []messenger.Button{{Text: i18n.T(lang, "bot.set.day", weekday(lang, s.ShopDay)), Data: "n:d"}, at(s.ShopHour, "n:h:shop")})
	}
	rows = append(rows, toggle("bot.set.today", s.Today, "n:t:today"))
	if s.Today {
		rows = append(rows, []messenger.Button{at(s.TodayHour, "n:h:today")})
	}
	rows = append(rows, toggle("bot.set.prep", s.Prep, "n:t:prep"))
	if s.Prep {
		rows = append(rows, []messenger.Button{at(s.PrepHour, "n:h:prep")})
	}
	rows = append(rows, toggle("bot.set.prepday", s.PrepDay, "n:t:prepday"), toggle("bot.set.week", s.Week, "n:t:week"))
	if chat.UserID != nil { // «как было?» записывается в аккаунт: без привязки спрашивать незачем
		rows = append(rows, toggle("bot.set.ask", !s.NoAsk, "n:t:ask"))
	}
	rows = append(rows, []messenger.Button{{Text: i18n.T(lang, "bot.set.tz", tzLabel(s.Tz)), Data: "n:z"}})
	return messenger.Message{Text: "<b>" + html.EscapeString(i18n.T(lang, "bot.set.title")) + "</b>\n" + html.EscapeString(i18n.T(lang, "bot.set.lead")), Rows: rows}
}

// settingsTap — нажатие в настройках: «n:t:<что>» включить или выключить, «n:h:<что>» и «n:H:<что>:<час>» —
// час, «n:d» и «n:D:<день>» — день магазина, «n:z» и «n:Z:<минуты>» — часовой пояс, «n:b» — назад.
func (b *Bots) settingsTap(ctx context.Context, c messenger.Client, u messenger.Update, chat domain.MessengerChat, lang i18n.Lang) error {
	parts := strings.Split(u.Data, ":")
	s := chat.Settings
	save := true
	switch {
	case len(parts) == 3 && parts[1] == "t":
		switch parts[2] {
		case "shop":
			if s.ShopHour >= 0 {
				s.ShopHour = -1
			} else {
				s.ShopHour = domain.DefaultNotify().ShopHour
			}
		case "today":
			s.Today = !s.Today
		case "prep":
			s.Prep = !s.Prep
		case "prepday":
			s.PrepDay = !s.PrepDay
		case "week":
			s.Week = !s.Week
		case "ask":
			s.NoAsk = !s.NoAsk
		default:
			return nil
		}
	case len(parts) == 3 && parts[1] == "h":
		return b.edit(ctx, c, u.ChatID, u.MessageID, hourPicker(parts[2], lang))
	case len(parts) == 4 && parts[1] == "H":
		h, err := strconv.Atoi(parts[3])
		if err != nil || h < 0 || h > 23 {
			return nil
		}
		switch parts[2] {
		case "shop":
			s.ShopHour = h
		case "today":
			s.TodayHour = h
		case "prep":
			s.PrepHour = h
		default:
			return nil
		}
	case len(parts) == 2 && parts[1] == "d":
		return b.edit(ctx, c, u.ChatID, u.MessageID, dayPicker(lang))
	case len(parts) == 3 && parts[1] == "D":
		d, err := strconv.Atoi(parts[2])
		if err != nil || d < 0 || d > 6 {
			return nil
		}
		s.ShopDay = d
	case len(parts) == 2 && parts[1] == "z":
		return b.edit(ctx, c, u.ChatID, u.MessageID, tzPicker(lang))
	case len(parts) == 3 && parts[1] == "Z":
		m, err := strconv.Atoi(parts[2])
		if err != nil || m < -14*60 || m > 14*60 {
			return nil
		}
		s.Tz = m
	case len(parts) == 2 && parts[1] == "b":
		save = false
	default:
		return nil
	}
	if save {
		if err := b.repo.SetSettings(ctx, chat.Platform, chat.ChatID, s); err != nil {
			return err
		}
		chat.Settings = s
	}
	return b.edit(ctx, c, u.ChatID, u.MessageID, b.settingsMessage(chat, lang))
}

// SetTz — часовой пояс чата из мини-приложения: браузер знает его точно, бот — только угадывает.
func (b *Bots) SetTz(ctx context.Context, platform, chatID string, tz int) error {
	if b == nil || tz < -14*60 || tz > 14*60 {
		return nil
	}
	chat, err := b.repo.Chat(ctx, platform, chatID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && chat.Settings.Tz == tz) {
		return nil
	}
	if err != nil {
		return err
	}
	s := chat.Settings
	s.Tz = tz
	return b.repo.SetSettings(ctx, platform, chatID, s)
}

// hourPicker — выбор часа: с 6 утра до 23, по шесть в ряд.
func hourPicker(field string, lang i18n.Lang) messenger.Message {
	var rows [][]messenger.Button
	for h := 6; h <= 23; h += 6 {
		row := make([]messenger.Button, 0, 6)
		for x := h; x < h+6; x++ {
			row = append(row, messenger.Button{Text: clock(x), Data: fmt.Sprintf("n:H:%s:%d", field, x)})
		}
		rows = append(rows, row)
	}
	rows = append(rows, []messenger.Button{{Text: i18n.T(lang, "bot.set.back"), Data: "n:b"}})
	return messenger.Message{Text: html.EscapeString(i18n.T(lang, "bot.set.pick.hour")), Rows: rows}
}

// dayPicker — день магазина с понедельника по воскресенье: короткие названия одним рядом,
// длинные (казахские пишутся полностью) — в два ряда, иначе мессенджер их обрежет.
func dayPicker(lang i18n.Lang) messenger.Message {
	days := make([]messenger.Button, 0, 7)
	long := false
	for i := 0; i < 7; i++ {
		w := (i + 1) % 7 // в настройках 0 — воскресенье, а неделя в ряду с понедельника
		name := weekday(lang, w)
		long = long || utf8.RuneCountInString(name) > 4
		days = append(days, messenger.Button{Text: name, Data: fmt.Sprintf("n:D:%d", w)})
	}
	rows := [][]messenger.Button{days}
	if long {
		rows = [][]messenger.Button{days[:4], days[4:]}
	}
	rows = append(rows, []messenger.Button{{Text: i18n.T(lang, "bot.set.back"), Data: "n:b"}})
	return messenger.Message{Text: html.EscapeString(i18n.T(lang, "bot.set.pick.day")), Rows: rows}
}

// tzPicker — разница с UTC: от −8 до +12 часов, по пять в ряд.
func tzPicker(lang i18n.Lang) messenger.Message {
	offsets := []int{-8, -7, -6, -5, -4, -3, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	var rows [][]messenger.Button
	for i := 0; i < len(offsets); i += 5 {
		row := []messenger.Button{}
		for _, h := range offsets[i:min(i+5, len(offsets))] {
			row = append(row, messenger.Button{Text: tzLabel(h * 60), Data: fmt.Sprintf("n:Z:%d", h*60)})
		}
		rows = append(rows, row)
	}
	rows = append(rows, []messenger.Button{{Text: i18n.T(lang, "bot.set.back"), Data: "n:b"}})
	return messenger.Message{Text: html.EscapeString(i18n.T(lang, "bot.set.pick.tz")), Rows: rows}
}

func clock(h int) string { return fmt.Sprintf("%02d:00", h) }

// weekday — короткое имя дня: в настройках 0 — воскресенье, в словаре day.0 — понедельник.
func weekday(lang i18n.Lang, w int) string { return i18n.T(lang, "day."+strconv.Itoa((w+6)%7)) }

// tzLabel — «UTC+3», «UTC−5», «UTC+5:30».
func tzLabel(minutes int) string {
	sign := "+"
	if minutes < 0 {
		sign, minutes = "−", -minutes
	}
	if minutes%60 != 0 {
		return fmt.Sprintf("UTC%s%d:%02d", sign, minutes/60, minutes%60)
	}
	return fmt.Sprintf("UTC%s%d", sign, minutes/60)
}

func bullets(items []string) string {
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("• " + html.EscapeString(it))
	}
	return b.String()
}
