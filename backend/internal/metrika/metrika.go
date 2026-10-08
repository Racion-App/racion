// Package metrika — сводка дня из API отчётов Яндекс Метрики для админки: визиты и сравнение с днём раньше,
// откуда пришли, с каких устройств, входные страницы, поисковые фразы, города и цели (шаги квиза и что
// сделали дальше). Нужен OAuth-токен с правом metrika:read (METRIKA_TOKEN); без него сводка выключена.
package metrika

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const api = "https://api-metrika.yandex.net"

type Client struct {
	counter, token string
	http           *http.Client
	sem            chan struct{} // Метрика отвечает 429, если у пользователя больше трёх запросов одновременно

	mu      sync.Mutex
	days    map[string]cachedDay // дата|язык
	goals   []goalDef
	goalsAt time.Time
}

type cachedDay struct {
	day *Day
	at  time.Time
	ttl time.Duration
}

type goalDef struct {
	id        int64
	key, name string
}

func New(counter, token string) *Client {
	return &Client{counter: counter, token: token, http: &http.Client{Timeout: 20 * time.Second}, sem: make(chan struct{}, 3), days: map[string]cachedDay{}}
}

// Enabled — есть и счётчик, и токен.
func (c *Client) Enabled() bool { return c != nil && c.counter != "" && c.token != "" }

// Counter — номер счётчика для ссылки «Открыть в Метрике».
func (c *Client) Counter() string { return c.counter }

type Totals struct {
	Visits    int     `json:"visits"`
	Users     int     `json:"users"`
	NewUsers  int     `json:"newUsers"`
	Pageviews int     `json:"pageviews"`
	Bounce    float64 `json:"bounce"`   // доля отказов, %
	Depth     float64 `json:"depth"`    // просмотров за визит
	Duration  int     `json:"duration"` // среднее время визита, секунды
}

type Row struct {
	Key    string `json:"key,omitempty"`
	Name   string `json:"name"`
	Visits int    `json:"visits"`
}

// Goal — цель-событие из счётчика: Key — идентификатор события (quiz_step_1, plan_created).
type Goal struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Visits  int    `json:"visits"`  // визитов, в которых цель достигнута
	Reaches int    `json:"reaches"` // сколько раз всего
}

type Day struct {
	Date       string    `json:"date"`
	Prev       string    `json:"prev"`
	Totals     Totals    `json:"totals"`
	PrevTotals Totals    `json:"prevTotals"`
	Sources    []Row     `json:"sources"`
	Engines    []Row     `json:"engines"`
	Sites      []Row     `json:"sites"`
	Devices    []Row     `json:"devices"`
	Pages      []Row     `json:"pages"`
	Phrases    []Row     `json:"phrases"`
	Cities     []Row     `json:"cities"`
	Goals      []Goal    `json:"goals"`
	Counter    string    `json:"counter"`
	Fetched    time.Time `json:"fetched"`
}

