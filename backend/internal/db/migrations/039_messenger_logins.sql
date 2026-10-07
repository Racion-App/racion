-- Вход на сайт через бота Telegram или MAX: страница входа заводит запрос и даёт ссылку на бота,
-- человек подтверждает вход в боте, страница забирает сессию. Хранятся только хеши: токен из ссылки
-- (его видит мессенджер) и секрет браузера (он только в cookie той страницы, что начала вход).
CREATE TABLE IF NOT EXISTS messenger_logins (
    token_hash TEXT PRIMARY KEY,                                  -- sha256 токена из t.me/<бот>?start=l<токен>
    poll_hash  TEXT NOT NULL UNIQUE,                              -- sha256 секрета браузера
    platform   TEXT NOT NULL,                                     -- telegram | max
    lang       TEXT NOT NULL DEFAULT 'ru',
    plan_id    TEXT NOT NULL DEFAULT '',                          -- неделя, которую забрать в аккаунт после входа
    agent      TEXT NOT NULL DEFAULT '',                          -- браузер и система: бот показывает их при подтверждении
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,       -- заполнен — человек подтвердил вход
    expires_at TIMESTAMPTZ NOT NULL
);
