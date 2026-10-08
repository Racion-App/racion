package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// PageVersions — хеши страниц и даты их смены (таблица page_versions).
type PageVersions struct{ pool *pgxpool.Pool }

func (r *PageVersions) All(ctx context.Context) (map[string]domain.PageVersion, error) {
	rows, err := r.pool.Query(ctx, `SELECT path, hash, changed_at FROM page_versions`)
	if err != nil {
		return nil, wrap("pages.all", err)
	}
	defer rows.Close()
	out := map[string]domain.PageVersion{}
	for rows.Next() {
		var path string
		var v domain.PageVersion
		if err := rows.Scan(&path, &v.Hash, &v.Changed); err != nil {
			return nil, wrap("pages.all", err)
		}
		out[path] = v
	}
	return out, wrap("pages.all", rows.Err())
}

// Save — новые и изменившиеся страницы одним запросом.
func (r *PageVersions) Save(ctx context.Context, rows map[string]domain.PageVersion) error {
	paths := make([]string, 0, len(rows))
	hashes := make([]string, 0, len(rows))
	times := make([]time.Time, 0, len(rows))
	for p, v := range rows {
		paths, hashes, times = append(paths, p), append(hashes, v.Hash), append(times, v.Changed)
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO page_versions (path, hash, changed_at)
		SELECT * FROM unnest($1::text[], $2::text[], $3::timestamptz[])
		ON CONFLICT (path) DO UPDATE SET hash = EXCLUDED.hash, changed_at = EXCLUDED.changed_at`, paths, hashes, times)
	return wrap("pages.save", err)
}
