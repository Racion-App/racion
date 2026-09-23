package http

import (
	"net/http"
	"time"

	"racion/internal/i18n"
	"racion/internal/planner"
)

// События: список с подписями на языке и сборка меню.

type occasionView struct {
	ID        string                  `json:"id"`
	Icon      string                  `json:"icon"`
	Title     string                  `json:"title"`
	Lead      string                  `json:"lead"`
	Guests    int                     `json:"guests"`
	Kind      string                  `json:"kind"`
	Season    bool                    `json:"season"` // актуально сейчас
	Preset    *planner.OccasionPreset `json:"preset,omitempty"`
	Courses   []string                `json:"courses"`
	Countries []string                `json:"countries"` // где отмечают; пусто — везде
}

func (s *Server) occasions(w http.ResponseWriter, r *http.Request) {
	l := i18n.FromRequest(r)
	now := time.Now()
	m := int(now.Month())
	out := []occasionView{}
	for _, o := range planner.Occasions(now) {
		v := occasionView{ID: o.ID, Icon: o.Icon, Title: i18n.T(l, "occasion."+o.ID+".title"), Lead: i18n.T(l, "occasion."+o.ID+".lead"), Guests: o.Guests, Kind: o.Kind, Preset: o.Preset, Countries: o.Countries}
		if v.Kind == "" {
			v.Kind = "menu"
		}
		for _, om := range o.Months {
			if om == m {
				v.Season = true
			}
		}
		for _, c := range o.Courses {
			v.Courses = append(v.Courses, i18n.T(l, "course."+c.Key))
		}
		if v.Courses == nil {
			v.Courses = []string{}
		}
		if v.Countries == nil {
			v.Countries = []string{}
		}
		out = append(out, v)
	}
	w.Header().Set("Cache-Control", "public, max-age=600")
	writeJSON(w, 200, out)
}

func (s *Server) createOccasion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Guests int            `json:"guests"`
		Params planner.Params `json:"params"`
	}
	if !decode(w, r, 32<<10, &body) {
		return
	}
	plan, err := s.svc.Plans.CreateOccasion(r.Context(), r.PathValue("id"), body.Params, body.Guests, langOf(r, body.Params.Lang), currentUser(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, plan)
}

// createBasket — стол из корзины: блюда выбрал человек, каждое со своим числом порций.
func (s *Server) createBasket(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Guests int                  `json:"guests"`
		Items  []planner.BasketItem `json:"items"`
		Params planner.Params       `json:"params"`
	}
	if !decode(w, r, 32<<10, &body) {
		return
	}
	if len(body.Items) == 0 || len(body.Items) > planner.BasketMax {
		writeErr(w, 400, "bad basket")
		return
	}
	plan, err := s.svc.Plans.CreateBasket(r.Context(), body.Items, body.Params, body.Guests, langOf(r, body.Params.Lang), currentUser(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, plan)
}
