package http

import (
	"bytes"
	"encoding/json"
	"html/template"
	"math"
	"net/http"
	"sort"
	"strconv"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
	"racion/internal/service"
)

// Подборки, влитые в более полные: старый адрес ведёт на новый, чтобы не держать две страницы на один запрос.
var mergedCollections = map[string]string{
	"quick-dinners": "quick20",
}

// Коллекции рецептов пользователя.

func (s *Server) myCollections(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	list, err := s.svc.Collections.List(r.Context(), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) createCollection(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, 1<<10, &in) {
		return
	}
	c, err := s.svc.Collections.Create(r.Context(), u.ID, in.Name)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, c)
}

func (s *Server) renameCollection(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, 1<<10, &in) {
		return
	}
	if err := s.svc.Collections.Rename(r.Context(), u.ID, r.PathValue("id"), in.Name); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) deleteCollection(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if err := s.svc.Collections.Delete(r.Context(), u.ID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// toggleCollectionItem — PUT добавляет, DELETE убирает рецепт.
func (s *Server) toggleCollectionItem(on bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := requireUser(w, r)
		if u == nil {
			return
		}
		if err := s.svc.Collections.Toggle(r.Context(), u.ID, r.PathValue("id"), r.PathValue("recipe"), on); err != nil {
			s.fail(w, r, err)
			return
		}
		w.WriteHeader(204)
	}
}

// publishCollection — открыть/закрыть свою коллекцию по ссылке: {public: bool} → коллекция со slug.
func (s *Server) publishCollection(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var in struct {
		Public bool `json:"public"`
	}
	if !decode(w, r, 1<<10, &in) {
		return
	}
	col, err := s.svc.Collections.Publish(r.Context(), u.ID, r.PathValue("id"), in.Public)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, col)
}

// publicCollections — редакционные подборки для приложения.
func (s *Server) publicCollections(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	lang := string(i18n.FromRequest(r))
	list := s.svc.Collections.Curated(r.Context())
	for i := range list {
		list[i] = list[i].Localized(lang)
	}
	writeJSON(w, 200, list)
}

func (s *Server) adminCollections(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermRecipes) == nil {
		return
	}
	list, err := s.svc.Collections.AdminList(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) adminSaveCollection(w http.ResponseWriter, r *http.Request) {
	u := s.requirePerm(w, r, service.PermRecipes)
	if u == nil {
		return
	}
	var in service.CuratedInput
	if !decode(w, r, 64<<10, &in) {
		return
	}
	col, err := s.svc.Collections.SaveCurated(r.Context(), *u, in)
	if err == nil && col.Public {
		s.notifySearch("/collection/"+col.Slug, "/collections")
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, col)
}

