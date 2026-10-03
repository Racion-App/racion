package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// Messenger — чаты ботов в Telegram и MAX, подключённые к ним недели и журнал напоминаний.
type Messenger struct{ pool *pgxpool.Pool }

// SaveChat заводит чат; человек вернулся к боту — снимаем отметку о блокировке. Язык мессенджера
// берём только для нового чата: дальше чат говорит на языке подключённой недели (SetLang).
// true — чат новый.
func (r *Messenger) SaveChat(ctx context.Context, platform, chatID, lang string) (bool, error) {
	var created bool
	err := r.pool.QueryRow(ctx, `INSERT INTO messenger_chats (platform, chat_id, lang) VALUES ($1, $2, $3)
		ON CONFLICT (platform, chat_id) DO UPDATE SET lang = CASE WHEN messenger_chats.lang = '' THEN EXCLUDED.lang ELSE messenger_chats.lang END, blocked = false
		RETURNING (xmax = 0)`, platform, chatID, lang).Scan(&created)
	return created, wrap("messenger.save", err)
}

// SetUser — аккаунт сайта, к которому привязан чат; nil — отвязан.
func (r *Messenger) SetUser(ctx context.Context, platform, chatID string, userID *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE messenger_chats SET user_id = $3 WHERE platform = $1 AND chat_id = $2`, platform, chatID, userID)
	return wrap("messenger.user", err)
}

// ForgetUser — аккаунт отвязал мессенджер: его чаты больше не знают аккаунт.
func (r *Messenger) ForgetUser(ctx context.Context, platform, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE messenger_chats SET user_id = NULL WHERE platform = $1 AND user_id = $2`, platform, userID)
	return wrap("messenger.forget", err)
}

// CreateLink — одноразовая ссылка привязки аккаунта; в базе только хеш токена.
func (r *Messenger) CreateLink(ctx context.Context, tokenHash, userID string, expires time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO messenger_links (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, tokenHash, userID, expires)
	return wrap("messenger.link_create", err)
}

// TakeLink — аккаунт по токену ссылки; токен сгорает. Просроченный или чужой — ErrNotFound.
func (r *Messenger) TakeLink(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.pool.QueryRow(ctx, `DELETE FROM messenger_links WHERE token_hash = $1 AND expires_at > now() RETURNING user_id::text`, tokenHash).Scan(&userID)
	return userID, wrap("messenger.link_take", err)
}

func (r *Messenger) SetLang(ctx context.Context, platform, chatID, lang string) error {
	_, err := r.pool.Exec(ctx, `UPDATE messenger_chats SET lang = $3 WHERE platform = $1 AND chat_id = $2`, platform, chatID, lang)
	return wrap("messenger.lang", err)
}

func (r *Messenger) Chat(ctx context.Context, platform, chatID string) (domain.MessengerChat, error) {
	c := domain.MessengerChat{Platform: platform, ChatID: chatID}
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT user_id::text, lang, settings, blocked FROM messenger_chats WHERE platform = $1 AND chat_id = $2`,
		platform, chatID).Scan(&c.UserID, &c.Lang, &raw, &c.Blocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, domain.ErrNotFound
	}
	if err != nil {
		return c, wrap("messenger.chat", err)
	}
	c.Settings = chatSettings(raw)
	return c, nil
}

func (r *Messenger) SetBlocked(ctx context.Context, platform, chatID string, blocked bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE messenger_chats SET blocked = $3 WHERE platform = $1 AND chat_id = $2`, platform, chatID, blocked)
	return wrap("messenger.blocked", err)
}

func (r *Messenger) SetSettings(ctx context.Context, platform, chatID string, s domain.NotifySettings) error {
	raw, _ := json.Marshal(s)
	_, err := r.pool.Exec(ctx, `UPDATE messenger_chats SET settings = $3 WHERE platform = $1 AND chat_id = $2`, platform, chatID, raw)
	return wrap("messenger.settings", err)
}

