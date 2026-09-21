-- Оценки рецептов 1–5: голосуют и гости (voter = cookie racion_voter), и пользователи (voter = user id).
-- Средняя и число оценок уходят в разметку Recipe (aggregateRating) от трёх голосов.
CREATE TABLE IF NOT EXISTS recipe_ratings (
    recipe_id  TEXT NOT NULL,
    voter      TEXT NOT NULL,
    stars      SMALLINT NOT NULL CHECK (stars BETWEEN 1 AND 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (recipe_id, voter)
);
CREATE INDEX IF NOT EXISTS recipe_ratings_recipe ON recipe_ratings (recipe_id);
