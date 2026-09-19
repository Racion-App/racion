package http

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"

	"go.uber.org/zap"

	"racion/internal/i18n"
	"racion/internal/service"
)

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
	var alts []altLink
	for _, l := range i18n.Langs {
		m := i18n.Meta(l)
		alts = append(alts, altLink{Lang: string(l), Href: base + prefix(l) + "/collection/" + col.Slug, Name: m.Name, English: m.English, Flag: m.Flag})
	}
	data := map[string]any{
		"Base": pageBase{Title: col.Name + " — " + i18n.T(pl.L, "page.brand"), Description: col.Description, Canonical: base + pl.P + "/collection/" + col.Slug, OGImage: base + "/og/collection/" + col.Slug + ".jpg?l=" + string(pl.L), OGType: "article", OGWide: true, Alternates: alts, JSONLD: collectionLD(base, pl, col.Name, col.Description, col.Slug, cards)},
		"L":    pl.L, "P": pl.P, "Country": pl.Country, "NavRecipes": true,
		"Col": col, "Cards": cards, "Cover": cover, "PlanHref": "/?s=1&collection=" + col.ID,
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
		"Base": pageBase{Title: i18n.T(pl.L, "coll.public.title") + " — " + i18n.T(pl.L, "page.brand"), Description: i18n.T(pl.L, "coll.public.lead"), Canonical: base + pl.P + "/collections", OGImage: brandOG(base, pl.L), OGWide: true, Alternates: s.alternates(r, "/collections")},
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
func collectionLD(base string, pl pageLocale, name, desc, slug string, cards []recipeCard) template.JS {
	items := make([]map[string]any, 0, len(cards))
	for i, c := range cards {
		items = append(items, map[string]any{"@type": "ListItem", "position": i + 1, "url": base + pl.P + "/recipe/" + c.ID, "name": c.Title})
	}
	url := base + pl.P + "/collection/" + slug
	list := map[string]any{"@type": "ItemList", "@id": url + "#list", "url": url, "name": name, "description": desc, "numberOfItems": len(cards), "itemListElement": items, "inLanguage": string(pl.L)}
	crumbs := breadcrumbLD([][2]string{{i18n.T(pl.L, "page.brand"), base + "/"}, {i18n.T(pl.L, "coll.public.title"), base + pl.P + "/collections"}, {name, ""}})
	b, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": []any{list, crumbs}})
	return template.JS(b)
}
