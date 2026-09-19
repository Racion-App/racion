package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// IndexNow — уведомление поисковиков о новых и изменённых страницах (Яндекс, Bing, Seznam, Naver и другие
// участники протокола; Google IndexNow не читает и узнаёт об изменениях из sitemap). Адреса копятся и
// уходят одним запросом раз в несколько секунд; ключ лежит в settings и отдаётся по /indexnow/<key>.txt.
type IndexNow struct {
	settings SettingsRepo
	base     string // публичный адрес сайта, например https://racion.app; пусто — уведомления выключены
	log      *zap.Logger
	client   *http.Client

	mu      sync.Mutex
	key     string
	pending map[string]struct{}
	timer   *time.Timer
}

const indexNowKeySetting = "indexnow.key"

func NewIndexNow(settings SettingsRepo, base string, log *zap.Logger) *IndexNow {
	return &IndexNow{settings: settings, base: strings.TrimRight(base, "/"), log: log, client: &http.Client{Timeout: 15 * time.Second}, pending: map[string]struct{}{}}
}

// Key — ключ протокола; создаётся при первом обращении и хранится в БД, чтобы не менялся между деплоями.
func (x *IndexNow) Key(ctx context.Context) string {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.key != "" {
		return x.key
	}
	if v, err := x.settings.Get(ctx, indexNowKeySetting); err == nil && len(v) >= 8 {
		x.key = v
		return v
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	v := hex.EncodeToString(b)
	if err := x.settings.Set(ctx, indexNowKeySetting, v); err != nil {
		x.log.Warn("indexnow key save", zap.Error(err))
		return ""
	}
	x.key = v
	return v
}

// Enabled — есть публичный адрес с https: локальный запуск поисковики не трогает.
func (x *IndexNow) Enabled() bool { return strings.HasPrefix(x.base, "https://") }

// Notify ставит пути в очередь; они уйдут одним запросом через 10 секунд после последнего вызова.
// Пути без языкового префикса (/recipe/borsch) разворачиваются на все языки вызывающей стороной.
func (x *IndexNow) Notify(paths ...string) {
	if !x.Enabled() || len(paths) == 0 {
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	for _, p := range paths {
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		x.pending[x.base+p] = struct{}{}
	}
	if x.timer != nil {
		x.timer.Stop()
	}
	x.timer = time.AfterFunc(10*time.Second, x.flush)
}

func (x *IndexNow) flush() {
	x.mu.Lock()
	urls := make([]string, 0, len(x.pending))
	for u := range x.pending {
		urls = append(urls, u)
	}
	x.pending = map[string]struct{}{}
	x.mu.Unlock()
	if len(urls) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	key := x.Key(ctx)
	if key == "" {
		return
	}
	host, _ := url.Parse(x.base)
	// не больше 10 000 адресов за запрос по протоколу
	if len(urls) > 10000 {
		urls = urls[:10000]
	}
	body, _ := json.Marshal(map[string]any{"host": host.Host, "key": key, "keyLocation": x.base + "/indexnow/" + key + ".txt", "urlList": urls})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.indexnow.org/indexnow", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res, err := x.client.Do(req)
	if err != nil {
		x.log.Warn("indexnow", zap.Error(err), zap.Int("urls", len(urls)))
		return
	}
	res.Body.Close()
	if res.StatusCode >= 300 {
		x.log.Warn("indexnow", zap.Int("status", res.StatusCode), zap.Int("urls", len(urls)))
		return
	}
	x.log.Info("indexnow sent", zap.Int("urls", len(urls)))
}
