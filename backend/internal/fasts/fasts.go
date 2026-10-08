// Package fasts — четыре многодневных православных поста по новому стилю: даты на любой год и что можно
// в каждый день по монастырскому уставу. Великий и Петров посты зависят от Пасхи, её дату считает Easter,
// поэтому календарь сам переходит от поста к посту — руками ничего обновлять не нужно.
//
// Правила по дням (сверены 08.10.2026 по общепринятым календарям питания):
//   - Рождественский (28.11–6.01): до 19 декабря рыба во вторник, четверг, субботу, воскресенье и 4 декабря;
//     с 20 декабря по 1 января рыба в субботу и воскресенье, во вторник и четверг — с маслом; со 2 по 6 января
//     рыбы нет, в субботу и воскресенье — с маслом; понедельник — без масла, среда и пятница — сухоядение;
//     6 января — Сочельник.
//   - Великий (Пасха − 48 … Пасха − 1): Чистый понедельник и Страстная пятница — без еды; понедельник, среда,
//     пятница — сухоядение, вторник и четверг — горячее без масла, суббота и воскресенье — с маслом; рыба на
//     Благовещение (7 апреля, если оно не на Страстной) и в Вербное воскресенье; Страстная — без масла.
//   - Петров (Пасха + 57 … 11 июля): рыба во вторник, четверг, субботу, воскресенье и 7 июля; понедельник —
//     горячее без масла, среда и пятница — сухоядение.
//   - Успенский (14–27 августа): понедельник, среда, пятница — сухоядение, вторник и четверг — горячее без
//     масла, суббота и воскресенье — с маслом; рыба только на Преображение, 19 августа.
package fasts

import (
	"sort"
	"time"
)

// Fast — пост с датами начала и конца включительно (полночь в часовом поясе, где считали).
type Fast struct {
	ID         string // nativity | great | apostles | dormition
	Start, End time.Time
}

// Days — сколько дней длится пост.
func (f Fast) Days() int { return int(f.End.Sub(f.Start).Hours()/24) + 1 }

// Easter — православная Пасха по новому стилю: формула Меёса для юлианского календаря плюс 13 дней
// (разница календарей верна для 1900–2099 годов).
func Easter(year int, loc *time.Location) time.Time {
	a, b, c := year%4, year%7, year%19
	d := (19*c + 15) % 30
	e := (2*a + 4*b - d + 34) % 7
	month := (d + e + 114) / 31
	day := (d+e+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc).AddDate(0, 0, 13)
}

// Of — посты, которые начинаются в году year, по порядку.
func Of(year int, loc *time.Location) []Fast {
	easter := Easter(year, loc)
	out := []Fast{{ID: "great", Start: easter.AddDate(0, 0, -48), End: easter.AddDate(0, 0, -1)}}
	// Петров пост бывает короче недели, а при самой поздней Пасхе и вовсе выпадает
	if ap := (Fast{ID: "apostles", Start: easter.AddDate(0, 0, 57), End: time.Date(year, 7, 11, 0, 0, 0, 0, loc)}); !ap.Start.After(ap.End) {
		out = append(out, ap)
	}
	out = append(out,
		Fast{ID: "dormition", Start: time.Date(year, 8, 14, 0, 0, 0, 0, loc), End: time.Date(year, 8, 27, 0, 0, 0, 0, loc)},
		Fast{ID: "nativity", Start: time.Date(year, 11, 28, 0, 0, 0, 0, loc), End: time.Date(year+1, 1, 6, 0, 0, 0, 0, loc)},
	)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Upcoming — пост, который идёт сейчас, или ближайший следующий; второе значение — идёт ли он сейчас.
func Upcoming(now time.Time) (Fast, bool) {
	today := midnight(now)
	for y := now.Year() - 1; y <= now.Year()+1; y++ {
		for _, f := range Of(y, now.Location()) {
			if !f.End.Before(today) {
				return f, !f.Start.After(today)
			}
		}
	}
	return Fast{}, false
}

// Soon — пост идёт или начнётся не позже чем через days дней: тогда событие «Постная неделя» в анкете — по сезону.
func Soon(now time.Time, days int) bool {
	f, on := Upcoming(now)
	return on || f.Start.Sub(midnight(now)).Hours()/24 <= float64(days)
}

// Kind — что можно в день d поста f: fish (рыба), oil (без рыбы, с растительным маслом), lean (без масла:
// горячее или сухоядение), none (по уставу без еды), eve (Сочельник), out (день вне поста).
func Kind(f Fast, d time.Time) string {
	d = midnight(d)
	if d.Before(f.Start) || d.After(f.End) {
		return "out"
	}
	wd := (int(d.Weekday()) + 6) % 7 // 0 — понедельник
	weekend := wd >= 5
	switch f.ID {
	case "nativity":
		md := int(d.Month())*100 + d.Day()
		switch {
		case d.Month() == time.January && d.Day() == 6:
			return "eve"
		case d.Month() == time.January && d.Day() >= 2:
			if weekend {
				return "oil"
			}
			return "lean"
		case md == 1204: // Введение во храм Пресвятой Богородицы
			return "fish"
		case d.Month() == time.November || md <= 1219:
			if wd == 1 || wd == 3 || weekend {
				return "fish"
			}
			return "lean"
		default: // 20 декабря — 1 января
			switch {
			case weekend:
				return "fish"
			case wd == 1 || wd == 3:
				return "oil"
			}
			return "lean"
		}
	case "great":
		easter := f.End.AddDate(0, 0, 1)
		holy := easter.AddDate(0, 0, -6) // понедельник Страстной седмицы
		annunciation := time.Date(d.Year(), 4, 7, 0, 0, 0, 0, d.Location())
		switch {
		case d.Equal(f.Start), d.Equal(easter.AddDate(0, 0, -2)): // Чистый понедельник, Страстная пятница
			return "none"
		case d.Equal(easter.AddDate(0, 0, -7)): // Вербное воскресенье
			return "fish"
		case d.Equal(annunciation):
			if !d.Before(holy) {
				return "oil"
			}
			return "fish"
		case !d.Before(holy): // Страстная седмица
			return "lean"
		case weekend:
			return "oil"
		}
		return "lean"
	case "apostles":
		if (d.Month() == time.July && d.Day() == 7) || wd == 1 || wd == 3 || weekend {
			return "fish"
		}
		return "lean"
	case "dormition":
		switch {
		case d.Month() == time.August && d.Day() == 19: // Преображение Господне
			return "fish"
		case weekend:
			return "oil"
		}
		return "lean"
	}
	return "lean"
}
