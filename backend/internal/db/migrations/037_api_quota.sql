-- Квота стороннего API: запросы с чужих сайтов, из скриптов и MCP — не больше 50 в час и 300 в сутки
-- на адрес. Счётчик в базе, чтобы деплой не обнулял сутки и оба экземпляра при плавном деплое считали вместе.
-- kind: 'h' — час, 'd' — сутки (UTC); bucket — начало часа или суток.
CREATE TABLE IF NOT EXISTS api_usage (
    key    TEXT NOT NULL,
    kind   CHAR(1) NOT NULL,
    bucket TIMESTAMPTZ NOT NULL,
    n      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (key, kind, bucket)
);
