-- Партнёрские магазины: куда ведут ссылки «где купить» (техника) и «собрать корзину» (доставка продуктов).
-- Дефолты заливает сервер при старте (ON CONFLICT DO NOTHING), правки из админки остаются.
CREATE TABLE IF NOT EXISTS partners (
  code       text PRIMARY KEY,
  country    text NOT NULL,
  kind       text NOT NULL CHECK (kind IN ('goods', 'grocery')),
  name       text NOT NULL,
  url        text NOT NULL,                      -- шаблон с {q}
  affiliate  boolean NOT NULL DEFAULT false,     -- партнёрская ссылка: показываем «Реклама» и erid
  erid       text NOT NULL DEFAULT '',
  active     boolean NOT NULL DEFAULT true,
  priority   int NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS partners_country_idx ON partners (country, kind, active);