// Day — сводка за день по Москве (часовой пояс счётчика). Прошедшие дни кэшируются на час: Метрика
// досчитывает вчерашние визиты в первые часы суток; сегодняшний — на пять минут.
func (c *Client) Day(ctx context.Context, day time.Time, lang string) (*Day, error) {
	if !c.Enabled() {
		return nil, errors.New("metrika: no token")
	}
	date := day.Format("2006-01-02")
	lang = apiLang(lang)
	key := date + "|" + lang
	c.mu.Lock()
	if v, ok := c.days[key]; ok && time.Since(v.at) < v.ttl {
		c.mu.Unlock()
		return v.day, nil
	}
	c.mu.Unlock()

	prev := day.AddDate(0, 0, -1).Format("2006-01-02")
	out := &Day{Date: date, Prev: prev, Counter: c.counter, Fetched: time.Now()}
	var (
		wg    sync.WaitGroup
		emu   sync.Mutex
		first error
	)
	run := func(f func() error) {
		wg.Go(func() {
			if err := f(); err != nil {
				emu.Lock()
				if first == nil {
					first = err
				}
				emu.Unlock()
			}
		})
	}
	run(func() error { return c.totals(ctx, prev, date, out) })
	rows := func(dst *[]Row, dim string, limit int) func() error {
		return func() error {
			r, err := c.rows(ctx, date, lang, dim, limit)
			*dst = r
			return err
		}
	}
	run(rows(&out.Sources, "ym:s:lastTrafficSource", 10))
	run(rows(&out.Engines, "ym:s:lastSearchEngineRoot", 5))
	run(rows(&out.Devices, "ym:s:deviceCategory", 5))
	run(rows(&out.Phrases, "ym:s:lastSearchPhrase", 10))
	run(rows(&out.Cities, "ym:s:regionCity", 6))
	var refs, socials []Row
	run(rows(&refs, "ym:s:lastReferalSource", 8))
	run(rows(&socials, "ym:s:lastSocialNetwork", 5))
	run(func() error {
		r, err := c.rows(ctx, date, lang, "ym:s:startURLPath", 100)
		out.Pages = groupPages(r, 10)
		return err
	})
	run(func() error {
		g, err := c.goalStats(ctx, date)
		out.Goals = g
		return err
	})
	wg.Wait()
	if first != nil {
		return nil, first
	}
	out.Sites = append(refs, socials...)
	sort.SliceStable(out.Sites, func(i, j int) bool { return out.Sites[i].Visits > out.Sites[j].Visits })

	ttl := time.Hour
	if date >= time.Now().In(Moscow).Format("2006-01-02") {
		ttl = 5 * time.Minute
	}
	c.mu.Lock()
	if len(c.days) > 60 {
		c.days = map[string]cachedDay{}
	}
	c.days[key] = cachedDay{day: out, at: time.Now(), ttl: ttl}
	c.mu.Unlock()
	return out, nil
}

// Moscow — часовой пояс счётчика: «вчера» считается по нему, а не по UTC сервера.
var Moscow = func() *time.Location {
	if loc, err := time.LoadLocation("Europe/Moscow"); err == nil {
		return loc
	}
	return time.FixedZone("MSK", 3*3600)
}()

// apiLang — названия источников, устройств и городов Метрика отдаёт на ru, en и tr.
func apiLang(l string) string {
	switch l {
	case "ru", "uk", "kk":
		return "ru"
	case "tr":
		return "tr"
	}
	return "en"
}

type report struct {
	Data []struct {
		Dimensions []struct {
			ID   any    `json:"id"`
			Name string `json:"name"`
		} `json:"dimensions"`
		Metrics []float64 `json:"metrics"`
	} `json:"data"`
}

func (c *Client) totals(ctx context.Context, prev, date string, out *Day) error {
	var r report
	q := url.Values{"metrics": {"ym:s:visits,ym:s:users,ym:s:newUsers,ym:s:pageviews,ym:s:bounceRate,ym:s:pageDepth,ym:s:avgVisitDurationSeconds"}, "dimensions": {"ym:s:date"}, "date1": {prev}, "date2": {date}}
	if err := c.get(ctx, "/stat/v1/data", q, &r); err != nil {
		return err
	}
	for _, d := range r.Data {
		if len(d.Dimensions) == 0 || len(d.Metrics) < 7 {
			continue
		}
		m := d.Metrics
		t := Totals{Visits: int(m[0]), Users: int(m[1]), NewUsers: int(m[2]), Pageviews: int(m[3]), Bounce: round1(m[4]), Depth: round1(m[5]), Duration: int(m[6] + 0.5)}
		switch d.Dimensions[0].Name {
		case date:
			out.Totals = t
		case prev:
			out.PrevTotals = t
		}
	}
	return nil
}

