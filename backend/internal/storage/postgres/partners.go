package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// Партнёрские магазины: таблица partners, дефолты заливаются при старте и не перетирают правки админа.
type Partners struct{ pool *pgxpool.Pool }

const partnerCols = `code, country, kind, name, url, affiliate, erid, active, priority`

func (r *Partners) List(ctx context.Context) ([]domain.Partner, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+partnerCols+` FROM partners ORDER BY country, kind, priority, name`)
	if err != nil {
		return nil, wrap("partners.list", err)
	}
	defer rows.Close()
	out := []domain.Partner{}
	for rows.Next() {
		var p domain.Partner
		if err := rows.Scan(&p.Code, &p.Country, &p.Kind, &p.Name, &p.URL, &p.Affiliate, &p.Erid, &p.Active, &p.Priority); err != nil {
			return nil, wrap("partners.scan", err)
		}
		out = append(out, p)
	}
	return out, wrap("partners.rows", rows.Err())
}

func (r *Partners) Upsert(ctx context.Context, p domain.Partner) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO partners (`+partnerCols+`, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		ON CONFLICT (code) DO UPDATE SET country = EXCLUDED.country, kind = EXCLUDED.kind, name = EXCLUDED.name, url = EXCLUDED.url,
		affiliate = EXCLUDED.affiliate, erid = EXCLUDED.erid, active = EXCLUDED.active, priority = EXCLUDED.priority, updated_at = now()`,
		p.Code, p.Country, p.Kind, p.Name, p.URL, p.Affiliate, p.Erid, p.Active, p.Priority)
	return wrap("partners.upsert", err)
}

// SeedDefaults — вставляет отсутствующие коды; существующие строки не трогает.
func (r *Partners) SeedDefaults(ctx context.Context, list []domain.Partner) error {
	for _, p := range list {
		if _, err := r.pool.Exec(ctx, `INSERT INTO partners (`+partnerCols+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (code) DO NOTHING`,
			p.Code, p.Country, p.Kind, p.Name, p.URL, p.Affiliate, p.Erid, p.Active, p.Priority); err != nil {
			return wrap("partners.seed", err)
		}
	}
	return nil
}

func (r *Partners) Delete(ctx context.Context, code string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM partners WHERE code = $1`, code)
	return wrap("partners.delete", err)
}
