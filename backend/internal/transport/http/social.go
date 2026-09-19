package http

import (
	"net/http"
	"strconv"

	"racion/internal/i18n"
	"racion/internal/service"
)

// ── Лайки, избранное, комментарии ──────────────────────────────────────────

func viewerID(r *http.Request) string {
	if u := currentUser(r); u != nil {
		return u.ID
	}
	return ""
}

func (s *Server) setLike(on bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := requireUser(w, r)
		if u == nil {
			return
		}
		st, err := s.svc.Social.Like(r.Context(), u.ID, r.PathValue("id"), on)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		writeJSON(w, 200, st)
	}
}

func (s *Server) setFavorite(on bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := requireUser(w, r)
		if u == nil {
			return
		}
		st, err := s.svc.Social.Favorite(r.Context(), u.ID, r.PathValue("id"), on)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		writeJSON(w, 200, st)
	}
}

// setFeedback — «как было?»: {liked: true|false}. Из приложения и из кнопок push-уведомления.
func (s *Server) setFeedback(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var in struct {
		Liked bool `json:"liked"`
	}
	if !decode(w, r, 1<<10, &in) {
		return
	}
	if err := s.svc.Social.Feedback(r.Context(), u.ID, r.PathValue("id"), in.Liked); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) recipeStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.svc.Social.Stats(r.Context(), r.PathValue("id"), viewerID(r)))
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.Social.Comments(r.Context(), r.PathValue("id"), viewerID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Body  string `json:"body"`
		Image string `json:"image"`
	}
	if !decode(w, r, 8<<10, &body) {
		return
	}
	c, err := s.svc.Social.AddComment(r.Context(), *u, r.PathValue("id"), body.Body, body.Image)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, c)
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("comment"), 10, 64)
	if err != nil {
		writeErr(w, 404, "not found")
		return
	}
	if err := s.svc.Social.DeleteComment(r.Context(), u.ID, id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// favoriteView — избранный рецепт в кабинете: название на языке, приём, свой ли.
type favoriteView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Slot  string `json:"slot"`
	Own   bool   `json:"own"`
	Image string `json:"image,omitempty"`
}

func (s *Server) myFavorites(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	list, err := s.svc.Social.Favorites(r.Context(), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	lang := i18n.FromRequest(r)
	out := make([]favoriteView, 0, len(list))
	for _, rc := range list {
		out = append(out, favoriteView{ID: rc.ID, Title: rc.LocalTitle(lang), Slot: rc.Slot, Own: rc.Own, Image: rc.Image})
	}
	writeJSON(w, 200, out)
}

// setOwnPublic — открыть или закрыть свой рецепт для всех по ссылке.
func (s *Server) setOwnPublic(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Public bool `json:"public"`
	}
	if !decode(w, r, 1<<10, &body) {
		return
	}
	if body.Public {
		status, err := s.svc.Moderation.Publish(r.Context(), u.ID, r.PathValue("id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": status})
		return
	}
	if err := s.svc.Recipes.SetPublic(r.Context(), u.ID, r.PathValue("id"), false); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": service.StatusPrivate})
}
