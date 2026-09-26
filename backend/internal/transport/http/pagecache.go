package http

import (
	"bytes"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Микрокэш готовых страниц каталога. Он нужен не ради скорости, а ради живучести.
// Скраперы обходят две тысячи рецептов на пятнадцати языках, каждая страница — отдельный рендер,
// а ядер у сервера два. В обходе примерно треть запросов приходится на уже виденные адреса:
// эту треть отдаём из памяти и не трогаем шаблоны вовсе.
//
// Кэшируем только то, что одинаково для всех: GET, аноним, ответ 200, text/html, без Set-Cookie.
// Ключ включает страну и язык, потому что цены в карточках считаются по стране посетителя, а она
// берётся из cookie или по адресу. Без страны в ключе москвич получил бы берлинские цены.
const (
	pageCacheTTL     = time.Minute
	pageCacheMax     = 3000
	pageCacheMaxBody = 512 << 10
)

type cachedPage struct {
	body    []byte
	header  http.Header
	expires time.Time
}

type pageCache struct {
	mu    sync.Mutex
	items map[string]*cachedPage
}

func newPageCache() *pageCache { return &pageCache{items: map[string]*cachedPage{}} }

func (c *pageCache) get(key string) *cachedPage {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.items[key]
	if !ok {
		return nil
	}
	if time.Now().After(p.expires) {
		delete(c.items, key)
		return nil
	}
	return p
}

// put кладёт страницу и держит размер карты в рамках: сначала выбрасывает протухшее, а если и после
// этого тесно — чистит всё. Вытеснять по частоте смысла нет: записи живут минуту, и обход всё равно
// идёт по новым адресам.
func (c *pageCache) put(key string, p *cachedPage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) >= pageCacheMax {
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.expires) {
				delete(c.items, k)
			}
		}
		if len(c.items) >= pageCacheMax {
			c.items = map[string]*cachedPage{}
		}
	}
	c.items[key] = p
}

// cacheablePath — тяжёлые страницы каталога, которые рендерятся из шаблонов. Карта сайта, robots и
// спецификация сюда не входят: они дешёвые и меняются от параметров запроса.
func cacheablePath(p string) bool {
	if len(p) > 3 && p[0] == '/' && p[3] == '/' {
		p = p[3:]
	}
	for _, pre := range []string{"/recipes", "/recipe/", "/collections", "/collection/", "/menu/", "/terms", "/privacy", "/developers"} {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}

// pageRecorder копит ответ в памяти, чтобы после рендера решить, годится ли он в кэш.
type pageRecorder struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
	over   bool // ответ перерос лимит: пишем клиенту, но не запоминаем
}

func (w *pageRecorder) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *pageRecorder) Write(b []byte) (int, error) {
	if !w.over {
		if w.buf.Len()+len(b) > pageCacheMaxBody {
			w.over = true
			w.buf.Reset()
		} else {
			w.buf.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (s *Server) withPageCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || currentUser(r) != nil || !cacheablePath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		pl, _ := s.localeFromPath(r)
		key := r.URL.Path + "?" + r.URL.RawQuery + "|" + string(pl.L) + "|" + pl.Country.Code
		if p := s.pages.get(key); p != nil {
			h := w.Header()
			for k, v := range p.header {
				h[k] = v
			}
			h.Set("X-Cache", "hit")
			_, _ = w.Write(p.body)
			return
		}
		rec := &pageRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if rec.over || rec.status != http.StatusOK || rec.buf.Len() == 0 {
			return
		}
		h := rec.Header()
		if len(h.Values("Set-Cookie")) > 0 || !strings.HasPrefix(h.Get("Content-Type"), "text/html") {
			return
		}
		saved := http.Header{}
		for _, k := range []string{"Content-Type", "Cache-Control", "Content-Language", "Link", "Vary"} {
			if v := h.Values(k); len(v) > 0 {
				saved[k] = append([]string(nil), v...)
			}
		}
		s.pages.put(key, &cachedPage{body: append([]byte(nil), rec.buf.Bytes()...), header: saved, expires: time.Now().Add(pageCacheTTL)})
	})
}
