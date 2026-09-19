-- Росстат: средние потребительские цены по территориям (месяц) и по РФ (неделя).

CREATE TABLE IF NOT EXISTS regions (
    code   TEXT PRIMARY KEY,   -- код территории Росстата (ОКАТО-подобный), '643' = РФ
    name   TEXT NOT NULL,
    kind   TEXT NOT NULL,      -- rf | district | region | city
    parent TEXT NOT NULL DEFAULT '',
    sort   INT  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS rosstat_prices (
    region TEXT NOT NULL,
    item   INT  NOT NULL,      -- код товара Росстата
    period TEXT NOT NULL,      -- 'YYYY-MM'
    price  NUMERIC(12,2) NOT NULL,
    PRIMARY KEY (region, item, period)
);

CREATE TABLE IF NOT EXISTS rosstat_weekly (
    item_name TEXT NOT NULL,   -- в недельном файле нет кодов, только названия
    week_date DATE NOT NULL,
    price     NUMERIC(12,2) NOT NULL,
    PRIMARY KEY (item_name, week_date)
);

CREATE TABLE IF NOT EXISTS rosstat_sync (
    id             INT PRIMARY KEY DEFAULT 1,
    monthly_period TEXT NOT NULL,
    weekly_date    DATE,
    fetched_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE ingredients
    ADD COLUMN IF NOT EXISTS rosstat_item   INT,
    ADD COLUMN IF NOT EXISTS rosstat_factor NUMERIC(6,3) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS rosstat_note   TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS rosstat_items (
    item INT PRIMARY KEY,
    name TEXT NOT NULL
);
