// Package rosstat загружает средние потребительские цены Росстата:
//   - месячный файл по территориям (регионы и города) — sred_potreb_cen_MM-YYYY.xlsx;
//   - недельный файл по РФ в целом — nedel_sred_cen.xlsx.
//
// Оба лежат на rosstat.gov.ru под сертификатом Минцифры, поэтому в клиент
// добавлены Russian Trusted Root CA и Sub CA: сервер отдаёт только листовой сертификат, без цепочки.
package rosstat

import (
	"go.uber.org/zap"

	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

//go:embed russian_trusted_root_ca.pem
var russianRootCA []byte

const (
	baseURL    = "https://rosstat.gov.ru/storage/mediabank/"
	weeklyFile = "nedel_sred_cen.xlsx"
	// Товары с кодом ниже — продукты питания; одежду, бензин и отели не храним.
	maxFoodCode = 3500
)

type Region struct {
	Code   string
	Name   string
	Kind   string // rf | district | region | city
	Parent string
}

type MonthlyPrice struct {
	Region string
	Item   int
	Price  float64
}

type Monthly struct {
	Period  string // YYYY-MM
	Regions []Region
	Prices  []MonthlyPrice
	Items   map[int]string // код → название
}

type Weekly struct {
	Date   time.Time
	Prices map[string]map[time.Time]float64 // название → дата → цена
}

func httpClient() *http.Client {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	pool.AppendCertsFromPEM(russianRootCA)
	return &http.Client{
		Timeout: 3 * time.Minute,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		},
	}
}

func fetch(ctx context.Context, name string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+name, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "racion/1.0 (+meal planner; rosstat average prices)")
	res, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s: http %d", name, res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 64<<20))
}

// FetchMonthly пробует текущий месяц и до трёх предыдущих — файл за месяц появляется с лагом.
func FetchMonthly(ctx context.Context, now time.Time) (*Monthly, error) {
	var lastErr error
	for back := 0; back < 4; back++ {
		t := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -back, 0)
		name := fmt.Sprintf("sred_potreb_cen_%02d-%d.xlsx", int(t.Month()), t.Year())
		b, err := fetch(ctx, name)
		if err != nil {
			lastErr = err
			continue
		}
		m, err := parseMonthly(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return m, nil
	}
	return nil, lastErr
}

var periodRe = regexp.MustCompile(`^(\d{2})\((\d{4})\)$`)

func parseMonthly(b []byte) (*Monthly, error) {
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Последний лист вида "08(2026)" — самый свежий месяц.
	var sheet, period string
	for _, s := range f.GetSheetList() {
		if m := periodRe.FindStringSubmatch(s); m != nil {
			sheet = s
			period = m[2] + "-" + m[1]
		}
	}
	if sheet == "" {
		return nil, fmt.Errorf("no monthly sheet found")
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) < 6 {
		return nil, fmt.Errorf("sheet %s too short", sheet)
	}
	codes, names := rows[3], rows[4]
	m := &Monthly{Period: period, Items: map[int]string{}}
	colItem := map[int]int{}
	for i := 2; i < len(codes); i++ {
		code, err := strconv.Atoi(strings.TrimSpace(codes[i]))
		if err != nil || code >= maxFoodCode {
			continue
		}
		colItem[i] = code
		if i < len(names) {
			m.Items[code] = strings.TrimSpace(names[i])
		}
	}
	for _, r := range rows[5:] {
		if len(r) < 2 || strings.TrimSpace(r[0]) == "" {
			continue
		}
		code := strings.TrimSpace(r[0])
		name := strings.TrimSpace(r[1])
		if _, err := strconv.ParseInt(code, 10, 64); err != nil {
			continue
		}
		m.Regions = append(m.Regions, Region{Code: code, Name: cleanRegionName(name), Kind: regionKind(code), Parent: regionParent(code)})
		for col, item := range colItem {
			if col >= len(r) {
				continue
			}
			v, ok := parsePrice(r[col])
			if !ok {
				continue
			}
			m.Prices = append(m.Prices, MonthlyPrice{Region: code, Item: item, Price: v})
		}
	}
	return m, nil
}

