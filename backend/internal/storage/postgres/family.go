package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// ── Семья: участники плана ─────────────────────────────────────────────────

type PlanMembers struct{ pool *pgxpool.Pool }

func (r *PlanMembers) Add(ctx context.Context, planID, userID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO plan_members (plan_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, planID, userID)
	return wrap("plan_members.add", err)
}

func (r *PlanMembers) Is(ctx context.Context, planID, userID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM plan_members WHERE plan_id = $1 AND user_id = $2)`, planID, userID).Scan(&ok)
	return ok, wrap("plan_members.is", err)
}

// Names — имена участников (владелец первым) для подписи «Семья: …».
func (r *PlanMembers) Names(ctx context.Context, planID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT COALESCE(NULLIF(u.name, ''), split_part(u.email, '@', 1)) FROM plan_members m JOIN users u ON u.id = m.user_id WHERE m.plan_id = $1 ORDER BY m.joined_at`, planID)
	if err != nil {
		return nil, wrap("plan_members.names", err)
	}
	names, err := scanStrings(rows)
	return names, wrap("plan_members.names", err)
}

// ── Push-подписки и настройки ──────────────────────────────────────────────

type Push struct{ pool *pgxpool.Pool }

func (r *Push) Save(ctx context.Context, userID string, s domain.PushSubscription) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth, lang) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth, lang = EXCLUDED.lang`, s.Endpoint, userID, s.P256dh, s.Auth, s.Lang)
	return wrap("push.save", err)
}

func (r *Push) Delete(ctx context.Context, endpoint string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return wrap("push.delete", err)
}

func (r *Push) ByUser(ctx context.Context, userID string) ([]domain.PushSubscription, error) {
	rows, err := r.pool.Query(ctx, `SELECT endpoint, p256dh, auth, lang FROM push_subscriptions WHERE user_id = $1`, userID)
	if err != nil {
		return nil, wrap("push.by_user", err)
	}
	defer rows.Close()
	var out []domain.PushSubscription
	for rows.Next() {
		var s domain.PushSubscription
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth, &s.Lang); err != nil {
			return nil, wrap("push.by_user", err)
		}
		out = append(out, s)
	}
	return out, wrap("push.by_user", rows.Err())
}

// Users — кто вообще подписан: id и настройки напоминаний.
func (r *Push) Users(ctx context.Context) ([]domain.NotifyUser, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT u.id, u.notify FROM push_subscriptions s JOIN users u ON u.id = s.user_id`)
	if err != nil {
		return nil, wrap("push.users", err)
	}
	defer rows.Close()
	var out []domain.NotifyUser
	for rows.Next() {
		var u domain.NotifyUser
		var raw []byte
		if err := rows.Scan(&u.UserID, &raw); err != nil {
			return nil, wrap("push.users", err)
		}
		u.Settings = domain.DefaultNotify()
		_ = json.Unmarshal(raw, &u.Settings)
		out = append(out, u)
	}
	return out, wrap("push.users", rows.Err())
}

func (r *Push) Settings(ctx context.Context, userID string) (domain.NotifySettings, error) {
	var raw []byte
	s := domain.DefaultNotify()
	if err := r.pool.QueryRow(ctx, `SELECT notify FROM users WHERE id = $1`, userID).Scan(&raw); err != nil {
		return s, wrap("push.settings", err)
	}
	_ = json.Unmarshal(raw, &s)
	return s, nil
}

func (r *Push) SetSettings(ctx context.Context, userID string, s domain.NotifySettings) error {
	b, _ := json.Marshal(s)
	_, err := r.pool.Exec(ctx, `UPDATE users SET notify = $2 WHERE id = $1`, userID, b)
	return wrap("push.set_settings", err)
}

// MarkSent пишет ключ отправки; false — уже слали.
func (r *Push) MarkSent(ctx context.Context, userID, key string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `INSERT INTO notifications_sent (user_id, key) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, key)
	if err != nil {
		return false, wrap("push.mark", err)
	}
	return tag.RowsAffected() > 0, nil
}

// PlansForReminders — планы пользователя (свои и семейные) с датой начала и числом неотмеченных позиций.
func (r *Push) PlansForReminders(ctx context.Context, userID string) ([]domain.PlanReminderInfo, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.id, p.plan FROM plans p WHERE p.user_id = $1 OR p.id IN (SELECT plan_id FROM plan_members WHERE user_id = $1)
		OR p.user_id IN (SELECT y.user_id FROM household_users x JOIN household_users y ON x.household_id = y.household_id WHERE x.user_id = $1)
		ORDER BY p.created_at DESC LIMIT 6`, userID)
	if err != nil {
		return nil, wrap("push.plans", err)
	}
	defer rows.Close()
	var out []domain.PlanReminderInfo
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, wrap("push.plans", err)
		}
		var pl struct {
			Params struct {
				StartDate string `json:"startDate"`
			} `json:"params"`
			Days []struct {
				Date   string `json:"date"`
				Dishes []struct {
					Title    string `json:"title"`
					Slot     string `json:"slot"`
					RecipeID string `json:"recipeId"`
					Leftover bool   `json:"leftover"`
				} `json:"dishes"`
			} `json:"days"`
			Totals struct {
				Items int `json:"items"`
			} `json:"totals"`
		}
		if json.Unmarshal(raw, &pl) != nil {
			continue
		}
		info := domain.PlanReminderInfo{ID: id, StartDate: pl.Params.StartDate, Items: pl.Totals.Items, Dishes: map[string][]string{}, Dinner: map[string]domain.DishRef{}}
		for _, d := range pl.Days {
			for _, x := range d.Dishes {
				info.Dishes[d.Date] = append(info.Dishes[d.Date], x.Title)
				if x.Slot == "dinner" && !x.Leftover {
					info.Dinner[d.Date] = domain.DishRef{RecipeID: x.RecipeID, Title: x.Title}
				}
			}
		}
		out = append(out, info)
	}
	if rows.Err() != nil {
		return nil, wrap("push.plans", rows.Err())
	}
	for i := range out {
		var checked int
		_ = r.pool.QueryRow(ctx, `SELECT count(*) FROM plan_checks WHERE plan_id = $1`, out[i].ID).Scan(&checked)
		out[i].Checked = checked
	}
	return out, nil
}

// ── Настройки сервера (ключи VAPID) ────────────────────────────────────────

type Settings struct{ pool *pgxpool.Pool }

func (r *Settings) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := r.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&v)
	return v, wrap("settings.get", err)
}

func (r *Settings) Set(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value)
	return wrap("settings.set", err)
}
