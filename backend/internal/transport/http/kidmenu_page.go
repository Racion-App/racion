package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Страницы «Меню ребёнка в 1 год / 1,5 / 2 / 3 года на неделю»: /menu/child-1 и т. д. Неделю собирает
// планировщик, как для «готовых меню»: меню только для ребёнка этого возраста, детская база с граммами
// под возраст, пять приёмов (завтрак, обед, полдник, ужин и кефир перед сном), один список покупок.
// Правила и цифры — из Программы оптимизации питания детей 1–3 лет (НМИЦ здоровья детей, 2019) и
// МР 2.3.1.0253-21; для 3 лет — нормы группы 3–6 лет. Языки — как у тем (ru, en, de).

type kidMenuPreset struct {
	Slug string
	Age  int    // мес
	Key  string // подписи: kidpage.<key>.*
}

var kidMenuPresets = []kidMenuPreset{
	{"child-1", 12, "1"},
	{"child-1-5", 18, "15"},
	{"child-2", 24, "2"},
	{"child-3", 36, "3"},
}

func kidMenuBySlug(slug string) (kidMenuPreset, bool) {
	for _, m := range kidMenuPresets {
		if m.Slug == slug {
			return m, true
		}
	}
	return kidMenuPreset{}, false
}

func (s *Server) kidMenuPlan(m kidMenuPreset, pl pageLocale) planner.Plan {
	key := "kid|" + m.Slug + "|" + string(pl.L) + "|" + pl.Country.Code
	if v, ok := menuCache.Load(key); ok {
		if c := v.(menuCached); time.Since(c.at) < time.Hour {
			return c.plan
		}
	}
	p := planner.Params{Adults: 0, Kids: []planner.Child{{AgeMonths: m.Age, Feeding: planner.FeedSeparate}}, Goal: "none",
		Lang: string(pl.L), Country: pl.Country.Code, Equipment: []string{"stove", "oven", "microwave"},
		Slots: []string{"breakfast", "lunch", "dinner"}}
	plan := s.catalog.Build(p)
	menuCache.Store(key, menuCached{plan, time.Now()})
	return plan
}

