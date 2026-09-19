// Package geo определяет страну посетителя по IP, чтобы в квизе первой стояла его страна.
// База — DB-IP Country Lite (CC BY 4.0), выходит раз в месяц, ~10 МБ. Скачивается в фоне, хранится в БД
// (таблица geo_db), чтобы после перезапуска не тянуть заново. Без базы страна просто не определяется.
package geo

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oschwald/maxminddb-golang/v2"
	"go.uber.org/zap"
)

const urlFmt = "https://download.db-ip.com/free/dbip-country-lite-%s.mmdb.gz"

// База городов (DB-IP City Lite, ~130 МБ) в БД не кладём: лежит файлом в Dir и открывается через mmap.
// Без неё определяется только страна.
const cityURLFmt = "https://download.db-ip.com/free/dbip-city-lite-%s.mmdb.gz"

type Resolver struct {
	reader atomic.Pointer[maxminddb.Reader]
	city   atomic.Pointer[maxminddb.Reader]
	Dir    string // папка для файла базы городов; пусто — города не определяем
}

type cityRecord struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// Place — город и регион по IP (английские названия из базы) или пусто.
type Place struct {
	Country, Region, City string
}

func (g *Resolver) Place(ip string) Place {
	r := g.city.Load()
	if r == nil {
		return Place{}
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil || addr.IsPrivate() || addr.IsLoopback() || addr.IsUnspecified() {
		return Place{}
	}
	var rec cityRecord
	if err := r.Lookup(addr).Decode(&rec); err != nil {
		return Place{}
	}
	p := Place{Country: rec.Country.ISOCode, City: rec.City.Names["en"]}
	if len(rec.Subdivisions) > 0 {
		p.Region = rec.Subdivisions[0].Names["en"]
	}
	return p
}

type record struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// Country — код страны по IP (ISO 3166-1 alpha-2) или пусто.
func (g *Resolver) Country(ip string) string {
	r := g.reader.Load()
	if r == nil {
		return ""
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil || addr.IsPrivate() || addr.IsLoopback() || addr.IsUnspecified() {
		return ""
	}
	var rec record
	if err := r.Lookup(addr).Decode(&rec); err != nil {
		return ""
	}
	return rec.Country.ISOCode
}

// ClientIP — адрес посетителя за прокси: X-Forwarded-For (первый), X-Real-IP, иначе RemoteAddr.
func ClientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.Index(v, ","); i > 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Start загружает базу из БД, если есть, и в фоне обновляет её раз в месяц.
func (g *Resolver) Start(ctx context.Context, pool *pgxpool.Pool, log *zap.Logger) {
	_, _ = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS geo_db (name TEXT PRIMARY KEY, body BYTEA NOT NULL, fetched_at TIMESTAMPTZ NOT NULL DEFAULT now())`)
	var body []byte
	var fetched time.Time
	if err := pool.QueryRow(ctx, `SELECT body, fetched_at FROM geo_db WHERE name = 'dbip-country-lite'`).Scan(&body, &fetched); err == nil {
		if err := g.load(body); err == nil {
			log.Info("geo db loaded", zap.Int("bytes", len(body)), zap.String("fetched", fetched.Format("2006-01-02")))
		}
	}
	go func() {
		sync := func() {
			if g.reader.Load() != nil && time.Since(fetched) < 30*24*time.Hour {
				return
			}
			sctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			b, err := download(sctx)
			if err != nil {
				log.Warn("geo db download failed", zap.Error(err))
				return
			}
			if err := g.load(b); err != nil {
				log.Warn("geo db invalid", zap.Error(err))
				return
			}
			fetched = time.Now()
			if _, err := pool.Exec(ctx, `INSERT INTO geo_db (name, body, fetched_at) VALUES ('dbip-country-lite', $1, now()) ON CONFLICT (name) DO UPDATE SET body = EXCLUDED.body, fetched_at = now()`, b); err != nil {
				log.Warn("geo db save", zap.Error(err))
			}
			log.Info("geo db updated", zap.Int("bytes", len(b)))
		}
		sync()
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				sync()
			}
		}
	}()
	if g.Dir != "" {
		go g.cityLoop(ctx, log)
	}
}

// cityLoop держит файл базы городов свежим: открывает готовый, раз в месяц скачивает новый.
func (g *Resolver) cityLoop(ctx context.Context, log *zap.Logger) {
	path := filepath.Join(g.Dir, "dbip-city-lite.mmdb")
	var fetched time.Time
	if st, err := os.Stat(path); err == nil {
		if r, err := maxminddb.Open(path); err == nil {
			g.city.Store(r)
			fetched = st.ModTime()
			log.Info("geo city db loaded", zap.Int64("bytes", st.Size()), zap.String("fetched", fetched.Format("2006-01-02")))
		}
	}
	sync := func() {
		if g.city.Load() != nil && time.Since(fetched) < 30*24*time.Hour {
			return
		}
		sctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
		tmp := path + ".part"
		if err := downloadTo(sctx, cityURLFmt, tmp, 512<<20); err != nil {
			log.Warn("geo city db download failed", zap.Error(err))
			return
		}
		r, err := maxminddb.Open(tmp)
		if err != nil {
			log.Warn("geo city db invalid", zap.Error(err))
			_ = os.Remove(tmp)
			return
		}
		r.Close()
		if err := os.Rename(tmp, path); err != nil {
			log.Warn("geo city db rename", zap.Error(err))
			return
		}
		if r, err = maxminddb.Open(path); err == nil {
			if old := g.city.Swap(r); old != nil {
				old.Close()
			}
			fetched = time.Now()
			log.Info("geo city db updated")
		}
	}
	sync()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sync()
		}
	}
}

// downloadTo — то же, что download, но потоком в файл (база городов слишком велика для памяти).
func downloadTo(ctx context.Context, format, path string, limit int64) error {
	now := time.Now()
	var lastErr error
	for _, m := range []time.Time{now, now.AddDate(0, -1, 0)} {
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf(format, m.Format("2006-01")), nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if res.StatusCode != 200 {
			res.Body.Close()
			lastErr = fmt.Errorf("http %d", res.StatusCode)
			continue
		}
		gz, err := gzip.NewReader(res.Body)
		if err != nil {
			res.Body.Close()
			return err
		}
		f, err := os.Create(path)
		if err != nil {
			res.Body.Close()
			return err
		}
		_, err = io.Copy(f, io.LimitReader(gz, limit))
		f.Close()
		res.Body.Close()
		if err != nil {
			_ = os.Remove(path)
			return err
		}
		return nil
	}
	return lastErr
}

func (g *Resolver) load(b []byte) error {
	r, err := maxminddb.OpenBytes(b)
	if err != nil {
		return err
	}
	g.reader.Store(r)
	return nil
}

// download берёт базу за текущий месяц, если её ещё нет — за прошлый.
func download(ctx context.Context) ([]byte, error) {
	now := time.Now()
	var lastErr error
	for _, m := range []time.Time{now, now.AddDate(0, -1, 0)} {
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf(urlFmt, m.Format("2006-01")), nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if res.StatusCode != 200 {
			res.Body.Close()
			lastErr = fmt.Errorf("http %d", res.StatusCode)
			continue
		}
		gz, err := gzip.NewReader(res.Body)
		if err != nil {
			res.Body.Close()
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(gz, 64<<20))
		res.Body.Close()
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	return nil, lastErr
}
