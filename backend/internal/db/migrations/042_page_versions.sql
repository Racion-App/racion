-- Когда страница менялась по-настоящему: хеш её содержимого и дата его смены. Отсюда <lastmod> в карте сайта,
-- dateModified в разметке рецепта и список адресов, которые после деплоя уходят поисковикам через IndexNow.
CREATE TABLE IF NOT EXISTS page_versions (
    path       TEXT PRIMARY KEY,
    hash       TEXT NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL
);
