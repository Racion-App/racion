-- Личный кабинет: пользователи, сессии, привязка планов, отметки покупок, свои товары, нелюбимые рецепты.

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    defaults      JSONB NOT NULL DEFAULT '{}',   -- последние ответы квиза, чтобы не заполнять заново
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_user ON sessions (user_id);

ALTER TABLE plans ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS plans_user ON plans (user_id, created_at DESC);

-- Отметки «куплено» по плану: одна строка на позицию списка.
CREATE TABLE IF NOT EXISTS plan_checks (
    plan_id    UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    item_id    TEXT NOT NULL,     -- ingredientId или extra:<id>
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (plan_id, item_id)
);

-- Свои товары в списке покупок плана.
CREATE TABLE IF NOT EXISTS plan_extras (
    id         BIGSERIAL PRIMARY KEY,
    plan_id    UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    qty        TEXT NOT NULL DEFAULT '',
    due        DATE,
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS plan_extras_plan ON plan_extras (plan_id);

-- История покупок пользователя: что и когда отметили купленным.
CREATE TABLE IF NOT EXISTS purchases (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id    UUID REFERENCES plans(id) ON DELETE SET NULL,
    item_id    TEXT NOT NULL,
    name       TEXT NOT NULL,
    qty        TEXT NOT NULL DEFAULT '',
    cost       NUMERIC(10,2) NOT NULL DEFAULT 0,
    bought_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS purchases_user_date ON purchases (user_id, bought_at DESC);

-- Рецепты, которые пользователь больше не хочет видеть.
CREATE TABLE IF NOT EXISTS user_dislikes (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipe_id  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, recipe_id)
);

-- Картинка и описание рецепта для страниц.
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS image TEXT NOT NULL DEFAULT '';
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
