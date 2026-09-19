package http

import (
	"net/http"
	"racion/internal/service"

	"racion/internal/i18n"
	"racion/internal/media"
)

// upload — фото: multipart-поле file, ?kind=recipe|comment|avatar → {url, thumb, w, h}.
// Файл пережимается в WebP на сервере, оригинал не хранится.
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if r.URL.Query().Get("kind") == "offer" && s.requirePerm(w, r, service.PermPartners) == nil {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUpload+64<<10)
	f, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, i18n.T(i18n.FromRequest(r), "photo.bad"))
		return
	}
	defer f.Close()
	p, err := s.svc.Media.Upload(r.Context(), u.ID, r.URL.Query().Get("kind"), f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, p)
}
