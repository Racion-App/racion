package http

import (
	"strconv"
	"strings"
	"time"

	"racion/internal/domain"
	"racion/internal/fasts"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Календарь поста в подборке «Что приготовить в пост»: тот пост, что идёт сейчас, или ближайший — Великий,
// Петров, Успенский или Рождественский. Даты и правила по дням считает пакет fasts (Великий и Петров — от
// Пасхи), поэтому страница сама переходит от поста к посту. Тексты — ключи fast.<id>.*, общие — fast.*.

type fastDay struct {
	Day      int
	Mon      string // короткий месяц у первой клетки поста и у первого числа
	Kind     string // fish | oil | lean | none | eve | out
	Label    string // что можно в этот день: подсказка и текст для экранных читалок
	Date     string // «1 декабря 2026» — заголовок панели, когда день выбран
	Today    bool
	Selected bool // день, открытый при загрузке: сегодня, если пост идёт, иначе первый день поста
}

// fastPane — что можно в день такого вида и блюда под него; панель показывается по нажатию на день.
type fastPane struct {
	Kind, Label, Text string
	Cards             []recipeCard
}

type fastKind struct{ Kind, Label string }

type fastCalendar struct {
	ID                string // nativity | great | apostles | dormition
	Title, Lead, Note string
	Status            string // «Идёт 12-й день поста из 40» или «До Великого поста 158 дней»
	Weekdays          []string
	Weeks             [][]fastDay
	Legend            []fastKind
	Kinds             map[string]bool // какие виды дней есть в этом посту: панели и обозначения только для них
	FAQ               []domain.QA
	Panes             []fastPane
	SelectedDate      string
	SelectedKind      string
}

// dayMonth — «28 ноября», «28 November», «28. November»: дата без года.
func dayMonth(l i18n.Lang, t time.Time) string {
	return strings.TrimSuffix(i18n.DayMonthYear(l, t.Day(), int(t.Month()), t.Year()), " "+strconv.Itoa(t.Year()))
}

// newFastCalendar — идущий или ближайший пост по неделям с понедельника.
func newFastCalendar(l i18n.Lang, now time.Time) *fastCalendar {
	f, on := fasts.Upcoming(now)
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	years := strconv.Itoa(f.Start.Year())
	if f.End.Year() != f.Start.Year() {
		years += "–" + strconv.Itoa(f.End.Year())
	}
	fc := &fastCalendar{
		ID:    f.ID,
		Title: i18n.T(l, "fast."+f.ID+".title", years),
		Lead:  i18n.T(l, "fast.lead", dayMonth(l, f.Start), dayMonth(l, f.End)),
		Note:  i18n.T(l, "fast.note"),
		Kinds: map[string]bool{},
	}
	for i := 0; i < 7; i++ {
		fc.Weekdays = append(fc.Weekdays, planner.DayLabel(l, i))
	}
	// открытый день: сегодня, если пост идёт, иначе первый день поста
	sel := f.Start
	if on {
		sel = today
		fc.Status = i18n.T(l, "fast.status.day", int(today.Sub(f.Start).Hours()/24)+1, f.Days())
	} else if left := int(f.Start.Sub(today).Hours() / 24); left > 0 {
		fc.Status = i18n.T(l, "fast.status.left", i18n.T(l, "fast."+f.ID+".to"), left, i18n.Plural(l, left, "fast.days"))
	}
	first := f.Start.AddDate(0, 0, -((int(f.Start.Weekday()) + 6) % 7))
	last := f.End.AddDate(0, 0, 6-(int(f.End.Weekday())+6)%7)
	var week []fastDay
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		k := fasts.Kind(f, d)
		fd := fastDay{Day: d.Day(), Kind: k, Today: d.Equal(today), Selected: d.Equal(sel)}
		if k != "out" {
			fc.Kinds[k] = true
			fd.Label = i18n.T(l, "fast.kind."+k)
			fd.Date = i18n.DayMonthYear(l, d.Day(), int(d.Month()), d.Year())
		}
		if fd.Selected {
			fc.SelectedDate, fc.SelectedKind = fd.Date, k
		}
		if d.Equal(f.Start) || (d.Day() == 1 && k != "out") {
			fd.Mon = shortMonth(l, int(d.Month()))
		}
		week = append(week, fd)
		if len(week) == 7 {
			fc.Weeks = append(fc.Weeks, week)
			week = nil
		}
	}
	for _, k := range []string{"fish", "oil", "lean", "none", "eve"} {
		if fc.Kinds[k] {
			fc.Legend = append(fc.Legend, fastKind{k, i18n.T(l, "fast.kind."+k)})
		}
	}
	// вопросы этого поста (даты подставляются), потом общие: что можно есть и где брать белок
	args := []any{dayMonth(l, f.Start), dayMonth(l, f.End), strconv.Itoa(f.Start.Year()), strconv.Itoa(f.End.Year())}
	for i := 1; ; i++ {
		k := "fast." + f.ID + ".faq" + strconv.Itoa(i)
		q := i18n.T(l, k+".q", args...)
		if q == k+".q" {
			break
		}
		fc.FAQ = append(fc.FAQ, domain.QA{Q: q, A: i18n.T(l, k+".a", args...)})
	}
	for _, k := range []string{"what", "protein"} {
		fc.FAQ = append(fc.FAQ, domain.QA{Q: i18n.T(l, "fast.faq."+k+".q"), A: i18n.T(l, "fast.faq."+k+".a")})
	}
	return fc
}
