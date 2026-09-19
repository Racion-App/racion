package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
	"racion/internal/planner"
)

// ── Планы ──────────────────────────────────────────────────────────────────

type Plans struct{ pool *pgxpool.Pool }

// Insert сохраняет план и возвращает id; план перезаписывается уже с id внутри.
func (r *Plans) Insert(ctx context.Context, plan planner.Plan, ownerID *string) (string, error) {
	params, _ := json.Marshal(plan.Params)
	body, _ := json.Marshal(plan)
	var id string
	if err := r.pool.QueryRow(ctx, `INSERT INTO plans (params, plan, seed, user_id) VALUES ($1, $2, $3, $4) RETURNING id`, params, body, plan.Seed, ownerID).Scan(&id); err != nil {
		return "", wrap("plans.insert", err)
	}
	plan.ID = id
	body, _ = json.Marshal(plan)
	_, err := r.pool.Exec(ctx, `UPDATE plans SET plan = $2 WHERE id = $1`, id, body)
	return id, wrap("plans.insert", err)
}

func (r *Plans) Get(ctx context.Context, id string) (domain.PlanRecord, error) {
	var raw []byte
	rec := domain.PlanRecord{ID: id}
	if err := r.pool.QueryRow(ctx, `SELECT plan, user_id FROM plans WHERE id = $1`, id).Scan(&raw, &rec.OwnerID); err != nil {
		return rec, wrap("plans.get", err)
	}
	if err := json.Unmarshal(raw, &rec.Plan); err != nil {
		return rec, domain.Internal("plans.get", err)
	}
	rec.Plan.ID = id
	return rec, nil
}

func (r *Plans) OwnerID(ctx context.Context, id string) (*string, error) {
	var owner *string
	err := r.pool.QueryRow(ctx, `SELECT user_id FROM plans WHERE id = $1`, id).Scan(&owner)
	return owner, wrap("plans.owner", err)
}

func (r *Plans) Save(ctx context.Context, plan planner.Plan) error {
	body, _ := json.Marshal(plan)
	_, err := r.pool.Exec(ctx, `UPDATE plans SET plan = $2, updated_at = now() WHERE id = $1`, plan.ID, body)
	return wrap("plans.save", err)
}

// Claim привязывает гостевой план к аккаунту; чужой план не трогает.
func (r *Plans) Claim(ctx context.Context, id, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE plans SET user_id = $2 WHERE id = $1 AND user_id IS NULL`, id, userID)
	return wrap("plans.claim", err)
}

func (r *Plans) Rename(ctx context.Context, id, userID, title string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE plans SET title = $3 WHERE id = $1 AND user_id = $2`, id, userID, title)
	if err != nil {
		return wrap("plans.rename", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Plans) Delete(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM plans WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return wrap("plans.delete", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ByUser — недели кабинета, новые сверху, с числом отмеченных позиций.
func (r *Plans) ByUser(ctx context.Context, userID string) ([]domain.PlanSummary, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.id, p.title, p.plan, p.created_at, (SELECT count(*) FROM plan_checks c WHERE c.plan_id = p.id), p.user_id IS DISTINCT FROM $1
		FROM plans p WHERE p.user_id = $1 OR p.id IN (SELECT plan_id FROM plan_members WHERE user_id = $1)
		OR p.user_id IN (SELECT y.user_id FROM household_users x JOIN household_users y ON x.household_id = y.household_id WHERE x.user_id = $1)
		ORDER BY p.created_at DESC LIMIT 50`, userID)
	if err != nil {
		return nil, wrap("plans.by_user", err)
	}
	defer rows.Close()
	out := []domain.PlanSummary{}
	for rows.Next() {
		var id, title string
		var raw []byte
		var created time.Time
		var checked int
		var shared bool
		if err := rows.Scan(&id, &title, &raw, &created, &checked, &shared); err != nil {
			return nil, wrap("plans.by_user", err)
		}
		var p planner.Plan
		_ = json.Unmarshal(raw, &p)
		ps := domain.PlanSummary{
			ID: id, Title: title, StartDate: p.Params.StartDate, Store: p.Store.Name, Cost: p.Totals.Cost, Country: planner.CountryOf(p.Params.Country), Portions: p.Portions,
			CreatedAt: created.UTC().Format(time.RFC3339), Checked: checked, Items: p.Totals.Items, Shared: shared, Occasion: p.Occasion,
		}
		if p.Occasion != nil && len(p.Days) > 0 {
			ps.Date = p.Days[0].Date
		}
		out = append(out, ps)
	}
	return out, wrap("plans.by_user", rows.Err())
}

// ── Отметки «куплено» ──────────────────────────────────────────────────────

type Checks struct{ pool *pgxpool.Pool }

func (r *Checks) List(ctx context.Context, planID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT item_id FROM plan_checks WHERE plan_id = $1`, planID)
	if err != nil {
		return nil, wrap("checks.list", err)
	}
	ids, err := scanStrings(rows)
	return ids, wrap("checks.list", err)
}

func (r *Checks) Set(ctx context.Context, planID, itemID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO plan_checks (plan_id, item_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, planID, itemID)
	return wrap("checks.set", err)
}

func (r *Checks) Unset(ctx context.Context, planID, itemID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM plan_checks WHERE plan_id = $1 AND item_id = $2`, planID, itemID)
	return wrap("checks.unset", err)
}

// ── Свои товары в списке ───────────────────────────────────────────────────

type Extras struct{ pool *pgxpool.Pool }

func (r *Extras) List(ctx context.Context, planID string) ([]domain.Extra, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, qty, due, note FROM plan_extras WHERE plan_id = $1 ORDER BY id`, planID)
	if err != nil {
		return nil, wrap("extras.list", err)
	}
	defer rows.Close()
	out := []domain.Extra{}
	for rows.Next() {
		var e domain.Extra
		var due *time.Time
		if err := rows.Scan(&e.ID, &e.Name, &e.Qty, &due, &e.Note); err != nil {
			return nil, wrap("extras.list", err)
		}
		if due != nil {
			d := due.Format("2006-01-02")
			e.Due = &d
		}
		out = append(out, e)
	}
	return out, wrap("extras.list", rows.Err())
}

func (r *Extras) Add(ctx context.Context, planID string, e domain.Extra, due *time.Time) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `INSERT INTO plan_extras (plan_id, name, qty, due, note) VALUES ($1, $2, $3, $4, $5) RETURNING id`, planID, e.Name, e.Qty, due, e.Note).Scan(&id)
	return id, wrap("extras.add", err)
}

// Delete убирает товар и его отметку «куплено».
func (r *Extras) Delete(ctx context.Context, planID string, id int64) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM plan_extras WHERE plan_id = $1 AND id = $2`, planID, id); err != nil {
		return wrap("extras.delete", err)
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM plan_checks WHERE plan_id = $1 AND item_id = $2`, planID, "extra:"+strconv.FormatInt(id, 10))
	return wrap("extras.delete", err)
}
