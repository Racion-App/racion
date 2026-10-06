package http

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"racion/internal/i18n"
)

// Журнал изменений «Рациона» — /changelog. Записи лежат в changelog.json (новые сверху) на русском и
// английском: как и страница для разработчиков, на остальных языках показываем английский текст —
// понятный английский лучше машинного перевода истории версий на тринадцать языков. Выкладка новой
// версии — новая запись в начало файла.

//go:embed changelog.json
var changelogJSON []byte

type changeEntry struct {
	Version string              `json:"version"`
	From    string              `json:"from,omitempty"` // YYYY-MM-DD, если версия собиралась несколько дней
	Date    string              `json:"date"`           // YYYY-MM-DD — день выкладки
	Title   map[string]string   `json:"title"`
	Items   map[string][]string `json:"items"`
}

var changelog = func() []changeEntry {
	var out []changeEntry
	if err := json.Unmarshal(changelogJSON, &out); err != nil {
		panic("changelog.json: " + err.Error())
	}
	return out
}()

// AppVersion — номер текущей версии для подвала: первая запись журнала.
func AppVersion() string {
	if len(changelog) == 0 {
		return ""
	}
	return changelog[0].Version
}

// changelogLang — язык текста журнала: русский для ru, uk и kk (как у страницы для разработчиков), иначе английский.
func changelogLang(l i18n.Lang) string {
	if l == i18n.RU || l == "uk" || l == "kk" {
		return "ru"
	}
	return "en"
}

// changeDate — «6 октября 2026» или «17–19 сентября 2026», если версия собиралась несколько дней.
func changeDate(e changeEntry, l i18n.Lang) string {
	to, err := time.Parse("2006-01-02", e.Date)
	if err != nil {
		return e.Date
	}
	label := i18n.DayMonthYear(l, to.Day(), int(to.Month()), to.Year())
	if from, err := time.Parse("2006-01-02", e.From); err == nil && from.Before(to) {
		if from.Month() == to.Month() && from.Year() == to.Year() {
			return fmt.Sprintf("%d–%s", from.Day(), label)
		}
		return i18n.DayMonthYear(l, from.Day(), int(from.Month()), from.Year()) + " – " + label
	}
	return label
}

func (s *Server) changelogPage(w http.ResponseWriter, r *http.Request) {
	pl, _ := s.localeFromPath(r)
	base := s.baseURL(r)
	cl := changelogLang(pl.L)
	dl := i18n.Lang(cl) // даты — на языке текста записи
	type view struct {
		Version, Date, DateLabel, Title string
		Items                           []string
	}
	entries := make([]view, 0, len(changelog))
	for _, e := range changelog {
		entries = append(entries, view{Version: e.Version, Date: e.Date, DateLabel: changeDate(e, dl), Title: e.Title[cl], Items: e.Items[cl]})
	}
	title := i18n.T(pl.L, "changelog.title")
	desc := i18n.T(pl.L, "changelog.desc")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = pageTpl.ExecuteTemplate(w, "changelog.html", map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: title + " — " + i18n.T(pl.L, "page.brand"), Description: desc,
			Canonical: base + pl.P + "/changelog", OGImage: brandOG(base, pl.L), OGWide: true, Alternates: s.alternates(r, "/changelog")},
		"L": pl.L, "P": pl.P, "Country": pl.Country,
		"Title": title, "Lead": i18n.T(pl.L, "changelog.lead"), "Entries": entries,
	})
}
