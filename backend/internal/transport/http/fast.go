package http

import (
	"strconv"
	"time"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Календарь Рождественского поста для страницы «Постное меню на неделю» (/menu/post). Правила — общепринятый
// календарь по монастырскому уставу: до 19 декабря рыба во вторник, четверг, субботу и воскресенье и на
// Введение (4 декабря); с 20 декабря по 1 января — только в субботу и воскресенье, во вторник и четверг
// горячее с маслом; со 2 по 6 января рыбы нет, в субботу и воскресенье — с маслом. Понедельник — горячее без
// масла, среда и пятница — сухоядение; на странице оба вида — «без масла». 6 января — Сочельник.

type fastDay struct {
	Day   int
	Mon   string // короткий месяц у первой клетки поста и у первого числа
	Kind  string // fish | oil | lean | eve | out
	Label string // что можно в этот день: подсказка и текст для экранных читалок
	Today bool
}

type fastKind struct{ Kind, Label string }

type fastCalendar struct {
	Title, Lead, Note string
	Weekdays          []string
	Weeks             [][]fastDay
	Legend            []fastKind
	FAQ               []domain.QA
}

// fastKindOf — что можно в день d поста, который идёт с start по end.
func fastKindOf(d, start, end time.Time) string {
	if d.Before(start) || d.After(end) {
		return "out"
	}
	wd := (int(d.Weekday()) + 6) % 7 // 0 — понедельник
	md := int(d.Month())*100 + d.Day()
	switch {
	case d.Month() == time.January && d.Day() == 6:
		return "eve"
	case d.Month() == time.January && d.Day() >= 2:
		if wd >= 5 {
			return "oil"
		}
		return "lean"
	case md == 1204: // Введение во храм Пресвятой Богородицы
		return "fish"
	case d.Month() == time.November || md <= 1219:
		switch wd {
		case 1, 3, 5, 6:
			return "fish"
		}
		return "lean"
	default: // 20 декабря — 1 января
		switch wd {
		case 5, 6:
			return "fish"
		case 1, 3:
			return "oil"
		}
		return "lean"
	}
}

// newFastCalendar — ближайший Рождественский пост: идущий сейчас или следующий, по неделям с понедельника.
func newFastCalendar(l i18n.Lang, now time.Time) *fastCalendar {
	loc := now.Location()
	y := now.Year()
	if now.Month() == time.January && now.Day() <= 6 {
		y-- // пост начался в ноябре прошлого года и ещё идёт
	}
	start := time.Date(y, 11, 28, 0, 0, 0, 0, loc)
	end := time.Date(y+1, 1, 6, 0, 0, 0, 0, loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	first := start.AddDate(0, 0, -((int(start.Weekday()) + 6) % 7))
	last := end.AddDate(0, 0, 6-(int(end.Weekday())+6)%7)
	fc := &fastCalendar{
		Title: i18n.T(l, "fast.title", strconv.Itoa(y), strconv.Itoa(y+1)),
		Lead:  i18n.T(l, "fast.lead"),
		Note:  i18n.T(l, "fast.note"),
	}
	for i := 0; i < 7; i++ {
		fc.Weekdays = append(fc.Weekdays, planner.DayLabel(l, i))
	}
	var week []fastDay
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		k := fastKindOf(d, start, end)
		fd := fastDay{Day: d.Day(), Kind: k, Today: d.Equal(today)}
		if k != "out" {
			fd.Label = i18n.T(l, "fast.kind."+k)
		}
		if d.Equal(start) || (d.Day() == 1 && k != "out") {
			fd.Mon = shortMonth(l, int(d.Month()))
		}
		week = append(week, fd)
		if len(week) == 7 {
			fc.Weeks = append(fc.Weeks, week)
			week = nil
		}
	}
	for _, k := range []string{"fish", "oil", "lean", "eve"} {
		fc.Legend = append(fc.Legend, fastKind{k, i18n.T(l, "fast.kind."+k)})
	}
	for i := 1; ; i++ {
		k := "fast.faq" + strconv.Itoa(i)
		q := i18n.T(l, k+".q", strconv.Itoa(y), strconv.Itoa(y+1))
		if q == k+".q" {
			break
		}
		fc.FAQ = append(fc.FAQ, domain.QA{Q: q, A: i18n.T(l, k+".a", strconv.Itoa(y), strconv.Itoa(y+1))})
	}
	return fc
}
