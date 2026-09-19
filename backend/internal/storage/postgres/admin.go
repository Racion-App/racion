package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// Admin — агрегаты для панели администратора: люди, недели, события, ошибки.
type Admin struct{ pool *pgxpool.Pool }

// Counters — числа «сейчас» одной строкой.
func (r *Admin) Counters(ctx context.Context) (domain.AdminCounters, error) {
	var c domain.AdminCounters
	err := r.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM users),
		(SELECT count(*) FROM users WHERE created_at > now() - interval '7 days'),
		(SELECT count(DISTINCT user_id) FROM sessions WHERE expires_at > now() AND created_at > now() - interval '7 days'),
		(SELECT count(*) FROM plans),
		(SELECT count(*) FROM plans WHERE created_at > now() - interval '7 days'),
		(SELECT count(*) FROM plans WHERE user_id IS NOT NULL),
		(SELECT count(*) FROM user_recipes),
		(SELECT count(*) FROM households),
		(SELECT count(DISTINCT user_id) FROM push_subscriptions),
		(SELECT count(*) FROM recipe_comments),
		(SELECT count(*) FROM recipe_feedback),
		(SELECT count(*) FROM purchases WHERE bought_at > now() - interval '7 days'),
		(SELECT count(*) FROM events WHERE name = 'js_error' AND ts > now() - interval '7 days')`).Scan(
		&c.Users, &c.UsersWeek, &c.ActiveWeek, &c.Plans, &c.PlansWeek, &c.PlansOwned, &c.OwnRecipes, &c.Households, &c.PushUsers, &c.Comments, &c.Feedback, &c.PurchasesWeek, &c.ErrorsWeek)
	return c, wrap("admin.counters", err)
}

// Daily — по дням за days дней: новые люди, новые недели, события квиза и js-ошибки.
func (r *Admin) Daily(ctx context.Context, days int) ([]domain.AdminDay, error) {
	rows, err := r.pool.Query(ctx, `WITH d AS (SELECT generate_series((now() - make_interval(days => $1 - 1))::date, now()::date, '1 day')::date AS day)
		SELECT d.day,
			(SELECT count(*) FROM users u WHERE u.created_at::date = d.day),
			(SELECT count(*) FROM plans p WHERE p.created_at::date = d.day),
			(SELECT count(DISTINCT sid) FROM events e WHERE e.ts::date = d.day),
			(SELECT count(DISTINCT sid) FROM events e WHERE e.ts::date = d.day AND e.name = 'quiz_step'),
			(SELECT count(*) FROM events e WHERE e.ts::date = d.day AND e.name = 'js_error')
		FROM d ORDER BY d.day`, days)
	if err != nil {
		return nil, wrap("admin.daily", err)
	}
	defer rows.Close()
	var out []domain.AdminDay
	for rows.Next() {
		var x domain.AdminDay
		var t time.Time
		if err := rows.Scan(&t, &x.Users, &x.Plans, &x.Visitors, &x.QuizStarts, &x.Errors); err != nil {
			return nil, wrap("admin.daily", err)
		}
		x.Day = t.Format("2006-01-02")
		out = append(out, x)
	}
	return out, wrap("admin.daily", rows.Err())
}

// Events — сколько раз какое событие за days дней (и сколько сессий).
func (r *Admin) Events(ctx context.Context, days int) ([]domain.AdminEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT name, count(*), count(DISTINCT sid) FROM events WHERE ts > now() - make_interval(days => $1)
		GROUP BY name ORDER BY count(*) DESC LIMIT 60`, days)
	if err != nil {
		return nil, wrap("admin.events", err)
	}
	defer rows.Close()
	var out []domain.AdminEvent
	for rows.Next() {
		var x domain.AdminEvent
		if err := rows.Scan(&x.Name, &x.Count, &x.Sessions); err != nil {
			return nil, wrap("admin.events", err)
		}
		out = append(out, x)
	}
	return out, wrap("admin.events", rows.Err())
}

