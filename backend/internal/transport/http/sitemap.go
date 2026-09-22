package http

import (
	"fmt"
	"net/http"
	"strings"

	"racion/internal/i18n"
	"racion/internal/service"
)

// sitemap — индекс: по одной карте на язык. В каждой карте у страницы перечислены версии на других
// языках (xhtml:link hreflang), как советуют Google и Яндекс для многоязычных сайтов.
func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, l := range i18n.Langs {
		fmt.Fprintf(&b, "<sitemap><loc>%s/sitemap/%s.xml</loc></sitemap>\n", base, l)
	}
	b.WriteString("</sitemapindex>\n")
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(b.String()))
}

// sitemapLang — карта одного языка: главная, каталог и его разделы, подборки, рецепты, юридические страницы.
func (s *Server) sitemapLang(w http.ResponseWriter, r *http.Request) {
	l, ok := i18n.Valid(strings.TrimSuffix(r.PathValue("file"), ".xml"))
	if !ok {
		s.notFoundPage(w, r)
		return
	}
	base := s.baseURL(r)
	p := prefix(l)
	community, _ := s.svc.Moderation.Approved(r.Context(), 500)
	curated := s.svc.Collections.Curated(r.Context())
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	// url пишет страницу с версиями на всех языках; path — без языкового префикса
	url := func(path, freq, prio string) {
		fmt.Fprintf(&b, "<url><loc>%s%s%s</loc>", base, p, path)
		for _, al := range i18n.Langs {
			fmt.Fprintf(&b, `<xhtml:link rel="alternate" hreflang="%s" href="%s%s%s"/>`, al, base, prefix(al), path)
		}
		fmt.Fprintf(&b, `<xhtml:link rel="alternate" hreflang="x-default" href="%s%s"/>`, base, path)
		fmt.Fprintf(&b, "<changefreq>%s</changefreq><priority>%s</priority></url>\n", freq, prio)
	}
	// главная: у каждого языка своя оболочка (/en, /de) со своим head, поэтому она в карте с альтернативами
	fmt.Fprintf(&b, "<url><loc>%s%s</loc>", base, homePath(l))
	for _, al := range i18n.Langs {
		fmt.Fprintf(&b, `<xhtml:link rel="alternate" hreflang="%s" href="%s%s"/>`, al, base, homePath(al))
	}
	fmt.Fprintf(&b, `<xhtml:link rel="alternate" hreflang="x-default" href="%s/"/>`, base)
	b.WriteString("<changefreq>weekly</changefreq><priority>1.0</priority></url>\n")
	url("/recipes", "weekly", "0.8")
	url("/collections", "weekly", "0.8")
	for _, g := range service.CatalogFilters[:3] {
		for _, o := range g.Options {
			url("/recipes?"+g.Param+"="+o, "weekly", "0.6")
		}
	}
	for _, col := range curated {
		url("/collection/"+col.Slug, "weekly", "0.7")
	}
	// «Что приготовить из …» есть только на ru, en, de
	if topicLang(l) {
		urlTopics := func(path string) {
			fmt.Fprintf(&b, "<url><loc>%s%s%s</loc>", base, p, path)
			for _, al := range topicLangs {
				fmt.Fprintf(&b, `<xhtml:link rel="alternate" hreflang="%s" href="%s%s%s"/>`, al, base, prefix(al), path)
			}
			fmt.Fprintf(&b, `<xhtml:link rel="alternate" hreflang="x-default" href="%s%s"/>`, base, path)
			b.WriteString("<changefreq>weekly</changefreq><priority>0.7</priority></url>\n")
		}
		for _, m := range menuPresets {
			urlTopics("/menu/" + m.Slug)
		}
		urlTopics("/recipes/from")
		for _, t := range topics {
			if len(s.topicRecipes(t)) >= 4 {
				urlTopics("/recipes/from/" + t.Slug)
			}
		}
	}
	for _, rc := range s.catalog.Recipes {
		if rc.Hidden {
			continue
		}
		url("/recipe/"+rc.ID, "monthly", "0.7")
	}
	for _, rc := range community {
		url("/recipe/"+rc.ID, "monthly", "0.5")
	}
	url("/terms", "yearly", "0.3")
	url("/privacy", "yearly", "0.3")
	b.WriteString("</urlset>\n")
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(b.String()))
}

// homePath — адрес главной на языке: «/» у русского, «/en» у остальных.
func homePath(l i18n.Lang) string {
	if l == i18n.RU {
		return "/"
	}
	return "/" + string(l)
}

// robots — закрытые разделы: API, личные планы и события, кабинет, режим готовки (SPA /cook/), админка,
// картинки превью и поиск по каталогу (?q=: страницы и так noindex, а боты перебирают тысячи запросов).
// Clean-param для Яндекса: параметры страны и цены не создают отдельных страниц каталога.
func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
	closed := "Disallow: /api/\nDisallow: /plan/\nDisallow: /event/\nDisallow: /cook/\nDisallow: /me\nDisallow: /login\nDisallow: /admin\nDisallow: /og/\nDisallow: /*?*q=\n"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\n%s\nUser-agent: Yandex\nAllow: /\n%sClean-param: country&pmin&price /recipes\n\nSitemap: %s/sitemap.xml\n", closed, closed, base)
}

// notifySearch — сообщить поисковикам об изменившихся страницах на всех языках (IndexNow).
func (s *Server) notifySearch(paths ...string) {
	all := make([]string, 0, len(paths)*len(i18n.Langs))
	for _, p := range paths {
		for _, l := range i18n.Langs {
			all = append(all, prefix(l)+p)
		}
	}
	s.svc.IndexNow.Notify(all...)
}

// indexNowKey — файл ключа по адресу /<key>.txt в корне: так поисковик проверяет, что запрос от владельца сайта
// (ключ в подпапке подтверждал бы только адреса из этой подпапки).
func (s *Server) indexNowKey(w http.ResponseWriter, r *http.Request) {
	key := s.svc.IndexNow.Key(r.Context())
	if key == "" || r.PathValue("file") != key+".txt" {
		s.notFoundPage(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(key))
}
