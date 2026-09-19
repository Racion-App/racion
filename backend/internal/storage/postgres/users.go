package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// ── Пользователи ───────────────────────────────────────────────────────────

type Users struct{ pool *pgxpool.Pool }

func (r *Users) Create(ctx context.Context, email, passwordHash, name string) (domain.User, error) {
	u := domain.User{Email: email, Name: name, Defaults: json.RawMessage(`{}`)}
	err := r.pool.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id`, email, passwordHash, name).Scan(&u.ID)
	return u, wrap("users.create", err)
}

// ByEmail возвращает пользователя и хеш пароля для входа.
func (r *Users) ByEmail(ctx context.Context, email string) (domain.User, string, error) {
	var u domain.User
	var hash string
	err := r.pool.QueryRow(ctx, `SELECT id, email, name, COALESCE(nick, ''), avatar, role, defaults, password_hash FROM users WHERE email = $1`, email).Scan(&u.ID, &u.Email, &u.Name, &u.Nick, &u.Avatar, &u.Role, &u.Defaults, &hash)
	return u, hash, wrap("users.by_email", err)
}

// SetRole — роль аккаунта: "" | moderator | admin.
func (r *Users) SetRole(ctx context.Context, id, role string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE users SET role = $2 WHERE id = $1`, id, role)
	if err != nil {
		return wrap("users.set_role", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetAvatar — ссылка на фото профиля; пусто — убрать.
func (r *Users) SetAvatar(ctx context.Context, id, url string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET avatar = $2 WHERE id = $1`, id, url)
	return wrap("users.set_avatar", err)
}

func (r *Users) SetName(ctx context.Context, id, name string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET name = $2 WHERE id = $1`, id, name)
	return wrap("users.set_name", err)
}

// SetNick — уникальный ник (без учёта регистра); занятый → ErrConflict.
func (r *Users) SetNick(ctx context.Context, id, nick string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET nick = NULLIF($2, '') WHERE id = $1`, id, nick)
	return wrap("users.set_nick", err)
}

// NickOf — ник владельца записи (для подписи автора своего рецепта).
func (r *Users) NickOf(ctx context.Context, id string) (string, error) {
	var nick string
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(nick, '') FROM users WHERE id = $1`, id).Scan(&nick)
	return nick, wrap("users.nick", err)
}

func (r *Users) SetDefaults(ctx context.Context, id string, defaults json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET defaults = $2 WHERE id = $1`, id, defaults)
	return wrap("users.set_defaults", err)
}

// ── Сессии ─────────────────────────────────────────────────────────────────

type Sessions struct{ pool *pgxpool.Pool }

func (r *Sessions) Create(ctx context.Context, token, userID string, expires time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)`, token, userID, expires)
	return wrap("sessions.create", err)
}

// UserByToken — пользователь по живой сессии.
func (r *Sessions) UserByToken(ctx context.Context, token string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx, `SELECT u.id, u.email, u.name, COALESCE(u.nick, ''), u.avatar, u.role, u.defaults FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > now()`, token).Scan(&u.ID, &u.Email, &u.Name, &u.Nick, &u.Avatar, &u.Role, &u.Defaults)
	return u, wrap("sessions.user", err)
}

func (r *Sessions) Delete(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return wrap("sessions.delete", err)
}

// ── Нелюбимые рецепты ──────────────────────────────────────────────────────

type Dislikes struct{ pool *pgxpool.Pool }

func (r *Dislikes) List(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT recipe_id FROM user_dislikes WHERE user_id = $1`, userID)
	if err != nil {
		return nil, wrap("dislikes.list", err)
	}
	ids, err := scanStrings(rows)
	return ids, wrap("dislikes.list", err)
}

func (r *Dislikes) Add(ctx context.Context, userID, recipeID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO user_dislikes (user_id, recipe_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, recipeID)
	return wrap("dislikes.add", err)
}

func (r *Dislikes) Remove(ctx context.Context, userID, recipeID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM user_dislikes WHERE user_id = $1 AND recipe_id = $2`, userID, recipeID)
	return wrap("dislikes.remove", err)
}

// ── Покупки ────────────────────────────────────────────────────────────────

type Purchases struct{ pool *pgxpool.Pool }

func (r *Purchases) Add(ctx context.Context, userID, planID, itemID, name, qty string, cost float64) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO purchases (user_id, plan_id, item_id, name, qty, cost) VALUES ($1, $2, $3, $4, $5, $6)`, userID, planID, itemID, name, qty, cost)
	return wrap("purchases.add", err)
}

// RemoveLatest убирает самую свежую запись об этой покупке (снятая отметка).
func (r *Purchases) RemoveLatest(ctx context.Context, userID, planID, itemID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM purchases WHERE id = (SELECT id FROM purchases WHERE user_id = $1 AND plan_id = $2 AND item_id = $3 ORDER BY bought_at DESC LIMIT 1)`, userID, planID, itemID)
	return wrap("purchases.remove", err)
}

func (r *Purchases) Recent(ctx context.Context, userID string, days int) ([]domain.Purchase, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, plan_id, item_id, name, qty, cost, bought_at FROM purchases
		WHERE user_id = $1 AND bought_at > now() - make_interval(days => $2) ORDER BY bought_at DESC LIMIT 1000`, userID, days)
	if err != nil {
		return nil, wrap("purchases.recent", err)
	}
	defer rows.Close()
	out := []domain.Purchase{}
	for rows.Next() {
		var p domain.Purchase
		var t time.Time
		if err := rows.Scan(&p.ID, &p.PlanID, &p.ItemID, &p.Name, &p.Qty, &p.Cost, &t); err != nil {
			return nil, wrap("purchases.recent", err)
		}
		p.BoughtAt = t.UTC().Format(time.RFC3339)
		out = append(out, p)
	}
	return out, wrap("purchases.recent", rows.Err())
}
