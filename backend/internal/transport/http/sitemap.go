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
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">` + "\n")
	// lastmod — только там, где дата настоящая (service.PageVersions); Google верит lastmod, пока он не врёт
	lastmod := func(path string) {
		if t := s.svc.Pages.Changed(path); !t.IsZero() {
			fmt.Fprintf(&b, "<lastmod>%s</lastmod>", t.Format("2006-01-02"))
		}
	}
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
			lastmod(path)
			b.WriteString("<changefreq>weekly</changefreq><priority>0.7</priority></url>\n")
		}
		for _, m := range menuPresets {
			urlTopics("/menu/" + m.Slug)
		}
		urlTopics("/recipes/from")
		urlTopics("/weaning")
		for _, h := range jarHubs {
			urlTopics(h.Path)
		}
		urlTopics("/menu/new-year")
		for _, m := range kidMenuPresets {
			urlTopics("/menu/" + m.Slug)
		}
		for _, t := range topics {
			if len(s.topicRecipes(t)) >= 4 {
				urlTopics("/recipes/from/" + t.Slug)
			}
		}
	}
	for _, rc := range s.catalog.Recipes {
		if rc.Hidden || !hasLang(rc, l) {
			continue
		}
		// hreflang — только языки, на которых у рецепта есть текст
		fmt.Fprintf(&b, "<url><loc>%s%s/recipe/%s</loc>", base, p, rc.ID)
		for _, al := range i18n.Langs {
			if hasLang(rc, al) {
				fmt.Fprintf(&b, `<xhtml:link rel="alternate" hreflang="%s" href="%s%s/recipe/%s"/>`, al, base, prefix(al), rc.ID)
			}
		}
		fmt.Fprintf(&b, `<xhtml:link rel="alternate" hreflang="x-default" href="%s/recipe/%s"/>`, base, rc.ID)
		lastmod("/recipe/" + rc.ID)
		// фото блюда — в поиск по картинкам: тот же JPEG, что в разметке рецепта
		if img := ogImage(base, rc.Image); img != "" {
			fmt.Fprintf(&b, "<image:image><image:loc>%s</image:loc></image:image>", img)
		}
		b.WriteString("<changefreq>monthly</changefreq><priority>0.7</priority></url>\n")
	}
	for _, rc := range community {
		url("/recipe/"+rc.ID, "monthly", "0.5")
	}
	url("/developers", "monthly", "0.5")
	url("/features", "monthly", "0.8")
	url("/changelog", "weekly", "0.4")
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

// robots — закрытые разделы: личные планы и события, кабинет, режим готовки (SPA /cook/), админка,
// картинки превью и поиск по каталогу (?q=: страницы и так noindex, а боты перебирают тысячи запросов).
// Чтение каталога через API открыто: оно описано в /openapi.json и /llms.txt, и ассистенты должны иметь
// возможность им пользоваться. Запись и личные данные под /api/ остаются закрытыми.
// Сочетания фильтров каталога («?…&…») закрыты от обхода: такие страницы и так отдают noindex, но робот
// тратил на них почти весь бюджет — у Googlebot уходило 16 тысяч запросов из 18 тысяч за день. В карте
// сайта только одиночные фильтры main, slot и tag, их обход по-прежнему открыт.
// ?country= закрыт по той же причине: страна умножает каждый рецепт на 22 варианта с одним и тем же
// каноническим адресом. Яндекс от этого спасала Clean-param, остальные роботы перебирали всё подряд.
func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	base := s.baseURL(r)
	closed := "Allow: /api/recipes\nAllow: /api/collections\nAllow: /api/meta\nAllow: /api/occasions\nAllow: /api/ingredients\n" +
		"Disallow: /api/\nDisallow: /plan/\nDisallow: /event/\nDisallow: /cook/\nDisallow: /table\nDisallow: /me\nDisallow: /login\nDisallow: /admin\nDisallow: /og/\nDisallow: /*?*q=\nDisallow: /*?*country=\nDisallow: /*recipes?*&\n"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\n%s\nUser-agent: Yandex\nAllow: /\n%s%sClean-param: country&pmin&price /recipes\n%s\nSitemap: %s/sitemap.xml\n", closed, closed, yandexClosedLangs(), yandexCleanParams, base)
}

// Параметры, которые не меняют страницу: шаг квиза и возраст ребёнка из ссылок детских подборок (/?s=2&kid=12…),
// метка установленного приложения и метки кампаний. Яндекс заводил такие ссылки в поиск отдельными копиями главной.
const yandexCleanParams = "Clean-param: s&kid&collection&source&utm_source&utm_medium&utm_campaign&utm_content&utm_term\n"

// yandexOpenLangs — языки, которые Яндекс видит. Остальные переводы он массово выкидывает из поиска как
// «некачественные» (в октябре 2026 — 63 из 67 исключений, чешские страницы) и тратит на них обход, а людей
// из его поиска на них нет. Google эти страницы читает и приводит на них людей — его правила не трогаем.
var yandexOpenLangs = map[i18n.Lang]bool{i18n.RU: true, "uk": true, "kk": true, "tr": true}

// yandexClosedLangs — /cs/ и сама /cs для каждого закрытого языка. Префикс без слеша закрыл бы и чужие
// адреса («/de» — это и /developers), поэтому главная языка закрыта точным адресом через «$».
func yandexClosedLangs() string {
	var b strings.Builder
	for _, l := range i18n.Langs {
		if yandexOpenLangs[l] {
			continue
		}
		fmt.Fprintf(&b, "Disallow: /%s/\nDisallow: /%s$\n", l, l)
	}
	return b.String()
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
