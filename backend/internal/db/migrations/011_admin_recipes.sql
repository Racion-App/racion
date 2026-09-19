-- Роли и редактирование рецептов из админки, модерация своих рецептов.
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT '';        -- '' | moderator | admin

-- Правки админа не должны затираться сидингом при рестарте: seed пропускает строки с edited_at.
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS edited_at TIMESTAMPTZ;
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS deleted BOOLEAN NOT NULL DEFAULT false;

-- Модерация: private (только автору) → checking (нейросеть) → approved | review (вручную) → approved | rejected.
ALTER TABLE user_recipes ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'private';
ALTER TABLE user_recipes ADD COLUMN IF NOT EXISTS review_note TEXT NOT NULL DEFAULT '';
ALTER TABLE user_recipes ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;
ALTER TABLE user_recipes ADD COLUMN IF NOT EXISTS checked_at TIMESTAMPTZ;
UPDATE user_recipes SET status = 'approved' WHERE public AND status = 'private';
CREATE INDEX IF NOT EXISTS user_recipes_status ON user_recipes (status, submitted_at);
