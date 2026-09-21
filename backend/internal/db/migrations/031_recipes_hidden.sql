-- Скрытые рецепты базы: есть в админке, но не в планировщике, каталоге, поиске, sitemap и подборках,
-- пока не вычитаны, не переведены и не сфотографированы. Новые рецепты через API создаются скрытыми.
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT false;
