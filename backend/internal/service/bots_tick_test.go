package service

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/messenger"
	"racion/internal/planner"
)

// memBotRepo — хранилище бота в памяти: чаты, журнал напоминаний, недели для напоминаний.
type memBotRepo struct {
	chats           map[string]domain.MessengerChat
	sent            map[string]bool
	plans           []domain.PlanReminderInfo
	blocked         map[string]bool
	busy            int // сколько первых попыток блокировка опроса занята
	leads, released int
	logins          map[string]*memLogin
}

func newMemBotRepo() *memBotRepo {
	return &memBotRepo{chats: map[string]domain.MessengerChat{}, sent: map[string]bool{}, blocked: map[string]bool{}}
}

func (m *memBotRepo) SaveChat(_ context.Context, p, id, lang string) (bool, error) {
	if _, ok := m.chats[p+id]; ok {
		return false, nil
	}
	m.chats[p+id] = domain.MessengerChat{Platform: p, ChatID: id, Lang: lang, Settings: domain.DefaultNotify()}
	return true, nil
}
func (m *memBotRepo) SetLang(_ context.Context, p, id, lang string) error {
	c := m.chats[p+id]
	c.Lang = lang
	m.chats[p+id] = c
	return nil
}
func (m *memBotRepo) SetUser(_ context.Context, p, id string, u *string) error {
	c := m.chats[p+id]
	c.UserID = u
	m.chats[p+id] = c
	return nil
}
func (m *memBotRepo) ForgetUser(context.Context, string, string) error { return nil }
func (m *memBotRepo) CreateLink(context.Context, string, string, time.Time) error {
	return nil
}
func (m *memBotRepo) TakeLink(context.Context, string) (string, error) { return "", domain.ErrNotFound }
func (m *memBotRepo) ReminderPlans(context.Context, string, string, *string) ([]domain.PlanReminderInfo, error) {
	return m.plans, nil
}
func (m *memBotRepo) Chat(_ context.Context, p, id string) (domain.MessengerChat, error) {
	c, ok := m.chats[p+id]
	if !ok {
		return c, domain.ErrNotFound
	}
	return c, nil
}
func (m *memBotRepo) SetBlocked(_ context.Context, p, id string, b bool) error {
	m.blocked[p+id] = b
	return nil
}
func (m *memBotRepo) SetSettings(_ context.Context, p, id string, s domain.NotifySettings) error {
	c := m.chats[p+id]
	c.Settings = s
	m.chats[p+id] = c
	return nil
}
func (m *memBotRepo) LinkPlan(context.Context, string, string, string) error { return nil }
func (m *memBotRepo) Plans(context.Context, string, string) ([]domain.ChatPlan, error) {
	return nil, nil
}
func (m *memBotRepo) MarkSent(_ context.Context, p, id, key string) (bool, error) {
	if m.sent[p+id+key] {
		return false, nil
	}
	m.sent[p+id+key] = true
	return true, nil
}

// Lead: первые busy попыток блокировка занята другим экземпляром сервера.
func (m *memBotRepo) Lead(context.Context, int64) (func(), bool, error) {
	m.leads++
	if m.leads <= m.busy {
		return nil, false, nil
	}
	return func() { m.released++ }, true, nil
}
func (m *memBotRepo) Active(context.Context) ([]domain.MessengerChat, error) {
	var out []domain.MessengerChat
	for _, c := range m.chats {
		out = append(out, c)
	}
	return out, nil
}

// fakeClient — мессенджер, который запоминает отправленное и правленое.
type fakeClient struct {
	sent, edited []messenger.Message
	fail         error
}

func (f *fakeClient) Platform() messenger.Platform { return messenger.Telegram }
func (f *fakeClient) Username() string             { return "racion_test_bot" }
func (f *fakeClient) StartLink(payload string) string {
	return "https://t.me/racion_test_bot?start=" + payload
}
func (f *fakeClient) Answer(context.Context, string, string) error { return nil }
func (f *fakeClient) SetWebhook(context.Context, string) error     { return nil }
func (f *fakeClient) SetProfile(context.Context, map[string]messenger.Profile) error {
	return nil
}
func (f *fakeClient) SetMenu(context.Context, string, string, string) error { return nil }
func (f *fakeClient) Parse(*http.Request) (messenger.Update, bool, error) {
	return messenger.Update{}, false, nil
}
func (f *fakeClient) Send(_ context.Context, _ string, m messenger.Message) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	f.sent = append(f.sent, m)
	return "1", nil
}
func (f *fakeClient) Edit(_ context.Context, _, _ string, m messenger.Message) error {
	f.edited = append(f.edited, m)
	return nil
}