// Top — популярное: страны и магазины недель, лайкнутые рецепты.
func (r *Admin) Top(ctx context.Context) (domain.AdminTop, error) {
	var t domain.AdminTop
	rows, err := r.pool.Query(ctx, `SELECT coalesce(nullif(params->>'country', ''), 'RU'), params->>'store', count(*) FROM plans WHERE created_at > now() - interval '30 days' GROUP BY 1, 2 ORDER BY 3 DESC LIMIT 15`)
	if err != nil {
		return t, wrap("admin.top", err)
	}
	for rows.Next() {
		var x domain.AdminPair
		var a, b *string
		if err := rows.Scan(&a, &b, &x.Count); err != nil {
			rows.Close()
			return t, wrap("admin.top", err)
		}
		x.Key = "RU" // старые недели без страны в параметрах — российские
		if a != nil && *a != "" {
			x.Key = *a
		}
		if b != nil {
			x.Key += " · " + *b
		}
		t.Stores = append(t.Stores, x)
	}
	rows.Close()
	rows, err = r.pool.Query(ctx, `SELECT recipe_id, count(*) FROM recipe_likes GROUP BY 1 ORDER BY 2 DESC LIMIT 15`)
	if err != nil {
		return t, wrap("admin.top", err)
	}
	defer rows.Close()
	for rows.Next() {
		var x domain.AdminPair
		if err := rows.Scan(&x.Key, &x.Count); err != nil {
			return t, wrap("admin.top", err)
		}
		t.Recipes = append(t.Recipes, x)
	}
	return t, wrap("admin.top", rows.Err())
}

// Errors — последние js-ошибки из аналитики: сообщение, адрес, браузер.
func (r *Admin) Errors(ctx context.Context, limit int) ([]domain.AdminError, error) {
	rows, err := r.pool.Query(ctx, `SELECT ts, sid, props->>'message', props->>'url', props->>'stack', props->>'ua' FROM events
		WHERE name = 'js_error' ORDER BY ts DESC LIMIT $1`, limit)
	if err != nil {
		return nil, wrap("admin.errors", err)
	}
	defer rows.Close()
	var out []domain.AdminError
	for rows.Next() {
		var x domain.AdminError
		var t time.Time
		var msg, url, stack, ua *string
		if err := rows.Scan(&t, &x.Sid, &msg, &url, &stack, &ua); err != nil {
			return nil, wrap("admin.errors", err)
		}
		x.At = t.UTC().Format(time.RFC3339)
		x.Message, x.URL, x.Stack, x.UA = deref(msg), deref(url), deref(stack), deref(ua)
		out = append(out, x)
	}
	return out, wrap("admin.errors", rows.Err())
}

// Users — последние аккаунты: почта, имя, когда пришли, сколько недель.
func (r *Admin) Users(ctx context.Context, limit int) ([]domain.AdminUser, error) {
	rows, err := r.pool.Query(ctx, `SELECT u.id, u.email, u.name, u.created_at, (SELECT count(*) FROM plans p WHERE p.user_id = u.id),
		(SELECT max(created_at) FROM sessions s WHERE s.user_id = u.id), u.role FROM users u ORDER BY u.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, wrap("admin.users", err)
	}
	defer rows.Close()
	var out []domain.AdminUser
	for rows.Next() {
		var x domain.AdminUser
		var created time.Time
		var seen *time.Time
		if err := rows.Scan(&x.ID, &x.Email, &x.Name, &created, &x.Plans, &seen, &x.Role); err != nil {
			return nil, wrap("admin.users", err)
		}
		x.CreatedAt = created.UTC().Format(time.RFC3339)
		if seen != nil {
			x.LastSeen = seen.UTC().Format(time.RFC3339)
		}
		out = append(out, x)
	}
	return out, wrap("admin.users", rows.Err())
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
