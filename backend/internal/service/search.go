package service

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"racion/internal/i18n"
	"racion/internal/planner"
)

// Поиск по рецептам: регистр и «ё» не важны, слова ищутся по началу, опечатка в одну букву прощается,
// несколько слов — все должны найтись (в названии, описании или продуктах). Синонимы — для самых ходовых.

var synonyms = map[string][]string{
	"курица":   {"кур", "цыпл", "грудк", "бедр", "голен", "крыл"},
	"куриный":  {"кур", "грудк"},
	"мясо":     {"говяд", "свин", "баран", "фарш", "индей"},
	"рыба":     {"рыб", "трес", "минт", "хек", "лосос", "горбуш", "скумбр", "тун", "сельд", "селёд", "селед"},
	"пп":       {"pp"},
	"диет":     {"pp"},
	"лёгк":     {"pp"},
	"легк":     {"pp"},
	"быстро":   {"quick"},
	"суп":      {"soup"},
	"салат":    {"salad"},
	"каша":     {"каш", "овсян", "гречн", "пшённ", "пшенн", "манн", "рисов"},
	"завтрак":  {"breakfast"},
	"обед":     {"lunch"},
	"ужин":     {"dinner"},
	"перекус":  {"snack"},
	"десерт":   {"sweet"},
	"сладкое":  {"sweet"},
	"острое":   {"spicy"},
	"детское":  {"kidmenu"},
	"ребёнок":  {"kidmenu"},
	"ребенок":  {"kidmenu"},
	"вегетар":  {"vegetarian"},
	"без мяса": {"vegetarian"},
	"творог":   {"творож", "сырник"},
	"паста":    {"макарон", "спагет", "лапш"},
	"макароны": {"паст", "лапш"},
	// english
	"chicken": {"chick", "breast", "thigh", "drum", "wing"}, "poultry": {"chick", "turkey"}, "meat": {"beef", "pork", "lamb", "ground", "turkey"},
	"fish": {"fish", "cod", "pollock", "hake", "salmon", "mackerel", "tuna", "herring"}, "healthy": {"pp"}, "light": {"pp"}, "diet": {"pp"},
	"quick": {"quick"}, "fast": {"quick"}, "soup": {"soup"}, "salad": {"salad"}, "porridge": {"oat", "buckwheat", "millet", "semolina", "rice"},
	"breakfast": {"breakfast"}, "lunch": {"lunch"}, "dinner": {"dinner"}, "snack": {"snack"}, "dessert": {"sweet"}, "sweet": {"sweet"}, "spicy": {"spicy"},
	"kids": {"kidmenu"}, "baby": {"kidmenu"}, "child": {"kidmenu"}, "vegetarian": {"vegetarian"}, "veggie": {"vegetarian"}, "cottage": {"cottage", "syrniki"},
	"pasta": {"pasta", "noodle", "spaghetti"},
	// deutsch
	"hähnchen": {"hähnchen", "huhn", "brust", "schenkel", "keule", "flügel"}, "huhn": {"hähnchen", "huhn"}, "fleisch": {"rind", "schwein", "lamm", "hack", "pute"},
	"fisch": {"fisch", "kabeljau", "seelachs", "seehecht", "lachs", "makrele", "thunfisch", "hering"}, "gesund": {"pp"}, "leicht": {"pp"}, "diät": {"pp"},
	"schnell": {"quick"}, "suppe": {"soup"}, "salat": {"salad"}, "brei": {"hafer", "buchweizen", "hirse", "grieß", "reis"},
	"frühstück": {"breakfast"}, "mittag": {"lunch"}, "abend": {"dinner"}, "süß": {"sweet"}, "scharf": {"spicy"},
	"kinder": {"kidmenu"}, "kind": {"kidmenu"}, "vegetarisch": {"vegetarian"}, "quark": {"quark", "syrniki"},
	"nudeln": {"nudel", "pasta", "spaghetti"},
}

// Norm — нижний регистр без «ё», как в поиске.
func Norm(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "ё", "е")
	return s
}

