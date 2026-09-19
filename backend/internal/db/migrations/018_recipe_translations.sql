-- Переводы своих рецептов: язык оригинала, тексты по языкам и очередь с состоянием по языкам
ALTER TABLE user_recipes
  ADD COLUMN IF NOT EXISTS lang TEXT NOT NULL DEFAULT 'ru',
  ADD COLUMN IF NOT EXISTS i18n JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE TABLE IF NOT EXISTS recipe_translations (
    recipe_id  TEXT NOT NULL REFERENCES user_recipes(id) ON DELETE CASCADE,
    lang       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'queued',   -- queued | running | done | error
    model      TEXT NOT NULL DEFAULT '',         -- provider/model, что перевёл
    error      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (recipe_id, lang)
);
CREATE INDEX IF NOT EXISTS recipe_translations_queue ON recipe_translations (status, updated_at);
