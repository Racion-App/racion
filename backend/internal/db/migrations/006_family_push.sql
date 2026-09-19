-- Семья: аккаунты, присоединившиеся к плану по ссылке, видят его в кабинете и могут менять блюда.
CREATE TABLE IF NOT EXISTS plan_members (
    plan_id   UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (plan_id, user_id)
);
CREATE INDEX IF NOT EXISTS plan_members_user ON plan_members (user_id);

-- Web Push: подписки устройств и настройки напоминаний.
CREATE TABLE IF NOT EXISTS push_subscriptions (
    endpoint   TEXT PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    lang       TEXT NOT NULL DEFAULT 'ru',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS push_subscriptions_user ON push_subscriptions (user_id);

ALTER TABLE users ADD COLUMN IF NOT EXISTS notify JSONB NOT NULL DEFAULT '{}'::jsonb;  -- {"shopDay":0,"shopHour":18,"prep":true,"week":true,"tz":180}

-- Что уже отправили, чтобы не слать дважды: ключ вида shop:<plan>, prep:<plan>:<date>, week:<monday>.
CREATE TABLE IF NOT EXISTS notifications_sent (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);

-- Ключи VAPID и прочие настройки сервера.
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
