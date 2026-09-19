-- Фото: свои рецепты, комментарии, аватар. Файлы лежат в S3 (MinIO) в WebP, тут — публичные ссылки.
ALTER TABLE user_recipes ADD COLUMN IF NOT EXISTS image TEXT NOT NULL DEFAULT '';
ALTER TABLE recipe_comments ADD COLUMN IF NOT EXISTS image TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar TEXT NOT NULL DEFAULT '';
