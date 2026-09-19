-- Рацион: базовая схема.
-- Цены в ingredients — ориентировочные средние по России; индекс сети в stores.

CREATE TABLE IF NOT EXISTS stores (
    code        TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL,            -- discounter | supermarket | hypermarket | premium | wholesale
    price_index NUMERIC(4,2) NOT NULL,    -- 1.00 = базовая цена
    note        TEXT NOT NULL DEFAULT '',
    sort        INT  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS ingredients (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    category    TEXT NOT NULL,            -- отдел магазина
    unit        TEXT NOT NULL,            -- g | ml | pcs
    pack        NUMERIC(10,2) NOT NULL,   -- размер типовой упаковки в unit
    price       NUMERIC(10,2) NOT NULL,   -- цена упаковки, ₽ (база)
    kcal        NUMERIC(8,2) NOT NULL,    -- на 100 g/ml или 1 pcs
    protein     NUMERIC(8,2) NOT NULL,
    fat         NUMERIC(8,2) NOT NULL,
    carb        NUMERIC(8,2) NOT NULL,
    allergens   TEXT[] NOT NULL DEFAULT '{}',
    perishable  BOOLEAN NOT NULL DEFAULT FALSE,
    pantry      BOOLEAN NOT NULL DEFAULT FALSE, -- специи/масло: обычно уже есть дома
    loose       BOOLEAN NOT NULL DEFAULT FALSE  -- продаётся на вес, округляем до 100 г, а не до упаковки
);

CREATE TABLE IF NOT EXISTS recipes (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    slot        TEXT NOT NULL,            -- breakfast | lunch | dinner | snack
    time_min    INT  NOT NULL,
    equipment   TEXT[] NOT NULL DEFAULT '{}',
    tags        TEXT[] NOT NULL DEFAULT '{}',
    batch       BOOLEAN NOT NULL DEFAULT FALSE,  -- готовится на два дня
    steps       TEXT[] NOT NULL
);

CREATE TABLE IF NOT EXISTS recipe_ingredients (
    recipe_id     TEXT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    ingredient_id TEXT NOT NULL REFERENCES ingredients(id),
    amount        NUMERIC(10,2) NOT NULL,        -- на 1 порцию, в unit ингредиента
    sort          INT NOT NULL DEFAULT 0,
    PRIMARY KEY (recipe_id, ingredient_id)
);

CREATE TABLE IF NOT EXISTS plans (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    params      JSONB NOT NULL,
    plan        JSONB NOT NULL,
    seed        BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Собственная аналитика: анонимный sid, имя события, свойства.
CREATE TABLE IF NOT EXISTS events (
    id          BIGSERIAL PRIMARY KEY,
    sid         TEXT NOT NULL,
    name        TEXT NOT NULL,
    props       JSONB NOT NULL DEFAULT '{}',
    ts          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS events_name_ts ON events (name, ts);
CREATE INDEX IF NOT EXISTS events_sid ON events (sid);
