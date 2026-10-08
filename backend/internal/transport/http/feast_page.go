package http

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// «Новогодний стол»: /menu/new-year. Не неделя, а стол события newyear (occasions.json), собранный тем же
// планировщиком, что и в приложении: на 4, 6 и 8 человек и недорогой на шестерых. У каждого варианта курсы
// с фото, цена стола и на гостя, полный список покупок по отделам. Варианты — вкладки на радиокнопках, все
// в HTML. Языки — как у тематических страниц.

type feastVariant struct {
	Key     string
	Guests  int
	Thrifty bool
}

var feastVariants = []feastVariant{{"4", 4, false}, {"6", 6, false}, {"8", 8, false}, {"cheap", 6, true}}

// кэш столов на час: вариант+язык+страна (сборка стола — проход по всему каталогу)
var feastCache sync.Map

func (s *Server) feastPlan(v feastVariant, pl pageLocale) (planner.Plan, bool) {
	key := v.Key + "|" + string(pl.L) + "|" + pl.Country.Code
	if c, ok := feastCache.Load(key); ok && time.Since(c.(menuCached).at) < time.Hour {
		return c.(menuCached).plan, true
	}
	o, ok := planner.OccasionByID("newyear")
	if !ok {
		return planner.Plan{}, false
	}
	p := planner.Params{Lang: string(pl.L), Country: pl.Country.Code, Equipment: []string{"stove", "oven", "microwave"}, Thrifty: v.Thrifty}
	plan, err := s.catalog.BuildOccasion(o, p, v.Guests)
	if err != nil || len(plan.Days) == 0 {
		return planner.Plan{}, false
	}
	feastCache.Store(key, menuCached{plan, time.Now()})
	return plan, true
}

type feastDish struct{ Title, Href, Image, Money string }

type feastCourse struct {
	Title  string
	Dishes []feastDish
}

type feastItem struct{ Name, Qty, Money string }

type feastGroup struct {
	Label, Money string
	Items        []feastItem
}

type feastView struct {
	Key, Tab, Title, Total, PerGuest, Href string
	DishesWord                             string // «блюд», «блюдо», «блюда» под число
	Guests, Dishes, Items, Salads          int
	Courses                                []feastCourse
	Groups                                 []feastGroup
	Checked                                bool
}

// newYearYear — какой Новый год встречаем: в январе — только что наступивший, дальше — следующий.
func newYearYear(now time.Time) int {
	if now.Month() == time.January {
		return now.Year()
	}
	return now.Year() + 1
}

