package http

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"racion/internal/geo"
	"racion/internal/i18n"
)

// limiter — token bucket на ключ (IP или IP+почта) в памяти. Один процесс, поэтому без Redis:
// на тысячу пользователей этого хватает, при горизонтальном масштабировании лимит станет «на инстанс».
type limiter struct {
	rate  float64 // токенов в секунду
	burst float64
	mu    sync.Mutex
	items map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(perMinute, burst int) *limiter {
	l := &limiter{rate: float64(perMinute) / 60, burst: float64(burst), items: map[string]*bucket{}}
	go l.sweep()
	return l
}

// allow списывает токен; false — лимит исчерпан.
func (l *limiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.items[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.items[key] = b
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweep раз в минуту выкидывает ключи, которые давно не трогали, чтобы карта не росла.
func (l *limiter) sweep() {
	for range time.Tick(time.Minute) {
		cut := time.Now().Add(-10 * time.Minute)
		l.mu.Lock()
		for k, b := range l.items {
			if b.last.Before(cut) {
				delete(l.items, k)
			}
		}
		l.mu.Unlock()
	}
}

// limits — лимиты по группам ручек. Ключ — IP, для входа ещё и почта, чтобы перебор пароля
// с разных адресов тоже упирался в потолок.
type limits struct {
	api    *limiter // всё /api: 600 в минуту с одного адреса
	auth   *limiter // вход и регистрация: 10 в минуту с адреса
	email  *limiter // вход: 20 попыток на почту за 10 минут
	build  *limiter // сборка недели и замены: 60 в минуту
	events *limiter // аналитика: 60 пачек в минуту
	write  *limiter // свои рецепты, товары, отметки: 120 в минуту
}

func newLimits() *limits {
	return &limits{
		api:    newLimiter(600, 100),
		auth:   newLimiter(10, 10),
		email:  newLimiter(2, 20),
		build:  newLimiter(60, 20),
		events: newLimiter(60, 30),
		write:  newLimiter(120, 40),
	}
}

func tooMany(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "60")
	writeErr(w, http.StatusTooManyRequests, i18n.T(i18n.FromRequest(r), "api.toomany"))
}

// unlimited — лимиты не считаем для ключей API (их выдают только админам) и для админов и модераторов,
// вошедших по сессии: их скрипты и пакетные загрузки не должны упираться в защиту от ботов.
func (s *Server) unlimited(r *http.Request) bool {
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer rk_") {
		return true
	}
	u := currentUser(r)
	return u != nil && s.svc.Admin.Role(u) != ""
}

// withLimit — общий лимит на /api по IP; точечные лимиты навешиваются на ручки через limited().
func (s *Server) withLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && !s.unlimited(r) && !s.lim.api.allow(geo.ClientIP(r)) {
			tooMany(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) limited(l *limiter, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.unlimited(r) && !l.allow(geo.ClientIP(r)) {
			tooMany(w, r)
			return
		}
		h(w, r)
	}
}