func (s *Server) adminDeleteCollection(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermRecipes) == nil {
		return
	}
	if err := s.svc.Collections.AdminDelete(r.Context(), r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// collectionPage — публичная подборка: /collection/{slug} и /{lang}/collection/{slug}.
func (s *Server) collectionPage(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.localeFromPath(r)
	if !ok {
		s.notFoundPage(w, r)
		return
	}
	rememberCountry(w, r)
	if to, ok := mergedCollections[r.PathValue("slug")]; ok {
		http.Redirect(w, r, pl.P+"/collection/"+to, http.StatusMovedPermanently)
		return
	}
	col, recipes, err := s.svc.Collections.BySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		s.notFoundPage(w, r)
		return
	}
	col = col.Localized(string(pl.L))
	cards := make([]recipeCard, 0, len(recipes))
	for _, rc := range recipes {
		cards = append(cards, s.card(rc, pl))
	}
	cover := col.CoverAuto
	base := s.baseURL(r)
	text := col.TextFor(string(pl.L))
	// факты для шапки: число рецептов, разброс цены порции, времени и калорий
	type fact struct{ Value, Label string }
	var facts []fact
	if len(cards) > 0 {
		minC, maxC, minT, maxT, minK, maxK := cards[0].Cost, cards[0].Cost, cards[0].TimeMin, cards[0].TimeMin, cards[0].Kcal, cards[0].Kcal
		for _, c := range cards[1:] {
			minC, maxC = math.Min(minC, c.Cost), math.Max(maxC, c.Cost)
			minT, maxT = min(minT, c.TimeMin), max(maxT, c.TimeMin)
			minK, maxK = math.Min(minK, c.Kcal), math.Max(maxK, c.Kcal)
		}
		facts = append(facts, fact{strconv.Itoa(len(cards)), i18n.Plural(pl.L, len(cards), "catalog.recipe")})
		if maxC > 0 {
			facts = append(facts, fact{rangeLabel(formatMoney(pl.Country, minC), formatMoney(pl.Country, maxC)), i18n.T(pl.L, "coll.fact.cost")})
		}
		facts = append(facts, fact{rangeLabel(strconv.Itoa(minT), strconv.Itoa(maxT)) + " " + i18n.T(pl.L, "min"), i18n.T(pl.L, "coll.fact.time")})
		facts = append(facts, fact{rangeLabel(strconv.Itoa(int(minK)), strconv.Itoa(int(maxK))), i18n.T(pl.L, "coll.fact.kcal")})
	}
	// рецепты по приёмам пищи в их порядке; одна группа — без заголовка группы, только счётчик
	type group struct {
		Key, Title string
		Cards      []recipeCard
	}
	var groups []group
	// праздничная подборка (большинство блюд с тегом festive) — по курсам стола: салаты, закуски, первое,
	// горячее, десерт; обычная — по приёмам пищи
	// признак стола: много салатов, закусок и десертов, а не блюд по приёмам пищи
	table := 0
	for _, rc := range recipes {
		if hasTag(rc, "festive") || hasTag(rc, "salad") || hasTag(rc, "dessert") || rc.Slot == "snack" {
			table++
		}
	}
	if len(recipes) > 0 && table*10 >= len(recipes)*4 {
		course := func(rc planner.Recipe) string {
			switch {
			case hasTag(rc, "salad"):
				return "salads"
			case hasTag(rc, "dessert") || hasTag(rc, "sweet") || hasTag(rc, "drink"):
				return "desserts"
			case hasTag(rc, "soup"):
				return "soup"
			case rc.Slot == "snack" || hasTag(rc, "nocook"):
				return "starters"
			}
			return "mains"
		}
		byCourse := map[string][]recipeCard{}
		for i, rc := range recipes {
			k := course(rc)
			byCourse[k] = append(byCourse[k], cards[i])
		}
		for _, k := range []string{"salads", "starters", "soup", "mains", "desserts"} {
			if g := byCourse[k]; len(g) > 0 {
				groups = append(groups, group{k, i18n.T(pl.L, "course."+k), g})
			}
		}
	} else {
		for _, slot := range []string{"breakfast", "lunch", "dinner", "snack"} {
			var g []recipeCard
			for _, c := range cards {
				if c.Slot == slot {
					g = append(g, c)
				}
			}
			if len(g) > 0 {
				groups = append(groups, group{slot, planner.SlotLabel(pl.L, slot), g})
			}
		}
	}
	if len(groups) == 1 {
		groups[0].Title = i18n.T(pl.L, "coll.recipes.title")
	}
	// другие подборки — перелинковка
	type colCard struct {
		Name, Slug, Cover, Href string
		Count                   int
	}
	var others []colCard
	allCurated := s.svc.Collections.Curated(r.Context())
	// сначала подборки с общими рецептами (они ближе по теме), потом остальные по порядку
	mine := map[string]bool{}
	for _, id := range col.Recipes {
		mine[id] = true
	}
	type scored struct {
		c     domain.Collection
		share int
	}
	var cand []scored
	for _, oc := range allCurated {
		if oc.ID == col.ID || !oc.Public {
			continue
		}
		n := 0
		for _, id := range oc.Recipes {
			if mine[id] {
				n++
			}
		}
		cand = append(cand, scored{oc, n})
	}
	sort.SliceStable(cand, func(i, j int) bool { return cand[i].share > cand[j].share })
	for _, sc := range cand {
		oc := sc.c.Localized(string(pl.L))
		others = append(others, colCard{Name: oc.Name, Slug: oc.Slug, Cover: oc.CoverAuto, Href: pl.P + "/collection/" + oc.Slug, Count: len(oc.Recipes)})
		if len(others) >= 8 {
			break
		}
	}
	var alts []altLink
	for _, l := range i18n.Langs {
		m := i18n.Meta(l)
		alts = append(alts, altLink{Lang: string(l), Href: base + prefix(l) + "/collection/" + col.Slug, Name: m.Name, English: m.English, Flag: m.Flag})
	}
	data := map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: col.Name + " — " + i18n.T(pl.L, "page.brand"), Description: col.Description, Canonical: base + pl.P + "/collection/" + col.Slug, OGImage: base + "/og/collection/" + col.Slug + ".jpg?l=" + string(pl.L), OGType: "article", OGWide: true, Alternates: alts, JSONLD: collectionLD(base, pl, col.Name, col.Description, col.Slug, cards, text.FAQ)},
		"L":    pl.L, "P": pl.P, "Country": pl.Country, "NavRecipes": true,
		"Col": col, "Cards": cards, "Cover": cover, "PlanHref": "/?s=1&collection=" + col.ID, "Text": text, "Facts": facts, "Groups": groups, "Others": others, "OthersTotal": len(allCurated),
	}
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "collection.html", data); err != nil {
		s.log.Error("collection page", zap.Error(err))
		s.errorPage(w, r, 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// collectionItems — рецепты коллекции в том же виде, что избранное.
func (s *Server) collectionItems(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	list, err := s.svc.Collections.Items(r.Context(), u.ID, r.PathValue("id"))
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

// collectionsPage — все редакционные подборки одной страницей (/collections, /{lang}/collections).
func (s *Server) collectionsPage(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.localeFromPath(r)
	if !ok {
		s.notFoundPage(w, r)
		return
	}
	type colCard struct {
		Name, Slug, Description, Cover, Href string
		Count                                int
	}
	var cols []colCard
	for _, col := range s.svc.Collections.Curated(r.Context()) {
		col = col.Localized(string(pl.L))
		cols = append(cols, colCard{Name: col.Name, Slug: col.Slug, Description: col.Description, Cover: col.CoverAuto, Href: pl.P + "/collection/" + col.Slug, Count: len(col.Recipes)})
	}
	base := s.baseURL(r)
	data := map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: i18n.T(pl.L, "coll.public.title") + " — " + i18n.T(pl.L, "page.brand"), Description: i18n.T(pl.L, "coll.public.lead"), Canonical: base + pl.P + "/collections", OGImage: brandOG(base, pl.L), OGWide: true, Alternates: s.alternates(r, "/collections")},
		"L":    pl.L, "P": pl.P, "Country": pl.Country, "NavRecipes": true, "Collections": cols,
	}
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "collections.html", data); err != nil {
		s.log.Error("collections page", zap.Error(err))
		s.errorPage(w, r, 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// collectionLD — подборка как ItemList рецептов плюс хлебные крошки: поисковики и ассистенты видят состав целиком.
func collectionLD(base string, pl pageLocale, name, desc, slug string, cards []recipeCard, faq []domain.QA) template.JS {
	items := make([]map[string]any, 0, len(cards))
	for i, c := range cards {
		items = append(items, map[string]any{"@type": "ListItem", "position": i + 1, "url": base + pl.P + "/recipe/" + c.ID, "name": c.Title})
	}
	url := base + pl.P + "/collection/" + slug
	list := map[string]any{"@type": "ItemList", "@id": url + "#list", "url": url, "name": name, "description": desc, "numberOfItems": len(cards), "itemListElement": items, "inLanguage": string(pl.L)}
	crumbs := breadcrumbLD([][2]string{{i18n.T(pl.L, "page.brand"), base + "/"}, {i18n.T(pl.L, "coll.public.title"), base + pl.P + "/collections"}, {name, ""}})
	graph := []any{list, crumbs}
	if len(faq) > 0 {
		var qs []map[string]any
		for _, x := range faq {
			qs = append(qs, map[string]any{"@type": "Question", "name": x.Q, "acceptedAnswer": map[string]any{"@type": "Answer", "text": x.A}})
		}
		graph = append(graph, map[string]any{"@type": "FAQPage", "mainEntity": qs})
	}
	b, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph})
	return template.JS(b)
}

// rangeLabel — «33–104» или одно значение, если границы совпали.
func rangeLabel(a, b string) string {
	if a == b {
		return a
	}
	return a + "–" + b
}
