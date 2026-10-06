package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"racion/internal/domain"
	"racion/internal/i18n"
)

type memPush struct {
	subs  map[string][]domain.PushSubscription
	sent  map[string]bool
	plans map[string][]domain.PlanReminderInfo
	set   map[string]domain.NotifySettings
	// устройства без аккаунта: подписка, настройки, недели по endpoint
	devices     map[string]domain.PushDevice
	devicePlans map[string][]string
	planInfo    map[string]domain.PlanReminderInfo
}

func (m *memPush) SaveDevice(_ context.Context, s domain.PushSubscription, set domain.NotifySettings, planID string) error {
	if _, ok := m.planInfo[planID]; !ok {
		return domain.ErrNotFound // как внешний ключ в базе: недели нет — подписка не сохраняется
	}
	m.devices[s.Endpoint] = domain.PushDevice{Sub: s, Settings: set}
	m.devicePlans[s.Endpoint] = append([]string{planID}, slices.DeleteFunc(m.devicePlans[s.Endpoint], func(x string) bool { return x == planID })...)
	return nil
}
func (m *memPush) DeleteDevice(_ context.Context, endpoint string) error {
	delete(m.devices, endpoint)
	delete(m.devicePlans, endpoint)
	return nil
}
func (m *memPush) Devices(_ context.Context) ([]domain.PushDevice, error) {
	var out []domain.PushDevice
	for e, d := range m.devices {
		if len(m.devicePlans[e]) > 0 {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *memPush) DevicePlans(_ context.Context, endpoint string) ([]domain.PlanReminderInfo, error) {
	var out []domain.PlanReminderInfo
	for _, id := range m.devicePlans[endpoint] {
		out = append(out, m.planInfo[id])
	}
	return out, nil
}
func (m *memPush) MarkSentDevice(_ context.Context, endpoint, key string) (bool, error) {
	return m.MarkSent(context.Background(), "device:"+endpoint, key)
}

func (m *memPush) Save(_ context.Context, u string, s domain.PushSubscription) error {
	m.subs[u] = append(m.subs[u], s)
	return nil
}
func (m *memPush) Delete(_ context.Context, endpoint string) error {
	delete(m.devices, endpoint)
	delete(m.devicePlans, endpoint)
	for u, list := range m.subs {
		var keep []domain.PushSubscription
		for _, s := range list {
			if s.Endpoint != endpoint {
				keep = append(keep, s)
			}
		}
		m.subs[u] = keep
	}
	return nil
}
func (m *memPush) ByUser(_ context.Context, u string) ([]domain.PushSubscription, error) {
	return m.subs[u], nil
}
func (m *memPush) Users(_ context.Context) ([]domain.NotifyUser, error) {
	var out []domain.NotifyUser
	for u := range m.subs {
		s, ok := m.set[u]
		if !ok {
			s = domain.DefaultNotify()
		}
		out = append(out, domain.NotifyUser{UserID: u, Settings: s})
	}
	return out, nil
}
func (m *memPush) Settings(_ context.Context, u string) (domain.NotifySettings, error) {
	return m.set[u], nil
}
func (m *memPush) SetSettings(_ context.Context, u string, s domain.NotifySettings) error {
	m.set[u] = s
	return nil
}
func (m *memPush) MarkSent(_ context.Context, u, key string) (bool, error) {
	if m.sent[u+"|"+key] {
		return false, nil
	}
	m.sent[u+"|"+key] = true
	return true, nil
}
func (m *memPush) PlansForReminders(_ context.Context, u string) ([]domain.PlanReminderInfo, error) {
	return m.plans[u], nil
}

type memSettings struct{ kv map[string]string }

func (m *memSettings) Get(_ context.Context, k string) (string, error) {
	v, ok := m.kv[k]
	if !ok {
		return "", domain.ErrNotFound
	}
	return v, nil
}
func (m *memSettings) Set(_ context.Context, k, v string) error { m.kv[k] = v; return nil }

func newMemPush() *memPush {
	return &memPush{subs: map[string][]domain.PushSubscription{}, sent: map[string]bool{}, plans: map[string][]domain.PlanReminderInfo{}, set: map[string]domain.NotifySettings{},
		devices: map[string]domain.PushDevice{}, devicePlans: map[string][]string{}, planInfo: map[string]domain.PlanReminderInfo{}}
}

// Напоминания: магазин в выбранный день и час, «завтра готовим» в 19:00, дубликаты не уходят,
// мёртвая подписка удаляется.
func TestReminders(t *testing.T) {
	repo := newMemPush()
	n := NewNotifications(repo, &memSettings{kv: map[string]string{}}, "mailto:test@example.com", "https://racion.test")
	if err := n.Init(context.Background()); err != nil || n.PublicKey() == "" {
		t.Fatalf("vapid keys: %v", err)
	}
	var got []domain.Notification
	n.send = func(_ context.Context, sub domain.PushSubscription, msg domain.Notification) (int, error) {
		if sub.Endpoint == "https://dead" {
			return 410, nil
		}
		got = append(got, msg)
		return 201, nil
	}
	ctx := context.Background()
	_ = n.Subscribe(ctx, "u1", domain.PushSubscription{Endpoint: "https://ok", P256dh: "k", Auth: "a", Lang: "ru"})
	_ = n.Subscribe(ctx, "u1", domain.PushSubscription{Endpoint: "https://dead", P256dh: "k", Auth: "a", Lang: "ru"})
	_ = n.SetSettings(ctx, "u1", domain.NotifySettings{ShopDay: 6, ShopHour: 18, Prep: true, PrepHour: 19, Week: true, Tz: 180}) // суббота 18:00 по Москве
	// неделя с понедельника 21 сентября 2026; сейчас суббота 19 сентября 18:10 МСК = 15:10 UTC
	repo.plans["u1"] = []domain.PlanReminderInfo{{ID: "p1", StartDate: "2026-09-21", Items: 40, Checked: 5, Dishes: map[string][]string{"2026-09-20": {"Сырники"}, "2026-09-21": {"Овсянка", "Борщ"}}}}
	now := time.Date(2026, 9, 19, 15, 10, 0, 0, time.UTC)
	sent, err := n.Tick(ctx, now)
	if err != nil || sent != 1 || len(got) != 1 || got[0].Tag != "shop" || got[0].URL != "https://racion.test/plan/p1?mode=shop" {
		t.Fatalf("shop reminder expected once: sent=%d err=%v got=%+v", sent, err, got)
	}
	if len(repo.subs["u1"]) != 1 {
		t.Fatalf("dead subscription must be removed, have %d", len(repo.subs["u1"]))
	}
	if sent, _ := n.Tick(ctx, now.Add(5*time.Minute)); sent != 0 {
		t.Fatalf("same hour must not resend")
	}
	// 19:05 МСК того же дня: «завтра готовим» с блюдами воскресенья
	got = nil
	if sent, _ := n.Tick(ctx, time.Date(2026, 9, 19, 16, 5, 0, 0, time.UTC)); sent != 1 || got[0].Tag != "prep" || got[0].Body != "Сырники" {
		t.Fatalf("prep reminder: sent=%d got=%+v", sent, got)
	}
	// воскресенье 12:00: план на понедельник есть — тишина
	got = nil
	if sent, _ := n.Tick(ctx, time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)); sent != 0 {
		t.Fatalf("week reminder must not fire when next week is planned: %+v", got)
	}
	// через неделю плана нет — напоминание собрать
	if sent, _ := n.Tick(ctx, time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)); sent != 1 || got[0].Tag != "week" {
		t.Fatalf("week reminder expected: sent=%d got=%+v", sent, got)
	}
}

