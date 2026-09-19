package http

import (
	"net/http"
	"strconv"

	"racion/internal/service"
)

// Админка: только для почт из ADMIN_EMAILS. Чужим — 404, чтобы не подсказывать, что ручка есть.

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermStats) == nil {
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	out, err := s.svc.Admin.Overview(r.Context(), days)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) adminErrors(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermErrors) == nil {
		return
	}
	out, err := s.svc.Admin.Errors(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermUsers) == nil {
		return
	}
	out, err := s.svc.Admin.Users(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

// adminLogs — последние записи из кольца: ?level=warn&n=200.
func (s *Server) adminLogs(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermLogs) == nil {
		return
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n <= 0 || n > 1000 {
		n = 200
	}
	level := r.URL.Query().Get("level")
	if level == "" {
		level = "info"
	}
	if s.logs == nil {
		writeJSON(w, 200, []any{})
		return
	}
	writeJSON(w, 200, s.logs.Last(n, level))
}
