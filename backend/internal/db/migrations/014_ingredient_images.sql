-- Картинка продукта для подсказки при наведении (frontend/public/images/ingredients/<id>.webp).
ALTER TABLE ingredients ADD COLUMN IF NOT EXISTS image TEXT NOT NULL DEFAULT '';
