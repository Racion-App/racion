-- Переводы названий и описаний редакционных подборок: {"en": "New Year table", ...}
ALTER TABLE collections
  ADD COLUMN IF NOT EXISTS name_i18n JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS description_i18n JSONB NOT NULL DEFAULT '{}';
