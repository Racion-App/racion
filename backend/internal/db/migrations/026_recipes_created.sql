-- Когда рецепт появился в базе: для дайджеста «новое за неделю» и сортировки по новизне.
-- Существующим строкам ставим дату миграции, а не now() при каждом сидинге.
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
