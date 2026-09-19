package http

import (
	"net/http"

	"racion/internal/domain"
	"racion/internal/service"
)

// Админка: рецепты базы (право recipes), модерация своих рецептов (moderation), роли (roles).

func (s *Server) requirePerm(w http.ResponseWriter, r *http.Request, perm string) *domain.User {
	u := currentUser(r)
	if u == nil || !s.svc.Admin.Can(u, perm) {
		writeErr(w, 404, "not found")
		return nil
	}
	return u
}

func (s *Server) adminRecipes(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermRecipes) == nil {
		return
	}
	writeJSON(w, 200, map[string]any{"items": s.svc.CatalogAdmin.Search(r.URL.Query().Get("q"), 40), "tags": s.svc.CatalogAdmin.Tags()})
}

func (s *Server) adminRecipe(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermRecipes) == nil {
		return
	}
	rc, err := s.svc.CatalogAdmin.Get(r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, rc)
}

func (s *Server) adminSaveRecipe(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermRecipes) == nil {
		return
	}
	var in domain.CatalogRecipeInput
	if !decode(w, r, 128<<10, &in) {
		return
	}
	rc, err := s.svc.CatalogAdmin.Save(r.Context(), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.notifySearch("/recipe/"+rc.ID, "/recipes")
	writeJSON(w, 200, rc)
}

func (s *Server) adminDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermRecipes) == nil {
		return
	}
	if err := s.svc.CatalogAdmin.Delete(r.Context(), r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) adminModeration(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermModeration) == nil {
		return
	}
	queue, err := s.svc.Moderation.Queue(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	recent, _ := s.svc.Moderation.Approved(r.Context(), 20)
	if recent == nil {
		recent = queue[:0]
	}
	writeJSON(w, 200, map[string]any{"queue": queue, "recent": recent})
}

func (s *Server) adminDecide(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermModeration) == nil {
		return
	}
	var in struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if !decode(w, r, 4<<10, &in) {
		return
	}
	if in.Approve {
		s.notifySearch("/recipe/" + r.PathValue("id"))
	}
	if err := s.svc.Moderation.Decide(r.Context(), r.PathValue("id"), in.Approve, in.Note); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) adminSetRole(w http.ResponseWriter, r *http.Request) {
	u := s.requirePerm(w, r, service.PermRoles)
	if u == nil {
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if !decode(w, r, 1<<10, &in) {
		return
	}
	if err := s.svc.Admin.SetRole(r.Context(), *u, r.PathValue("id"), in.Role); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// suggestionOwn — решение автора по подробной версии от нейросети: {accept: true|false}.
func (s *Server) suggestionOwn(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var in struct {
		Accept bool `json:"accept"`
	}
	if !decode(w, r, 1<<10, &in) {
		return
	}
	status, err := s.svc.Moderation.Suggestion(r.Context(), u.ID, r.PathValue("id"), in.Accept)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": status})
}

// publishOwn — автор отправляет свой рецепт на публикацию; в ответ — статус проверки.
func (s *Server) publishOwn(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	status, err := s.svc.Moderation.Publish(r.Context(), u.ID, r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": status})
}
