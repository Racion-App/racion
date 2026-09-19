-- Ключи API для админов: тот же пользователь и права, что у сессии, но без cookie — для скриптов и MCP.
-- Храним только sha256; сам ключ показывается один раз при создании.
CREATE TABLE IF NOT EXISTS api_keys (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  prefix       TEXT NOT NULL,           -- первые символы для списка: rk_ab12…
  hash         TEXT NOT NULL UNIQUE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS api_keys_user ON api_keys (user_id, created_at);
