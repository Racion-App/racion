package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// Messenger — чаты ботов в Telegram и MAX, подключённые к ним недели и журнал напоминаний.
type Messenger struct{ pool *pgxpool.Pool }

// SaveChat заводит чат; человек вернулся к боту — снимаем отметку о блокировке. Язык мессенджера
// берём только для нового чата: дальше чат говорит на языке подключённой недели (SetLang).
func (r *Messenger) SaveChat(ctx context.Context, platform, chatID, lang string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO messenger_chats (platform, chat_id, lang) VALUES ($1, $2, $3)
		ON CONFLICT (platform, chat_id) DO UPDATE SET lang = CASE WHEN messenger_chats.lang = '' THEN EXCLUDED.lang ELSE messenger_chats.lang END, blocked = false`,
		platform, chatID, lang)
	return wrap("messenger.save", err)
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

// Plans — недели чата, последние подключённые первыми.
func (r *Messenger) Plans(ctx context.Context, platform, chatID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT plan_id::text FROM messenger_plans WHERE platform = $1 AND chat_id = $2 ORDER BY created_at DESC LIMIT 6`,
		platform, chatID)
	if err != nil {
		return nil, wrap("messenger.plans", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, wrap("messenger.plans", err)
		}
		out = append(out, id)
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

// Active — чаты, которым есть что напоминать: не заблокированы и с подключённой неделей.
func (r *Messenger) Active(ctx context.Context) ([]domain.MessengerChat, error) {
	rows, err := r.pool.Query(ctx, `SELECT c.platform, c.chat_id, c.user_id::text, c.lang, c.settings FROM messenger_chats c
		WHERE NOT c.blocked AND EXISTS (SELECT 1 FROM messenger_plans p WHERE p.platform = c.platform AND p.chat_id = c.chat_id)`)
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
