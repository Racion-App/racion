-- Очередь переводов теперь и для рецептов базы (recipes), поэтому внешний ключ на user_recipes снят.
ALTER TABLE recipe_translations DROP CONSTRAINT IF EXISTS recipe_translations_recipe_id_fkey;
