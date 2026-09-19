package http

import (
	"net/http"
	"strings"

	"racion/internal/domain"
	"racion/internal/service"
)

// Партнёрские магазины страны: техника «где купить» и доставка продуктов для корзины. Кэш минуту — правки
// из админки доезжают быстро, а запросов с одной страницы несколько.
func (s *Server) partners(w http.ResponseWriter, r *http.Request) {
	country := strings.ToUpper(r.URL.Query().Get("country"))
	if len(country) != 2 {
		pl, _ := s.localeFromPath(r)
		country = pl.Country.Code
	}
	v, err := s.svc.Partners.ForCountry(r.Context(), country)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, 200, v)
}

func (s *Server) adminPartners(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermPartners) == nil {
		return
	}
	list, err := s.svc.Partners.AdminList(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) adminSavePartner(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermPartners) == nil {
		return
	}
	var p domain.Partner
	if !decode(w, r, 8<<10, &p) {
		return
	}
	saved, err := s.svc.Partners.Save(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, saved)
}

func (s *Server) adminDeletePartner(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermPartners) == nil {
		return
	}
	if err := s.svc.Partners.Delete(r.Context(), r.PathValue("code")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}