func (s *Server) kidMenuPage(w http.ResponseWriter, r *http.Request, m kidMenuPreset, pl pageLocale) {
	l := pl.L
	base := s.baseURL(r)
	plan := s.kidMenuPlan(m, pl)
	if len(plan.KidsMenus) == 0 || len(plan.KidsMenus[0].Days) < 7 {
		s.notFoundPage(w, r)
		return
	}
	km := plan.KidsMenus[0]
	age := planner.Child{AgeMonths: m.Age}.AgeLabel(l)
	start := ""
	if len(plan.Days) > 0 {
		if t, err := time.Parse("2006-01-02", plan.Days[0].Date); err == nil {
			start = i18n.DayMonthYear(l, t.Day(), int(t.Month()), t.Year())
		}
	}

	type dishView struct {
		Slot, Title, Href, Kcal, Money, Note string
	}
	type dayView struct {
		Label, Kcal string
		Dishes      []dishView
	}
	var days []dayView
	dishes := 0
	var kcalSum float64
	for _, d := range km.Days {
		dv := dayView{Label: d.Label}
		var kcal float64
		for _, x := range d.Dishes {
			v := dishView{Slot: planner.SlotLabel(l, x.Slot), Title: x.Title, Note: x.Note}
			if x.RecipeID != "" {
				v.Href = pl.P + "/recipe/" + x.RecipeID
				v.Kcal = i18n.T(l, "recipe.kcal", strconv.Itoa(int(math.Round(x.Kcal))))
				v.Money = formatMoney(pl.Country, x.Cost)
				kcal += x.Kcal
				dishes++
			}
			dv.Dishes = append(dv.Dishes, v)
		}
		dv.Kcal = strconv.Itoa(int(math.Round(kcal)))
		kcalSum += kcal
		days = append(days, dv)
	}
	type groupView struct {
		Label, Money string
		Items        int
	}
	var groups []groupView
	for _, g := range plan.Shopping {
		if len(g.Items) > 0 {
			groups = append(groups, groupView{g.Label, formatMoney(pl.Country, g.Cost), len(g.Items)})
		}
	}
	// «съест за неделю» — то, что реально уходит в тарелку, без остатков упаковок: на одного малыша
	// покупки целыми пачками (масло, мука) заметно больше, их называем отдельно
	week := formatMoney(pl.Country, plan.Totals.UsedCost)
	perDay := formatMoney(pl.Country, plan.Totals.UsedCost/7)
	bought := formatMoney(pl.Country, plan.Totals.Cost)
	k := "kidpage." + m.Key
	h1 := i18n.T(l, k+".h1")
	title := i18n.T(l, k+".title")
	desc := i18n.T(l, "kidpage.desc", age, dishes, week)
	group := "toddler" // 1–3 года: программа питания 1–3 лет
	if m.Age >= 36 {
		group = "preschool"
	}
	intro := []string{i18n.T(l, k+".intro"), i18n.T(l, "kidpage.intro2", week, perDay, i18n.T(l, "country."+pl.Country.Code), bought, plan.Totals.Items)}
	var rules []string
	for i := 1; ; i++ {
		key := fmt.Sprintf("kidpage.%s.rule%d", group, i)
		if t := i18n.T(l, key); t != key {
			rules = append(rules, t)
			continue
		}
		break
	}
	if m.Age < 18 {
		rules = append([]string{i18n.T(l, "kidpage.rule.mash")}, rules...)
	} else if m.Age < 36 {
		rules = append([]string{i18n.T(l, "kidpage.rule.pieces")}, rules...)
	}
	var faq []domain.QA
	for _, f := range []string{"kcal", "meals", "table", "banned", "dairy"} {
		key := fmt.Sprintf("kidpage.faq.%s.%s", group, f)
		q := i18n.T(l, key+".q", age)
		if q == key+".q" {
			continue
		}
		faq = append(faq, domain.QA{Q: q, A: i18n.T(l, key+".a")})
	}
	if len(faq) > 0 {
		faq[0].A = planner.KidNorm(m.Age, l) // калории и белок — по норме возраста, одной строкой с источником
	}
	type link struct{ Name, Href string }
	var others []link
	for _, o := range kidMenuPresets {
		if o.Slug != m.Slug {
			others = append(others, link{i18n.T(l, "kidpage."+o.Key+".h1"), pl.P + "/menu/" + o.Slug})
		}
	}
	others = append(others, link{i18n.T(l, "weaning.page.h1"), pl.P + "/weaning"})
	var alts []altLink
	for _, al := range topicLangs {
		mt := i18n.Meta(al)
		alts = append(alts, altLink{Lang: string(al), Href: base + prefix(al) + "/menu/" + m.Slug, Name: mt.Name, English: mt.English, Flag: mt.Flag})
	}
	data := map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: title + " — " + i18n.T(l, "page.brand"), Description: desc, Canonical: base + pl.P + "/menu/" + m.Slug,
			OGImage: brandOG(base, l), OGWide: true, OGType: "article", Alternates: alts, JSONLD: kidMenuLD(base, pl, h1, desc, m.Slug, faq)},
		"L": l, "P": pl.P, "Country": pl.Country, "NavRecipes": true,
		"H1": h1, "Intro": intro, "Norm": planner.KidNorm(m.Age, l), "Rules": rules, "Days": days, "Groups": groups, "FAQ": faq, "Others": others,
		"Week": week, "PerDay": perDay, "KcalDay": int(math.Round(kcalSum / 7)), "Items": plan.Totals.Items, "Dishes": dishes, "Age": age,
		"StartDate": start, "PlanHref": "/?s=2&kid=" + strconv.Itoa(m.Age),
	}
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "kidmenu.html", data); err != nil {
		s.log.Error("kid menu page", zap.Error(err))
		http.Error(w, "template error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = buf.WriteTo(w)
}

func kidMenuLD(base string, pl pageLocale, name, desc, slug string, faq []domain.QA) template.JS {
	u := base + pl.P + "/menu/" + slug
	page := map[string]any{"@type": "WebPage", "@id": u, "url": u, "name": name, "description": desc, "inLanguage": string(pl.L)}
	crumbs := breadcrumbLD([][2]string{{i18n.T(pl.L, "page.brand"), base + "/"}, {name, ""}})
	var qs []map[string]any
	for _, x := range faq {
		qs = append(qs, map[string]any{"@type": "Question", "name": x.Q, "acceptedAnswer": map[string]any{"@type": "Answer", "text": strings.TrimSpace(x.A)}})
	}
	b, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": []any{page, crumbs, map[string]any{"@type": "FAQPage", "mainEntity": qs}}})
	return template.JS(b)
}

// kidMenuLinks — ссылки на меню по возрасту для страницы прикорма и подвала.
func kidMenuLinks(pl pageLocale) []struct{ Name, Href string } {
	var out []struct{ Name, Href string }
	for _, o := range kidMenuPresets {
		out = append(out, struct{ Name, Href string }{i18n.T(pl.L, "kidpage."+o.Key+".h1"), pl.P + "/menu/" + o.Slug})
	}
	return out
}
