package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// Коллекции рецептов: папки пользователя («на дачу», «когда болеем») с рецептами базы и своими.
type Collections struct{ pool *pgxpool.Pool }

func (r *Collections) List(ctx context.Context, userID string) ([]domain.Collection, error) {
	return r.query(ctx, `WHERE c.user_id = $1 AND NOT c.curated`, userID)
}

const collCols = `c.id, c.name, c.created_at, c.public, c.curated, COALESCE(c.slug, ''), c.description, c.cover, COALESCE(u.nick, ''), c.name_i18n, c.description_i18n,
	COALESCE(array_agg(i.recipe_id ORDER BY i.added_at) FILTER (WHERE i.recipe_id IS NOT NULL), '{}')`

func (r *Collections) query(ctx context.Context, where string, args ...any) ([]domain.Collection, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+collCols+` FROM collections c JOIN users u ON u.id = c.user_id LEFT JOIN collection_items i ON i.collection_id = c.id `+where+` GROUP BY c.id, u.nick ORDER BY c.curated DESC, c.created_at`, args...)
	if err != nil {
		return nil, wrap("collections.list", err)
	}
	defer rows.Close()
	out := []domain.Collection{}
	for rows.Next() {
		var c domain.Collection
		var t time.Time
		if err := rows.Scan(&c.ID, &c.Name, &t, &c.Public, &c.Curated, &c.Slug, &c.Description, &c.Cover, &c.Author, &c.Names, &c.Descriptions, &c.Recipes); err != nil {
			return nil, wrap("collections.list", err)
		}
		c.CreatedAt = t.UTC().Format(time.RFC3339)
		out = append(out, c)
	}
	return out, wrap("collections.list", rows.Err())
}

// Public — открытые подборки; curatedOnly — только редакционные (для каталога и sitemap).
func (r *Collections) Public(ctx context.Context, curatedOnly bool) ([]domain.Collection, error) {
	if curatedOnly {
		return r.query(ctx, `WHERE c.public AND c.curated`)
	}
	return r.query(ctx, `WHERE c.public`)
}

// BySlug — публичная подборка по адресу.
func (r *Collections) BySlug(ctx context.Context, slug string) (domain.Collection, error) {
	list, err := r.query(ctx, `WHERE c.public AND c.slug = $1`, slug)
	if err != nil {
		return domain.Collection{}, err
	}
	if len(list) == 0 {
		return domain.Collection{}, domain.ErrNotFound
	}
	return list[0], nil
}

// Publish — открыть/закрыть подборку; slug задаётся при первой публикации.
func (r *Collections) Publish(ctx context.Context, userID, id string, public bool, slug string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE collections SET public = $3, slug = COALESCE(slug, NULLIF($4, '')) WHERE id = $1 AND user_id = $2`, id, userID, public, slug)
	if err != nil {
		return wrap("collections.publish", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Curate — редакционная подборка (админ): описание, обложка, слаг, публичность и флаг curated.
func (r *Collections) Curate(ctx context.Context, id string, name, description, cover, slug string, public bool, names, descriptions map[string]string) error {
	if names == nil {
		names = map[string]string{}
	}
	if descriptions == nil {
		descriptions = map[string]string{}
	}
	tag, err := r.pool.Exec(ctx, `UPDATE collections SET name = $2, description = $3, cover = $4, slug = NULLIF($5, ''), public = $6, curated = true, name_i18n = $7, description_i18n = $8 WHERE id = $1`, id, name, description, cover, slug, public, names, descriptions)
	if err != nil {
		return wrap("collections.curate", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// AdminList — все редакционные подборки (и черновики) для админки.
func (r *Collections) AdminList(ctx context.Context) ([]domain.Collection, error) {
	return r.query(ctx, `WHERE c.curated`)
}

// Get — подборка по id без проверки владельца (админка и публичный доступ решаются выше).
func (r *Collections) Get(ctx context.Context, id string) (domain.Collection, error) {
	list, err := r.query(ctx, `WHERE c.id = $1`, id)
	if err != nil {
		return domain.Collection{}, err
	}
	if len(list) == 0 {
		return domain.Collection{}, domain.ErrNotFound
	}
	return list[0], nil
}

// SetItems — заменить состав (админка): порядок сохраняется через added_at.
func (r *Collections) SetItems(ctx context.Context, id string, recipes []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return wrap("collections.items", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM collection_items WHERE collection_id = $1`, id); err != nil {
		return wrap("collections.items", err)
	}
	for n, rid := range recipes {
		if _, err := tx.Exec(ctx, `INSERT INTO collection_items (collection_id, recipe_id, added_at) VALUES ($1, $2, now() + make_interval(secs => $3)) ON CONFLICT DO NOTHING`, id, rid, n); err != nil {
			return wrap("collections.items", err)
		}
	}
	return wrap("collections.items", tx.Commit(ctx))
}

// AdminDelete — удалить подборку без проверки владельца.
func (r *Collections) AdminDelete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM collections WHERE id = $1`, id)
	return wrap("collections.delete", err)
}

func (r *Collections) Create(ctx context.Context, userID, name string) (domain.Collection, error) {
	var c domain.Collection
	var t time.Time
	err := r.pool.QueryRow(ctx, `INSERT INTO collections (user_id, name) VALUES ($1, $2) RETURNING id, name, created_at`, userID, name).Scan(&c.ID, &c.Name, &t)
	c.CreatedAt = t.UTC().Format(time.RFC3339)
	c.Recipes = []string{}
	return c, wrap("collections.create", err)
}

func (r *Collections) Rename(ctx context.Context, userID, id, name string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE collections SET name = $3 WHERE id = $1 AND user_id = $2`, id, userID, name)
	if err != nil {
		return wrap("collections.rename", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Collections) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM collections WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return wrap("collections.delete", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Toggle — добавить или убрать рецепт; коллекция должна быть своей.
func (r *Collections) Toggle(ctx context.Context, userID, id, recipeID string, on bool) error {
	var owner string
	if err := r.pool.QueryRow(ctx, `SELECT user_id FROM collections WHERE id = $1`, id).Scan(&owner); err != nil {
		return wrap("collections.toggle", err)
	}
	if owner != userID {
		return domain.ErrForbidden
	}
	var err error
	if on {
		_, err = r.pool.Exec(ctx, `INSERT INTO collection_items (collection_id, recipe_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, recipeID)
	} else {
		_, err = r.pool.Exec(ctx, `DELETE FROM collection_items WHERE collection_id = $1 AND recipe_id = $2`, id, recipeID)
	}
	return wrap("collections.toggle", err)
}

// Recipes — id рецептов коллекции, если она своя или публичная.
func (r *Collections) Recipes(ctx context.Context, userID, id string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT i.recipe_id FROM collection_items i JOIN collections c ON c.id = i.collection_id WHERE c.id = $1 AND (c.user_id = $2 OR c.public) ORDER BY i.added_at`, id, userID)
	if err != nil {
		return nil, wrap("collections.recipes", err)
	}
	ids, err := scanStrings(rows)
	return ids, wrap("collections.recipes", err)
}
