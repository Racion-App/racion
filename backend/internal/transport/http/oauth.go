package http

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"racion/internal/oauth"
	"racion/internal/service"
)

// Вход через внешние сервисы. start кладёт state в cookie и уводит к провайдеру, callback меняет код на профиль,
// открывает сессию и возвращает на сайт. Список кнопок зависит от страны по IP: России — VK и Яндекс, остальным все.

const oauthCookie = "racion_oauth"

func (s *Server) oauthProviders(w http.ResponseWriter, r *http.Request) {
	type pv struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	out := []pv{}
	if s.oauth != nil {
		for _, p := range s.oauth.Available(s.geoCountry(r)) {
			out = append(out, pv{p.ID, p.Name})
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, map[string]any{"providers": out})
}

func (s *Server) oauthRedirect(r *http.Request, provider string) string {
	return s.baseURL(r) + "/api/auth/oauth/" + provider + "/callback"
}

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	p, ok := s.oauthProvider(r)
	if !ok {
		s.notFoundPage(w, r)
		return
	}
	next := r.URL.Query().Get("next")
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		next = ""
	}
	st := oauth.NewState(r.URL.Query().Get("plan"), next, p.PKCE)
	raw, _ := json.Marshal(st)
	http.SetCookie(w, &http.Cookie{
		Name: oauthCookie, Value: base64.RawURLEncoding.EncodeToString(raw), Path: "/api/auth/oauth", HttpOnly: true,
		// Apple возвращает POST с чужого сайта: с Lax cookie не придёт, нужен None
		SameSite: http.SameSiteNoneMode, Secure: true, MaxAge: 600,
	})
	http.Redirect(w, r, s.oauth.AuthURL(p, s.oauthRedirect(r, p.ID), st), http.StatusFound)
}

func (s *Server) oauthProvider(r *http.Request) (oauth.Provider, bool) {
	if s.oauth == nil {
		return oauth.Provider{}, false
	}
	return s.oauth.Get(r.PathValue("provider"))
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	p, ok := s.oauthProvider(r)
	if !ok {
		s.notFoundPage(w, r)
		return
	}
	fail := func(reason string, err error) {
		s.log.Warn("oauth", zap.String("provider", p.ID), zap.String("reason", reason), zap.Error(err))
		http.Redirect(w, r, "/login?error=oauth", http.StatusFound)
	}
	var st oauth.State
	if c, err := r.Cookie(oauthCookie); err == nil {
		if raw, err := base64.RawURLEncoding.DecodeString(c.Value); err == nil {
			_ = json.Unmarshal(raw, &st)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: oauthCookie, Value: "", Path: "/api/auth/oauth", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteNoneMode})
	if st.Nonce == "" {
		fail("no state cookie", nil)
		return
	}
	_ = r.ParseForm()
	cb := oauth.Callback{Code: r.Form.Get("code"), State: r.Form.Get("state"), DeviceID: r.Form.Get("device_id"), User: r.Form.Get("user")}
	if e := r.Form.Get("error"); e != "" {
		fail("provider error: "+e, nil)
		return
	}
	pr, err := s.oauth.Exchange(r.Context(), p, s.oauthRedirect(r, p.ID), st, cb)
	if err != nil {
		fail("exchange", err)
		return
	}
	u, sess, err := s.svc.Accounts.LoginOAuth(r.Context(), service.OAuthProfile{Provider: p.ID, ID: pr.ID, Email: pr.Email, Name: pr.Name, Avatar: pr.Avatar}, st.Plan)
	if err != nil {
		fail("login", err)
		return
	}
	setSessionCookie(w, r, sess)
	s.log.Info("oauth login", zap.String("provider", p.ID), zap.String("user", u.ID))
	to := "/me"
	switch {
	case st.Next != "":
		to = st.Next
	case st.Plan != "":
		to = "/plan/" + url.PathEscape(st.Plan)
	}
	http.Redirect(w, r, to, http.StatusFound)
}
