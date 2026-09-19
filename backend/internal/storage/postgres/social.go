package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// ── Лайки, избранное, комментарии ──────────────────────────────────────────

type Social struct{ pool *pgxpool.Pool }

func (r *Social) Like(ctx context.Context, userID, recipeID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO recipe_likes (user_id, recipe_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, recipeID)
	return wrap("likes.add", err)
}

func (r *Social) Unlike(ctx context.Context, userID, recipeID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM recipe_likes WHERE user_id = $1 AND recipe_id = $2`, userID, recipeID)
	return wrap("likes.remove", err)
}

func (r *Social) Favorite(ctx context.Context, userID, recipeID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO recipe_favorites (user_id, recipe_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, recipeID)
	return wrap("favorites.add", err)
}

func (r *Social) Unfavorite(ctx context.Context, userID, recipeID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM recipe_favorites WHERE user_id = $1 AND recipe_id = $2`, userID, recipeID)
	return wrap("favorites.remove", err)
}

// SetFeedback — ответ «как было?»: liked=true снимает счётчик «не зашло», liked=false его увеличивает.
func (r *Social) SetFeedback(ctx context.Context, userID, recipeID string, liked bool) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO recipe_feedback (user_id, recipe_id, liked, meh) VALUES ($1, $2, $3, CASE WHEN $3 THEN 0 ELSE 1 END)
		ON CONFLICT (user_id, recipe_id) DO UPDATE SET liked = EXCLUDED.liked, meh = CASE WHEN EXCLUDED.liked THEN 0 ELSE recipe_feedback.meh + 1 END, updated_at = now()`,
		userID, recipeID, liked)
	return wrap("feedback.set", err)
}

// Feedback — вкусы пользователя: id → liked, id → сколько раз «не зашло».
func (r *Social) Feedback(ctx context.Context, userID string) (map[string]bool, map[string]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT recipe_id, liked, meh FROM recipe_feedback WHERE user_id = $1`, userID)
	if err != nil {
		return nil, nil, wrap("feedback.list", err)
	}
	defer rows.Close()
	liked, meh := map[string]bool{}, map[string]int{}
	for rows.Next() {
		var id string
		var l bool
		var m int
		if err := rows.Scan(&id, &l, &m); err != nil {
			return nil, nil, wrap("feedback.list", err)
		}
		liked[id] = l
		meh[id] = m
	}
	return liked, meh, wrap("feedback.list", rows.Err())
}

// Favorites — id избранных рецептов пользователя, свежие сверху.
func (r *Social) Favorites(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT recipe_id FROM recipe_favorites WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, wrap("favorites.list", err)
	}
	ids, err := scanStrings(rows)
	return ids, wrap("favorites.list", err)
}

// Stats — счётчики рецепта и отметки текущего пользователя (userID может быть пустым).
func (r *Social) Stats(ctx context.Context, recipeID, userID string) (domain.RecipeStats, error) {
	var s domain.RecipeStats
	err := r.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM recipe_likes WHERE recipe_id = $1),
		(SELECT count(*) FROM recipe_comments WHERE recipe_id = $1),
		(SELECT count(*) FROM recipe_favorites WHERE recipe_id = $1),
		$2 <> '' AND EXISTS (SELECT 1 FROM recipe_likes WHERE recipe_id = $1 AND user_id::text = $2),
		$2 <> '' AND EXISTS (SELECT 1 FROM recipe_favorites WHERE recipe_id = $1 AND user_id::text = $2)`, recipeID, userID).
		Scan(&s.Likes, &s.Comments, &s.Favorites, &s.Liked, &s.Favorite)
	return s, wrap("social.stats", err)
}

// LikeCounts — лайки для набора рецептов (карточки каталога).
func (r *Social) LikeCounts(ctx context.Context) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT recipe_id, count(*) FROM recipe_likes GROUP BY recipe_id`)
	if err != nil {
		return nil, wrap("likes.counts", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, wrap("likes.counts", err)
		}
		out[id] = n
	}
	return out, wrap("likes.counts", rows.Err())
}

func (r *Social) Comments(ctx context.Context, recipeID string, limit int) ([]domain.Comment, error) {
	rows, err := r.pool.Query(ctx, `SELECT c.id, c.user_id, COALESCE(u.nick, ''), COALESCE(NULLIF(u.name, ''), split_part(u.email, '@', 1)), c.body, c.created_at, c.image, u.avatar
		FROM recipe_comments c JOIN users u ON u.id = c.user_id WHERE c.recipe_id = $1 ORDER BY c.created_at DESC LIMIT $2`, recipeID, limit)
	if err != nil {
		return nil, wrap("comments.list", err)
	}
	defer rows.Close()
	out := []domain.Comment{}
	for rows.Next() {
		var c domain.Comment
		var t time.Time
		if err := rows.Scan(&c.ID, &c.UserID, &c.Nick, &c.Name, &c.Body, &t, &c.Image, &c.Avatar); err != nil {
			return nil, wrap("comments.list", err)
		}
		c.CreatedAt = t.UTC().Format(time.RFC3339)
		out = append(out, c)
	}
	return out, wrap("comments.list", rows.Err())
}

func (r *Social) AddComment(ctx context.Context, recipeID, userID, body, image string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `INSERT INTO recipe_comments (recipe_id, user_id, body, image) VALUES ($1, $2, $3, $4) RETURNING id`, recipeID, userID, body, image).Scan(&id)
	return id, wrap("comments.add", err)
}

// DeleteComment удаляет свой комментарий.
func (r *Social) DeleteComment(ctx context.Context, id int64, userID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM recipe_comments WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return wrap("comments.delete", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CountRecent — сколько комментариев пользователь оставил за последний час (антиспам).
func (r *Social) CountRecent(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM recipe_comments WHERE user_id = $1 AND created_at > now() - interval '1 hour'`, userID).Scan(&n)
	return n, wrap("comments.recent", err)
}
