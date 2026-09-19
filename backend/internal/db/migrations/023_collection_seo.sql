-- Редакционный текст подборки по языкам: вступление, как пользоваться, вопросы-ответы (cmd/racionai collections).
ALTER TABLE collections ADD COLUMN IF NOT EXISTS seo JSONB NOT NULL DEFAULT '{}';
