package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// Замеры для страницы /status: компонент, удача, миллисекунды. Хранятся месяц.
type Health struct{ pool *pgxpool.Pool }

func NewHealth(pool *pgxpool.Pool) *Health { return &Health{pool: pool} }

func (r *Health) Add(ctx context.Context, component string, ok bool, ms int) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO health_samples (component, ok, ms) VALUES ($1, $2, $3)`, component, ok, ms)
	return wrap("health.add", err)
}

func (r *Health) Days(ctx context.Context, days int) ([]domain.HealthDay, error) {
	rows, err := r.pool.Query(ctx, `SELECT component, (ts AT TIME ZONE 'UTC')::date AS day, count(*), count(*) FILTER (WHERE ok)
		FROM health_samples WHERE ts > now() - ($1::int || ' days')::interval GROUP BY 1, 2 ORDER BY 1, 2`, days)
	if err != nil {
		return nil, wrap("health.days", err)
	}
	defer rows.Close()
	var out []domain.HealthDay
	for rows.Next() {
		var d domain.HealthDay
		if err := rows.Scan(&d.Component, &d.Day, &d.Total, &d.OK); err != nil {
			return nil, wrap("health.days", err)
		}
		out = append(out, d)
	}
	return out, wrap("health.days", rows.Err())
}

func (r *Health) Latency(ctx context.Context, component string, since time.Time) (int, int, error) {
	var avg *float64
	var n int
	err := r.pool.QueryRow(ctx, `SELECT avg(ms), count(*) FROM health_samples WHERE component = $1 AND ok AND ts > $2`, component, since).Scan(&avg, &n)
	if err != nil || avg == nil {
		return 0, n, wrap("health.latency", err)
	}
	return int(*avg), n, nil
}

func (r *Health) Prune(ctx context.Context, keep time.Duration) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM health_samples WHERE ts < $1`, time.Now().Add(-keep))
	return wrap("health.prune", err)
}
