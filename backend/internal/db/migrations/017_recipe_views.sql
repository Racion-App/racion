-- Просмотры страницы своего рецепта: автор видит их в кабинете рядом с лайками и комментариями
ALTER TABLE user_recipes ADD COLUMN IF NOT EXISTS views INTEGER NOT NULL DEFAULT 0;
