package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/domain"
)

// StorePrices — цены сетей с открытым каталогом по последней сверке.
type StorePrices struct{ pool *pgxpool.Pool }

// Claim — взять сверку сети на себя, если прошлая начиналась больше every назад. false — сверка свежая
// или её уже начал другой экземпляр сервера.
func (r *StorePrices) Claim(ctx context.Context, store string, every time.Duration) (bool, error) {
	var s string
	err := r.pool.QueryRow(ctx, `INSERT INTO store_price_runs (store, started_at) VALUES ($1, now())
		ON CONFLICT (store) DO UPDATE SET started_at = now() WHERE store_price_runs.started_at < now() - make_interval(secs => $2)
		RETURNING store`, store, every.Seconds()).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, wrap("storeprices.claim", err)
}

func (r *StorePrices) Save(ctx context.Context, store string, p domain.StorePrice) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO store_prices (store, xml_id, name, url, unit, weight, price, found, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (store, xml_id) DO UPDATE SET name = CASE WHEN EXCLUDED.found THEN EXCLUDED.name ELSE store_prices.name END,
			url = CASE WHEN EXCLUDED.found THEN EXCLUDED.url ELSE store_prices.url END,
			unit = CASE WHEN EXCLUDED.found THEN EXCLUDED.unit ELSE store_prices.unit END,
			weight = CASE WHEN EXCLUDED.found THEN EXCLUDED.weight ELSE store_prices.weight END,
			price = CASE WHEN EXCLUDED.found THEN EXCLUDED.price ELSE store_prices.price END,
			found = EXCLUDED.found, checked_at = EXCLUDED.checked_at`,
		store, p.XMLID, p.Name, p.URL, p.Unit, p.Weight, p.Price, p.Found, p.CheckedAt)
	return wrap("storeprices.save", err)
}

// Load — последняя сверка по каждому товару сети.
func (r *StorePrices) Load(ctx context.Context, store string) (map[int]domain.StorePrice, error) {
	rows, err := r.pool.Query(ctx, `SELECT xml_id, name, url, unit, weight, price, found, checked_at FROM store_prices WHERE store = $1`, store)
	if err != nil {
		return nil, wrap("storeprices.load", err)
	}
	defer rows.Close()
	out := map[int]domain.StorePrice{}
	for rows.Next() {
		var p domain.StorePrice
		if err := rows.Scan(&p.XMLID, &p.Name, &p.URL, &p.Unit, &p.Weight, &p.Price, &p.Found, &p.CheckedAt); err != nil {
			return nil, wrap("storeprices.load", err)
		}
		out[p.XMLID] = p
	}
	return out, wrap("storeprices.load", rows.Err())
}
