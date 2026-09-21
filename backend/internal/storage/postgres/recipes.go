package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
	"racion/internal/planner"
)

// ── Свои рецепты ───────────────────────────────────────────────────────────

type UserRecipes struct{ pool *pgxpool.Pool }

const ownRecipeCols = `id, title, description, slot, time_min, equipment, tags, steps, ingredients, public, user_id, image, status, review_note, submitted_at, suggestion, views, lang, i18n`

func scanOwn(row pgx.Row) (planner.Recipe, error) {
	var r planner.Recipe
	var submitted *time.Time
	var suggestion, i18nRaw []byte
	err := row.Scan(&r.ID, &r.Title, &r.Description, &r.Slot, &r.TimeMin, &r.Equipment, &r.Tags, &r.Steps, &r.Ingredients, &r.Public, &r.OwnerID, &r.Image, &r.Status, &r.Note, &submitted, &suggestion, &r.Views, &r.Lang, &i18nRaw)
	if len(i18nRaw) > 2 {
		_ = json.Unmarshal(i18nRaw, &r.I18n)
	}
	if len(suggestion) > 0 {
		var sg planner.RecipeText
		if json.Unmarshal(suggestion, &sg) == nil && sg.Title != "" {
			r.Suggestion = &sg
		}
	}
	if submitted != nil {
		r.SubmittedAt = submitted.UTC().Format(time.RFC3339)
	}
	r.Own = true
	return r, err
}

// ByUser — рецепты пользователя, новые сверху.
func (r *UserRecipes) ByUser(ctx context.Context, userID string) ([]planner.Recipe, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+ownRecipeCols+` FROM user_recipes WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, wrap("user_recipes.by_user", err)
	}
	defer rows.Close()
	var out []planner.Recipe
	for rows.Next() {
		rc, err := scanOwn(rows)
		if err != nil {
			return nil, wrap("user_recipes.by_user", err)
		}
		out = append(out, rc)
	}
	return out, wrap("user_recipes.by_user", rows.Err())
}

// SetI18n — записать перевод на язык (jsonb-слияние).
func (r *UserRecipes) SetI18n(ctx context.Context, id, lang string, text planner.RecipeText) error {
	raw, _ := json.Marshal(map[string]planner.RecipeText{lang: text})
	_, err := r.pool.Exec(ctx, `UPDATE user_recipes SET i18n = i18n || $2::jsonb WHERE id = $1`, id, raw)
	return wrap("user_recipes.i18n", err)
}

// SetI18n — перевод рецепта базы (загруженного через API: в seed его нет, переводы живут только в базе).
func (r *CatalogRecipes) SetI18n(ctx context.Context, id, lang string, text planner.RecipeText) error {
	raw, _ := json.Marshal(map[string]planner.RecipeText{lang: text})
	_, err := r.pool.Exec(ctx, `UPDATE recipes SET i18n = i18n || $2::jsonb WHERE id = $1`, id, raw)
	return wrap("recipes.i18n", err)
}

// ClearI18n — стереть переводы (текст рецепта изменился).
func (r *UserRecipes) ClearI18n(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE user_recipes SET i18n = '{}'::jsonb WHERE id = $1`, id)
	return wrap("user_recipes.i18n", err)
}

// SetLang — язык оригинала (с какого переводить).
func (r *UserRecipes) SetLang(ctx context.Context, id, lang string) error {
	_, err := r.pool.Exec(ctx, `UPDATE user_recipes SET lang = $2 WHERE id = $1`, id, lang)
	return wrap("user_recipes.lang", err)
}

// AddView — плюс один просмотр страницы (только чужие: автор себя не считает).
func (r *UserRecipes) AddView(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE user_recipes SET views = views + 1 WHERE id = $1`, id)
	return wrap("user_recipes.view", err)
}

func (r *UserRecipes) Get(ctx context.Context, id string) (planner.Recipe, error) {
	rc, err := scanOwn(r.pool.QueryRow(ctx, `SELECT `+ownRecipeCols+` FROM user_recipes WHERE id = $1`, id))
	return rc, wrap("user_recipes.get", err)
}

func (r *UserRecipes) Count(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM user_recipes WHERE user_id = $1`, userID).Scan(&n)
	return n, wrap("user_recipes.count", err)
}

