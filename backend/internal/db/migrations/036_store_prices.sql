-- Цены сетей с открытым каталогом (ВкусВилл) по последней сверке. Каким товаром сети закрыт наш продукт,
-- решают данные проекта, и это правится руками; здесь — что каждый такой товар стоит сейчас и сколько весит.
CREATE TABLE IF NOT EXISTS store_prices (
    store      TEXT NOT NULL,
    xml_id     INTEGER NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    url        TEXT NOT NULL DEFAULT '',
    unit       TEXT NOT NULL DEFAULT '',             -- «шт» — упаковка, «кг» — на развес
    weight     DOUBLE PRECISION NOT NULL DEFAULT 0,  -- вес упаковки, кг
    price      DOUBLE PRECISION NOT NULL DEFAULT 0,  -- обычная цена, без скидки по карте
    found      BOOLEAN NOT NULL DEFAULT true,        -- false — сеть сняла товар с продажи
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (store, xml_id)
);

-- Когда начиналась последняя сверка сети: её берёт один экземпляр сервера, а не оба при плавном деплое.
CREATE TABLE IF NOT EXISTS store_price_runs (
    store      TEXT PRIMARY KEY,
    started_at TIMESTAMPTZ NOT NULL
);