// fakePlans — одна неделя из botPlan, без отметок и своих покупок.
type fakePlans struct{}

func (fakePlans) Get(context.Context, string, i18n.Lang) (planner.Plan, error) { return botPlan(), nil }
func (fakePlans) Original(context.Context, string) (planner.Plan, i18n.Lang, error) {
	return botPlan(), i18n.RU, nil
}
func (fakePlans) Extras(context.Context, string) ([]domain.Extra, error) { return nil, nil }
func (fakePlans) Checks(context.Context, string) ([]string, error)       { return nil, nil }
func (fakePlans) SetCheck(context.Context, string, domain.CheckInput, *domain.User) error {
	return nil
}
func (fakePlans) Mine(context.Context, string) ([]domain.PlanSummary, error) { return nil, nil }

type fakeTaste struct{ got []string }

func (f *fakeTaste) Feedback(_ context.Context, userID, recipeID string, liked bool) error {
	v := "meh"
	if liked {
		v = "like"
	}
	f.got = append(f.got, userID+":"+recipeID+":"+v)
	return nil
}

type fakeJournal struct{ keys map[string]bool }

func (f *fakeJournal) MarkSent(_ context.Context, userID, key string) (bool, error) {
	if f.keys[userID+"|"+key] {
		return false, nil
	}
	f.keys[userID+"|"+key] = true
	return true, nil
}

func testBots() (*Bots, *memBotRepo, *fakeClient, *fakeTaste, *fakeJournal) {
	repo, client, taste, journal := newMemBotRepo(), &fakeClient{}, &fakeTaste{}, &fakeJournal{keys: map[string]bool{}}
	b := NewBots(BotDeps{Repo: repo, Plans: fakePlans{}, Taste: taste, Journal: journal}, "https://racion.app", zap.NewNop())
	b.pause = 0
	b.Add(client, false)
	return b, repo, client, taste, journal
}

// Неделя с понедельника 21 сентября 2026: в субботу в 18:00 по Москве пора в магазин, в 20:00 —
// «как было?» про ужин, в воскресенье в полдень — собрать следующую неделю.
func reminderWeek() []domain.PlanReminderInfo {
	return []domain.PlanReminderInfo{{ID: botPlan().ID, StartDate: "2026-09-21", Items: 40, Checked: 5,
		Dishes: map[string][]string{"2026-09-19": {"Сырники", "Борщ"}, "2026-09-20": {"Омлет"}},
		Dinner: map[string]domain.DishRef{"2026-09-19": {RecipeID: "borsch_classic", Title: "борщ"}}}}
}

func TestDueReminders(t *testing.T) {
	s := domain.NotifySettings{ShopDay: 6, ShopHour: 18, Today: true, TodayHour: 8, Prep: true, PrepHour: 19, Week: true, Tz: 180}
	plans := reminderWeek()
	at := func(day, hour int) time.Time { return time.Date(2026, 9, day, hour, 5, 0, 0, time.UTC) } // уже местное время
	kinds := func(rs []Reminder) string {
		var k []string
		for _, r := range rs {
			k = append(k, r.Kind+"="+r.Key)
		}
		return strings.Join(k, ",")
	}
	id := botPlan().ID
	for _, c := range []struct {
		local time.Time
		want  string
	}{
		{at(19, 18), "shop=shop:" + id + ":2026-09-19"},
		{at(19, 8), "today=today:" + id + ":2026-09-19"},
		{at(19, 19), "prep=prep:" + id + ":2026-09-20"},
		{at(19, 20), "ask=ask:" + id + ":2026-09-19"},
		{at(27, 12), "week=week:2026-09-28"},
		{at(20, 12), ""}, // на понедельник 21-го неделя есть
		{at(19, 15), ""},
	} {
		if got := kinds(dueReminders(c.local, s, plans)); got != c.want {
			t.Errorf("%s: %q, ждали %q", c.local.Format("02 15:04"), got, c.want)
		}
	}
	s.NoAsk, s.ShopHour = true, -1
	if got := dueReminders(at(19, 20), s, plans); len(got) != 0 {
		t.Errorf("«как было?» выключен: %+v", got)
	}
	if got := dueReminders(at(19, 18), s, plans); len(got) != 0 {
		t.Errorf("магазин выключен: %+v", got)
	}
}

