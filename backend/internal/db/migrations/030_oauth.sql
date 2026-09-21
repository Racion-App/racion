-- Вход через внешние сервисы: одна запись на пару провайдер + id в нём. Пользователь без пароля хранит password_hash = ''.
CREATE TABLE IF NOT EXISTS oauth_accounts (
    provider    TEXT NOT NULL,   -- vk | yandex | google | github | apple
    provider_id TEXT NOT NULL,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, provider_id)
);
CREATE INDEX IF NOT EXISTS oauth_accounts_user ON oauth_accounts (user_id);
