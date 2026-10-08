package http

import (
	"net/http"
	"time"

	"go.uber.org/zap"

	"racion/internal/i18n"
	"racion/internal/metrika"
	"racion/internal/service"
)

// adminMetrika — сводка дня из Метрики: ?date=2026-10-07, по умолчанию вчера по Москве.
// Без токена отвечает {enabled:false}, и вкладка подсказывает, какую переменную задать.
func (s *Server) adminMetrika(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermStats) == nil {
		return
	}
	if !s.stats.Enabled() {
		writeJSON(w, 200, map[string]any{"enabled": false})
		return
	}
	today := time.Now().In(metrika.Moscow)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, metrika.Moscow)
	day := today.AddDate(0, 0, -1)
	if v := r.URL.Query().Get("date"); v != "" {
		d, err := time.ParseInLocation("2006-01-02", v, metrika.Moscow)
		if err != nil || d.After(today) {
			writeErr(w, 400, "bad date")
			return
		}
		day = d
	}
	out, err := s.stats.Day(r.Context(), day, string(i18n.FromRequest(r)))
	if err != nil {
		s.log.Warn("metrika", zap.Error(err))
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"enabled": true, "today": today.Format("2006-01-02"), "day": out})
}
