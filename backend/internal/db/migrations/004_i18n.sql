-- Страны и переводы.
ALTER TABLE stores ADD COLUMN IF NOT EXISTS country TEXT NOT NULL DEFAULT 'RU';
ALTER TABLE ingredients ADD COLUMN IF NOT EXISTS names JSONB NOT NULL DEFAULT '{}'::jsonb;   -- {"en": "...", "de": "..."}
ALTER TABLE ingredients ADD COLUMN IF NOT EXISTS prices JSONB NOT NULL DEFAULT '{}'::jsonb;  -- {"BY": 2.5, "KZ": 520, ...} цена упаковки в местной валюте
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS i18n JSONB NOT NULL DEFAULT '{}'::jsonb;        -- {"en": {"title","description","steps"}, "de": {...}}

-- Живые цены других стран (аналог rosstat_prices): цена упаковки продукта в местной валюте.
CREATE TABLE IF NOT EXISTS local_prices (
  country TEXT NOT NULL,
  ingredient_id TEXT NOT NULL,
  pack_price NUMERIC(12,4) NOT NULL,
  period DATE NOT NULL,
  source TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (country, ingredient_id)
);

CREATE TABLE IF NOT EXISTS local_sync (
  country TEXT PRIMARY KEY,
  period DATE NOT NULL,
  source TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
