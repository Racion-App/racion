package http

import (
	"fmt"
	"net/http"
	"time"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/service"
)

// Страница состояния /status и /api/status: что работает прямо сейчас, полоски за 30 дней по каждому
// компоненту, доля удачных проверок и средний отклик. Данные собирает service.Health раз в минуту.

type statusDay struct {
	Class string // ok | warn | bad | none
	Title string
}

type statusComponent struct {
	Key    string
	Name   string
	Desc   string
	OK     bool
	Ms     string
	Static bool
	Uptime string
	Days   []statusDay
}

func (s *Server) statusData(r *http.Request, l i18n.Lang) (map[string]any, error) {
	hist, err := s.monitor.History(r.Context(), 30)
	if err != nil {
		return nil, err
	}
	snap := s.monitor.Snapshot()
	allOK := true
	var comps []statusComponent
	for _, st := range snap {
		c := statusComponent{Key: st.Key, Name: i18n.T(l, "status.c."+st.Key), Desc: i18n.T(l, "status.c."+st.Key+".desc"), OK: st.OK, Ms: msLabel(st.Ms), Static: st.Static}
		if !st.OK {
			allOK = false
		}
		if days, ok := hist[st.Key]; ok {
			total, good := 0, 0
			for _, d := range days {
				total += d.Total
				good += d.OK
				c.Days = append(c.Days, dayClass(l, d))
			}
			if total > 0 {
				c.Uptime = fmt.Sprintf("%.2f", float64(good)*100/float64(total)) + " %"
			}
		}
		comps = append(comps, c)
	}
	avgMs, _, _ := s.monitor.Latency(r.Context(), "db", time.Now().Add(-24*time.Hour))
	up := time.Since(s.monitor.Started())
	return map[string]any{
		"AllOK":    allOK,
		"Comps":    comps,
		"Uptime30": fmt.Sprintf("%.2f", service.Uptime(hist)) + " %",
		"AvgMs":    msLabel(avgMs),
		"Since":    sinceLabel(l, up),
		"Checked":  time.Now().UTC().Format("15:04") + " UTC",
		"Recipes":  len(s.catalog.Recipes),
		"Langs":    len(i18n.Langs),
	}, nil
}

// msLabel — «<1» для нуля, чтобы не выглядело как отсутствие замера
func msLabel(ms int) string {
	if ms < 1 {
		return "<1"
	}
	return fmt.Sprintf("%d", ms)
}

func dayClass(l i18n.Lang, d domain.HealthDay) statusDay {
	date := d.Day.Format("02.01")
	if d.Total == 0 {
		return statusDay{Class: "none", Title: date + " · " + i18n.T(l, "status.nodata")}
	}
	p := float64(d.OK) * 100 / float64(d.Total)
	cls := "ok"
	if p < 99.5 {
		cls = "warn"
	}
	if p < 95 {
		cls = "bad"
	}
	return statusDay{Class: cls, Title: fmt.Sprintf("%s · %.1f %% (%d/%d)", date, p, d.OK, d.Total)}
}

// sinceLabel — «3 дня 4 ч» / «4 ч 12 мин»
func sinceLabel(l i18n.Lang, d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return i18n.T(l, "status.since.days", days, hours)
	}
	return i18n.T(l, "status.since.hours", hours, mins)
}

func (s *Server) statusPage(w http.ResponseWriter, r *http.Request) {
	pl, _ := s.localeFromPath(r)
	data, err := s.statusData(r, pl.L)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	base := s.baseURL(r)
	title := i18n.T(pl.L, "status.title")
	data["Base"] = pageBase{User: currentUser(r) != nil, Title: title + " — " + i18n.T(pl.L, "page.brand"), Description: i18n.T(pl.L, "status.desc"), Canonical: base + pl.P + "/status", OGImage: brandOG(base, pl.L), OGWide: true, Alternates: s.alternates(r, "/status")}
	data["L"], data["P"], data["Country"], data["Title"] = pl.L, pl.P, pl.Country, title
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_ = pageTpl.ExecuteTemplate(w, "status.html", data)
}

// statusAPI — то же в JSON для внешних мониторов: 200, если всё работает, иначе 503
func (s *Server) statusAPI(w http.ResponseWriter, r *http.Request) {
	snap := s.monitor.Snapshot()
	ok := true
	for _, c := range snap {
		if !c.OK {
			ok = false
		}
	}
	code := 200
	if !ok {
		code = 503
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, code, map[string]any{"ok": ok, "components": snap, "since": s.monitor.Started().UTC().Format(time.RFC3339), "recipes": s.visibleRecipes()})
}
