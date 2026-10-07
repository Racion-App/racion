package recipeimport

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
)

// Product — продукт базы для сопоставления: названия на всех языках, единица счёта (g, ml, pcs).
type Product struct {
	ID     string
	Name   string // на языке рецепта — для подсказки нейросети
	Names  []string
	Unit   string
	Pantry bool
}

// Item — строка рецепта, сопоставленная с продуктом: количество уже на одну порцию в единице продукта.
type Item struct {
	ID     string  `json:"ingredientId"`
	Amount float64 `json:"amount"`
	Line   string  `json:"line"` // как было на сайте: человек сверит пересчёт
}

// JSONer — то, что нужно от пула нейросетей.
type JSONer interface {
	JSON(ctx context.Context, system, user string, out any) error
}

// DefaultPortions — на сколько порций считать, если сайт не сказал.
const DefaultPortions = 4

// ── Нейросеть ──────────────────────────────────────────────────────────────

const matchSystem = `You map the ingredient lines of a recipe to products of a grocery catalog for a meal-planning app.
Input JSON: {"lines": the ingredient lines as written on the site, "catalog": "id|name|unit" per line}.
For every line return {"i": line index from 0, "id": a catalog id or null, "total": the amount the line asks for, converted to the catalog unit, or null when the line gives no amount}.
- Units: g = grams, ml = millilitres, pcs = pieces. Convert spoons, cups, pieces, cloves and bunches with typical kitchen weights (1 tbsp flour 10 g, 1 tbsp sugar 15 g, 1 tbsp oil 15 ml, 1 tsp salt 6 g, 1 cup 240 ml, a medium onion 100 g, a garlic clove 5 g, a medium potato 150 g, a medium carrot 80 g, a bunch of herbs 30 g).
- "total" is for the whole recipe exactly as written: do not divide it by portions (2 eggs → 2, 6 tbsp flour → 60, 500 g beef → 500).
- "To taste", "for serving" and lines without an amount: "total": null.
- When the catalog has no exact product, pick the one a shopper would buy instead (cherry tomatoes → tomatoes, chicken breast → chicken fillet, vegetable oil → sunflower oil, parmesan → hard cheese). Return null when nothing is a reasonable substitute, and for water and ice.
- Never invent ids: use only ids from the catalog.
Return JSON: {"items": [{"i": 0, "id": "...", "total": 60}, ...]} with one entry per line.`

type aiItem struct {
	I     int      `json:"i"`
	ID    *string  `json:"id"`
	Total *float64 `json:"total"`
}

// MatchAI сопоставляет строки с базой нейросетью. Нейросеть только переводит меры в единицу продукта на весь
// рецепт; делит на порции сервер: модели путались и делили одни строки, а другие нет. Ответ проверяется:
// только id из базы, разумные количества.
func MatchAI(ctx context.Context, ai JSONer, lines []string, portions int, products []Product) ([]Item, []string, error) {
	if portions <= 0 {
		portions = DefaultPortions
	}
	var cat strings.Builder
	byID := make(map[string]Product, len(products))
	for _, p := range products {
		byID[p.ID] = p
		name := p.Name
		if name == "" && len(p.Names) > 0 {
			name = p.Names[0]
		}
		fmt.Fprintf(&cat, "%s|%s|%s\n", p.ID, name, p.Unit)
	}
	in, _ := json.Marshal(map[string]any{"lines": lines, "catalog": cat.String()})
	var out struct {
		Items []aiItem `json:"items"`
	}
	if err := ai.JSON(ctx, matchSystem, string(in), &out); err != nil {
		return nil, nil, err
	}
	got := map[int]Item{}
	for _, it := range out.Items {
		if it.I < 0 || it.I >= len(lines) || it.ID == nil {
			continue
		}
		p, ok := byID[*it.ID]
		if !ok {
			continue
		}
		var amt float64
		if it.Total == nil || *it.Total <= 0 || math.IsNaN(*it.Total) {
			amt = toUnit(Line{Taste: true}, p) // «по вкусу»: немного на порцию
		} else {
			amt = *it.Total / float64(portions)
		}
		if amt > 5000 {
			continue
		}
		got[it.I] = Item{ID: p.ID, Amount: round(amt, p.Unit), Line: lines[it.I]}
	}
	// что нейросеть оставила без пары, проверяем правилами, но берём только уверенные совпадения:
	// модели иногда пропускают очевидное («соль — 1 щепотка»), а угадывать за них не нужно
	var rest []string
	var at []int
	for i, l := range lines {
		if _, ok := got[i]; !ok {
			rest, at = append(rest, l), append(at, i)
		}
	}
	for j, it := range matchRules(rest, portions, products, true) {
		got[at[j]] = it
	}
	items, unmatched := collect(lines, got)
	return items, unmatched, nil
}

