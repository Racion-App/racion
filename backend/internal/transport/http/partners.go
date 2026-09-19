package http

import (
	"net/http"
	"strings"

	"racion/internal/domain"
	"racion/internal/geo"
	"racion/internal/planner"
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
	if !s.svc.Ads.Enabled(r.Context()) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		writeJSON(w, 200, service.CountryView{Goods: []domain.Partner{}, Grocery: []domain.Partner{}, Queries: map[string]string{}})
		return
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

// Рекламные предложения для контекста: place=cart|recipe|plan, country, region (код Росстата из плана;
// без него — регион по IP), match — id техники и продуктов страницы через запятую.
func (s *Server) offers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	country := strings.ToUpper(q.Get("country"))
	if len(country) != 2 {
		pl, _ := s.localeFromPath(r)
		country = pl.Country.Code
	}
	oq := service.OfferQuery{Country: country, Place: q.Get("place"), Region: q.Get("region")}
	if m := strings.TrimSpace(q.Get("match")); m != "" {
		oq.Match = strings.Split(m, ",")
	}
	s.fillRegion(r, &oq)
	w.Header().Set("Cache-Control", "private, max-age=60")
	if !s.svc.Ads.Enabled(r.Context()) {
		writeJSON(w, 200, []domain.Offer{})
		return
	}
	writeJSON(w, 200, s.svc.Offers.Pick(r.Context(), oq, 1))
}

// fillRegion — регион для таргетинга: переданный код или регион по IP; родитель города — его область.
func (s *Server) fillRegion(r *http.Request, oq *service.OfferQuery) {
	cy := planner.CountryOf(oq.Country)
	if !cy.HasRegions {
		return
	}
	regions := s.svc.Catalog.Regions()
	if oq.Region == "" && s.geo != nil {
		if pl := s.geo.Place(geo.ClientIP(r)); pl.Country == cy.Code {
			oq.Region = planner.RegionByPlace(regions, pl.Region, pl.City)
		}
	}
	for _, rg := range regions {
		if rg.Code == oq.Region {
			oq.Parent = rg.Parent
			break
		}
	}
}

func (s *Server) adminOffers(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermPartners) == nil {
		return
	}
	list, err := s.svc.Offers.AdminList(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) adminSaveOffer(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermPartners) == nil {
		return
	}
	var o domain.Offer
	if !decode(w, r, 16<<10, &o) {
		return
	}
	saved, err := s.svc.Offers.Save(r.Context(), o)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, saved)
}

func (s *Server) adminDeleteOffer(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermPartners) == nil {
		return
	}
	if err := s.svc.Offers.Delete(r.Context(), r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// Общий выключатель рекламы: GET → {enabled}, PUT {enabled} — только с правом partners.
func (s *Server) adminAds(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermPartners) == nil {
		return
	}
	if r.Method == http.MethodPut {
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if !decode(w, r, 1<<10, &in) {
			return
		}
		if err := s.svc.Ads.SetEnabled(r.Context(), in.Enabled); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	writeJSON(w, 200, map[string]bool{"enabled": s.svc.Ads.Enabled(r.Context())})
}