func (r *UserRecipes) Insert(ctx context.Context, userID string, rc planner.Recipe) error {
	if rc.Lang == "" {
		rc.Lang = "ru"
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO user_recipes (id, user_id, title, description, slot, time_min, equipment, tags, steps, ingredients, image, lang) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		rc.ID, userID, rc.Title, rc.Description, rc.Slot, rc.TimeMin, rc.Equipment, rc.Tags, rc.Steps, rc.Ingredients, rc.Image, rc.Lang)
	return wrap("user_recipes.insert", err)
}

func (r *UserRecipes) Update(ctx context.Context, userID string, rc planner.Recipe) error {
	tag, err := r.pool.Exec(ctx, `UPDATE user_recipes SET title = $3, description = $4, slot = $5, time_min = $6, equipment = $7, tags = $8, steps = $9, ingredients = $10, image = $11, lang = COALESCE(NULLIF($12, ''), lang), updated_at = now() WHERE id = $1 AND user_id = $2`,
		rc.ID, userID, rc.Title, rc.Description, rc.Slot, rc.TimeMin, rc.Equipment, rc.Tags, rc.Steps, rc.Ingredients, rc.Image, rc.Lang)
	if err != nil {
		return wrap("user_recipes.update", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetPublic открывает или закрывает свой рецепт для всех по ссылке.
func (r *UserRecipes) SetPublic(ctx context.Context, userID, id string, public bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE user_recipes SET public = $3, status = CASE WHEN $3 THEN 'approved' ELSE 'private' END WHERE id = $1 AND user_id = $2`, id, userID, public)
	if err != nil {
		return wrap("user_recipes.public", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ── Модерация ──────────────────────────────────────────────────────────────

// SetStatus — статус и заметка; approved одновременно открывает рецепт (public).
func (r *UserRecipes) SetStatus(ctx context.Context, id, status, note string, public bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE user_recipes SET status = $2, review_note = $3, public = $4,
		submitted_at = CASE WHEN $2 = 'checking' OR ($2 = 'review' AND submitted_at IS NULL) THEN now() ELSE submitted_at END,
		checked_at = CASE WHEN $2 IN ('approved', 'rejected') THEN now() ELSE checked_at END WHERE id = $1`, id, status, note, public)
	if err != nil {
		return wrap("user_recipes.status", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetSuggestion — статус improve с предложенным текстом; автор примет или откажется.
func (r *UserRecipes) SetSuggestion(ctx context.Context, id, note string, suggestion any) error {
	raw, _ := json.Marshal(suggestion)
	_, err := r.pool.Exec(ctx, `UPDATE user_recipes SET status = 'improve', review_note = $2, suggestion = $3, public = false WHERE id = $1`, id, note, raw)
	return wrap("user_recipes.suggestion", err)
}

// Suggestion — предложенный текст (nil, если нет).
func (r *UserRecipes) Suggestion(ctx context.Context, id string) ([]byte, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT suggestion FROM user_recipes WHERE id = $1`, id).Scan(&raw)
	return raw, wrap("user_recipes.suggestion", err)
}

// ApplyText — заменить название, подводку и шаги (принятое предложение нейросети).
func (r *UserRecipes) ApplyText(ctx context.Context, id, title, description string, steps []string) error {
	_, err := r.pool.Exec(ctx, `UPDATE user_recipes SET title = $2, description = $3, steps = $4, suggestion = NULL, updated_at = now() WHERE id = $1`, id, title, description, steps)
	return wrap("user_recipes.apply", err)
}

// Queue — рецепты на ручной проверке (и зависшие на автопроверке дольше часа), старые сверху.
func (r *UserRecipes) Queue(ctx context.Context) ([]planner.Recipe, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+ownRecipeCols+` FROM user_recipes WHERE status = 'review' OR (status = 'checking' AND submitted_at < now() - interval '1 hour')
		ORDER BY submitted_at NULLS FIRST LIMIT 200`)
	if err != nil {
		return nil, wrap("user_recipes.queue", err)
	}
	defer rows.Close()
	var out []planner.Recipe
	for rows.Next() {
		rc, err := scanOwn(rows)
		if err != nil {
			return nil, wrap("user_recipes.queue", err)
		}
		out = append(out, rc)
	}
	return out, wrap("user_recipes.queue", rows.Err())
}

// Approved — опубликованные, свежие сверху.
func (r *UserRecipes) Approved(ctx context.Context, limit int) ([]planner.Recipe, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+ownRecipeCols+` FROM user_recipes WHERE status = 'approved' ORDER BY checked_at DESC NULLS LAST LIMIT $1`, limit)
	if err != nil {
		return nil, wrap("user_recipes.approved", err)
	}
	defer rows.Close()
	var out []planner.Recipe
	for rows.Next() {
		rc, err := scanOwn(rows)
		if err != nil {
			return nil, wrap("user_recipes.approved", err)
		}
		out = append(out, rc)
	}
	return out, wrap("user_recipes.approved", rows.Err())
}

// ── Рецепты базы из админки ───────────────────────────────────────────────

type CatalogRecipes struct{ pool *pgxpool.Pool }

// Save — вставка или правка с отметкой edited_at; продукты перезаписываются.
func (r *CatalogRecipes) Save(ctx context.Context, rc planner.Recipe, isNew bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return wrap("catalog.save", err)
	}
	defer tx.Rollback(ctx)
	if isNew {
		if _, err := tx.Exec(ctx, `INSERT INTO recipes (id, title, slot, time_min, equipment, tags, batch, steps, image, description, i18n, edited_at, keep_days, can_freeze, hidden) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'{}', now(), $11, $12, $13)`,
			rc.ID, rc.Title, rc.Slot, rc.TimeMin, rc.Equipment, rc.Tags, rc.Batch, rc.Steps, rc.Image, rc.Description, rc.KeepDays, rc.Freeze, rc.Hidden); err != nil {
			return wrap("catalog.save", err)
		}
	} else if _, err := tx.Exec(ctx, `UPDATE recipes SET title=$2, slot=$3, time_min=$4, equipment=$5, tags=$6, batch=$7, steps=$8, image=$9, description=$10, edited_at=now(), deleted=false, keep_days=$11, can_freeze=$12, hidden=$13 WHERE id=$1`,
		rc.ID, rc.Title, rc.Slot, rc.TimeMin, rc.Equipment, rc.Tags, rc.Batch, rc.Steps, rc.Image, rc.Description, rc.KeepDays, rc.Freeze, rc.Hidden); err != nil {
		return wrap("catalog.save", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM recipe_ingredients WHERE recipe_id = $1`, rc.ID); err != nil {
		return wrap("catalog.save", err)
	}
	for n, ri := range rc.Ingredients {
		if _, err := tx.Exec(ctx, `INSERT INTO recipe_ingredients (recipe_id, ingredient_id, amount, sort) VALUES ($1,$2,$3,$4)`, rc.ID, ri.IngredientID, ri.Amount, n); err != nil {
			return wrap("catalog.save", err)
		}
	}
	return wrap("catalog.save", tx.Commit(ctx))
}

// SoftDelete — рецепт исчезает из каталога, но остаётся в старых планах и не возвращается сидингом.
func (r *CatalogRecipes) SoftDelete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE recipes SET deleted = true, edited_at = now() WHERE id = $1`, id)
	return wrap("catalog.delete", err)
}

func (r *UserRecipes) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM user_recipes WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return wrap("user_recipes.delete", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ── Аналитика ──────────────────────────────────────────────────────────────

type Events struct{ pool *pgxpool.Pool }

func (r *Events) AddBatch(ctx context.Context, sid string, events []domain.Event) error {
	batch := &pgx.Batch{}
	for _, e := range events {
		batch.Queue(`INSERT INTO events (sid, name, props, ts) VALUES ($1, $2, $3, $4)`, sid, e.Name, e.Props, e.At)
	}
	return wrap("events.add", r.pool.SendBatch(ctx, batch).Close())
}
