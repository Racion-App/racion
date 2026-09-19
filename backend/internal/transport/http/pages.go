package http

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"racion/internal/i18n"
	"racion/internal/planner"
	"racion/internal/service"
	"racion/locales"
)

//go:embed templates/*.html
var templateFS embed.FS

var pageTpl = template.Must(template.New("").Funcs(template.FuncMap{
	"ogLocale": ogLocale,
	"t":        func(l i18n.Lang, key string, args ...any) string { return i18n.T(l, key, args...) },
	"money":    formatMoney,
	"qty":      formatQty,
	"kcal":     func(v float64) string { return strconv.Itoa(int(math.Round(v))) },
	"mul":      func(a, b float64) float64 { return a * b },
	"json":     func(v any) template.JS { b, _ := json.Marshal(v); return template.JS(b) },
	"safeHTML": func(s string) template.HTML { return template.HTML(s) }, // только для HTML, собранного сервером из Markdown
	"add":      func(a, b int) int { return a + b },
	"sub":      func(a, b int) int { return a - b },
	"plural":   func(l i18n.Lang, n int, key string) string { return i18n.Plural(l, n, key) },
	"langMeta": func(l i18n.Lang) locales.Meta { return i18n.Meta(l) },
}).ParseFS(templateFS, "templates/*.html"))

// pageLang — язык страницы по префиксу пути: /en/..., /de/...; без префикса — русский.
// Страна для цен: ru → Россия (Росстат), en → США, de → Германия.
type pageLocale struct {
	L       i18n.Lang
	P       string // префикс ссылок: "" | "/en" | "/de"
	Country planner.Country
}

// langCountry — страна по умолчанию для языка из _meta локали.
func langCountry(l i18n.Lang) string { return i18n.Meta(l).Country }

func (s *Server) localeFromPath(r *http.Request) (pageLocale, bool) {
	l := i18n.RU
	if p := r.URL.Path; len(p) > 3 && p[0] == '/' && p[3] == '/' {
		if lv, ok := i18n.Valid(p[1:3]); ok && lv != i18n.RU {
			l = lv
		}
	}
	// ?country= в адресе важнее cookie: по клику на чип страна меняется сразу, cookie догоняет (rememberCountry)
	if cy, ok := countryValid(r.URL.Query().Get("country")); ok {
		return pageLocale{L: l, P: prefix(l), Country: cy}, true
	}
	if c, err := r.Cookie("racion_country"); err == nil {
		if cy, ok := countryValid(c.Value); ok {
			return pageLocale{L: l, P: prefix(l), Country: cy}, true
		}
	}
	// без выбора — страна по IP, если она среди поддерживаемых; иначе по языку
	if cy, ok := countryValid(s.geoCountry(r)); ok {
		return pageLocale{L: l, P: prefix(l), Country: cy}, true
	}
	return pageLocale{L: l, P: prefix(l), Country: planner.CountryOf(langCountry(l))}, true
}

func countryValid(code string) (planner.Country, bool) {
	for _, c := range planner.Countries {
		if c.Code == code {
			return c, true
		}
	}
	return planner.Country{}, false
}

func prefix(l i18n.Lang) string {
	if l == i18n.RU {
		return ""
	}
	return "/" + string(l)
}

// formatMoney — сумма в валюте страны: «1 234 ₽», «12,50 Br», «1 500 ₸», «12,50 €», «$12.50», «£3.20», «45 kr».
func formatMoney(c planner.Country, v float64) string {
	v = c.RoundMoney(v)
	neg := v < 0
	if neg {
		v = -v
	}
	whole := int64(math.Floor(v + 1e-9))
	frac := int64(math.Round((v - float64(whole)) * math.Pow(10, float64(c.Decimals))))
	if c.Decimals > 0 && frac >= int64(math.Pow(10, float64(c.Decimals))) {
		whole++
		frac = 0
	}
	s := strconv.FormatInt(whole, 10)
	thou, dec := c.ThouSep, c.DecSep
	if thou == "" {
		thou = " "
	}
	if dec == "" {
		dec = ","
	}
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(thou)
		}
		b.WriteRune(ch)
	}
	num := b.String()
	if c.Decimals > 0 {
		num += dec + fmt.Sprintf("%0*d", c.Decimals, frac)
	}
	if neg {
		num = "−" + num
	}
	if c.SymbolBefore {
		return c.Symbol + num
	}
	return num + " " + c.Symbol
}

