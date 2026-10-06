package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// weaningPage — /weaning: прикорм по месяцам. Таблица объёмов из программы вскармливания 2019, порядок
// введения, правила, примерный день на каждый месяц со ссылками на рецепты прикорма и кнопка «собрать
// неделю». Языки — как у тематических страниц (ru, en, de): текст про здоровье детей машинным переводом
// на остальные не публикуем.
func (s *Server) weaningPage(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.stableLocale(r)
	if !ok || !topicLang(pl.L) {
		s.notFoundPage(w, r)
		return
	}
	l := pl.L
	base := s.baseURL(r)
	cat := s.svc.Catalog.Base()

	type row struct {
		Label string
		Cells [5]string
	}
	var table []row
	for _, rr := range planner.WeaningTable() {
		table = append(table, row{i18n.T(l, "weaning.row."+rr.Group), rr.Cells})
	}

	// порядок введения: продукты по месяцам, с которых их вводят, со ссылками на рецепты
	type food struct{ Name, Href string }
	type step struct {
		Month int
		Label string
		Foods []food
	}
	var order []step
	for _, f := range planner.WeaningFoods {
		name := cat.WeaningName(f.ID, l)
		if f.Group == "yolk" {
			name = i18n.T(l, "weaning.yolk")
		}
		href := ""
		if f.Recipe != "" {
			if _, ok := cat.RecipeByID[f.Recipe]; ok {
				href = pl.P + "/recipe/" + f.Recipe
			}
		}
		if len(order) == 0 || order[len(order)-1].Month != f.From {
			order = append(order, step{Month: f.From, Label: i18n.T(l, "weaning.page.from", f.From)})
		}
		order[len(order)-1].Foods = append(order[len(order)-1].Foods, food{name, href})
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i].Month < order[j].Month })

	// примерный день на 6–11 месяцев
	type day struct {
		Month int
		Title string
		Feeds []planner.WeaningFeed
	}
	var days []day
	for _, m := range []int{6, 7, 8, 9} { // с 9 до 12 месяцев объёмы те же, один день на всех
		smp := cat.WeaningSample(m, l)
		if len(smp.Days) == 0 {
			continue
		}
		feeds := smp.Days[0].Feeds
		for i := range feeds {
			for j := range feeds[i].Items {
				if id := feeds[i].Items[j].Recipe; id != "" {
					if _, ok := cat.RecipeByID[id]; ok {
						feeds[i].Items[j].Recipe = pl.P + "/recipe/" + id
						continue
					}
				}
				feeds[i].Items[j].Recipe = ""
			}
		}
		label := fmt.Sprint(m)
		if m == 9 {
			label = "9–12"
		}
		days = append(days, day{m, i18n.T(l, "weaning.page.day", label), feeds})
	}

	// рецепты прикорма — по возрасту
	var recipes []planner.Recipe
	for _, rc := range cat.Recipes {
		if strings.HasPrefix(rc.ID, "wean_") && !rc.Hidden {
			recipes = append(recipes, rc)
		}
	}
	age := func(rc planner.Recipe) int {
		for _, t := range rc.Tags {
			var n int
			if k, _ := fmt.Sscanf(t, "age%d", &n); k == 1 {
				return n
			}
		}
		return 12
	}
	sort.SliceStable(recipes, func(i, j int) bool { return age(recipes[i]) < age(recipes[j]) })
	cards := make([]recipeCard, 0, len(recipes))
	for _, rc := range recipes {
		cards = append(cards, s.card(rc, pl))
	}

	faq := []domain.QA{}
	for _, k := range []string{"start", "first", "grams6", "meat", "yolk", "fish", "curd", "salt"} {
		faq = append(faq, domain.QA{Q: i18n.T(l, "weaning.faq."+k+".q"), A: i18n.T(l, "weaning.faq."+k+".a")})
	}
	rules := []string{i18n.T(l, "weaning.rule.one"), i18n.T(l, "weaning.page.rule.morning"), i18n.T(l, "weaning.rule.health"),
		i18n.T(l, "weaning.rule.puree"), i18n.T(l, "weaning.rule.mashed"), i18n.T(l, "weaning.rule.water")}

	var alts []altLink
	for _, al := range topicLangs {
		m := i18n.Meta(al)
		alts = append(alts, altLink{Lang: string(al), Href: base + prefix(al) + "/weaning", Name: m.Name, English: m.English, Flag: m.Flag})
	}
	title := i18n.T(l, "weaning.page.title")
	desc := i18n.T(l, "weaning.page.desc")
	data := map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: title + " — " + i18n.T(l, "page.brand"), Description: desc, Canonical: base + pl.P + "/weaning",
			OGImage: brandOG(base, l), OGType: "article", OGWide: true, Alternates: alts, JSONLD: weaningLD(base, pl, title, desc, faq)},
		"L": l, "P": pl.P, "Country": pl.Country, "NavRecipes": true,
		"H1": i18n.T(l, "weaning.page.h1"), "Intro": []string{i18n.T(l, "weaning.page.intro1"), i18n.T(l, "weaning.page.intro2")},
		"Table": table, "Order": order, "Days": days, "Cards": cards, "FAQ": faq, "Rules": rules,
		"Source": i18n.T(l, "weaning.source"),
	}
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "weaning.html", data); err != nil {
		s.log.Error("weaning page", zap.Error(err))
		http.Error(w, "template error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = buf.WriteTo(w)
}

// weaningLD — разметка страницы: статья, вопросы-ответы и крошки.
func weaningLD(base string, pl pageLocale, title, desc string, faq []domain.QA) template.JS {
	var qa []map[string]any
	for _, f := range faq {
		qa = append(qa, map[string]any{"@type": "Question", "name": f.Q, "acceptedAnswer": map[string]any{"@type": "Answer", "text": f.A}})
	}
	url := base + pl.P + "/weaning"
	graph := []any{
		map[string]any{"@type": "Article", "headline": title, "description": desc, "url": url, "inLanguage": string(pl.L),
			"publisher": map[string]any{"@type": "Organization", "name": i18n.T(pl.L, "page.brand"), "url": base + "/"}},
		map[string]any{"@type": "FAQPage", "mainEntity": qa},
		breadcrumbLD([][2]string{{i18n.T(pl.L, "page.brand"), base + "/"}, {title, url}}),
	}
	b, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph})
	return template.JS(b)
}
