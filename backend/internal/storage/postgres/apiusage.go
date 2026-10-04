package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// APIUsage — счётчики квоты стороннего API по часу и суткам (UTC).
type APIUsage struct{ pool *pgxpool.Pool }

// Hit засчитывает запрос и возвращает, сколько их уже за текущий час и текущие сутки, вместе с этим.
func (r *APIUsage) Hit(ctx context.Context, key string) (hour, day int, err error) {
	rows, err := r.pool.Query(ctx, `INSERT INTO api_usage (key, kind, bucket, n) VALUES
			($1, 'h', date_trunc('hour', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC', 1),
			($1, 'd', date_trunc('day', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC', 1)
		ON CONFLICT (key, kind, bucket) DO UPDATE SET n = api_usage.n + 1
		RETURNING kind, n`, key)
	if err != nil {
		return 0, 0, wrap("api_usage.hit", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return 0, 0, wrap("api_usage.hit", err)
		}
		if kind == "h" {
			hour = n
		} else {
			day = n
		}
	}
	return hour, day, wrap("api_usage.hit", rows.Err())
}
