package http

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"racion/internal/geo"
	"racion/internal/i18n"
)

// Квота стороннего API: запросы не с нашего сайта (чужие сайты, скрипты, MCP) — не больше apiHourQuota
// в час и apiDayQuota в сутки (UTC) с одного адреса. Сайт и мини-приложение в Telegram в квоту не входят,
// админские ключи и сессии — тоже (unlimited).
const (
	apiHourQuota = 50
	apiDayQuota  = 300
)

// quotaHeaders — что браузер стороннего сайта может прочитать из ответа (Access-Control-Expose-Headers).
const quotaHeaders = "Retry-After, X-RateLimit-Limit-Hour, X-RateLimit-Remaining-Hour, X-RateLimit-Limit-Day, X-RateLimit-Remaining-Day"

// APIQuota — счётчик запросов по часу и суткам; postgres.APIUsage.
type APIQuota interface {
	Hit(ctx context.Context, key string) (hour, day int, err error)
}

// firstParty — запрос нашего сайта или мини-приложения. Браузер сам пишет Sec-Fetch-Site, а старые
// браузеры — Origin или Referer. Подделать заголовки можно: квота защищает от чужой нагрузки по незнанию,
// а от злого умысла остаётся общий лимит по адресу.
func (s *Server) firstParty(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin":
		return true
	case "cross-site", "same-site", "none":
		return false
	}
	host := hostOnly(r.Host)
	for _, h := range []string{"Origin", "Referer"} {
		if u, err := url.Parse(r.Header.Get(h)); err == nil && u.Host != "" {
			return hostOnly(u.Host) == host
		}
	}
	return false
}

// quotaKey — адрес для квоты; IPv6 — по сети /64: провайдер выдаёт её одному абоненту целиком,
// и смена адреса внутри неё не должна обнулять квоту.
func quotaKey(ip string) string {
	a := net.ParseIP(ip)
	if a == nil || a.To4() != nil {
		return ip
	}
	return a.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

func hostOnly(h string) string {
	if host, _, err := net.SplitHostPort(h); err == nil {
		return strings.ToLower(host)
	}
	return strings.ToLower(h)
}

// overQuota засчитывает сторонний запрос и отвечает 429, если квота кончилась. Заголовки X-RateLimit-*
// показывают остаток, чтобы разработчик видел его до отказа. База недоступна — пропускаем: за наш сбой
// разработчик не отвечает, общий лимит по адресу всё равно действует.
func (s *Server) overQuota(w http.ResponseWriter, r *http.Request) bool {
	if s.quota == nil || r.Method == http.MethodOptions || s.firstParty(r) {
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	hour, day, err := s.quota.Hit(ctx, quotaKey(geo.ClientIP(r)))
	if err != nil {
		s.log.Warn("api quota", zap.Error(err))
		return false
	}
	h := w.Header()
	h.Set("X-RateLimit-Limit-Hour", strconv.Itoa(apiHourQuota))
	h.Set("X-RateLimit-Remaining-Hour", strconv.Itoa(max(0, apiHourQuota-hour)))
	h.Set("X-RateLimit-Limit-Day", strconv.Itoa(apiDayQuota))
	h.Set("X-RateLimit-Remaining-Day", strconv.Itoa(max(0, apiDayQuota-day)))
	if hour <= apiHourQuota && day <= apiDayQuota {
		return false
	}
	now := time.Now().UTC()
	next := now.Truncate(time.Hour).Add(time.Hour)
	if day > apiDayQuota {
		next = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	}
	h.Set("Retry-After", strconv.Itoa(int(next.Sub(now).Seconds())+1))
	if r.Header.Get("Origin") != "" { // отказ уходит до обработчика с CORS: без этого браузер покажет ошибку CORS, а не 429
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Expose-Headers", quotaHeaders)
	}
	writeErr(w, http.StatusTooManyRequests, i18n.T(i18n.FromRequest(r), "api.quota", apiHourQuota, apiDayQuota))
	return true
}
