-- Заготовки (закрутки и быстрые маринады): порция — одна банка jar_l литров; shelf_months — сколько месяцев
-- хранится закрытая банка в прохладном тёмном месте (0 — только холодильник, срок в keep_days).
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS jar_l REAL NOT NULL DEFAULT 0;
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS shelf_months INT NOT NULL DEFAULT 0;