func words(s string) []string {
	return strings.FieldsFunc(Norm(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// lev1 — совпадение с одной опечаткой (замена, вставка, пропуск) для слов от 5 букв.
func lev1(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	if len(ra) < 5 || len(rb) < 5 {
		return false
	}
	if len(ra) == len(rb) {
		diff := 0
		for i := range ra {
			if ra[i] != rb[i] {
				diff++
				if diff > 1 {
					return false
				}
			}
		}
		return true
	}
	if len(ra)+1 == len(rb) {
		ra, rb = rb, ra
	}
	if len(ra) != len(rb)+1 {
		return false
	}
	// ra длиннее на одну руну
	i, j, skipped := 0, 0, false
	for i < len(ra) && j < len(rb) {
		if ra[i] == rb[j] {
			i++
			j++
			continue
		}
		if skipped {
			return false
		}
		skipped = true
		i++
	}
	return true
}

// tokenMatch — токен запроса найден в тексте: как начало слова, по синониму или с одной опечаткой.
func tokenMatch(tok string, textWords []string, tags []string) bool {
	for _, w := range textWords {
		if strings.HasPrefix(w, tok) || (len([]rune(tok)) >= 5 && strings.HasPrefix(tok, w) && len([]rune(w)) >= 4) || lev1(tok, w) {
			return true
		}
	}
	for _, t := range tags {
		if t == tok {
			return true
		}
	}
	for key, alts := range synonyms {
		if strings.HasPrefix(key, tok) || strings.HasPrefix(tok, key) {
			for _, alt := range alts {
				for _, t := range tags {
					if t == alt {
						return true
					}
				}
				for _, w := range textWords {
					if strings.HasPrefix(w, alt) {
						return true
					}
				}
			}
		}
	}
	return false
}

type searchDoc struct {
	r        planner.Recipe
	title    []string
	all      []string // название + описание + продукты
	tags     []string
	slotTags []string
}

// searchDocs — разобранные тексты рецептов; каталог неизменен, поэтому считаются один раз на язык.
func (c *Catalog) searchDocs(l i18n.Lang) []searchDoc {
	if v, ok := c.docs.Load(l); ok {
		return v.([]searchDoc)
	}
	docs := c.buildDocs(l)
	c.docs.Store(l, docs)
	return docs
}

func (c *Catalog) buildDocs(l i18n.Lang) []searchDoc {
	docs := make([]searchDoc, 0, len(c.catalog.Load().Recipes))
	for _, r := range c.catalog.Load().Recipes {
		tx := r.Text(l)
		d := searchDoc{r: r, title: words(tx.Title), tags: append([]string{r.Slot}, r.Tags...)}
		text := tx.Title + " " + tx.Description
		for _, ri := range r.Ingredients {
			text += " " + c.catalog.Load().Ingredients[ri.IngredientID].LocalName(l)
		}
		d.all = words(text)
		docs = append(docs, d)
	}
	return docs
}

// Search возвращает рецепты по запросу, отсортированные по релевантности: совпадения в названии выше.
func (c *Catalog) Search(q string, pool []planner.Recipe, l i18n.Lang) []planner.Recipe {
	toks := words(q)
	if len(toks) == 0 {
		return pool
	}
	inPool := map[string]bool{}
	for _, r := range pool {
		inPool[r.ID] = true
	}
	type hit struct {
		r     planner.Recipe
		score int
	}
	var hits []hit
	for _, d := range c.searchDocs(l) {
		if !inPool[d.r.ID] {
			continue
		}
		score := 0
		ok := true
		for _, tok := range toks {
			switch {
			case tokenMatch(tok, d.title, nil):
				score += 3
			case tokenMatch(tok, d.all, d.tags):
				score += 1
			default:
				ok = false
			}
			if !ok {
				break
			}
		}
		if ok {
			if strings.HasPrefix(Norm(d.r.LocalTitle(l)), Norm(q)) {
				score += 5
			}
			hits = append(hits, hit{d.r, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].r.LocalTitle(l) < hits[j].r.LocalTitle(l)
	})
	out := make([]planner.Recipe, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.r)
	}
	return out
}

// ── Фильтры каталога ────────────────────────────────────────────────────────

// filterGroup — группа фильтров каталога. Подписи берутся из i18n по ключу filter.<param>.<id>;
// ценовые пороги (price 1/2/3) — из Country.PriceLevels.
type FilterGroup struct {
	Param   string // имя query-параметра
	Options []string
	Multi   bool
}

// CatalogFilters — группы фильтров каталога в порядке показа.
var CatalogFilters = []FilterGroup{
	{"slot", []string{"breakfast", "lunch", "dinner", "snack"}, false},
	{"tag", []string{"pp", "protein", "soup", "salad", "vegetarian", "sweet", "spicy", "hearty", "batch", "premium", "kidmenu"}, true},
	{"main", []string{"poultry", "meat", "fish", "veg", "dairy"}, false},
	{"time", []string{"20", "40", "60"}, false},
	{"kcal", []string{"300", "500", "501"}, false},
	{"price", []string{"1", "2", "3"}, false},
	{"pmin", nil, false}, // нижняя граница цены порции с ползунка; вариантов нет — только число
	{"eq", []string{"stove", "nooven", "nocook", "oven", "airfryer", "multicooker", "steamer", "microwave", "grill"}, false},
	{"have", nil, true}, // «есть дома»: id продуктов, вариантов нет — их собирает ResolveHave
}

var mainCategories = map[string][]string{
	"poultry": {"meat"},
	"meat":    {"meat"},
	"fish":    {"fish", "frozen"},
	"veg":     {"vegetables", "grains", "canned", "frozen", "fruits", "bakery"},
	"dairy":   {"dairy", "eggs"},
}

var poultryIDs = map[string]bool{"chicken_breast": true, "chicken_thigh": true, "chicken_drumstick": true, "chicken_whole": true, "turkey_fillet": true, "ground_chicken": true, "turkey_ground": true, "chicken_wings": true, "liver_chicken": true}

// ActiveFilters — выбранные значения по имени параметра.
type ActiveFilters map[string][]string

func (f ActiveFilters) Has(param, id string) bool {
	for _, v := range f[param] {
		if v == id {
			return true
		}
	}
	return false
}

// Matches — проходит ли рецепт все активные фильтры.
func (c *Catalog) Matches(r planner.Recipe, f ActiveFilters, country planner.Country) bool {
	if r.Hidden {
		return false
	}
	if v := f["slot"]; len(v) > 0 && r.Slot != v[0] {
		return false
	}
	for _, t := range f["tag"] {
		if t == "batch" {
			if !r.Batch {
				return false
			}
			continue
		}
		if !HasTag(r, t) {
			return false
		}
	}
	if !f.Has("tag", "kidmenu") && HasTag(r, "kidmenu") {
		return false
	}
	if v := f["have"]; len(v) > 0 {
		if used, _, _ := c.HaveMatch(r, v); used == 0 {
			return false
		}
	}
	if v := f["main"]; len(v) > 0 && len(r.Ingredients) > 0 {
		main := c.catalog.Load().Ingredients[r.Ingredients[0].IngredientID]
		switch v[0] {
		case "poultry":
			if !poultryIDs[main.ID] {
				return false
			}
		case "meat":
			if main.Category != "meat" || poultryIDs[main.ID] {
				return false
			}
		case "fish":
			if main.Category != "fish" && main.ID != "shrimp" && main.ID != "crab_sticks" {
				return false
			}
		case "veg":
			if !HasTag(r, "vegetarian") && main.Category != "vegetables" && main.Category != "grains" && main.Category != "canned" {
				return false
			}
		case "dairy":
			if main.Category != "dairy" && main.Category != "eggs" {
				return false
			}
		}
	}
	if v := f["time"]; len(v) > 0 {
		lim := map[string]int{"20": 20, "40": 40, "60": 60}[v[0]]
		if lim > 0 && r.TimeMin > lim {
			return false
		}
	}
	if v := f["kcal"]; len(v) > 0 {
		kcal, _, _, _ := c.catalog.Load().Nutrition(r)
		switch v[0] {
		case "300":
			if kcal > 300 {
				return false
			}
		case "500":
			if kcal > 500 {
				return false
			}
		case "501":
			if kcal <= 500 {
				return false
			}
		}
	}
	if v := f["price"]; len(v) > 0 {
		cost, ok := c.catalog.Load().PortionCost(r, country.Code)
		limit := 0.0
		if idx, legacy := map[string]int{"1": 0, "2": 1, "3": 2}[v[0]]; legacy {
			limit = country.PriceLevels[idx]
		} else {
			limit, _ = strconv.ParseFloat(v[0], 64)
		}
		if ok && limit > 0 && cost > limit {
			return false
		}
	}
	if v := f["pmin"]; len(v) > 0 {
		cost, ok := c.catalog.Load().PortionCost(r, country.Code)
		if lo, _ := strconv.ParseFloat(v[0], 64); ok && lo > 0 && cost < lo {
			return false
		}
	}
	if v := f["eq"]; len(v) > 0 {
		switch v[0] {
		case "stove":
			for _, e := range r.Equipment {
				if e != "stove" {
					return false
				}
			}
		case "nooven":
			for _, e := range r.Equipment {
				if e == "oven" {
					return false
				}
			}
		case "nocook":
			if len(r.Equipment) > 0 {
				return false
			}
		case "airfryer": // аэрогриль заменяет духовку: показываем и рецепты для духовки
			if !slices.Contains(r.Equipment, "airfryer") && !slices.Contains(r.Equipment, "oven") {
				return false
			}
		default: // конкретная техника: рецепт должен её требовать
			if !slices.Contains(r.Equipment, v[0]) {
				return false
			}
		}
	}
	return true
}

// haveFamilies — варианты одного продукта для «есть дома»: есть любая часть курицы — подходят рецепты с
// любой частью, есть сметана 20% — подходят и со сметаной 15%. Разные продукты не смешиваем: говяжий фарш
// не куриный, зелёный лук не репчатый.
var haveFamilies = [][]string{
	{"chicken_breast", "chicken_thigh", "chicken_drumstick", "chicken_whole", "chicken_legs", "chicken_wings"},
	{"ground_chicken", "chicken_mince_breast"},
	{"ground_beef", "beef_ground_lean"},
	{"sour_cream", "sour_cream_20", "sour_cream_25"},
	{"milk", "milk_32"},
	{"kefir", "kefir_1"},
	{"cottage_cheese", "cottage_cheese_9", "cottage_cheese_0", "cottage_soft"},
	{"cheese_hard", "cheese_russian", "cheese_gouda", "cheese_cheddar"},
	{"potato", "potato_baby"},
	{"rice", "rice_round", "rice_basmati", "rice_brown"},
	{"pasta", "spaghetti", "wholegrain_pasta"},
	{"tomato", "tomatoes_plum", "cherry_tomatoes"},
	{"cucumber", "cucumber_smooth"},
	{"broccoli", "broccoli_fresh"},
	{"cauliflower", "cauliflower_fresh"},
	{"spinach_frozen", "spinach_fresh"},
	{"green_beans", "green_beans_fresh"},
	{"cream", "cream_20", "heavy_cream"},
	{"yogurt_plain", "yogurt_greek"},
}

// haveAliases — как продукты называют в быту (русский: в базе «Картофель», дома «картошка»).
var haveAliases = map[string]string{
	"картошка": "potato", "картоха": "potato", "курица": "chicken_breast", "курятина": "chicken_breast", "куриное филе": "chicken_breast",
	"фарш": "ground_mixed", "яйца": "eggs", "яйцо": "eggs", "помидор": "tomato", "помидоры": "tomato", "огурец": "cucumber",
	"огурцы": "cucumber", "морковка": "carrot", "морковь": "carrot", "макароны": "pasta", "сыр": "cheese_hard", "лук": "onion",
	"chicken": "chicken_breast", "potatoes": "potato", "egg": "eggs", "tomatoes": "tomato", "cheese": "cheese_hard",
}

// haveSet — продукты «есть дома» вместе с их вариантами.
func haveSet(have []string) map[string]bool {
	out := make(map[string]bool, len(have))
	for _, id := range have {
		out[id] = true
		for _, fam := range haveFamilies {
			if slices.Contains(fam, id) {
				for _, x := range fam {
					out[x] = true
				}
			}
		}
	}
	return out
}

// HaveMatch — сколько продуктов рецепта уже есть дома и чего не хватает. Кладовка (соль, масло, специи,
// мука — Pantry) считается всегда имеющейся: её нет ни в «есть», ни в «докупить».
func (c *Catalog) HaveMatch(r planner.Recipe, have []string) (used, need int, missing []planner.Ingredient) {
	ings := c.catalog.Load().Ingredients
	set := haveSet(have)
	for _, ri := range r.Ingredients {
		ing, ok := ings[ri.IngredientID]
		if !ok || ing.Pantry {
			continue
		}
		need++
		if set[ing.ID] {
			used++
		} else {
			missing = append(missing, ing)
		}
	}
	return used, need, missing
}

// ResolveHave — продукты «есть дома» из адреса: id продукта или название, как его ввёл человек на языке
// страницы (точное совпадение, потом начало названия — из таких самый частый в рецептах: «сметана» —
// это сметана 15%, а не 25%). Кладовку не берём: она и так считается имеющейся.
func (c *Catalog) ResolveHave(vals []string, l i18n.Lang) []string {
	cat := c.catalog.Load()
	ings := cat.Ingredients
	uses := map[string]int{}
	for _, r := range cat.Recipes {
		for _, ri := range r.Ingredients {
			uses[ri.IngredientID]++
		}
	}
	var out []string
	add := func(id string) {
		if !slices.Contains(out, id) && len(out) < 12 {
			out = append(out, id)
		}
	}
	for _, v := range vals {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		if id, ok := haveAliases[v]; ok {
			v = id
		}
		if ing, ok := ings[v]; ok {
			if !ing.Pantry {
				add(ing.ID)
			}
			continue
		}
		var exact, prefix string
		for id, ing := range ings {
			if ing.Pantry {
				continue
			}
			name := strings.ToLower(ing.LocalName(l))
			if name == v || strings.ToLower(ing.Name) == v {
				exact = id
				break
			}
			if strings.HasPrefix(name, v) && (prefix == "" || uses[id] > uses[prefix]) {
				prefix = id
			}
		}
		// не нашлось по началу названия — по корню слова: «помидорчики», «морковочка»
		if exact == "" && prefix == "" && len([]rune(v)) > 4 {
			stem := string([]rune(v)[:4])
			for id, ing := range ings {
				if !ing.Pantry && strings.HasPrefix(strings.ToLower(ing.LocalName(l)), stem) && (prefix == "" || uses[id] > uses[prefix]) {
					prefix = id
				}
			}
		}
		if exact != "" {
			add(exact)
		} else if prefix != "" {
			add(prefix)
		}
	}
	return out
}

// HaveOption — продукт в подсказке «есть дома».
type HaveOption struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"img,omitempty"`
	Uses  int    `json:"-"`
}

// HaveOptions — продукты для подсказки в поле «есть дома»: всё, кроме кладовки, сначала самые частые в рецептах.
func (c *Catalog) HaveOptions(l i18n.Lang) []HaveOption {
	cat := c.catalog.Load()
	uses := map[string]int{}
	for _, r := range cat.Recipes {
		if r.Hidden {
			continue
		}
		for _, ri := range r.Ingredients {
			uses[ri.IngredientID]++
		}
	}
	var out []HaveOption
	for _, ing := range cat.Ingredients {
		if !ing.Pantry && uses[ing.ID] > 0 {
			out = append(out, HaveOption{ID: ing.ID, Name: ing.LocalName(l), Image: ing.Image, Uses: uses[ing.ID]})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Uses != out[j].Uses {
			return out[i].Uses > out[j].Uses
		}
		return out[i].Name < out[j].Name
	})
	return out
}