// Прикорм: в понедельник утром — продукт недели, в воскресенье в 18 — «как прошла неделя», в воскресенье
// в полдень без плана на следующую неделю — какой продукт следующий. NoWean выключает оба первых.
func TestWeaningReminders(t *testing.T) {
	s := domain.DefaultNotify()
	plan := domain.PlanReminderInfo{ID: "p1", StartDate: "2026-10-12", Dishes: map[string][]string{}, Dinner: map[string]domain.DishRef{},
		Weaning: []domain.WeaningReminder{{Age: "8 мес", Food: "Брокколи", First: 5, Last: 150, Unit: "g", Next: "Тыква"}}}
	kinds := func(local time.Time, s domain.NotifySettings) []string {
		var out []string
		for _, r := range dueReminders(local, s, []domain.PlanReminderInfo{plan}) {
			out = append(out, r.Kind)
		}
		return out
	}
	monday := time.Date(2026, 10, 12, s.TodayHour, 0, 0, 0, time.UTC)
	if k := kinds(monday, s); !slices.Contains(k, "wean") {
		t.Fatalf("понедельник утром: %v", k)
	}
	sunday := time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC)
	if k := kinds(sunday, s); !slices.Contains(k, "wean-diary") {
		t.Fatalf("воскресенье в 18: %v", k)
	}
	noon := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	rs := dueReminders(noon, s, []domain.PlanReminderInfo{plan})
	if len(rs) == 0 || rs[0].Kind != "week" || len(rs[0].Weaning) == 0 || rs[0].Weaning[0].Next != "Тыква" {
		t.Fatalf("воскресенье в полдень: %+v", rs)
	}
	s.NoWean = true
	if k := kinds(monday, s); slices.Contains(k, "wean") {
		t.Fatalf("напоминания про прикорм выключены, а пришло: %v", k)
	}
	if got := weanLines(i18n.RU, plan.Weaning); len(got) != 1 || !strings.Contains(got[0], "5 г") || !strings.Contains(got[0], "150 г") {
		t.Fatalf("строка напоминания: %v", got)
	}
}

