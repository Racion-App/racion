package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// Очередь переводов своих рецептов: по строке на язык.
type Translations struct{ pool *pgxpool.Pool }

func (r *Translations) Enqueue(ctx context.Context, recipeID string, langs []string) error {
	for _, l := range langs {
		if _, err := r.pool.Exec(ctx, `INSERT INTO recipe_translations (recipe_id, lang, status, model, error, updated_at) VALUES ($1, $2, 'queued', '', '', now())
			ON CONFLICT (recipe_id, lang) DO UPDATE SET status = 'queued', model = '', error = '', updated_at = now() WHERE recipe_translations.status <> 'done' OR EXCLUDED.status = 'queued'`, recipeID, l); err != nil {
			return wrap("translations.enqueue", err)
		}
	}
	return nil
}

// Next — самая старая задача в очереди.
func (r *Translations) Next(ctx context.Context) (string, string, bool, error) {
	var id, lang string
	err := r.pool.QueryRow(ctx, `SELECT recipe_id, lang FROM recipe_translations WHERE status = 'queued' ORDER BY updated_at LIMIT 1`).Scan(&id, &lang)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, wrap("translations.next", err)
	}
	return id, lang, true, nil
}

func (r *Translations) SetStatus(ctx context.Context, recipeID, lang, status, model, errText string) error {
	_, err := r.pool.Exec(ctx, `UPDATE recipe_translations SET status = $3, model = CASE WHEN $4 = '' THEN model ELSE $4 END, error = $5, updated_at = now() WHERE recipe_id = $1 AND lang = $2`, recipeID, lang, status, model, errText)
	return wrap("translations.status", err)
}

func (r *Translations) ByRecipe(ctx context.Context, recipeID string) ([]domain.Translation, error) {
	rows, err := r.pool.Query(ctx, `SELECT lang, status, model, error, updated_at FROM recipe_translations WHERE recipe_id = $1 ORDER BY lang`, recipeID)
	if err != nil {
		return nil, wrap("translations.list", err)
	}
	defer rows.Close()
	out := []domain.Translation{}
	for rows.Next() {
		var t domain.Translation
		var at time.Time
		if err := rows.Scan(&t.Lang, &t.Status, &t.Model, &t.Error, &at); err != nil {
			return nil, wrap("translations.list", err)
		}
		t.UpdatedAt = at.UTC().Format(time.RFC3339)
		out = append(out, t)
	}
	return out, wrap("translations.list", rows.Err())
}

// Summary — сводка по рецептам: сколько готово, есть ли ошибки, какой язык переводится сейчас.
func (r *Translations) Summary(ctx context.Context, ids []string) (map[string]domain.TranslationSummary, error) {
	out := map[string]domain.TranslationSummary{}
	rows, err := r.pool.Query(ctx, `SELECT recipe_id, lang, status, model FROM recipe_translations WHERE recipe_id = ANY($1) ORDER BY updated_at`, ids)
	if err != nil {
		return nil, wrap("translations.summary", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, lang, status, model string
		if err := rows.Scan(&id, &lang, &status, &model); err != nil {
			return nil, wrap("translations.summary", err)
		}
		s := out[id]
		s.Total++
		switch status {
		case "done":
			s.Done++
		case "error":
			s.Errors++
		case "running":
			s.Running = lang
		}
		if model != "" {
			s.Model = model
		}
		out[id] = s
	}
	return out, wrap("translations.summary", rows.Err())
}

// ResetStale — «running» дольше olderThan (сервер перезапустили) — обратно в очередь.
func (r *Translations) ResetStale(ctx context.Context, olderThan time.Duration) error {
	_, err := r.pool.Exec(ctx, `UPDATE recipe_translations SET status = 'queued', updated_at = now() WHERE status = 'running' AND updated_at < now() - $1::interval`, olderThan.String())
	return wrap("translations.reset", err)
}
