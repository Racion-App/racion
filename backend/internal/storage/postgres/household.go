package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// ── Семья ──────────────────────────────────────────────────────────────────

type Households struct{ pool *pgxpool.Pool }

const hhCols = `h.id, h.owner_id, h.name, h.adults, h.kids, COALESCE(h.invite_token, '')`

func scanHH(row interface{ Scan(...any) error }) (domain.Household, error) {
	var h domain.Household
	var adults, kids []byte
	if err := row.Scan(&h.ID, &h.OwnerID, &h.Name, &adults, &kids, &h.InviteToken); err != nil {
		return h, err
	}
	_ = json.Unmarshal(adults, &h.Adults)
	_ = json.Unmarshal(kids, &h.Kids)
	return h, nil
}

// ByUser — семья, в которой состоит аккаунт.
func (r *Households) ByUser(ctx context.Context, userID string) (domain.Household, error) {
	h, err := scanHH(r.pool.QueryRow(ctx, `SELECT `+hhCols+` FROM households h JOIN household_users hu ON hu.household_id = h.id WHERE hu.user_id = $1`, userID))
	return h, wrap("household.by_user", err)
}

func (r *Households) ByToken(ctx context.Context, token string) (domain.Household, error) {
	h, err := scanHH(r.pool.QueryRow(ctx, `SELECT `+hhCols+` FROM households h WHERE h.invite_token = $1`, token))
	return h, wrap("household.by_token", err)
}

// Create заводит семью и сразу добавляет владельца.
func (r *Households) Create(ctx context.Context, ownerID string) (domain.Household, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Household{}, wrap("household.create", err)
	}
	defer tx.Rollback(ctx)
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO households (owner_id) VALUES ($1) RETURNING id`, ownerID).Scan(&id); err != nil {
		return domain.Household{}, wrap("household.create", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO household_users (household_id, user_id) VALUES ($1, $2)`, id, ownerID); err != nil {
		return domain.Household{}, wrap("household.create", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Household{}, wrap("household.create", err)
	}
	return domain.Household{ID: id, OwnerID: ownerID}, nil
}

func (r *Households) Save(ctx context.Context, h domain.Household) error {
	adults, _ := json.Marshal(h.Adults)
	kids, _ := json.Marshal(h.Kids)
	_, err := r.pool.Exec(ctx, `UPDATE households SET name = $2, adults = $3, kids = $4, updated_at = now() WHERE id = $1`, h.ID, h.Name, adults, kids)
	return wrap("household.save", err)
}

func (r *Households) SetInvite(ctx context.Context, id, token string) error {
	_, err := r.pool.Exec(ctx, `UPDATE households SET invite_token = NULLIF($2, '') WHERE id = $1`, id, token)
	return wrap("household.invite", err)
}

// Accounts — аккаунты семьи: имя, ник, владелец ли.
func (r *Households) Accounts(ctx context.Context, id string) ([]domain.HouseholdAccount, error) {
	rows, err := r.pool.Query(ctx, `SELECT u.id, COALESCE(NULLIF(u.name, ''), split_part(u.email, '@', 1)), COALESCE(u.nick, ''), u.id = h.owner_id
		FROM household_users hu JOIN users u ON u.id = hu.user_id JOIN households h ON h.id = hu.household_id WHERE hu.household_id = $1 ORDER BY hu.joined_at`, id)
	if err != nil {
		return nil, wrap("household.accounts", err)
	}
	defer rows.Close()
	out := []domain.HouseholdAccount{}
	for rows.Next() {
		var a domain.HouseholdAccount
		if err := rows.Scan(&a.UserID, &a.Name, &a.Nick, &a.Owner); err != nil {
			return nil, wrap("household.accounts", err)
		}
		out = append(out, a)
	}
	return out, wrap("household.accounts", rows.Err())
}

// AddUser переносит аккаунт в семью: из прежней выходит; пустая прежняя семья удаляется.
func (r *Households) AddUser(ctx context.Context, id, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return wrap("household.add", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM household_users WHERE user_id = $1`, userID); err != nil {
		return wrap("household.add", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM households WHERE owner_id = $1 AND NOT EXISTS (SELECT 1 FROM household_users WHERE household_id = households.id)`, userID); err != nil {
		return wrap("household.add", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO household_users (household_id, user_id) VALUES ($1, $2)`, id, userID); err != nil {
		return wrap("household.add", err)
	}
	return wrap("household.add", tx.Commit(ctx))
}

// RemoveUser выводит аккаунт из семьи (не владельца).
func (r *Households) RemoveUser(ctx context.Context, id, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM household_users WHERE household_id = $1 AND user_id = $2`, id, userID)
	return wrap("household.remove", err)
}

// MemberCount — сколько аккаунтов в семье.
func (r *Households) MemberCount(ctx context.Context, id string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM household_users WHERE household_id = $1`, id).Scan(&n)
	return n, wrap("household.count", err)
}

// SameHousehold — состоят ли два аккаунта в одной семье.
func (r *Households) SameHousehold(ctx context.Context, a, b string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM household_users x JOIN household_users y ON x.household_id = y.household_id WHERE x.user_id = $1 AND y.user_id = $2)`, a, b).Scan(&ok)
	return ok, wrap("household.same", err)
}
