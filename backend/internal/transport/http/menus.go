package http

import (
	"bytes"
	"encoding/json"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Страницы «Меню на неделю для семьи из N человек»: /menu/{slug}. Планировщик собирает настоящую неделю
// по пресету (состав семьи, цель, бюджет) с ценами страны читателя; неделя воспроизводима (seed из
// параметров и даты понедельника), поэтому страница стабильна семь дней и обновляется каждую неделю.
// Языки как у тем: ru, en, de.

type menuPreset struct {
	Slug   string
	Params func(country planner.Country) planner.Params
}

var menuPresets = []menuPreset{
	{"family-4", func(cy planner.Country) planner.Params {
		return planner.Params{Adults: 2, Kids: []planner.Child{{AgeMonths: 60, Feeding: "shared"}, {AgeMonths: 120, Feeding: "shared"}}, Goal: "none"}
	}},
	{"family-3", func(cy planner.Country) planner.Params {
		return planner.Params{Adults: 2, Kids: []planner.Child{{AgeMonths: 72, Feeding: "shared"}}, Goal: "none"}
	}},
	{"family-2", func(cy planner.Country) planner.Params { return planner.Params{Adults: 2, Goal: "none"} }},
	{"lose", func(cy planner.Country) planner.Params { return planner.Params{Adults: 1, Goal: "lose"} }},
	{"pp", func(cy planner.Country) planner.Params { return planner.Params{Adults: 1, Goal: "healthy"} }},
	{"budget", func(cy planner.Country) planner.Params {
		return planner.Params{Adults: 2, Goal: "none", BudgetMode: "perPersonDay", BudgetValue: math.Round(cy.Default * 0.7)}
	}},
}

func menuBySlug(slug string) (menuPreset, bool) {
	for _, m := range menuPresets {
		if m.Slug == slug {
			return m, true
		}
	}
	return menuPreset{}, false
}

// кэш недель: slug+lang+country → план на час (сборка недели дорогая для страницы под поисковик)
var menuCache sync.Map

type menuCached struct {
	plan planner.Plan
	at   time.Time
}

func (s *Server) menuPlan(m menuPreset, pl pageLocale) planner.Plan {
	key := m.Slug + "|" + string(pl.L) + "|" + pl.Country.Code
	if v, ok := menuCache.Load(key); ok {
		if c := v.(menuCached); time.Since(c.at) < time.Hour {
			return c.plan
		}
	}
	p := m.Params(pl.Country)
	p.Lang = string(pl.L)
	p.Country = pl.Country.Code
	p.Equipment = []string{"stove", "oven", "microwave"}
	p.Slots = []string{"breakfast", "lunch", "dinner"}
	plan := s.catalog.Build(p)
	menuCache.Store(key, menuCached{plan, time.Now()})
	return plan
}

func (s *Server) menuPage(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.localeFromPath(r)
	if !ok || !topicLang(pl.L) {
		s.notFoundPage(w, r)
		return
	}
	m, ok := menuBySlug(r.PathValue("slug"))
	if !ok {
		s.notFoundPage(w, r)
		return
	}
	rememberCountry(w, r)
	l := pl.L
	base := s.baseURL(r)
	plan := s.menuPlan(m, pl)
	if len(plan.Days) < 7 {
		s.notFoundPage(w, r)
		return
	}
	// «на человека» считаем по головам, а не по порциям планировщика: дети едят меньше, но это всё же люди
	people := plan.Params.Adults + len(plan.Params.Kids)
	if people <= 0 {
		people = 1
	}
	perPersonDay := plan.Totals.Cost / float64(people) / 7
	start := plan.Days[0].Date
	if t, err := time.Parse("2006-01-02", start); err == nil {
		start = i18n.DayMonthYear(l, t.Day(), int(t.Month()), t.Year())
	}

	type dishView struct {
		Slot, Title, Href, Money, KcalLabel, Time string
		Leftover, Batch                           bool
	}
	type dayView struct {
		Label, Money, Kcal, Cook string
		Dishes                   []dishView
	}
	var days []dayView
	dishes := 0
	for _, d := range plan.Days {
		dv := dayView{Label: d.Label, Money: formatMoney(pl.Country, d.Cost), Kcal: strconv.Itoa(int(math.Round(d.Kcal))), Cook: i18n.Minutes(l, d.CookMin)}
		for _, x := range d.Dishes {
			dishes++
			dv.Dishes = append(dv.Dishes, dishView{Slot: planner.SlotLabel(l, x.Slot), Title: x.Title, Href: pl.P + "/recipe/" + x.RecipeID,
				Money: formatMoney(pl.Country, x.Cost), KcalLabel: i18n.T(l, "recipe.kcal", strconv.Itoa(int(math.Round(x.Kcal)))), Time: i18n.Minutes(l, x.TimeMin), Leftover: x.Leftover, Batch: x.Batch})
		}
		days = append(days, dv)
	}
	type groupView struct {
		Label, Money string
		Items        int
	}
	var groups []groupView
	for _, g := range plan.Shopping {
		if len(g.Items) == 0 {
			continue
		}
		groups = append(groups, groupView{g.Label, formatMoney(pl.Country, g.Cost), len(g.Items)})
	}
	h1 := i18n.T(l, "menu."+m.Slug+".h1")
	title := i18n.T(l, "menu."+m.Slug+".title", formatMoney(pl.Country, plan.Totals.Cost))
	desc := i18n.T(l, "menu.desc", dishes, formatMoney(pl.Country, plan.Totals.Cost), formatMoney(pl.Country, perPersonDay), int(math.Round(plan.Totals.KcalPerDay)), plan.Totals.Items)
	intro := []string{
		i18n.T(l, "menu."+m.Slug+".intro"),
		i18n.T(l, "menu.intro2", formatMoney(pl.Country, plan.Totals.Cost), people, formatMoney(pl.Country, perPersonDay), int(math.Round(plan.Totals.KcalPerDay)), plan.Totals.Items, i18n.T(l, "country."+pl.Country.Code)),
	}
	faq := []domain.QA{
		{Q: i18n.T(l, "menu.faq.cost.q", people), A: i18n.T(l, "menu.faq.cost.a", formatMoney(pl.Country, plan.Totals.Cost), people, formatMoney(pl.Country, perPersonDay), i18n.T(l, "country."+pl.Country.Code))},
		{Q: i18n.T(l, "menu.faq.change.q"), A: i18n.T(l, "menu.faq.change.a")},
		{Q: i18n.T(l, "menu.faq.list.q"), A: i18n.T(l, "menu.faq.list.a", plan.Totals.Items, len(groups))},
		{Q: i18n.T(l, "menu.faq.fresh.q"), A: i18n.T(l, "menu.faq.fresh.a")},
	}
	type link struct{ Name, Href string }
	var others []link
	for _, o := range menuPresets {
		if o.Slug != m.Slug {
			others = append(others, link{i18n.T(l, "menu."+o.Slug+".h1"), pl.P + "/menu/" + o.Slug})
		}
	}
	var alts []altLink
	for _, al := range topicLangs {
		mt := i18n.Meta(al)
		alts = append(alts, altLink{Lang: string(al), Href: base + prefix(al) + "/menu/" + m.Slug, Name: mt.Name, English: mt.English, Flag: mt.Flag})
	}
	data := map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: title + " — " + i18n.T(l, "page.brand"), Description: desc, Canonical: base + pl.P + "/menu/" + m.Slug,
			OGImage: brandOG(base, l), OGWide: true, OGType: "article", Alternates: alts, JSONLD: menuLD(base, pl, h1, desc, m.Slug, faq)},
		"L": l, "P": pl.P, "Country": pl.Country, "NavRecipes": true,
		"H1": h1, "Intro": intro, "Days": days, "Groups": groups, "FAQ": faq, "Others": others,
		"Week": formatMoney(pl.Country, plan.Totals.Cost), "PerDay": formatMoney(pl.Country, perPersonDay), "KcalDay": int(math.Round(plan.Totals.KcalPerDay)), "Items": plan.Totals.Items, "People": people, "Dishes": dishes,
		"StartDate": start, "PlanHref": "/?s=1",
	}
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "menu.html", data); err != nil {
		s.log.Error("menu page", zap.Error(err))
		http.Error(w, "template error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = buf.WriteTo(w)
}

func menuLD(base string, pl pageLocale, name, desc, slug string, faq []domain.QA) template.JS {
	u := base + pl.P + "/menu/" + slug
	page := map[string]any{"@type": "WebPage", "@id": u, "url": u, "name": name, "description": desc, "inLanguage": string(pl.L)}
	crumbs := breadcrumbLD([][2]string{{i18n.T(pl.L, "page.brand"), base + "/"}, {name, ""}})
	var qs []map[string]any
	for _, x := range faq {
		qs = append(qs, map[string]any{"@type": "Question", "name": x.Q, "acceptedAnswer": map[string]any{"@type": "Answer", "text": x.A}})
	}
	b, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": []any{page, crumbs, map[string]any{"@type": "FAQPage", "mainEntity": qs}}})
	return template.JS(b)
}
