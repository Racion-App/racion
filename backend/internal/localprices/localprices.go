// Package localprices — живые цены других стран, по образцу Росстата для России:
// США — BLS Average Price Data (API без ключа), Беларусь — месячный xls Белстата,
// Казахстан — еженедельный xlsx Бюро нацстатистики, Британия — средние цены ONS × CPI групп,
// Европа — ориентир Германии × уровень цен страны (Eurostat PLI) × HICP групп × курс.
// Каждый адаптер сам приводит единицы и сопоставляет товары с продуктами базы.
package localprices

import (
	"go.uber.org/zap"

	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/planner"
)

// PackInfo — упаковка продукта из базы: адаптер переводит цену за кг/л/десяток в цену упаковки.
type PackInfo struct {
	Pack     float64
	Unit     string             // g | ml | pcs
	Category string             // категория базы: meat, dairy, vegetables … (для групп HICP/CPI)
	Local    map[string]float64 // ручные ориентиры по странам; "DE" — якорь для Европы
}

// Result — ценник страны: период и цена упаковки по продуктам.
type Result struct {
	Period time.Time
	Pack   map[string]float64
}

// Source — источник цен одной страны.
type Source interface {
	Country() string
	Key() string // ключ i18n названия источника
	Fetch(ctx context.Context, packs map[string]PackInfo) (*Result, error)
}

// Sources — все подключённые источники: свои у США, Беларуси, Казахстана и Британии,
// Eurostat — у каждой страны Европы из списка planner.Countries.
func Sources() []Source {
	out := []Source{BLS{}, Belstat{}, KazStat{}, ONS{}}
	for _, c := range planner.Countries {
		if c.Eurostat != "" {
			out = append(out, Eurostat{Code: c.Code, Geo: c.Eurostat, Cur: c.Currency})
		}
	}
	return out
}

var client = &http.Client{Timeout: 3 * time.Minute}

func get(ctx context.Context, url string, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; racion/1.0; meal planner; average prices)")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s: http %d", url, res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 32<<20))
}

// Save пишет ценник страны в local_prices и отмечает синхронизацию.
func Save(ctx context.Context, pool *pgxpool.Pool, src Source, r *Result) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM local_prices WHERE country = $1`, src.Country()); err != nil {
		return err
	}
	batch := &pgx.Batch{}
	for id, price := range r.Pack {
		batch.Queue(`INSERT INTO local_prices (country, ingredient_id, pack_price, period, source) VALUES ($1, $2, $3, $4, $5)`,
			src.Country(), id, price, r.Period, src.Key())
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO local_sync (country, period, source, updated_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (country) DO UPDATE SET period = EXCLUDED.period, source = EXCLUDED.source, updated_at = now()`, src.Country(), r.Period, src.Key()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Stale — надо ли обновлять страну: нет записи или прошло больше суток.
func Stale(ctx context.Context, pool *pgxpool.Pool, country string) bool {
	var updated time.Time
	if err := pool.QueryRow(ctx, `SELECT updated_at FROM local_sync WHERE country = $1`, country).Scan(&updated); err != nil {
		return true
	}
	return time.Since(updated) > 24*time.Hour
}

// Sync обновляет все устаревшие страны. Ошибка одной страны не мешает остальным и не роняет сервер.
// Возвращает коды стран, где ценник обновился.
func Sync(ctx context.Context, pool *pgxpool.Pool, packs map[string]PackInfo, log *zap.Logger) []string {
	var updated []string
	for _, src := range Sources() {
		if !Stale(ctx, pool, src.Country()) {
			continue
		}
		sctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		r, err := src.Fetch(sctx, packs)
		cancel()
		if err != nil {
			log.Warn("local prices: fetch failed, keeping previous", zap.String("country", src.Country()), zap.Error(err))
			continue
		}
		if len(r.Pack) == 0 {
			log.Warn("local prices: empty result", zap.String("country", src.Country()))
			continue
		}
		if err := Save(ctx, pool, src, r); err != nil {
			log.Warn("local prices: save failed", zap.String("country", src.Country()), zap.Error(err))
			continue
		}
		log.Info("local prices synced", zap.String("country", src.Country()), zap.Int("items", len(r.Pack)), zap.String("period", r.Period.Format("2006-01-02")))
		updated = append(updated, src.Country())
	}
	return updated
}

// packPrice — цена упаковки из цены за единицу измерения источника.
// perUnit — сколько г/мл/шт в единице источника (кг → 1000, фунт → 453.6, галлон → 3785, десяток → 10).
func packPrice(unitPrice, perUnit float64, p PackInfo, factor float64) float64 {
	if factor <= 0 {
		factor = 1
	}
	return unitPrice / perUnit * p.Pack * factor
}
