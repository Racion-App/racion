package service

import (
	"context"
	"testing"
	"time"

	"racion/internal/domain"
)

type memPush struct {
	subs  map[string][]domain.PushSubscription
	sent  map[string]bool
	plans map[string][]domain.PlanReminderInfo
	set   map[string]domain.NotifySettings
}

func (m *memPush) Save(_ context.Context, u string, s domain.PushSubscription) error {
	m.subs[u] = append(m.subs[u], s)
	return nil
}
func (m *memPush) Delete(_ context.Context, endpoint string) error {
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

// Напоминания: магазин в выбранный день и час, «завтра готовим» в 19:00, дубликаты не уходят,
// мёртвая подписка удаляется.
func TestReminders(t *testing.T) {
	repo := &memPush{subs: map[string][]domain.PushSubscription{}, sent: map[string]bool{}, plans: map[string][]domain.PlanReminderInfo{}, set: map[string]domain.NotifySettings{}}
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
