package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// Рекламные предложения: таблица offers, правятся только из админки.
type Offers struct{ pool *pgxpool.Pool }

const offerCols = `id, country, regions, place, match, title, body, cta, url, image, promo, affiliate, erid, starts_at, ends_at, active, priority`

func (r *Offers) List(ctx context.Context) ([]domain.Offer, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+offerCols+` FROM offers ORDER BY country, place, priority DESC, title`)
	if err != nil {
		return nil, wrap("offers.list", err)
	}
	defer rows.Close()
	out := []domain.Offer{}
	for rows.Next() {
		var o domain.Offer
		var from, to *time.Time
		if err := rows.Scan(&o.ID, &o.Country, &o.Regions, &o.Place, &o.Match, &o.Title, &o.Body, &o.CTA, &o.URL, &o.Image, &o.Promo, &o.Affiliate, &o.Erid, &from, &to, &o.Active, &o.Priority); err != nil {
			return nil, wrap("offers.scan", err)
		}
		o.StartsAt, o.EndsAt = dateStr(from), dateStr(to)
		if o.Regions == nil {
			o.Regions = []string{}
		}
		if o.Match == nil {
			o.Match = []string{}
		}
		out = append(out, o)
	}
	return out, wrap("offers.rows", rows.Err())
}

func (r *Offers) Upsert(ctx context.Context, o domain.Offer) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO offers (`+offerCols+`, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now())
		ON CONFLICT (id) DO UPDATE SET country = EXCLUDED.country, regions = EXCLUDED.regions, place = EXCLUDED.place, match = EXCLUDED.match,
		title = EXCLUDED.title, body = EXCLUDED.body, cta = EXCLUDED.cta, url = EXCLUDED.url, image = EXCLUDED.image, promo = EXCLUDED.promo,
		affiliate = EXCLUDED.affiliate, erid = EXCLUDED.erid, starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at,
		active = EXCLUDED.active, priority = EXCLUDED.priority, updated_at = now()`,
		o.ID, o.Country, o.Regions, o.Place, o.Match, o.Title, o.Body, o.CTA, o.URL, o.Image, o.Promo, o.Affiliate, o.Erid, dateVal(o.StartsAt), dateVal(o.EndsAt), o.Active, o.Priority)
	return wrap("offers.upsert", err)
}

func (r *Offers) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM offers WHERE id = $1`, id)
	return wrap("offers.delete", err)
}

func dateStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func dateVal(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil
	}
	return &t
}
