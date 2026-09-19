-- Нейросеть может предложить автору подробную версию рецепта перед публикацией (status = improve).
ALTER TABLE user_recipes ADD COLUMN IF NOT EXISTS suggestion JSONB;
