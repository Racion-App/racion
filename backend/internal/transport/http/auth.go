package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/service"
)

const sessionCookie = "racion_session"

type ctxKey int

const userKey ctxKey = 1

// withUser читает cookie сессии и кладёт пользователя в контекст. Без сессии запрос идёт дальше как гостевой.
func (s *Server) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			if u, err := s.svc.Accounts.UserByToken(r.Context(), c.Value); err == nil && u != nil {
				r = r.WithContext(context.WithValue(r.Context(), userKey, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func currentUser(r *http.Request) *domain.User {
	u, _ := r.Context().Value(userKey).(*domain.User)
	return u
}

func requireUser(w http.ResponseWriter, r *http.Request) *domain.User {
	u := currentUser(r)
	if u == nil {
		writeErr(w, 401, i18n.T(i18n.FromRequest(r), "auth.required"))
	}
	return u
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, sess domain.Session) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: sess.Token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		MaxAge: int(service.SessionTTL.Seconds()),
	})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var c service.Credentials
	if !decode(w, r, 8<<10, &c) {
		return
	}
	u, sess, err := s.svc.Accounts.Register(r.Context(), c, r.URL.Query().Get("plan"))
	if err != nil {
		if ve, ok := service.IsValidation(err); ok && ve.Key == "auth.exists" {
			writeErr(w, 409, i18n.T(i18n.FromRequest(r), ve.Key))
			return
		}
		s.fail(w, r, err)
		return
	}
	setSessionCookie(w, r, sess)
	writeJSON(w, 201, u)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var c service.Credentials
	if !decode(w, r, 8<<10, &c) {
		return
	}
	if !s.lim.email.allow(strings.ToLower(strings.TrimSpace(c.Email))) {
		tooMany(w, r)
		return
	}
	u, sess, err := s.svc.Accounts.Login(r.Context(), c, r.URL.Query().Get("plan"))
	if errors.Is(err, domain.ErrUnauthorized) {
		writeErr(w, 401, i18n.T(i18n.FromRequest(r), "auth.wrong"))
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	setSessionCookie(w, r, sess)
	writeJSON(w, 200, u)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = s.svc.Accounts.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	w.WriteHeader(204)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if u := currentUser(r); u != nil {
		writeJSON(w, 200, map[string]any{"user": u, "admin": s.svc.Admin.Is(u), "perms": s.svc.Admin.Perms(u), "role": s.svc.Admin.Role(u)})
		return
	}
	writeJSON(w, 200, map[string]any{"user": nil})
}

// updateMe — имя и сохранённые ответы квиза.
func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var upd service.ProfileUpdate
	if !decode(w, r, 64<<10, &upd) {
		return
	}
	out, err := s.svc.Accounts.UpdateProfile(r.Context(), *u, upd)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

// ── Кабинет ────────────────────────────────────────────────────────────────

func (s *Server) myPlans(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	out, err := s.svc.Plans.Mine(r.Context(), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) listDislikes(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	out, err := s.svc.Accounts.Dislikes(r.Context(), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) addDislike(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if err := s.svc.Accounts.AddDislike(r.Context(), u.ID, r.PathValue("recipe")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) removeDislike(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	_ = s.svc.Accounts.RemoveDislike(r.Context(), u.ID, r.PathValue("recipe"))
	w.WriteHeader(204)
}

// myBudget — динамика бюджета по неделям и месяцам.
func (s *Server) myBudget(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	rep, err := s.svc.Accounts.Budget(r.Context(), u.ID, 8, time.Now())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, rep)
}

func (s *Server) myPurchases(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	days := 0
	for _, ch := range r.URL.Query().Get("days") {
		if ch < '0' || ch > '9' {
			days = 0
			break
		}
		days = days*10 + int(ch-'0')
	}
	out, err := s.svc.Accounts.Purchases(r.Context(), u.ID, days)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}
