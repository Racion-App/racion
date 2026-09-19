-- Обратная связь после ужина: «понравилось» подкрепляет блюдо для этой семьи, «не зашло» уводит его.
-- meh — сколько раз ответили «не зашло»; после второго раза блюдо больше не предлагается.
CREATE TABLE IF NOT EXISTS recipe_feedback (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipe_id  TEXT NOT NULL,
    liked      BOOLEAN NOT NULL,
    meh        INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, recipe_id)
);
