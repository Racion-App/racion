-- Доступность продукта по магазинам: '' — везде, 'super' — супермаркеты и крупнее, 'premium' — гипермаркеты, оптовики и премиум-сети.
ALTER TABLE ingredients ADD COLUMN IF NOT EXISTS tier text NOT NULL DEFAULT '';
