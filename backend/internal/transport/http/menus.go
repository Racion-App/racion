package http

import (
	"bytes"
	"encoding/json"
	"html/template"
	"math"
	"net/http"
	"sort"
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

// menuMoved — страницы, которые слились с подборками своих тем: один адрес на тему, без дублей в поиске.
var menuMoved = map[string]string{"new-year": "new-year-table", "post": "lent-menu"}

func menuBySlug(slug string) (menuPreset, bool) {
	for _, m := range menuPresets {
		if m.Slug == slug {
			return m, true
		}
	}
	return menuPreset{}, false
}

// menuSlots — три приёма пищи готовых недель: колонки таблицы недели на странице.
var menuSlots = []string{"breakfast", "lunch", "dinner"}

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
	p.Slots = menuSlots
	plan := s.catalog.Build(p)
	menuCache.Store(key, menuCached{plan, time.Now()})
	return plan
}

func (s *Server) menuPage(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.stableLocale(r)
	if !ok || !topicLang(pl.L) {
		s.notFoundPage(w, r)
		return
	}
	if km, ok := kidMenuBySlug(r.PathValue("slug")); ok {
		s.kidMenuPage(w, r, km, pl) // «Меню ребёнка в N лет» — своя страница с детской неделей
		return
	}
	if to, ok := menuMoved[r.PathValue("slug")]; ok {
		http.Redirect(w, r, pl.P+"/collection/"+to, http.StatusMovedPermanently)
		return
	}
	m, ok := menuBySlug(r.PathValue("slug"))
	if !ok {
		s.notFoundPage(w, r)
		return
	}
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

	// Неделя таблицей: строка — день, колонка — приём пищи; у блюда фото из каталога, если оно есть.
	type dishView struct {
		Slot, Title, Href, Image, Money, Time string
		Leftover, Batch                       bool
	}
	type cellView struct {
		Key    string
		Dishes []dishView
	}
	type dayView struct {
		Label, Date, Money, Kcal, Cook string
		Cells                          []cellView
	}
	type slotHead struct{ Key, Label string }
	var heads []slotHead
	for _, k := range menuSlots {
		heads = append(heads, slotHead{k, planner.SlotLabel(l, k)})
	}
	cat := s.svc.Catalog.Base()
	var days []dayView
	dishes := 0
	for _, d := range plan.Days {
		dv := dayView{Label: d.Label, Date: d.Date, Money: formatMoney(pl.Country, d.Cost), Kcal: strconv.Itoa(int(math.Round(d.Kcal))), Cook: i18n.Minutes(l, d.CookMin)}
		if t, err := time.Parse("2006-01-02", d.Date); err == nil {
			dv.Date = dayMonth(l, t)
		}
		for _, k := range menuSlots {
			dv.Cells = append(dv.Cells, cellView{Key: k})
		}
		for _, x := range d.Dishes {
			dishes++
			img := ""
			if rc, ok := cat.RecipeByID[x.RecipeID]; ok {
				img = rc.Image
			}
			v := dishView{Slot: planner.SlotLabel(l, x.Slot), Title: x.Title, Href: pl.P + "/recipe/" + x.RecipeID, Image: img,
				Money: formatMoney(pl.Country, x.Cost), Time: i18n.Minutes(l, x.TimeMin), Leftover: x.Leftover, Batch: x.Batch}
			i := len(dv.Cells) - 1 // приём вне трёх колонок — к ужину, чтобы блюдо не потерялось
			for j, c := range dv.Cells {
				if c.Key == x.Slot {
					i = j
				}
			}
			dv.Cells[i].Dishes = append(dv.Cells[i].Dishes, v)
		}
		days = append(days, dv)
	}

	// Чек недели: отделы магазина от дорогих к дешёвым; то, что обычно есть дома (соль, масло, специи),
	// в сумму не входит — в чеке у такого отдела вместо цены «дома».
	type checkLine struct {
		Label, Money string
		Items        int
		cost         float64
	}
	var lines []checkLine
	var groups []feastGroup
	for _, g := range plan.Shopping {
		if len(g.Items) == 0 {
			continue
		}
		cl := checkLine{Label: g.Label, Items: len(g.Items), cost: g.Cost}
		if g.Cost >= 0.5 {
			cl.Money = formatMoney(pl.Country, g.Cost)
		}
		lines = append(lines, cl)
		fg := feastGroup{Label: g.Label, Money: cl.Money}
		for _, it := range g.Items {
			fg.Items = append(fg.Items, feastItem{Name: it.Name, Qty: formatQty(l, it.Buy, it.Unit)})
		}
		groups = append(groups, fg)
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].cost > lines[j].cost })

	// Экономное меню рядом с обычной неделей на тех же двоих: разница — главный факт страницы.
	type saving struct{ Regular, Week, Month, Href, Label string }
	var save *saving
	if m.Slug == "budget" {
		if reg, ok := menuBySlug("family-2"); ok {
			rp := s.menuPlan(reg, pl)
			if d := rp.Totals.Cost - plan.Totals.Cost; d > 0 && len(rp.Days) == 7 {
				save = &saving{Regular: formatMoney(pl.Country, rp.Totals.Cost), Week: formatMoney(pl.Country, d),
					Month: formatMoney(pl.Country, math.Round(d*30/7/100)*100), Href: pl.P + "/menu/family-2", Label: i18n.T(l, "menu.family-2.h1")}
			}
		}
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
	others = append(others, link{i18n.T(l, "feast.link.newyear"), pl.P + "/collection/new-year-table"}, link{i18n.T(l, "feast.link.lent"), pl.P + "/collection/lent-menu"})
	for _, o := range kidMenuPresets {
		others = append(others, link{i18n.T(l, "kidpage."+o.Key+".h1"), pl.P + "/menu/" + o.Slug})
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
		"H1": h1, "Intro": intro, "Days": days, "Heads": heads, "Lines": lines, "Groups": groups, "Save": save, "FAQ": faq, "Others": others,
		"Stores": s.feastStores(plan, pl), "CountryName": i18n.T(l, "country."+pl.Country.Code),
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
