package http

import (
	"net/http"

	"racion/internal/service"
)

// ── Семья: состав и аккаунты ───────────────────────────────────────────────

func (s *Server) getFamily(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	v, err := s.svc.Family.Get(r.Context(), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) saveFamily(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var in service.FamilyInput
	if !decode(w, r, 64<<10, &in) {
		return
	}
	v, err := s.svc.Family.Save(r.Context(), u.ID, in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) familyInvite(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	tok, err := s.svc.Family.Invite(r.Context(), u.ID, r.URL.Query().Get("reset") == "1")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"token": tok})
}

func (s *Server) familyJoin(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if !decode(w, r, 1<<10, &body) {
		return
	}
	v, err := s.svc.Family.Join(r.Context(), u.ID, body.Token)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) familyLeave(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if err := s.svc.Family.Leave(r.Context(), u.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) familyRemove(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if err := s.svc.Family.Remove(r.Context(), u.ID, r.PathValue("user")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}
