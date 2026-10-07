package recipeimport

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Line — строка ингредиента, разобранная правилами: «2 ст. л. муки» → 2, tbsp, «муки».
type Line struct {
	Qty   float64 // 0 — количества нет
	Unit  string  // g kg ml l tbsp tsp cup pcs clove pinch bunch slice can oz lb head; "" — штуки или не сказано
	Name  string
	Taste bool // «по вкусу», «для подачи»
}

var fractions = strings.NewReplacer("½", " 1/2", "⅓", " 1/3", "⅔", " 2/3", "¼", " 1/4", "¾", " 3/4", "⅕", " 1/5", "⅛", " 1/8", "⅙", " 1/6", "⁄", "/")

var (
	reQty   = regexp.MustCompile(`(\d+(?:[.,]\d+)?)(?:\s+(\d+)/(\d+))?(?:/(\d+))?(?:\s*[-–—]\s*(\d+(?:[.,]\d+)?))?`)
	reParen = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)
	reTaste = regexp.MustCompile(`(?i)по\s+вкусу|для\s+подачи|для\s+украшения|по\s+желанию|to\s+taste|for\s+serving|optional|nach\s+geschmack|au\s+goût|q\.?\s*b\.?|al\s+gusto|a\s+gusto|do\s+smaku|naar\s+smaak|podle\s+chuti`)
)

type unitRule struct {
	re   *regexp.Regexp
	unit string
}

// единицы сразу после числа; окончание — не буква (в Go \b не знает кириллицу)
var units = func() []unitRule {
	end := `(?:[^\p{L}]|$)`
	list := []struct{ pat, unit string }{
		{`кг|килограмм\p{L}*|kg|kilos?|kilograms?`, "kg"},
		{`мг|mg`, "mg"},
		{`г|гр|грамм\p{L}*|g|gr|grams?|gramm\p{L}*|grammes?`, "g"},
		{`мл|миллилитр\p{L}*|ml|milliliters?|millilitres?`, "ml"},
		{`л|литр\p{L}*|l|liters?|litres?|liter`, "l"},
		{`ст\.?\s*л\.?|ст\.?\s*лож\p{L}*|столов\p{L}*\s+лож\p{L}*|tbsp\.?|tbs\.?|tablespoons?|el|essl\p{L}*`, "tbsp"},
		{`ч\.?\s*л\.?|ч\.?\s*лож\p{L}*|чайн\p{L}*\s+лож\p{L}*|tsp\.?|teaspoons?|tl|teel\p{L}*`, "tsp"},
		{`стакан\p{L}*|cups?|tassen?`, "cup"},
		{`шт\.?|штук\p{L}*|pcs\.?|pc\.?|pieces?|stück|stk\.?`, "pcs"},
		{`зубч\p{L}*|cloves?|zehen?`, "clove"},
		{`щепот\p{L}*|pinch(?:es)?|prisen?`, "pinch"},
		{`пуч\p{L}*|bunch(?:es)?|bund`, "bunch"},
		{`веточ\p{L}*|веток|sprigs?|zweige?`, "sprig"},
		{`ломтик\p{L}*|кусоч\p{L}*|slices?|scheiben?`, "slice"},
		{`банк\p{L}*|cans?|tins?|dosen?`, "can"},
		{`головк\p{L}*|кочан\p{L}*|heads?`, "head"},
		{`oz\.?|ounces?`, "oz"},
		{`lbs?\.?|pounds?`, "lb"},
	}
	out := make([]unitRule, 0, len(list))
	for _, u := range list {
		out = append(out, unitRule{regexp.MustCompile(`(?i)^\s*(?:` + u.pat + `)` + end), u.unit})
	}
	return out
}()

// ParseLine разбирает строку правилами. Число в названии («сметана 15%», «молоко 2,5%») количеством не считается.
func ParseLine(s string) Line {
	src := fractions.Replace(s)
	var l Line
	src = reParen.ReplaceAllString(src, " ") // «Сахар (по вкусу) — 15 г»: пояснение в скобках количество не отменяет
	if reTaste.MatchString(src) {
		l.Taste = true
		src = reTaste.ReplaceAllString(src, " ")
	}
	type hit struct {
		start, end int
		qty        float64
		unit       string
	}
	var best *hit
	for _, m := range reQty.FindAllStringSubmatchIndex(src, -1) {
		if m[1] < len(src) && src[m[1]] == '%' {
			continue
		}
		if prev, _ := utf8.DecodeLastRuneInString(src[:m[0]]); unicode.IsLetter(prev) {
			continue // «B12», «Омега3»
		}
		h := hit{start: m[0], end: m[1], qty: num(src[m[2]:m[3]])}
		if m[4] >= 0 { // 1 1/2
			h.qty += num(src[m[4]:m[5]]) / max(num(src[m[6]:m[7]]), 1)
		}
		if m[8] >= 0 { // 1/2
			h.qty /= max(num(src[m[8]:m[9]]), 1)
		}
		if m[10] >= 0 { // 2-3 → 2,5
			h.qty = (h.qty + num(src[m[10]:m[11]])) / 2
		}
		rest := src[m[1]:]
		for _, u := range units {
			if loc := u.re.FindStringIndex(rest); loc != nil {
				h.unit = u.unit
				end := loc[1]
				if end > 0 && end <= len(rest) && !unicode.IsLetter(lastRune(rest[:end])) && lastRune(rest[:end]) != '.' {
					end -= len(string(lastRune(rest[:end]))) // разделитель после единицы оставляем названию
				}
				h.end = m[1] + end
				break
			}
		}
		if best == nil || (best.unit == "" && h.unit != "") {
			hh := h
			best = &hh
		}
		if best.unit != "" {
			break
		}
	}
	name := src
	if best != nil {
		l.Qty, l.Unit = best.qty, best.unit
		name = src[:best.start] + " " + src[best.end:]
	}
	name = strings.Trim(reSpace.ReplaceAllString(name, " "), " \t—–-:,;.*•·")
	l.Name = strings.TrimSpace(name)
	return l
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return v
}

func lastRune(s string) rune {
	r := []rune(s)
	if len(r) == 0 {
		return 0
	}
	return r[len(r)-1]
}

// words — слова строки в нижнем регистре, от трёх букв.
func words(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len([]rune(w)) >= 3 {
			out = append(out, strings.ReplaceAll(w, "ё", "е"))
		}
	}
	return out
}

// stem — грубая основа слова: «муки»/«мука» → «мук», «картофеля» → «карто». Совпадение основ проверяется
// префиксом, поэтому «onio» (onion) и «onion» (onions) тоже сходятся.
func stem(w string) string {
	r := []rune(w)
	switch {
	case len(r) >= 6:
		return string(r[:5])
	case len(r) >= 4:
		return string(r[:len(r)-1])
	}
	return w
}
