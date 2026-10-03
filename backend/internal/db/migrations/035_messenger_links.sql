-- Привязка аккаунта к мессенджеру: кабинет выдаёт одноразовую ссылку на бота, бот по ней узнаёт аккаунт.
-- Сама привязка живёт в oauth_accounts (provider = telegram | max): по ней же работает вход в мини-приложении.
CREATE TABLE IF NOT EXISTS messenger_links (
    token_hash TEXT PRIMARY KEY,                -- sha256 токена из ссылки t.me/<бот>?start=u<токен>
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS messenger_chats_user ON messenger_chats (user_id) WHERE user_id IS NOT NULL;