// LinkPlan подключает неделю к чату; повторное подключение просто освежает время.
func (r *Messenger) LinkPlan(ctx context.Context, platform, chatID, planID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO messenger_plans (platform, chat_id, plan_id) VALUES ($1, $2, $3)
		ON CONFLICT (platform, chat_id, plan_id) DO UPDATE SET created_at = now()`, platform, chatID, planID)
	return wrap("messenger.link", err)
}

// Plans — недели чата, последние подключённые первыми, со временем подключения.
func (r *Messenger) Plans(ctx context.Context, platform, chatID string) ([]domain.ChatPlan, error) {
	rows, err := r.pool.Query(ctx, `SELECT plan_id::text, created_at FROM messenger_plans WHERE platform = $1 AND chat_id = $2 ORDER BY created_at DESC LIMIT 6`,
		platform, chatID)
	if err != nil {
		return nil, wrap("messenger.plans", err)
	}
	defer rows.Close()
	var out []domain.ChatPlan
	for rows.Next() {
		var p domain.ChatPlan
		if err := rows.Scan(&p.ID, &p.At); err != nil {
			return nil, wrap("messenger.plans", err)
		}
		out = append(out, p)
	}
	return out, wrap("messenger.plans", rows.Err())
}

// MarkSent — true, если напоминание с таким ключом ещё не отправляли в этот чат.
func (r *Messenger) MarkSent(ctx context.Context, platform, chatID, key string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `INSERT INTO messenger_sent (platform, chat_id, key) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		platform, chatID, key)
	if err != nil {
		return false, wrap("messenger.mark", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Lead — блокировка в базе на всё время работы: true — этот экземпляр сервера главный, пока не вызовет
// release или не умрёт (соединение закроется, и Postgres снимет блокировку сам). Нужна там, где работа
// должна идти в одном месте, а экземпляров при плавном деплое два.
func (r *Messenger) Lead(ctx context.Context, key int64) (func(), bool, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, false, wrap("messenger.lead", err)
	}
	var ok bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&ok); err != nil || !ok {
		conn.Release()
		return nil, false, wrap("messenger.lead", err)
	}
	return func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key)
		conn.Release()
	}, true, nil
}

// ReminderPlans — недели для напоминаний в чат: подключённые к нему и, если аккаунт привязан,
// свои и семейные недели аккаунта. Последние сначала.
func (r *Messenger) ReminderPlans(ctx context.Context, platform, chatID string, userID *string) ([]domain.PlanReminderInfo, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.id, p.plan FROM plans p
		WHERE p.id IN (SELECT plan_id FROM messenger_plans WHERE platform = $1 AND chat_id = $2)
		OR ($3::uuid IS NOT NULL AND (p.user_id = $3::uuid OR p.id IN (SELECT plan_id FROM plan_members WHERE user_id = $3::uuid)
			OR p.user_id IN (SELECT y.user_id FROM household_users x JOIN household_users y ON x.household_id = y.household_id WHERE x.user_id = $3::uuid)))
		ORDER BY p.created_at DESC LIMIT 6`, platform, chatID, userID)
	if err != nil {
		return nil, wrap("messenger.reminder_plans", err)
	}
	return reminderPlans(ctx, r.pool, rows)
}

// Active — чаты, которым есть что напоминать: не заблокированы, с подключённой неделей или привязанным аккаунтом.
func (r *Messenger) Active(ctx context.Context) ([]domain.MessengerChat, error) {
	rows, err := r.pool.Query(ctx, `SELECT c.platform, c.chat_id, c.user_id::text, c.lang, c.settings FROM messenger_chats c
		WHERE NOT c.blocked AND (c.user_id IS NOT NULL OR EXISTS (SELECT 1 FROM messenger_plans p WHERE p.platform = c.platform AND p.chat_id = c.chat_id))`)
	if err != nil {
		return nil, wrap("messenger.active", err)
	}
	defer rows.Close()
	var out []domain.MessengerChat
	for rows.Next() {
		var c domain.MessengerChat
		var raw []byte
		if err := rows.Scan(&c.Platform, &c.ChatID, &c.UserID, &c.Lang, &raw); err != nil {
			return nil, wrap("messenger.active", err)
		}
		c.Settings = chatSettings(raw)
		out = append(out, c)
	}
	return out, wrap("messenger.active", rows.Err())
}

// chatSettings — настройки чата поверх значений по умолчанию: в новом чате поле пустое.
func chatSettings(raw []byte) domain.NotifySettings {
	s := domain.DefaultNotify()
	if len(raw) > 2 {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}