func TestBotTick(t *testing.T) {
	b, repo, client, _, journal := testBots()
	ctx := context.Background()
	user := "u1"
	repo.chats["telegram55"] = domain.MessengerChat{Platform: "telegram", ChatID: "55", Lang: "ru", UserID: &user,
		Settings: domain.NotifySettings{ShopDay: 6, ShopHour: 18, Prep: true, PrepHour: 19, Tz: 180}}
	repo.chats["telegram77"] = domain.MessengerChat{Platform: "telegram", ChatID: "77", Lang: "ru", // без аккаунта
		Settings: domain.NotifySettings{ShopDay: 6, ShopHour: -1, Tz: 180}}
	repo.plans = reminderWeek()

	// суббота 18:05 по Москве: в магазин — сразу списком
	n, err := b.Tick(ctx, time.Date(2026, 9, 19, 15, 5, 0, 0, time.UTC))
	if err != nil || n != 1 || len(client.sent) != 1 {
		t.Fatalf("магазин: n=%d err=%v sent=%d", n, err, len(client.sent))
	}
	if m := client.sent[0]; !strings.HasPrefix(m.Text, "<b>Пора в магазин</b>\n<i>") || !strings.HasPrefix(m.Rows[0][0].Data, "t:") {
		t.Errorf("напоминание о магазине — список: %q", m.Text)
	}
	if !journal.keys["u1|shop:"+botPlan().ID+":2026-09-19"] {
		t.Error("привязанному аккаунту напоминание отмечено и для веб-пуша")
	}
	if n, _ := b.Tick(ctx, time.Date(2026, 9, 19, 15, 15, 0, 0, time.UTC)); n != 0 {
		t.Error("в тот же час второй раз не шлём")
	}

	// 20:05: «как было?» — только привязанному чату, с кнопками ответа
	client.sent = nil
	if n, _ := b.Tick(ctx, time.Date(2026, 9, 19, 17, 5, 0, 0, time.UTC)); n != 1 {
		t.Fatalf("«как было?»: %d", n)
	}
	ask := client.sent[0]
	if !strings.Contains(ask.Text, "Как борщ?") || ask.Rows[0][0].Data != "f:borsch_classic:1" || ask.Rows[1][0].Data != "f:borsch_classic:x" {
		t.Errorf("«как было?»: %+v", ask)
	}

	// бот заблокирован: чат отмечается, остальным ошибка не мешает
	client.fail = messenger.ErrBlocked
	if n, err := b.Tick(ctx, time.Date(2026, 9, 19, 16, 5, 0, 0, time.UTC)); err != nil || n != 0 || !repo.blocked["telegram55"] {
		t.Errorf("блокировка: n=%d err=%v", n, err)
	}
}

// pollClient — мессенджер с опросом: отдаёт одно обновление и ждёт остановки.
type pollClient struct {
	fakeClient
	polls   int
	handled chan struct{}
}

func (p *pollClient) Poll(ctx context.Context, handle func(messenger.Update)) error {
	p.polls++
	handle(messenger.Update{Platform: messenger.Telegram, ChatID: "9", UserID: "9", Text: "/list"})
	p.handled <- struct{}{}
	<-ctx.Done()
	return ctx.Err()
}

// Опрос идёт только у того экземпляра, что держит блокировку: занято — ждём и пробуем снова;
// после остановки блокировка снимается.
func TestBotPollLeader(t *testing.T) {
	repo, client := newMemBotRepo(), &pollClient{handled: make(chan struct{}, 1)}
	repo.busy = 2
	b := NewBots(BotDeps{Repo: repo, Plans: fakePlans{}}, "https://racion.app", zap.NewNop())
	b.retry = 10 * time.Millisecond
	b.Add(client, true)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.poll(ctx, client); close(done) }()
	select {
	case <-client.handled:
	case <-time.After(2 * time.Second):
		t.Error("обновление не обработано")
	}
	cancel()
	<-done
	if repo.leads != 3 || client.polls != 1 || len(client.sent) != 1 || repo.released != 1 {
		t.Errorf("попыток %d, опросов %d, ответов %d, снято %d", repo.leads, client.polls, len(client.sent), repo.released)
	}
}

