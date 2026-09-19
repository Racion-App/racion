package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

type APIKeys struct{ pool *pgxpool.Pool }

func (r *APIKeys) Create(ctx context.Context, userID, name, prefix, hash string) (domain.APIKey, error) {
	var k domain.APIKey
	var t time.Time
	err := r.pool.QueryRow(ctx, `INSERT INTO api_keys (user_id, name, prefix, hash) VALUES ($1, $2, $3, $4) RETURNING id, name, prefix, created_at`, userID, name, prefix, hash).Scan(&k.ID, &k.Name, &k.Prefix, &t)
	k.CreatedAt = t.UTC().Format(time.RFC3339)
	return k, wrap("apikeys.create", err)
}

func (r *APIKeys) List(ctx context.Context, userID string) ([]domain.APIKey, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, prefix, created_at, last_used_at FROM api_keys WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, wrap("apikeys.list", err)
	}
	defer rows.Close()
	out := []domain.APIKey{}
	for rows.Next() {
		var k domain.APIKey
		var c time.Time
		var u *time.Time
		if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &c, &u); err != nil {
			return nil, wrap("apikeys.scan", err)
		}
		k.CreatedAt = c.UTC().Format(time.RFC3339)
		if u != nil {
			k.LastUsedAt = u.UTC().Format(time.RFC3339)
		}
		out = append(out, k)
	}
	return out, wrap("apikeys.rows", rows.Err())
}

func (r *APIKeys) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return wrap("apikeys.delete", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UserByHash — владелец ключа; отмечает использование.
func (r *APIKeys) UserByHash(ctx context.Context, hash string) (*domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx, `WITH k AS (UPDATE api_keys SET last_used_at = now() WHERE hash = $1 RETURNING user_id)
		SELECT u.id, u.email, u.name, COALESCE(u.nick, ''), u.avatar, u.role, u.defaults FROM k JOIN users u ON u.id = k.user_id`, hash).Scan(&u.ID, &u.Email, &u.Name, &u.Nick, &u.Avatar, &u.Role, &u.Defaults)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, wrap("apikeys.lookup", err)
	}
	return &u, nil
}
