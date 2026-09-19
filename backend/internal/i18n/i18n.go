// Package i18n — строки интерфейса на любом языке из backend/locales/*.json.
// Язык выбирается на запрос: ?lang= → cookie racion_lang → Accept-Language → русский.
package i18n

import (
	"fmt"
	"net/http"
	"strings"

	"racion/locales"
)

type Lang string

const (
	RU Lang = "ru"
	EN Lang = "en"
	DE Lang = "de"
)

// Langs — коды языков в порядке показа.
var Langs []Lang

func init() {
	for _, c := range locales.Order {
		Langs = append(Langs, Lang(c))
	}
}

// Valid — есть ли такой язык.
func Valid(s string) (Lang, bool) {
	if _, ok := locales.All[s]; ok {
		return Lang(s), true
	}
	return "", false
}

// Meta — сведения о языке (флаг, страна, месяцы).
func Meta(l Lang) locales.Meta {
	if loc, ok := locales.All[string(l)]; ok {
		return loc.Meta
	}
	return locales.All["ru"].Meta
}

// FromAccept — первый поддерживаемый язык из Accept-Language, иначе пусто.
func FromAccept(header string) Lang {
	for _, part := range strings.Split(header, ",") {
		code := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if len(code) >= 2 {
			if l, ok := Valid(code[:2]); ok {
				return l
			}
		}
	}
	return ""
}

// FromRequest — язык запроса: ?lang= → cookie → Accept-Language → ru.
func FromRequest(r *http.Request) Lang {
	if l, ok := Valid(r.URL.Query().Get("lang")); ok {
		return l
	}
	if c, err := r.Cookie("racion_lang"); err == nil {
		if l, ok := Valid(c.Value); ok {
			return l
		}
	}
	if l := FromAccept(r.Header.Get("Accept-Language")); l != "" {
		return l
	}
	return RU
}

func lookup(l Lang, key string) (string, bool) {
	if loc, ok := locales.All[string(l)]; ok {
		if s, ok := loc.Strings[key]; ok {
			return s, true
		}
	}
	// чего нет в языке — берём из английского, потом из русского
	for _, fb := range []string{"en", "ru"} {
		if loc, ok := locales.All[fb]; ok {
			if s, ok := loc.Strings[key]; ok {
				return s, true
			}
		}
	}
	return "", false
}

// T — строка по ключу. Плейсхолдеры {0}, {1}… (или любые {имя}) заполняются аргументами по порядку.
func T(l Lang, key string, args ...any) string {
	s, ok := lookup(l, key)
	if !ok {
		return key
	}
	if len(args) == 0 {
		return s
	}
	var b strings.Builder
	i := 0
	for {
		open := strings.IndexByte(s, '{')
		if open < 0 || i >= len(args) {
			b.WriteString(s)
			break
		}
		close := strings.IndexByte(s[open:], '}')
		if close < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:open])
		b.WriteString(fmtArg(args[i]))
		i++
		s = s[open+close+1:]
	}
	return b.String()
}

func fmtArg(v any) string {
	switch x := v.(type) {
	case float64:
		return fmt.Sprintf("%.0f", x)
	case float32:
		return fmt.Sprintf("%.0f", x)
	}
	return fmt.Sprint(v)
}

// PluralForm — one | few | many по правилу языка.
func PluralForm(l Lang, n int) string {
	switch Meta(l).Plural {
	case "east-slavic": // ru, uk, be
		switch {
		case n%10 == 1 && n%100 != 11:
			return "one"
		case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
			return "few"
		default:
			return "many"
		}
	case "polish":
		switch {
		case n == 1:
			return "one"
		case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
			return "few"
		default:
			return "many"
		}
	case "czech": // cs, sk: 1 — one, 2–4 — few
		switch {
		case n == 1:
			return "one"
		case n >= 2 && n <= 4:
			return "few"
		default:
			return "many"
		}
	case "none": // zh, ja, ko, tr, kk
		return "many"
	}
	if n == 1 {
		return "one"
	}
	return "many"
}

// Plural — форма слова по числу: ключи key.one / key.few / key.many; нет формы — берётся many.
func Plural(l Lang, n int, key string) string {
	form := PluralForm(l, n)
	if _, ok := lookup(l, key+"."+form); !ok {
		form = "many"
	}
	return T(l, key+"."+form)
}

func month(l Lang, m int, gen bool) string {
	meta := Meta(l)
	if gen && len(meta.MonthsGen) == 12 {
		return meta.MonthsGen[m-1]
	}
	if len(meta.Months) == 12 {
		return meta.Months[m-1]
	}
	return Meta(RU).Months[m-1]
}

// MonthYear — «август 2026» / «August 2026».
func MonthYear(l Lang, m, y int) string {
	f := Meta(l).MonthYear
	if f == "" {
		f = "{month} {y}"
	}
	return strings.NewReplacer("{month}", month(l, m, false), "{y}", fmt.Sprint(y)).Replace(f)
}

// DayMonthYear — «14 сентября 2026» / «14 September 2026» / «14. September 2026» по формату языка.
func DayMonthYear(l Lang, d, m, y int) string {
	f := Meta(l).Date
	if f == "" {
		f = "{d} {month} {y}"
	}
	r := strings.NewReplacer("{d}", fmt.Sprint(d), "{month}", month(l, m, true), "{y}", fmt.Sprint(y))
	return r.Replace(f)
}

// LangByCountry — язык по стране (для запроса в магазин страны и подсказки языка по IP); пусто — неизвестно.
func LangByCountry(country string) string {
	byCountry := map[string]string{
		"RU": "ru", "BY": "ru", "KZ": "ru", "KG": "ru", "UA": "uk", "PL": "pl", "DE": "de", "AT": "de", "CH": "de", "LI": "de",
		"ES": "es", "MX": "es", "AR": "es", "CO": "es", "CL": "es", "PE": "es", "FR": "fr", "BE": "fr", "CA": "en", "IT": "it",
		"PT": "pt", "BR": "pt", "TR": "tr", "NL": "nl", "CZ": "cs", "SE": "sv", "NO": "no", "FI": "fi", "LT": "lt", "LV": "lv", "EE": "et",
		"US": "en", "GB": "en", "IE": "en", "AU": "en", "NZ": "en", "IN": "en", "CN": "zh", "JP": "ja", "KR": "ko",
	}
	if l, ok := byCountry[country]; ok {
		if _, valid := Valid(l); valid {
			return l
		}
	}
	for _, l := range Langs {
		if Meta(l).Country == country {
			return string(l)
		}
	}
	return ""
}
