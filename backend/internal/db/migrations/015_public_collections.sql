-- Публичные подборки: редакционные (curated, показываются в каталоге) и открытые по ссылке пользовательские.
ALTER TABLE collections ADD COLUMN IF NOT EXISTS public BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE collections ADD COLUMN IF NOT EXISTS curated BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE collections ADD COLUMN IF NOT EXISTS slug TEXT;
ALTER TABLE collections ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE collections ADD COLUMN IF NOT EXISTS cover TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS collections_slug ON collections (slug) WHERE slug IS NOT NULL;
CREATE INDEX IF NOT EXISTS collections_public ON collections (public, curated, created_at DESC);
