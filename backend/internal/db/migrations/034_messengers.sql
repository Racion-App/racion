-- Боты в Telegram и MAX: чат подключает неделю по ссылке со страницы плана и получает список покупок
-- по отделам и напоминания. Аккаунт необязателен: «без регистрации» обещано и здесь.
CREATE TABLE IF NOT EXISTS messenger_chats (
    platform   TEXT NOT NULL,                       -- telegram | max
    chat_id    TEXT NOT NULL,                       -- Telegram: chat.id; MAX: chat_id диалога
    user_id    UUID REFERENCES users(id) ON DELETE SET NULL, -- аккаунт, если человек привязал его в боте
    lang       TEXT NOT NULL DEFAULT '',             -- язык недели; пусто — язык мессенджера не известен, пишем по-русски
    settings   JSONB NOT NULL DEFAULT '{}',          -- те же поля, что у настроек веб-пуша
    blocked    BOOLEAN NOT NULL DEFAULT false,       -- бота заблокировали: больше не пишем
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (platform, chat_id)
);

-- Какие недели подключены к чату: по ним бот будет напоминать о покупках и готовке.
CREATE TABLE IF NOT EXISTS messenger_plans (
    platform     TEXT NOT NULL,
    chat_id      TEXT NOT NULL,
    plan_id      UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (platform, chat_id, plan_id),
    FOREIGN KEY (platform, chat_id) REFERENCES messenger_chats (platform, chat_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS messenger_plans_plan ON messenger_plans (plan_id);

-- Отправленные напоминания: одно и то же не приходит дважды.
CREATE TABLE IF NOT EXISTS messenger_sent (
    platform TEXT NOT NULL,
    chat_id  TEXT NOT NULL,
    key      TEXT NOT NULL,
    sent_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (platform, chat_id, key)
);
