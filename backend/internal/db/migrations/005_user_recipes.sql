-- Свои рецепты пользователей: видны только владельцу, попадают в его недели.
-- Продукты — из общей базы (id + количество), поэтому калории и цена считаются как у обычных рецептов.
CREATE TABLE IF NOT EXISTS user_recipes (
    id          TEXT PRIMARY KEY,                 -- u_<uuid>, чтобы не пересекаться с базовыми id
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    slot        TEXT NOT NULL,                    -- breakfast | lunch | dinner | snack
    time_min    INT NOT NULL,
    equipment   TEXT[] NOT NULL DEFAULT '{}',
    tags        TEXT[] NOT NULL DEFAULT '{}',
    steps       TEXT[] NOT NULL DEFAULT '{}',
    ingredients JSONB NOT NULL DEFAULT '[]',      -- [{"ingredientId": "...", "amount": 150}]
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS user_recipes_user ON user_recipes (user_id, created_at DESC);
