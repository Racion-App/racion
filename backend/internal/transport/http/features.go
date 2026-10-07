package http

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"racion/internal/i18n"
)

// featureCount — сколько строк в «чеке возможностей» (ключи features.N.t / features.N.d).
const featureCount = 13

// featuresPage — «Что умеет Рацион»: возможности строками чека, у каждой цена «0 ₽», внизу «Итого 0 ₽».
// Страница для людей и для поиска: нейросети пересказывают сервис по ней, а не по заголовку главной.
// Сети магазинов — страны посетителя, сумма — в её валюте.
func (s *Server) featuresPage(w http.ResponseWriter, r *http.Request) {
	pl, _ := s.localeFromPath(r)
	base := s.baseURL(r)
	stores := s.storeNames(pl.Country.Code)
	type row struct{ Title, Text string }
	rows := make([]row, 0, featureCount)
	for i := 1; i <= featureCount; i++ {
		n := strconv.Itoa(i)
		rows = append(rows, row{Title: i18n.T(pl.L, "features."+n+".t"), Text: i18n.T(pl.L, "features."+n+".d", stores)})
	}
	type qa struct{ Q, A string }
	faq := make([]qa, 0, 5)
	for i := 1; i <= 5; i++ {
		n := strconv.Itoa(i)
		faq = append(faq, qa{Q: i18n.T(pl.L, "features.faq."+n+".q"), A: i18n.T(pl.L, "features.faq."+n+".a", stores)})
	}
	title := i18n.T(pl.L, "features.title")
	desc := i18n.T(pl.L, "features.meta")
	canonical := base + pl.P + "/features"

	// WebApplication со списком возможностей и FAQ: по ним поисковые ответы описывают сервис
	feats := make([]string, 0, len(rows))
	for _, x := range rows {
		feats = append(feats, x.Title)
	}
	app := map[string]any{"@type": "WebApplication", "@id": base + "/#app", "name": i18n.T(pl.L, "page.brand"), "url": base + pl.P + "/",
		"applicationCategory": "LifestyleApplication", "operatingSystem": "Web, iOS, Android", "inLanguage": string(pl.L),
		"description": i18n.T(pl.L, "features.lead"), "featureList": feats,
		"offers": map[string]any{"@type": "Offer", "price": "0", "priceCurrency": pl.Country.Currency}}
	page := map[string]any{"@type": "WebPage", "@id": canonical, "url": canonical, "name": title, "description": desc, "inLanguage": string(pl.L), "about": map[string]any{"@id": base + "/#app"}}
	var qs []map[string]any
	for _, x := range faq {
		qs = append(qs, map[string]any{"@type": "Question", "name": x.Q, "acceptedAnswer": map[string]any{"@type": "Answer", "text": x.A}})
	}
	crumbs := breadcrumbLD([][2]string{{i18n.T(pl.L, "page.brand"), base + "/"}, {title, ""}})
	ld, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": []any{app, page, crumbs, map[string]any{"@type": "FAQPage", "mainEntity": qs}}})

	var buf bytes.Buffer
	err := pageTpl.ExecuteTemplate(&buf, "features.html", map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: title + " — " + i18n.T(pl.L, "page.brand"), Description: desc,
			Canonical: canonical, OGImage: brandOG(base, pl.L), OGWide: true, Alternates: s.alternates(r, "/features"), JSONLD: template.JS(ld)},
		"L": pl.L, "P": pl.P, "Country": pl.Country,
		"Title": title, "Lead": i18n.T(pl.L, "features.lead"), "Rows": rows, "FAQ": faq, "Zero": pl.Country.Money(0),
	})
	if err != nil {
		s.log.Error("features page", zap.Error(err))
		http.Error(w, "template error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = buf.WriteTo(w)
}

// storeNames — сети страны через запятую в порядке каталога: «Пятёрочка, Магнит, …».
func (s *Server) storeNames(country string) string {
	var names []string
	for _, st := range s.catalog.StoresOf(country) {
		names = append(names, st.Name)
	}
	return strings.Join(names, ", ")
}
