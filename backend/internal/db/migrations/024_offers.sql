-- Точечные рекламные предложения: конкретный товар или акция магазина, показанные там, где уместны:
-- у корзины (промокод на первый заказ), в рецепте (техника или продукт из этого рецепта), в плане.
-- Таргетинг: страна и, при желании, регионы или города Росстата (пусто — вся страна).
CREATE TABLE IF NOT EXISTS offers (
  id         text PRIMARY KEY,
  country    text NOT NULL,
  regions    text[] NOT NULL DEFAULT '{}',          -- коды регионов/городов; пусто = вся страна
  place      text NOT NULL CHECK (place IN ('cart', 'recipe', 'plan')),
  match      text[] NOT NULL DEFAULT '{}',          -- id техники или продуктов, при которых показывать (recipe); пусто = всегда
  title      text NOT NULL,
  body       text NOT NULL DEFAULT '',
  cta        text NOT NULL DEFAULT '',
  url        text NOT NULL,
  image      text NOT NULL DEFAULT '',
  promo      text NOT NULL DEFAULT '',              -- промокод, который можно скопировать
  affiliate  boolean NOT NULL DEFAULT true,         -- реклама: подпись «Реклама» и erid
  erid       text NOT NULL DEFAULT '',
  starts_at  timestamptz,
  ends_at    timestamptz,
  active     boolean NOT NULL DEFAULT true,
  priority   int NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS offers_country_place_idx ON offers (country, place, active);