func (s *Server) feastPage(w http.ResponseWriter, r *http.Request, pl pageLocale) {
	l := pl.L
	base := s.baseURL(r)
	var views []feastView
	for _, v := range feastVariants {
		plan, ok := s.feastPlan(v, pl)
		if !ok {
			continue
		}
		fv := feastView{Key: v.Key, Guests: v.Guests, Checked: v.Key == "6", Items: plan.Totals.Items,
			Total: formatMoney(pl.Country, plan.Totals.Cost), PerGuest: formatMoney(pl.Country, plan.Totals.Cost/float64(v.Guests)),
			Href: pl.P + "/event/newyear?guests=" + strconv.Itoa(v.Guests)}
		if v.Thrifty {
			fv.Tab, fv.Title = i18n.T(l, "feast.tab.cheap"), i18n.T(l, "feast.table.cheap", v.Guests, i18n.Plural(l, v.Guests, "feast.people"))
		} else {
			fv.Tab, fv.Title = i18n.T(l, "feast.tab.guests", v.Guests), i18n.T(l, "feast.table", v.Guests, i18n.Plural(l, v.Guests, "feast.people"))
		}
		for _, d := range plan.Days[0].Dishes {
			if len(fv.Courses) == 0 || fv.Courses[len(fv.Courses)-1].Title != i18n.T(l, "course."+d.Course) {
				fv.Courses = append(fv.Courses, feastCourse{Title: i18n.T(l, "course."+d.Course)})
			}
			img := ""
			if rc, ok := s.catalog.RecipeByID[d.RecipeID]; ok {
				img = rc.Image
			}
			c := &fv.Courses[len(fv.Courses)-1]
			c.Dishes = append(c.Dishes, feastDish{Title: d.Title, Href: pl.P + "/recipe/" + d.RecipeID, Image: img, Money: formatMoney(pl.Country, d.Cost)})
			fv.Dishes++
			if d.Course == "salads" {
				fv.Salads++
			}
		}
		for _, g := range plan.Shopping {
			fg := feastGroup{Label: g.Label, Money: formatMoney(pl.Country, g.Cost)}
			for _, it := range g.Items {
				if it.Pantry {
					continue // соль, масло и специи обычно дома, в цене стола их нет
				}
				fg.Items = append(fg.Items, feastItem{Name: it.Name, Qty: formatQty(l, it.Buy, it.Unit), Money: formatMoney(pl.Country, it.Cost)})
			}
			if len(fg.Items) > 0 {
				fv.Groups = append(fv.Groups, fg)
			}
		}
		fv.DishesWord = i18n.Plural(l, fv.Dishes, "feast.dish")
		views = append(views, fv)
	}
	if len(views) == 0 {
		s.notFoundPage(w, r)
		return
	}
	def := views[0]
	for _, v := range views {
		if v.Checked {
			def = v
		}
	}
	cheap := def
	for _, v := range views {
		if v.Key == "cheap" {
			cheap = v
		}
	}
	salads := map[string]int{}
	for _, v := range views {
		salads[v.Key] = v.Salads
	}

	year := strconv.Itoa(newYearYear(time.Now().In(moscow)))
	country := i18n.T(l, "country."+pl.Country.Code)
	h1 := i18n.T(l, "feast.h1", year)
	title := i18n.T(l, "feast.title", year)
	desc := i18n.T(l, "feast.desc", year, def.Guests, def.Total, def.PerGuest, def.Items)
	faq := []domain.QA{
		{Q: i18n.T(l, "feast.faq.cost.q", def.Guests), A: i18n.T(l, "feast.faq.cost.a", def.Guests, def.Total, def.PerGuest, country, cheap.Total)},
		{Q: i18n.T(l, "feast.faq.salads.q"), A: i18n.T(l, "feast.faq.salads.a", salads["4"], salads["6"], salads["8"])},
		{Q: i18n.T(l, "feast.faq.ahead.q"), A: i18n.T(l, "feast.faq.ahead.a")},
		{Q: i18n.T(l, "feast.faq.own.q"), A: i18n.T(l, "feast.faq.own.a")},
	}
	type link struct{ Name, Href string }
	others := []link{
		{i18n.T(l, "feast.coll.newyear"), pl.P + "/collection/new-year-table"},
		{i18n.T(l, "feast.coll.christmas"), pl.P + "/collection/christmas-table"},
		{i18n.T(l, "menu.post.h1"), pl.P + "/menu/post"},
		{i18n.T(l, "menu.family-4.h1"), pl.P + "/menu/family-4"},
	}
	var alts []altLink
	for _, al := range topicLangs {
		mt := i18n.Meta(al)
		alts = append(alts, altLink{Lang: string(al), Href: base + prefix(al) + "/menu/new-year", Name: mt.Name, English: mt.English, Flag: mt.Flag})
	}
	data := map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: title + " — " + i18n.T(l, "page.brand"), Description: desc, Canonical: base + pl.P + "/menu/new-year",
			OGImage: brandOG(base, l), OGWide: true, OGType: "article", Alternates: alts, JSONLD: feastLD(base, pl, h1, desc, def, faq)},
		"L": l, "P": pl.P, "Country": pl.Country, "NavRecipes": true,
		"H1": h1, "Lead": i18n.T(l, "feast.lead", country), "Views": views, "FAQ": faq, "Others": others,
	}
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "feast.html", data); err != nil {
		s.log.Error("feast page", zap.Error(err))
		http.Error(w, "template error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = buf.WriteTo(w)
}

// feastLD — страница, блюда стола по умолчанию списком, вопросы-ответы и крошки.
func feastLD(base string, pl pageLocale, name, desc string, v feastView, faq []domain.QA) template.JS {
	u := base + pl.P + "/menu/new-year"
	var items []map[string]any
	for _, c := range v.Courses {
		for _, d := range c.Dishes {
			items = append(items, map[string]any{"@type": "ListItem", "position": len(items) + 1, "url": base + d.Href, "name": d.Title})
		}
	}
	var qs []map[string]any
	for _, x := range faq {
		qs = append(qs, map[string]any{"@type": "Question", "name": x.Q, "acceptedAnswer": map[string]any{"@type": "Answer", "text": x.A}})
	}
	graph := []any{
		map[string]any{"@type": "WebPage", "@id": u, "url": u, "name": name, "description": desc, "inLanguage": string(pl.L),
			"mainEntity": map[string]any{"@type": "ItemList", "numberOfItems": len(items), "itemListElement": items}},
		breadcrumbLD([][2]string{{i18n.T(pl.L, "page.brand"), base + "/"}, {name, ""}}),
		map[string]any{"@type": "FAQPage", "mainEntity": qs},
	}
	b, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph})
	return template.JS(b)
}