func (c *Client) rows(ctx context.Context, date, lang, dim string, limit int) ([]Row, error) {
	var r report
	q := url.Values{"metrics": {"ym:s:visits"}, "dimensions": {dim}, "date1": {date}, "date2": {date}, "sort": {"-ym:s:visits"}, "limit": {strconv.Itoa(limit)}, "lang": {lang}}
	if err := c.get(ctx, "/stat/v1/data", q, &r); err != nil {
		return nil, err
	}
	out := []Row{}
	for _, d := range r.Data {
		if len(d.Dimensions) == 0 || len(d.Metrics) == 0 || d.Dimensions[0].Name == "" {
			continue
		}
		key := ""
		if d.Dimensions[0].ID != nil {
			key = fmt.Sprint(d.Dimensions[0].ID)
		}
		out = append(out, Row{Key: key, Name: d.Dimensions[0].Name, Visits: int(d.Metrics[0])})
	}
	return out, nil
}

var idSegment = regexp.MustCompile(`/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|/[0-9a-f]{16,}`)

// groupPages — входные страницы без личных адресов: все /plan/<uuid> одной строкой /plan/…
func groupPages(rows []Row, limit int) []Row {
	sum := map[string]int{}
	var order []string
	for _, r := range rows {
		p := idSegment.ReplaceAllString(r.Name, "/…")
		if _, ok := sum[p]; !ok {
			order = append(order, p)
		}
		sum[p] += r.Visits
	}
	out := make([]Row, 0, len(order))
	for _, p := range order {
		out = append(out, Row{Name: p, Visits: sum[p]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Visits > out[j].Visits })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// goalStats — цели-события счётчика (тип action) в том порядке, в каком их завели: шаги квиза, потом неделя
// и остальное. Список целей берётся из настроек счётчика раз в сутки, так что новая цель появится сама.
func (c *Client) goalStats(ctx context.Context, date string) ([]Goal, error) {
	defs, err := c.goalDefs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Goal, len(defs))
	for i, g := range defs {
		out[i] = Goal{Key: g.key, Name: g.name}
	}
	const per = 10 // по две метрики на цель, у Метрики не больше 20 метрик в запросе
	for i := 0; i < len(defs); i += per {
		part := defs[i:min(i+per, len(defs))]
		var ms []string
		for _, g := range part {
			ms = append(ms, fmt.Sprintf("ym:s:goal%dvisits", g.id), fmt.Sprintf("ym:s:goal%dreaches", g.id))
		}
		var r struct {
			Totals []float64 `json:"totals"`
		}
		if err := c.get(ctx, "/stat/v1/data", url.Values{"metrics": {strings.Join(ms, ",")}, "date1": {date}, "date2": {date}}, &r); err != nil {
			return nil, err
		}
		for j := range part {
			if 2*j+1 < len(r.Totals) {
				out[i+j].Visits = int(r.Totals[2*j])
				out[i+j].Reaches = int(r.Totals[2*j+1])
			}
		}
	}
	return out, nil
}

func (c *Client) goalDefs(ctx context.Context) ([]goalDef, error) {
	c.mu.Lock()
	if c.goals != nil && time.Since(c.goalsAt) < 24*time.Hour {
		defer c.mu.Unlock()
		return c.goals, nil
	}
	c.mu.Unlock()
	var r struct {
		Goals []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Type       string `json:"type"`
			Conditions []struct {
				URL string `json:"url"`
			} `json:"conditions"`
		} `json:"goals"`
	}
	if err := c.get(ctx, "/management/v1/counter/"+url.PathEscape(c.counter)+"/goals", nil, &r); err != nil {
		return nil, err
	}
	defs := []goalDef{}
	for _, g := range r.Goals {
		if g.Type != "action" || len(g.Conditions) == 0 {
			continue
		}
		defs = append(defs, goalDef{id: g.ID, key: g.Conditions[0].URL, name: g.Name})
	}
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].id < defs[j].id })
	c.mu.Lock()
	c.goals, c.goalsAt = defs, time.Now()
	c.mu.Unlock()
	return defs, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values, dst any) error {
	if path == "/stat/v1/data" {
		q.Set("ids", c.counter)
		q.Set("accuracy", "full")
	}
	u := api + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-c.sem }()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "OAuth "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &e)
		if e.Message == "" {
			e.Message = http.StatusText(res.StatusCode)
		}
		return fmt.Errorf("metrika %d: %s", res.StatusCode, e.Message)
	}
	return json.Unmarshal(body, dst)
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