func parsePrice(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" || s == "..." || s == "…" || s == "-" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

func regionKind(code string) string {
	switch {
	case code == "643":
		return "rf"
	case len(code) <= 2:
		return "district"
	case len(code) == 11 && strings.HasSuffix(code, "000000000"):
		return "region"
	default:
		return "city"
	}
}

func regionParent(code string) string {
	if len(code) == 11 && !strings.HasSuffix(code, "000000000") {
		return code[:2] + "000000000"
	}
	return ""
}

// cleanRegionName убирает бюрократические хвосты: «Город Москва столица Российской Федерации город федерального значения» → «Москва».
func cleanRegionName(n string) string {
	n = strings.TrimSpace(n)
	repl := []string{
		" столица Российской Федерации город федерального значения", "",
		" город федерального значения", "",
		"Город ", "",
		" (Татарстан)", "",
		" (Якутия)", "",
		" - Югра", " — Югра",
		" (Чувашия)", "",
	}
	n = strings.NewReplacer(repl...).Replace(n)
	return n
}

// FetchWeekly читает недельный файл: лист текущего года, колонки «на 14 сентября».
func FetchWeekly(ctx context.Context, now time.Time) (*Weekly, error) {
	b, err := fetch(ctx, weeklyFile)
	if err != nil {
		return nil, err
	}
	return parseWeekly(b, now)
}

var months = map[string]time.Month{
	"января": 1, "февраля": 2, "марта": 3, "апреля": 4, "мая": 5, "июня": 6,
	"июля": 7, "августа": 8, "сентября": 9, "октября": 10, "ноября": 11, "декабря": 12,
}

func parseWeekly(b []byte, now time.Time) (*Weekly, error) {
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	w := &Weekly{Prices: map[string]map[time.Time]float64{}}
	for _, year := range []int{now.Year(), now.Year() - 1} {
		sheet := strconv.Itoa(year)
		rows, err := f.GetRows(sheet)
		if err != nil || len(rows) < 5 {
			continue
		}
		hdr := rows[3]
		dates := map[int]time.Time{}
		for i := 1; i < len(hdr); i++ {
			parts := strings.Fields(strings.TrimSpace(hdr[i]))
			if len(parts) != 3 || parts[0] != "на" {
				continue
			}
			day, err := strconv.Atoi(parts[1])
			mon, ok := months[parts[2]]
			if err != nil || !ok {
				continue
			}
			d := time.Date(year, mon, day, 0, 0, 0, 0, time.UTC)
			dates[i] = d
			if d.After(w.Date) {
				w.Date = d
			}
		}
		for _, r := range rows[4:] {
			if len(r) == 0 {
				continue
			}
			name := normalizeName(r[0])
			if name == "" {
				continue
			}
			for col, d := range dates {
				if col >= len(r) {
					continue
				}
				if v, ok := parsePrice(r[col]); ok {
					if w.Prices[name] == nil {
						w.Prices[name] = map[time.Time]float64{}
					}
					w.Prices[name][d] = v
				}
			}
		}
	}
	if w.Date.IsZero() {
		return nil, fmt.Errorf("weekly: no dated columns")
	}
	return w, nil
}

// normalizeName сводит названия из недельного и месячного файла к одному виду.
func normalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// Save пишет месяц и неделю в БД одной транзакцией.
func Save(ctx context.Context, pool *pgxpool.Pool, m *Monthly, w *Weekly) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for i, r := range m.Regions {
		if _, err := tx.Exec(ctx, `INSERT INTO regions (code, name, kind, parent, sort) VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name, kind=EXCLUDED.kind, parent=EXCLUDED.parent, sort=EXCLUDED.sort`,
			r.Code, r.Name, r.Kind, r.Parent, i); err != nil {
			return err
		}
	}
	for item, name := range m.Items {
		if _, err := tx.Exec(ctx, `INSERT INTO rosstat_items (item, name) VALUES ($1,$2) ON CONFLICT (item) DO UPDATE SET name=EXCLUDED.name`, item, name); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM rosstat_prices WHERE period = $1`, m.Period); err != nil {
		return err
	}
	rows := make([][]any, 0, len(m.Prices))
	for _, p := range m.Prices {
		rows = append(rows, []any{p.Region, p.Item, m.Period, p.Price})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"rosstat_prices"}, []string{"region", "item", "period", "price"}, pgx.CopyFromRows(rows)); err != nil {
		return err
	}
	if w != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM rosstat_weekly`); err != nil {
			return err
		}
		wrows := make([][]any, 0, 4096)
		for name, byDate := range w.Prices {
			for d, v := range byDate {
				wrows = append(wrows, []any{name, d, v})
			}
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"rosstat_weekly"}, []string{"item_name", "week_date", "price"}, pgx.CopyFromRows(wrows)); err != nil {
			return err
		}
	}
	var wd any
	if w != nil {
		wd = w.Date
	}
	if _, err := tx.Exec(ctx, `INSERT INTO rosstat_sync (id, monthly_period, weekly_date, fetched_at) VALUES (1, $1, $2, now())
		ON CONFLICT (id) DO UPDATE SET monthly_period=EXCLUDED.monthly_period, weekly_date=EXCLUDED.weekly_date, fetched_at=now()`, m.Period, wd); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Stale — надо ли обновлять: нет записи или прошло больше суток.
func Stale(ctx context.Context, pool *pgxpool.Pool) bool {
	var fetched time.Time
	if err := pool.QueryRow(ctx, `SELECT fetched_at FROM rosstat_sync WHERE id = 1`).Scan(&fetched); err != nil {
		return true
	}
	return time.Since(fetched) > 24*time.Hour
}

// Sync — полный цикл: скачать, распарсить, сохранить. Ошибка сети не роняет сервер — цены остаются старыми.
func Sync(ctx context.Context, pool *pgxpool.Pool, log *zap.Logger) error {
	now := time.Now()
	m, err := FetchMonthly(ctx, now)
	if err != nil {
		return fmt.Errorf("monthly: %w", err)
	}
	w, err := FetchWeekly(ctx, now)
	if err != nil {
		log.Warn("rosstat weekly unavailable, using monthly only", zap.Error(err))
		w = nil
	}
	if err := Save(ctx, pool, m, w); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	log.Info("rosstat synced", zap.String("period", m.Period), zap.Int("regions", len(m.Regions)), zap.Int("prices", len(m.Prices)), zap.Bool("weekly", w != nil))
	return nil
}
