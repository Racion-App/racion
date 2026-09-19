package http

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"racion/internal/i18n"
	"racion/internal/planner"
)

// Превью ссылок для мессенджеров на страницы приложения (React): главная, неделя, событие. Nginx отдаёт
// ботам (Telegram, VK, WhatsApp, Facebook, X, Discord, Slack) вместо index.html этот ответ: минимальный HTML
// с og-разметкой и мета-редиректом на ту же ссылку для случайного человека. Язык — из cookie или Accept-Language,
// для недели — язык плана.

var ogTpl = template.Must(template.New("og").Parse(`<!doctype html><html lang="{{.Lang}}"><head><meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.URL}}">
<meta property="og:title" content="{{.Title}}"><meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.URL}}"><meta property="og:type" content="website"><meta property="og:site_name" content="{{.Brand}}">
<meta property="og:locale" content="{{.Locale}}">
<meta property="og:image" content="{{.Image}}"><meta property="og:image:width" content="1200"><meta property="og:image:height" content="630"><meta property="og:image:alt" content="{{.Title}}">
<meta name="twitter:card" content="summary_large_image"><meta name="twitter:title" content="{{.Title}}"><meta name="twitter:description" content="{{.Description}}"><meta name="twitter:image" content="{{.Image}}">
<meta http-equiv="refresh" content="0; url={{.URL}}">
</head><body><a href="{{.URL}}">{{.Title}}</a></body></html>`))

type ogData struct {
	Lang, Locale, Title, Description, URL, Image, Brand string
}

func (s *Server) writeOG(w http.ResponseWriter, d ogData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = ogTpl.Execute(w, d)
}

// ogHome — /  (и /?s=…): брендовая карточка на языке.
func (s *Server) ogHome(w http.ResponseWriter, r *http.Request) {
	l := i18n.FromRequest(r)
	base := s.baseURL(r)
	s.writeOG(w, ogData{Lang: string(l), Locale: ogLocale(l), Brand: i18n.T(l, "page.brand"),
		Title: i18n.T(l, "page.brand") + " — " + i18n.T(l, "og.home.title"), Description: i18n.T(l, "foot.tag"),
		URL: base + "/", Image: brandOG(base, l)})
}

// ogPlan — /plan/{id}: название недели или стола, сумма и число блюд; картинка брендовая.
func (s *Server) ogPlan(w http.ResponseWriter, r *http.Request) {
	l := i18n.FromRequest(r)
	base := s.baseURL(r)
	id := r.PathValue("id")
	plan, err := s.svc.Plans.Get(r.Context(), id, l)
	if err != nil {
		s.ogHome(w, r)
		return
	}
	dishes := 0
	for _, d := range plan.Days {
		dishes += len(d.Dishes)
	}
	title := planTitle(plan, l)
	desc := i18n.T(l, "og.plan.desc", dishes, i18n.Plural(l, dishes, "dishes"), formatMoney(plan.Country, plan.Totals.Cost), plan.Store.Name)
	s.writeOG(w, ogData{Lang: string(l), Locale: ogLocale(l), Brand: i18n.T(l, "page.brand"), Title: title + " — " + i18n.T(l, "page.brand"),
		Description: desc, URL: base + "/plan/" + id, Image: brandOG(base, l)})
}

func planTitle(plan planner.Plan, l i18n.Lang) string {
	if plan.Occasion != nil {
		return i18n.T(l, "og.table.title", plan.Occasion.Title, plan.Occasion.Guests, i18n.Plural(l, plan.Occasion.Guests, "guests"))
	}
	if len(plan.Days) > 0 {
		return i18n.T(l, "og.week.title", humanDate(l, plan.Days[0].Date))
	}
	return i18n.T(l, "page.brand")
}

// ogEvent — /event/{id}: название и подводка события.
func (s *Server) ogEvent(w http.ResponseWriter, r *http.Request) {
	l := i18n.FromRequest(r)
	base := s.baseURL(r)
	id := r.PathValue("id")
	if _, ok := planner.OccasionByID(id); !ok {
		s.ogHome(w, r)
		return
	}
	s.writeOG(w, ogData{Lang: string(l), Locale: ogLocale(l), Brand: i18n.T(l, "page.brand"),
		Title: i18n.T(l, "occasion."+id+".title") + " — " + i18n.T(l, "page.brand"), Description: i18n.T(l, "occasion."+id+".lead"),
		URL: base + "/event/" + id, Image: brandOG(base, l)})
}

// humanDate — «21 сентября 2026» по шаблону date из _meta локали (месяц в родительном падеже, где он есть).
func humanDate(l i18n.Lang, iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	m := i18n.Meta(l)
	months := m.MonthsGen
	if len(months) != 12 {
		months = m.Months
	}
	if len(months) != 12 {
		return iso
	}
	f := m.Date
	if f == "" {
		f = "{d} {month} {y}"
	}
	r := strings.NewReplacer("{d}", strconv.Itoa(t.Day()), "{month}", months[t.Month()-1], "{y}", strconv.Itoa(t.Year()))
	return r.Replace(f)
}