func TestBotSettingsAndFeedback(t *testing.T) {
	b, repo, client, taste, _ := testBots()
	ctx := context.Background()
	user := "u1"
	repo.chats["telegram55"] = domain.MessengerChat{Platform: "telegram", ChatID: "55", Lang: "ru", UserID: &user, Settings: domain.DefaultNotify()}
	tap := func(data string) {
		if err := b.Handle(ctx, messenger.Update{Platform: messenger.Telegram, ChatID: "55", UserID: "55", Callback: "cb", Data: data, MessageID: "9"}); err != nil {
			t.Fatalf("%s: %v", data, err)
		}
	}
	// /settings: строки включения, под магазином — день и час, внизу часовой пояс
	if err := b.Handle(ctx, messenger.Update{Platform: messenger.Telegram, ChatID: "55", Text: "/settings"}); err != nil {
		t.Fatal(err)
	}
	set := client.sent[0]
	if !strings.Contains(set.Text, "Напоминания") || set.Rows[0][0].Data != "n:t:shop" || set.Rows[1][0].Data != "n:d" || set.Rows[len(set.Rows)-1][0].Text != "Часовой пояс: UTC+3" {
		t.Fatalf("настройки: %+v", set)
	}
	tap("n:t:shop")
	if repo.chats["telegram55"].Settings.ShopHour != -1 {
		t.Error("магазин не выключился")
	}
	tap("n:h:today")
	if picker := client.edited[len(client.edited)-1]; len(picker.Rows) != 4 || picker.Rows[0][0].Data != "n:H:today:6" {
		t.Errorf("выбор часа: %+v", picker.Rows)
	}
	tap("n:H:today:9")
	tap("n:Z:300")
	tap("n:D:2")
	if s := repo.chats["telegram55"].Settings; s.TodayHour != 9 || s.Tz != 300 || s.ShopDay != 2 {
		t.Errorf("час, пояс, день: %+v", s)
	}
	tap("n:H:today:99") // мусор не сохраняем
	if repo.chats["telegram55"].Settings.TodayHour != 9 {
		t.Error("час 99 принят")
	}
	// «как было?»: ответ уходит в аккаунт, «не спрашивать» выключает вопрос
	tap("f:borsch_classic:0")
	tap("f:borsch_classic:x")
	if len(taste.got) != 1 || taste.got[0] != "u1:borsch_classic:meh" || !repo.chats["telegram55"].Settings.NoAsk {
		t.Errorf("ответ %v, noAsk %v", taste.got, repo.chats["telegram55"].Settings.NoAsk)
	}
}

func (m *memBotRepo) CreateLogin(_ context.Context, l domain.MessengerLogin, _ time.Time) error {
	if m.logins == nil {
		m.logins = map[string]*memLogin{}
	}
	m.logins[l.TokenHash] = &memLogin{l: l}
	return nil
}
func (m *memBotRepo) PendingLogin(_ context.Context, h string) (domain.MessengerLogin, error) {
	if x, ok := m.logins[h]; ok && x.user == "" {
		return x.l, nil
	}
	return domain.MessengerLogin{}, domain.ErrNotFound
}
func (m *memBotRepo) ConfirmLogin(_ context.Context, h, userID string) error {
	x, ok := m.logins[h]
	if !ok || x.user != "" {
		return domain.ErrNotFound
	}
	x.user = userID
	return nil
}
func (m *memBotRepo) DropLogin(_ context.Context, h string) error {
	delete(m.logins, h)
	return nil
}
func (m *memBotRepo) TakeLogin(_ context.Context, poll string) (string, string, error) {
	for h, x := range m.logins {
		if x.l.PollHash != poll {
			continue
		}
		if x.user == "" {
			return "", "", nil
		}
		delete(m.logins, h)
		return x.user, x.l.PlanID, nil
	}
	return "", "", domain.ErrNotFound
}

type memLogin struct {
	l    domain.MessengerLogin
	user string
}
