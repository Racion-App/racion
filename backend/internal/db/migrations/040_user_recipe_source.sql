-- Свой рецепт, взятый по ссылке с чужого сайта, помнит адрес страницы: на странице рецепта он показывается
-- ссылкой на источник. Пусто — рецепт написан самим человеком.
ALTER TABLE user_recipes ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '';