func formatQty(l i18n.Lang, v float64, unit string) string {
	dec := i18n.Meta(l).Decimal
	if dec == "" {
		dec = ","
	}
	switch unit {
	case "pcs":
		if v == math.Trunc(v) {
			return fmt.Sprintf("%d %s", int(v), i18n.T(l, "unit.pcs"))
		}
		return strings.Replace(fmt.Sprintf("%.1f %s", v, i18n.T(l, "unit.pcs")), ".", dec, 1)
	case "ml":
		if v >= 1000 {
			return strings.Replace(fmt.Sprintf("%.2g %s", v/1000, i18n.T(l, "unit.l")), ".", dec, 1)
		}
		return fmt.Sprintf("%d %s", int(math.Round(v)), i18n.T(l, "unit.ml"))
	default:
		if v >= 1000 {
			return strings.Replace(fmt.Sprintf("%.2g %s", v/1000, i18n.T(l, "unit.kg")), ".", dec, 1)
		}
		return fmt.Sprintf("%d %s", int(math.Round(v)), i18n.T(l, "unit.g"))
	}
}

func hasTag(r planner.Recipe, t string) bool { return service.HasTag(r, t) }

type recipeCard struct {
	ID        string
	Title     string
	Slot      string
	Slot_     string
	TimeMin   int
	Kcal      float64
	Cost      float64
	Money     string
	TimeLabel string
	KcalLabel string
	Image     string
	Tags      []string
	Kid       bool
	Desc      string
	Href      string
}

func (s *Server) card(r planner.Recipe, pl pageLocale) recipeCard {
	kcal, _, _, _ := s.catalog.Nutrition(r)
	cost, priced := s.catalog.PortionCost(r, pl.Country.Code)
	tx := r.Text(pl.L)
	c := recipeCard{
		ID: r.ID, Title: tx.Title, Slot: r.Slot, Slot_: planner.SlotLabel(pl.L, r.Slot), TimeMin: r.TimeMin,
		Kcal: kcal, Cost: cost, Image: r.Image, Tags: r.Tags, Kid: hasTag(r, "kidmenu"), Desc: tx.Description,
		Href:      pl.P + "/recipe/" + r.ID,
		TimeLabel: i18n.T(pl.L, "recipe.min", r.TimeMin),
		KcalLabel: i18n.T(pl.L, "recipe.kcal", strconv.Itoa(int(math.Round(kcal)))),
	}
	if priced && cost > 0 {
		c.Money = formatMoney(pl.Country, cost)
	}
	return c
}

type pageBase struct {
	Title       string
	Description string
	Canonical   string
	OGImage     string
	OGType      string // article для рецепта и подборки, website для остального
	OGWide      bool   // картинка 1200×630 (брендовая), иначе фото блюда 768×512
	JSONLD      template.JS
	Alternates  []altLink // hreflang
	NoIndex     bool      // свой рецепт пользователя: поисковикам не показываем
}

type altLink struct {
	Lang    string
	Href    string
	Name    string // название языка на нём самом
	English string
	Flag    string
}