// Магазин в воскресенье: неделя, которая сегодня кончается, в магазин не зовёт, а та, что начинается завтра, зовёт.
func TestShopSkipsEndingWeek(t *testing.T) {
	s := domain.DefaultNotify() // магазин в воскресенье в 12:00
	sunday := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ending := domain.PlanReminderInfo{ID: "old", StartDate: "2026-10-12", Items: 30, Checked: 2, Dishes: map[string][]string{}, Dinner: map[string]domain.DishRef{}}
	if rs := dueReminders(sunday, s, []domain.PlanReminderInfo{ending}); slices.ContainsFunc(rs, func(r Reminder) bool { return r.Kind == "shop" }) {
		t.Fatalf("неделя кончается сегодня, а зовём в магазин: %+v", rs)
	}
	next := domain.PlanReminderInfo{ID: "new", StartDate: "2026-10-19", Items: 30, Dishes: map[string][]string{}, Dinner: map[string]domain.DishRef{}}
	if rs := dueReminders(sunday, s, []domain.PlanReminderInfo{next, ending}); len(rs) != 1 || rs[0].Kind != "shop" || rs[0].PlanID != "new" {
		t.Fatalf("неделя начинается завтра: %+v", rs)
	}
}

// Без аккаунта: устройство подписывается со страницы недели и получает напоминания по ней — утром что
// готовим, в воскресенье собрать следующую. «Как было?» не спрашиваем, мёртвое устройство удаляется.
func TestDeviceReminders(t *testing.T) {
	repo := newMemPush()
	n := NewNotifications(repo, &memSettings{kv: map[string]string{}}, "mailto:test@example.com", "https://racion.test")
	if err := n.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	var got []domain.Notification
	n.send = func(_ context.Context, sub domain.PushSubscription, msg domain.Notification) (int, error) {
		if sub.Endpoint == "https://dead" {
			return 410, nil
		}
		got = append(got, msg)
		return 201, nil
	}
	ctx := context.Background()
	repo.planInfo["p1"] = domain.PlanReminderInfo{ID: "p1", StartDate: "2026-10-12", Items: 30,
		Dishes: map[string][]string{"2026-10-12": {"Сырники", "Борщ"}, "2026-10-13": {"Омлет"}},
		Dinner: map[string]domain.DishRef{"2026-10-12": {RecipeID: "borsch", Title: "Борщ"}}}
	sub := domain.PushSubscription{Endpoint: "https://ok", P256dh: "k", Auth: "a", Lang: "ru"}
	if err := n.SubscribeDevice(ctx, sub, "nope", 180, true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("неделя, которой нет: %v", err)
	}
	if err := n.SubscribeDevice(ctx, domain.PushSubscription{Endpoint: "http://x", P256dh: "k", Auth: "a"}, "p1", 180, true); !errors.Is(err, domain.ErrBadInput) {
		t.Fatalf("подписка не по https: %v", err)
	}
	if err := n.SubscribeDevice(ctx, sub, "p1", 180, true); err != nil {
		t.Fatal(err)
	}
	// понедельник 08:05 по Москве: что готовим сегодня
	if sent, _ := n.Tick(ctx, time.Date(2026, 10, 12, 5, 5, 0, 0, time.UTC)); sent != 1 || got[0].Tag != "today" || got[0].URL != "https://racion.test/plan/p1?day=2026-10-12" {
		t.Fatalf("утро: sent=%d got=%+v", sent, got)
	}
	// 20:00: ужин был, но без аккаунта ответ «как было?» записать некуда
	got = nil
	if sent, _ := n.Tick(ctx, time.Date(2026, 10, 12, 17, 0, 0, 0, time.UTC)); sent != 0 {
		t.Fatalf("вечером без аккаунта не спрашиваем: %+v", got)
	}
	// утреннее напоминание выключили в карточке: во вторник тишина
	if err := n.SubscribeDevice(ctx, sub, "p1", 180, false); err != nil {
		t.Fatal(err)
	}
	if sent, _ := n.Tick(ctx, time.Date(2026, 10, 13, 5, 5, 0, 0, time.UTC)); sent != 0 {
		t.Fatalf("утро выключено: %+v", got)
	}
	// воскресенье в полдень: неделя кончается сегодня, следующей нет — «собрать неделю», без магазина
	if sent, _ := n.Tick(ctx, time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)); sent != 1 || got[0].Tag != "week" || got[0].URL != "https://racion.test/" {
		t.Fatalf("воскресенье: sent=%d got=%+v", sent, got)
	}
	// push-сервис ответил «подписки нет»: устройство удаляется вместе с неделями
	dead := domain.PushSubscription{Endpoint: "https://dead", P256dh: "k", Auth: "a", Lang: "ru"}
	if err := n.SubscribeDevice(ctx, dead, "p1", 180, true); err != nil {
		t.Fatal(err)
	}
	n.Tick(ctx, time.Date(2026, 10, 13, 5, 5, 0, 0, time.UTC))
	if _, ok := repo.devices["https://dead"]; ok {
		t.Fatalf("мёртвое устройство должно удалиться")
	}
}
