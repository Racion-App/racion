-- Ник, лайки, избранное и комментарии к рецептам.
ALTER TABLE users ADD COLUMN IF NOT EXISTS nick TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS users_nick ON users (lower(nick)) WHERE nick IS NOT NULL;

CREATE TABLE IF NOT EXISTS recipe_likes (
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipe_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, recipe_id)
);
CREATE INDEX IF NOT EXISTS recipe_likes_recipe ON recipe_likes (recipe_id);

CREATE TABLE IF NOT EXISTS recipe_favorites (
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipe_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, recipe_id)
);

CREATE TABLE IF NOT EXISTS recipe_comments (
    id         BIGSERIAL PRIMARY KEY,
    recipe_id  TEXT NOT NULL,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS recipe_comments_recipe ON recipe_comments (recipe_id, created_at DESC);

-- Свой рецепт можно открыть для всех по ссылке; автор виден по нику.
ALTER TABLE user_recipes ADD COLUMN IF NOT EXISTS public BOOLEAN NOT NULL DEFAULT false;
