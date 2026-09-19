-- Режим заготовок: сколько дней готовое блюдо стоит в холодильнике (NULL — по правилам prep.go) и морозится ли.
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS keep_days INT;
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS freeze BOOLEAN NOT NULL DEFAULT FALSE;