const extractSystem = `You extract a cooking recipe from the text of a web page for a meal-planning app.
Copy the recipe as it is on the page: do not invent ingredients, amounts or steps, and keep the page's language.
Return JSON: {"found": true, "title": "...", "description": "one or two sentences from the page about the dish, or empty", "portions": number or 0, "timeMin": total minutes or 0, "ingredients": ["each ingredient line with its amount, as written"], "steps": ["each cooking step, in order"]}.
If the page has no recipe (a list of recipes, an article without amounts, an error page), return {"found": false}.`

// ExtractAI — рецепт из видимого текста страницы, когда разметки schema.org нет.
func ExtractAI(ctx context.Context, ai JSONer, title, text string) (Recipe, error) {
	in, _ := json.Marshal(map[string]string{"page_title": title, "text": text})
	var out struct {
		Found       bool     `json:"found"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Portions    float64  `json:"portions"`
		TimeMin     float64  `json:"timeMin"`
		Ingredients []string `json:"ingredients"`
		Steps       []string `json:"steps"`
	}
	if err := ai.JSON(ctx, extractSystem, string(in), &out); err != nil {
		return Recipe{}, err
	}
	if !out.Found {
		return Recipe{}, nil
	}
	r := Recipe{Title: clean(out.Title), Description: clean(out.Description), Portions: int(out.Portions), TimeMin: int(out.TimeMin)}
	for _, s := range out.Ingredients {
		r.Ingredients = append(r.Ingredients, clean(s))
	}
	for _, s := range out.Steps {
		r.Steps = append(r.Steps, clean(s))
	}
	r.normalize()
	return r, nil
}

// ── Правила ────────────────────────────────────────────────────────────────

// синонимы, которых нет в названиях базы: слово строки → слово названия продукта
var synonyms = map[string]string{
	"картошка": "картофель", "картошки": "картофель", "картофелины": "картофель", "картофелин": "картофель",
	"томаты": "помидоры", "томата": "помидоры", "томатов": "помидоры", "томат": "помидоры",
	"растительное": "подсолнечное", "растительного": "подсолнечное", "постное": "подсолнечное",
	"грудка": "филе", "грудки": "филе", "грудку": "филе",
	"яиц": "яйца", "яйцо": "яйца", "яйца": "яйца", "желтки": "яйца", "белки": "яйца", "желтка": "яйца",
	"луковица": "лук", "луковицы": "лук", "луковиц": "лук",
	"сахарная": "сахар", "сахарной": "сахар",
	"пармезан": "твердый", "пармезана": "твердый",
	"vegetable": "sunflower",
	"scallions": "green", "breast": "fillet", "breasts": "fillet",
}

// MatchRules — запасной путь без нейросети: продукт по совпадению основ слов, количество по таблице мер.
func MatchRules(lines []string, portions int, products []Product) ([]Item, []string) {
	return collect(lines, matchRules(lines, portions, products, false))
}

// matchRules — строки, для которых нашёлся продукт, по номеру строки. strict — только уверенные пары:
// название продукта покрыто строкой целиком и хотя бы одно слово совпало точно («Соль — 1 щепотка» → соль).
func matchRules(lines []string, portions int, products []Product, strict bool) map[int]Item {
	if portions <= 0 {
		portions = DefaultPortions
	}
	type entry struct {
		p     Product
		stems [][]string // основы слов каждого названия
		words [][]string // сами слова: целое совпадение важнее совпадения основы (flour — не flounder)
	}
	idx := make([]entry, 0, len(products))
	for _, p := range products {
		e := entry{p: p}
		for _, n := range p.Names {
			var st []string
			ws := words(n)
			for _, w := range ws {
				st = append(st, stem(w))
			}
			if len(st) > 0 {
				e.stems = append(e.stems, st)
				e.words = append(e.words, ws)
			}
		}
		idx = append(idx, e)
	}
	got := map[int]Item{}
	for i, raw := range lines {
		if isWater(raw) {
			continue
		}
		l := ParseLine(raw)
		pct := rePercent.FindAllString(raw, -1)
		var ls, lw []string
		for _, w := range words(l.Name) {
			if s, ok := synonyms[w]; ok {
				w = s
			}
			ls = append(ls, stem(w))
			lw = append(lw, w)
		}
		if len(ls) == 0 {
			continue
		}
		latin := lw[0][0] < 0x80
		head := "" // главное слово строки на латинице: последнее, кроме «chopped», «sticks» и подобных
		for j := len(lw) - 1; j >= 0 && latin; j-- {
			if !tailWords[lw[j]] {
				head = stem(lw[j])
				break
			}
		}
		var best *Product
		bestScore := 0.0
		for k := range idx {
			for n, ns := range idx[k].stems {
				hit, covered, exact := 0, 0, 0
				for _, a := range lw {
					if slices.Contains(idx[k].words[n], a) {
						exact++
					}
				}
				for _, a := range ls {
					for _, b := range ns {
						if sameStem(a, b) {
							hit += min(len([]rune(a)), len([]rune(b)))
							break
						}
					}
				}
				for _, b := range ns {
					for _, a := range ls {
						if sameStem(a, b) {
							covered++
							break
						}
					}
				}
				if hit < 3 || covered == 0 || (strict && (exact == 0 || covered < len(ns))) {
					continue
				}
				// главное — сколько слов строки нашлось; при равенстве — продукт, название которого покрыто целиком
				score := float64(hit) + 2*float64(covered)/float64(len(ns)) + 2*float64(exact) - 0.01*float64(len(ns))
				// в английском и других языках на латинице главное слово последнее: plain flour — мука, а не йогурт
				if latin && head != "" && slices.ContainsFunc(ns, func(b string) bool { return sameStem(head, b) }) {
					score += 1.5
				}
				for _, p := range pct { // «Творог 5%» — к творогу 5%, а не 9%
					if strings.Contains(strings.Join(idx[k].p.Names, " "), p) {
						score++
					}
				}
				if score > bestScore {
					bestScore = score
					p := idx[k].p
					best = &p
				}
			}
		}
		if best == nil {
			continue
		}
		amt := toUnit(l, *best)
		if !l.Taste && l.Qty > 0 {
			amt /= float64(portions)
		}
		if amt <= 0 || amt > 5000 { // больше 5 кг на порцию форма не примет — значит, строку прочитали не так
			continue
		}
		got[i] = Item{ID: best.ID, Amount: round(amt, best.Unit), Line: raw}
	}
	return got
}

// слова после названия продукта в английских строках: как нарезать, чем подать, в каком виде
var tailWords = map[string]bool{"chopped": true, "sliced": true, "diced": true, "grated": true, "minced": true, "peeled": true, "picked": true,
	"trimmed": true, "deseeded": true, "crushed": true, "halved": true, "softened": true, "melted": true, "beaten": true, "finely": true,
	"roughly": true, "thinly": true, "plus": true, "extra": true, "serve": true, "serving": true, "garnish": true, "whole": true, "half": true,
	"optional": true, "rest": true, "left": true, "and": true, "the": true, "for": true, "sticks": true, "stick": true, "leaves": true, "leaf": true,
	"sprigs": true, "sprig": true, "cloves": true, "clove": true, "wedges": true, "pieces": true, "slices": true, "rashers": true, "stalks": true, "cubed": true, "drained": true}

var rePercent = regexp.MustCompile(`\d+(?:[.,]\d+)?\s*%`)

var waterWords = map[string]bool{"вода": true, "воды": true, "воду": true, "водой": true, "кипяток": true, "кипятка": true, "лед": true, "льда": true,
	"water": true, "ice": true, "wasser": true, "eis": true, "eau": true, "agua": true, "acqua": true, "woda": true, "wody": true, "voda": true, "vody": true}

// isWater — строка про воду или лёд: «Вода — 200 мл», «water for boiling».
func isWater(line string) bool {
	ws := words(ParseLine(line).Name)
	if len(ws) == 0 || len(ws) > 4 {
		return false
	}
	for _, w := range ws {
		if waterWords[w] {
			return true
		}
	}
	return false
}

func sameStem(a, b string) bool {
	if len([]rune(a)) < 3 || len([]rune(b)) < 3 {
		return a == b
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

// вес одной штуки, г — для «2 луковицы», «3 картофелины»
var pieceGrams = map[string]float64{
	"onion": 100, "potato": 150, "carrot": 80, "tomato": 120, "cucumber": 120, "bell_pepper": 150, "apple": 180,
	"banana": 120, "lemon": 100, "orange": 200, "zucchini": 300, "eggplant": 250, "beet": 200, "pear": 170,
	"sweet_potato": 200, "garlic": 5, "chicken_drumstick": 100, "chicken_thigh": 120, "sausages": 50, "tortilla": 40,
	"lavash": 80, "cabbage": 1500, "cabbage_peking": 800, "ginger": 20, "bread_white": 30, "bread_rye": 30,
	"mozzarella": 125, "bay_leaf": 0.2,
}

// toUnit — общее количество строки в единице продукта; без количества — немного «на вкус» на порцию.
func toUnit(l Line, p Product) float64 {
	if l.Qty <= 0 || l.Taste {
		switch {
		case p.Unit == "pcs":
			return 0.25 // кубик бульона, яйцо для смазки — на всю кастрюлю, не на порцию
		case p.ID == "salt":
			return 2
		case pieceGrams[p.ID] > 0 && pieceGrams[p.ID] < 2:
			return pieceGrams[p.ID] // лавровый лист — листик, а не пять граммов
		case strings.Contains(p.ID, "pepper") && p.Pantry:
			return 0.5
		case p.Pantry && p.Unit == "ml":
			return 5
		case p.Pantry:
			return 2
		}
		return 10
	}
	q := l.Qty
	piece := pieceGrams[p.ID]
	if piece == 0 {
		piece = 100
	}
	grams := 0.0 // сколько это в граммах (для g) или миллилитрах (для ml)
	switch l.Unit {
	case "g":
		grams = q
	case "kg":
		grams = q * 1000
	case "mg":
		grams = q / 1000
	case "ml":
		grams = q
	case "l":
		grams = q * 1000
	case "tbsp":
		grams = q * 15
		if p.ID == "flour" || p.ID == "starch" || p.ID == "cocoa" {
			grams = q * 10
		}
	case "tsp":
		grams = q * 5
	case "cup":
		grams = q * 240
		if p.Unit == "g" {
			grams = q * 200
			if p.ID == "flour" {
				grams = q * 130
			}
		}
	case "pinch":
		grams = q * 0.5
	case "clove":
		grams = q * 5
	case "bunch":
		grams = q * 30
	case "sprig":
		grams = q * 2
	case "slice":
		grams = q * 25
	case "can":
		grams = q * 400
	case "head":
		grams = q * piece
		if p.ID == "garlic" {
			grams = q * 40
		}
	case "oz":
		grams = q * 28.35
	case "lb":
		grams = q * 453.6
	case "pcs", "":
		if p.Unit == "pcs" {
			return q
		}
		grams = q * piece
	}
	if p.Unit == "pcs" {
		// «100 г авокадо» → штуки по весу штуки
		return grams / piece
	}
	return grams
}

// round — под поле формы: граммы и миллилитры целыми (мелочь — до половинки), штуки — четвертями.
func round(v float64, unit string) float64 {
	if unit == "pcs" {
		return math.Max(math.Round(v*4)/4, 0.25)
	}
	if v < 10 {
		return math.Max(math.Round(v*2)/2, 0.5)
	}
	return math.Round(v)
}

// collect — в порядке строк; одинаковые продукты (масло в тесто и для жарки) складываются в одну строку.
func collect(lines []string, got map[int]Item) ([]Item, []string) {
	items := []Item{}
	pos := map[string]int{}
	var unmatched []string
	for i, line := range lines {
		it, ok := got[i]
		if !ok {
			if !isWater(line) { // воду не покупают — и в «не нашли» ей не место
				unmatched = append(unmatched, line)
			}
			continue
		}
		if k, dup := pos[it.ID]; dup {
			items[k].Amount += it.Amount
			items[k].Line += "; " + it.Line
			continue
		}
		pos[it.ID] = len(items)
		items = append(items, it)
	}
	return items, unmatched
}
