package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"racion/internal/domain"
)

// CreateLogin — запрос входа через бота; в базе только хеши токена и секрета браузера.
func (r *Messenger) CreateLogin(ctx context.Context, l domain.MessengerLogin, expires time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO messenger_logins (token_hash, poll_hash, platform, lang, plan_id, agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, l.TokenHash, l.PollHash, l.Platform, l.Lang, l.PlanID, l.Agent, expires)
	return wrap("messenger.login_create", err)
}

// PendingLogin — запрос по токену из ссылки, ещё не подтверждённый и не просроченный.
func (r *Messenger) PendingLogin(ctx context.Context, tokenHash string) (domain.MessengerLogin, error) {
	l := domain.MessengerLogin{TokenHash: tokenHash}
	err := r.pool.QueryRow(ctx, `SELECT platform, lang, plan_id, agent FROM messenger_logins
		WHERE token_hash = $1 AND user_id IS NULL AND expires_at > now()`, tokenHash).Scan(&l.Platform, &l.Lang, &l.PlanID, &l.Agent)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, domain.ErrNotFound
	}
	return l, wrap("messenger.login_pending", err)
}

// ConfirmLogin — человек подтвердил вход в боте: запрос узнаёт аккаунт. Чужой или просроченный — ErrNotFound.
func (r *Messenger) ConfirmLogin(ctx context.Context, tokenHash, userID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE messenger_logins SET user_id = $2
		WHERE token_hash = $1 AND user_id IS NULL AND expires_at > now()`, tokenHash, userID)
	if err != nil {
		return wrap("messenger.login_confirm", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DropLogin — человек ответил «это не я»: запрос удаляется, страница входа его больше не дождётся.
func (r *Messenger) DropLogin(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM messenger_logins WHERE token_hash = $1`, tokenHash)
	return wrap("messenger.login_drop", err)
}

// TakeLogin — что с запросом браузера: подтверждённый сгорает и отдаёт аккаунт и неделю, ждущий
// отвечает ok=false без аккаунта, удалённый или просроченный — ErrNotFound.
func (r *Messenger) TakeLogin(ctx context.Context, pollHash string) (userID, planID string, err error) {
	err = r.pool.QueryRow(ctx, `DELETE FROM messenger_logins WHERE poll_hash = $1 AND user_id IS NOT NULL AND expires_at > now()
		RETURNING user_id::text, plan_id`, pollHash).Scan(&userID, &planID)
	if err == nil {
		return userID, planID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", wrap("messenger.login_take", err)
	}
	var waiting bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM messenger_logins WHERE poll_hash = $1 AND expires_at > now())`, pollHash).Scan(&waiting); err != nil {
		return "", "", wrap("messenger.login_wait", err)
	}
	if !waiting {
		return "", "", domain.ErrNotFound
	}
	return "", "", nil
}