func (s *Server) baseURL(r *http.Request) string {
	if s.publicURL != "" {
		return s.publicURL
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}

// alternates — та же страница на всех языках для hreflang и переключателя.
func (s *Server) alternates(r *http.Request, path string) []altLink {
	base := s.baseURL(r)
	out := make([]altLink, 0, 3)
	for _, l := range i18n.Langs {
		m := i18n.Meta(l)
		out = append(out, altLink{Lang: string(l), Href: base + prefix(l) + path, Name: m.Name, English: m.English, Flag: m.Flag})
	}
	return out
}

const perPage = 36

type filterView struct {
	Param   string
	Label   string
	Options []struct {
		ID, Label, Href string
		On              bool
	}
}

// recipesPage — /recipes: каталог с фильтрами, умным поиском и постраничкой. Обычный HTML, без React.
// rememberCountry — ?country=XX на странице: запоминаем в cookie, как выбор в квизе.
func rememberCountry(w http.ResponseWriter, r *http.Request) {
	if cy, ok := countryValid(r.URL.Query().Get("country")); ok {
		http.SetCookie(w, &http.Cookie{Name: "racion_country", Value: cy.Code, Path: "/", MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode})
	}
}

func (s *Server) recipesPage(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.localeFromPath(r)
	if !ok {
		s.notFoundPage(w, r)
		return
	}
	rememberCountry(w, r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 80 {
		q = q[:80]
	}
	// старые ссылки вида ?f=pp
	legacy := r.URL.Query().Get("f")
	active := service.ActiveFilters{}
	for _, g := range service.CatalogFilters {
		for _, v := range r.URL.Query()[g.Param] {
			for _, o := range g.Options {
				if o == v && !active.Has(g.Param, v) {
					active[g.Param] = append(active[g.Param], v)
				}
			}
		}
		if !g.Multi && len(active[g.Param]) > 1 {
			active[g.Param] = active[g.Param][:1]
		}
	}
	switch legacy {
	case "breakfast", "lunch", "dinner", "snack":
		active["slot"] = []string{legacy}
	case "pp", "soup", "salad":
		active["tag"] = append(active["tag"], legacy)
	case "kids":
		active["tag"] = append(active["tag"], "kidmenu")
	case "quick":
		active["time"] = []string{"20"}
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("p"))
	if page < 1 {
		page = 1
	}

	var pool []planner.Recipe
	for _, rc := range s.catalog.Recipes {
		if s.svc.Catalog.Matches(rc, active, pl.Country) {
			pool = append(pool, rc)
		}
	}
	if q != "" {
		pool = s.svc.Catalog.Search(q, pool, pl.L)
	} else {
		sort.Slice(pool, func(i, j int) bool { return pool[i].LocalTitle(pl.L) < pool[j].LocalTitle(pl.L) })
	}
	all := make([]recipeCard, 0, len(pool))
	for _, rc := range pool {
		all = append(all, s.card(rc, pl))
	}
	total := len(all)
	pages := (total + perPage - 1) / perPage
	if pages == 0 {
		pages = 1
	}
	if page > pages {
		page = pages
	}
	lo, hi := (page-1)*perPage, page*perPage
	if hi > total {
		hi = total
	}

	// ссылка с набором фильтров; toggle переключает одно значение
	query := func(f service.ActiveFilters, p int) string {
		v := url.Values{}
		if q != "" {
			v.Set("q", q)
		}
		for _, g := range service.CatalogFilters {
			for _, x := range f[g.Param] {
				v.Add(g.Param, x)
			}
		}
		if p > 1 {
			v.Set("p", strconv.Itoa(p))
		}
		if enc := v.Encode(); enc != "" {
			return "/recipes?" + enc
		}
		return "/recipes"
	}
	link := func(f service.ActiveFilters, p int) string { return pl.P + query(f, p) }
	toggle := func(param, id string, multi bool) service.ActiveFilters {
		out := service.ActiveFilters{}
		if id == "" { // снять фильтр целиком
			for k, v := range active {
				if k != param {
					out[k] = v
				}
			}
			return out
		}
		for k, v := range active {
			out[k] = append([]string{}, v...)
		}
		if out.Has(param, id) {
			var rest []string
			for _, x := range out[param] {
				if x != id {
					rest = append(rest, x)
				}
			}
			out[param] = rest
		} else if multi {
			out[param] = append(out[param], id)
		} else {
			out[param] = []string{id}
		}
		return out
	}
	views := make([]filterView, 0, len(service.CatalogFilters))
	activeCount := 0
	var titleParts []string
	// быстрые вкладки по приёму пищи над сеткой: «Все» снимает slot, остальное — обычные фильтры
	type slotTab struct {
		Label, Href string
		On          bool
	}
	slotTabs := []slotTab{{Label: i18n.T(pl.L, "catalog.all"), Href: link(toggle("slot", "", false), 1), On: len(active["slot"]) == 0}}
	for _, g := range service.CatalogFilters {
		if g.Param != "slot" {
			continue
		}
		for _, o := range g.Options {
			slotTabs = append(slotTabs, slotTab{Label: filterLabel(pl, g.Param, o), Href: link(toggle(g.Param, o, g.Multi), 1), On: active.Has(g.Param, o)})
		}
	}
	sectionTitle := i18n.T(pl.L, "catalog.section.all")
	if len(active["slot"]) == 1 {
		sectionTitle = filterLabel(pl, "slot", active["slot"][0])
	}
	if q != "" {
		sectionTitle = i18n.T(pl.L, "catalog.section.search", q)
	}
	for _, g := range service.CatalogFilters {
		fv := filterView{Param: g.Param, Label: i18n.T(pl.L, "filter."+g.Param)}
		for _, o := range g.Options {
			on := active.Has(g.Param, o)
			label := filterLabel(pl, g.Param, o)
			if on {
				activeCount++
				titleParts = append(titleParts, strings.ToLower(label))
			}
			fv.Options = append(fv.Options, struct {
				ID, Label, Href string
				On              bool
			}{o, label, link(toggle(g.Param, o, g.Multi), 1), on})
		}
		views = append(views, fv)
	}

	base := s.baseURL(r)
	title := i18n.T(pl.L, "catalog.title") + " — " + i18n.T(pl.L, "page.brand")
	if len(titleParts) > 0 {
		title = i18n.T(pl.L, "catalog.title") + ": " + strings.Join(titleParts, ", ") + " — " + i18n.T(pl.L, "page.brand")
	}
	if q != "" {
		title = "«" + q + "» — " + i18n.T(pl.L, "catalog.search") + " · " + i18n.T(pl.L, "page.brand")
	}
	data := map[string]any{
		"Base": pageBase{
			Title:       title,
			Description: i18n.T(pl.L, "catalog.meta", total, i18n.Plural(pl.L, total, "catalog.recipe")),
			Canonical:   base + link(active, page),
			OGImage:     brandOG(base, pl.L),
			OGWide:      true,
			Alternates:  s.alternates(r, query(active, page)),
		},
		"L": pl.L, "P": pl.P, "Country": pl.Country,
		"NavRecipes": true, "SlotTabs": slotTabs, "SectionTitle": sectionTitle,
		"Filters": views, "ActiveCount": activeCount, "Q": q, "ResetHref": link(service.ActiveFilters{}, 1),
		"Items": all[lo:hi], "Total": total, "Page": page, "Pages": pages,
		"Prev": "", "Next": "",
	}
	if page > 1 {
		data["Prev"] = link(active, page-1)
	}
	if page < pages {
		data["Next"] = link(active, page+1)
	}
	// номера страниц: первая, последняя, соседи текущей; пропуски — многоточием
	type pageLink struct {
		N    int
		Href string
		On   bool
		Gap  bool
	}
	var pager []pageLink
	last := 0
	for n := 1; n <= pages; n++ {
		if n == 1 || n == pages || (n >= page-2 && n <= page+2) {
			if last > 0 && n-last > 1 {
				pager = append(pager, pageLink{Gap: true})
			}
			pager = append(pager, pageLink{N: n, Href: link(active, n), On: n == page})
			last = n
		}
	}
	data["Pager"] = pager
	// страна цен: чипы в фильтрах, текущая помечена; ссылка ставит cookie
	type countryLink struct {
		Code, Label, Symbol, Href string
		On                        bool
	}
	var cls []countryLink
	for _, cy := range planner.Countries {
		v := url.Values{}
		v.Set("country", cy.Code)
		cls = append(cls, countryLink{Code: cy.Code, Label: i18n.T(pl.L, "country."+cy.Code), Symbol: cy.Symbol, Href: pl.P + "/recipes?" + v.Encode(), On: cy.Code == pl.Country.Code})
	}
	data["Countries"] = cls
	// опубликованные рецепты пользователей — на первой странице без фильтров и поиска
	if page == 1 && q == "" && activeCount == 0 {
		if approved, err := s.svc.Moderation.Approved(r.Context(), 12); err == nil && len(approved) > 0 {
			var cards []recipeCard
			for _, rc := range approved {
				cards = append(cards, s.card(rc, pl))
			}
			data["Community"] = cards
		}
	}
	data["CountryLabel"] = i18n.T(pl.L, "country."+pl.Country.Code)
	// редакционные подборки — на первой странице каталога
	if page == 1 && q == "" && activeCount == 0 {
		type colCard struct {
			Name, Slug, Description, Cover, Href string
			Count                                int
		}
		var cols []colCard
		all := s.svc.Collections.Curated(r.Context())
		data["CollectionsTotal"] = len(all)
		if len(all) > 8 {
			all = all[:8]
		}
		for _, col := range all {
			col = col.Localized(string(pl.L))
			cols = append(cols, colCard{Name: col.Name, Slug: col.Slug, Description: col.Description, Cover: col.CoverAuto, Href: pl.P + "/collection/" + col.Slug, Count: len(col.Recipes)})
		}
		data["Collections"] = cols
	}
	// скрытые поля формы поиска — чтобы фильтры не терялись при вводе запроса
	var hidden []struct{ Name, Value string }
	for _, g := range service.CatalogFilters {
		for _, x := range active[g.Param] {
			hidden = append(hidden, struct{ Name, Value string }{g.Param, x})
		}
	}
	data["Hidden"] = hidden
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "recipes.html", data); err != nil {
		s.log.Error("recipes page", zap.Error(err))
		s.errorPage(w, r, 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// filterLabel — подпись варианта фильтра; ценовые пороги зависят от страны.
func filterLabel(pl pageLocale, param, id string) string {
	if param == "price" {
		idx, _ := strconv.Atoi(id)
		if idx >= 1 && idx <= 3 {
			return i18n.T(pl.L, "filter.price.upto", formatMoney(pl.Country, pl.Country.PriceLevels[idx-1]))
		}
	}
	if param == "eq" && !slices.Contains([]string{"stove", "nooven", "nocook"}, id) {
		return planner.EquipmentLabel(pl.L, id) // техника: подписи как в квизе, на всех языках
	}
	return i18n.T(pl.L, "filter."+param+"."+id)
}

func equipmentLabels(l i18n.Lang, ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, planner.EquipmentLabel(l, id))
	}
	return out
}

func (s *Server) notFoundPage(w http.ResponseWriter, r *http.Request) { s.errorPage(w, r, 404) }

// errorPage — страница ошибки в общем оформлении: 404, 403, 500, 503. Тексты — page.<код>.title / .text.
func (s *Server) errorPage(w http.ResponseWriter, r *http.Request, code int) {
	pl, ok := s.localeFromPath(r)
	if !ok {
		pl = pageLocale{L: i18n.FromRequest(r), Country: planner.CountryOf("RU")}
		if pl.L != i18n.RU {
			pl.P = "/" + string(pl.L)
		}
	}
	key := strconv.Itoa(code)
	if _, known := map[int]bool{404: true, 403: true, 500: true, 503: true}[code]; !known {
		key = "500"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = pageTpl.ExecuteTemplate(w, "error.html", map[string]any{
		"Base": pageBase{Title: i18n.T(pl.L, "page."+key+".title") + " — " + i18n.T(pl.L, "page.brand"), Canonical: s.baseURL(r) + r.URL.Path, NoIndex: true},
		"L":    pl.L, "P": pl.P, "Country": pl.Country, "Code": code,
		"Title": i18n.T(pl.L, "page."+key+".title"), "Text": i18n.T(pl.L, "page."+key+".text"),
	})
}

func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	fmt.Fprintf(&b, "<url><loc>%s/</loc><changefreq>weekly</changefreq><priority>1.0</priority></url>\n", base)
	community, _ := s.svc.Moderation.Approved(r.Context(), 500)
	curated := s.svc.Collections.Curated(r.Context())
	for _, l := range i18n.Langs {
		p := prefix(l)
		fmt.Fprintf(&b, "<url><loc>%s%s/recipes</loc><changefreq>weekly</changefreq><priority>0.8</priority></url>\n", base, p)
		for _, g := range service.CatalogFilters[:3] {
			for _, o := range g.Options {
				fmt.Fprintf(&b, "<url><loc>%s%s/recipes?%s=%s</loc><changefreq>weekly</changefreq><priority>0.6</priority></url>\n", base, p, g.Param, o)
			}
		}
		for _, rc := range s.catalog.Recipes {
			fmt.Fprintf(&b, "<url><loc>%s%s/recipe/%s</loc><changefreq>monthly</changefreq><priority>0.7</priority></url>\n", base, p, rc.ID)
		}
		for _, col := range curated {
			fmt.Fprintf(&b, "<url><loc>%s%s/collection/%s</loc><changefreq>weekly</changefreq><priority>0.7</priority></url>\n", base, p, col.Slug)
		}
		for _, rc := range community {
			fmt.Fprintf(&b, "<url><loc>%s%s/recipe/%s</loc><changefreq>monthly</changefreq><priority>0.5</priority></url>\n", base, p, rc.ID)
		}
	}
	b.WriteString("</urlset>\n")
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /api/\nDisallow: /plan/\nDisallow: /me\nDisallow: /login\nSitemap: %s/sitemap.xml\n", s.baseURL(r))
}

// recipesAPI — тот же каталог в JSON для приложения (поиск, «больше не предлагать»).
func (s *Server) recipesAPI(w http.ResponseWriter, r *http.Request) {
	l := i18n.FromRequest(r)
	pl := pageLocale{L: l, P: prefix(l), Country: planner.CountryOf(r.URL.Query().Get("country"))}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	active := service.ActiveFilters{}
	if slot := r.URL.Query().Get("slot"); slot != "" {
		active["slot"] = []string{slot}
	}
	var pool []planner.Recipe
	for _, rc := range s.catalog.Recipes {
		if s.svc.Catalog.Matches(rc, active, pl.Country) {
			pool = append(pool, rc)
		}
	}
	pool = s.svc.Catalog.Search(q, pool, l)
	out := make([]recipeCard, 0, len(pool))
	for _, rc := range pool {
		out = append(out, s.card(rc, pl))
	}
	if q == "" {
		sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	}
	writeJSON(w, 200, out)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// ogLocale — og:locale по языку страницы.
func ogLocale(l i18n.Lang) string {
	m := map[i18n.Lang]string{"ru": "ru_RU", "en": "en_US", "de": "de_DE", "es": "es_ES", "fr": "fr_FR", "it": "it_IT", "pt": "pt_BR", "pl": "pl_PL", "uk": "uk_UA", "tr": "tr_TR", "kk": "kk_KZ", "nl": "nl_NL", "cs": "cs_CZ", "zh": "zh_CN", "ja": "ja_JP"}
	if v, ok := m[l]; ok {
		return v
	}
	return "en_US"
}
