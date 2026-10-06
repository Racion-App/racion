package planner

import (
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"racion/internal/i18n"
)

// Прикорм по месяцам для ребёнка 4–11 месяцев. Источник — «Программа оптимизации вскармливания детей
// первого года жизни в Российской Федерации» (НМИЦ здоровья детей, Союз педиатров России, 2019),
// глава 5 и таблица 5.1 «Примерная схема введения продуктов детям первого года жизни»:
//   - прикорм с 4–6 мес, здоровому ребёнку предпочтительно с 5; первым — овощное пюре или безмолочная каша
//     из одной крупы, безглютеновые (рис, гречка, кукуруза) раньше глютенсодержащих;
//   - мясное пюре с 6 мес; фрукты не первыми, оптимально во втором полугодии после мяса;
//   - желток с 7 мес, рыба с 8 мес 2 раза в неделю вместо мяса, творог с 8 мес не больше 50 г,
//     кефир с 8 мес не больше 200 мл, хлеб с 8 мес;
//   - новый продукт — с малого количества, за 5–7 дней до объёма по возрасту, в первой половине дня;
//   - в 9–10 мес пюре постепенно меняют на мелко измельчённое;
//   - между кормлениями вода 150–200 мл в сутки.
// Частота кормлений — по ВОЗ (Guideline for complementary feeding of infants and young children 6–23
// months, 2023): 2–3 приёма прикорма в 6–8 мес, 3–4 в 9–11, остальное — грудное молоко или смесь.

// WeaningFood — продукт прикорма: группа, с какого месяца по программе и каким продуктом базы покупается.
type WeaningFood struct {
	ID         string  // id продукта базы, им же продукт отмечают «уже ввели»
	Group      string  // veg, cereal, meat, fruit, yolk, curd, kefir, fish, bread
	From       int     // с какого месяца
	RawPerUnit float64 // сколько продукта купить на 1 г блюда: каша из сухой крупы, мясо из сырого
	Recipe     string  // как приготовить: рецепт из recipes_weaning.json
}

// WeaningFoods — продукты в порядке, в каком их обычно вводят: внутри группы сначала самые мягкие для
// пищеварения (кабачок, цветная капуста, брокколи; рис и гречка без глютена; индейка и кролик).
var WeaningFoods = []WeaningFood{
	{"zucchini", "veg", 4, 1.15, "wean_zucchini_puree"},
	{"cauliflower", "veg", 4, 1.1, "wean_cauliflower_puree"},
	{"broccoli", "veg", 4, 1.1, "wean_broccoli_puree"},
	{"buckwheat", "cereal", 4, 0.12, "wean_buckwheat_porridge"},
	{"rice_round", "cereal", 4, 0.12, "wean_rice_porridge"},
	{"cornmeal", "cereal", 4, 0.12, "wean_corn_porridge"},
	{"pumpkin", "veg", 5, 1.25, "wean_pumpkin_puree"},
	{"carrot", "veg", 6, 1.15, "wean_carrot_puree"},
	{"potato", "veg", 6, 1.2, "wean_potato_zucchini_puree"},
	{"turkey_fillet", "meat", 6, 1.4, "wean_turkey_puree"},
	{"rabbit", "meat", 6, 1.6, "wean_rabbit_puree"},
	{"oats", "cereal", 6, 0.12, "wean_oat_porridge"},
	{"apple", "fruit", 6, 1.2, "wean_apple_puree"},
	{"pear", "fruit", 6, 1.2, "wean_pear_puree"},
	{"veal", "meat", 6, 1.4, "wean_veal_puree"},
	{"chicken_breast", "meat", 6, 1.4, "wean_chicken_puree"},
	{"banana", "fruit", 7, 1.3, "wean_banana_puree"},
	{"plum", "fruit", 7, 1.2, "wean_plum_puree"},
	{"eggs", "yolk", 7, 1, "wean_yolk_veg"},
	{"cottage_soft", "curd", 8, 1, "wean_cottage_fruit"},
	{"kefir", "kefir", 8, 1, ""},
	{"cod_fillet", "fish", 8, 1.3, "wean_cod_puree"},
	{"hake_fillet", "fish", 8, 1.3, "wean_hake_puree"},
	{"bread_white", "bread", 8, 1, ""},
}

// weaningGrams — объём блюда в сутки по группе и возрасту из таблицы 5.1 (для мяса и рыбы — отварное
// домашнего приготовления, верхняя граница диапазона). Индекс: 0 — 4–5 мес, 1 — 6, 2 — 7, 3 — 8, 4 — 9–12.
// Желток — в штуках, кефир — в мл.
var weaningGrams = map[string][5]float64{
	"veg":    {150, 150, 150, 150, 150},
	"cereal": {150, 150, 150, 180, 200},
	"meat":   {0, 15, 30, 35, 50},
	"fruit":  {50, 60, 70, 80, 100},
	"yolk":   {0, 0, 0.25, 0.5, 0.5},
	"curd":   {0, 0, 0, 40, 50},
	"kefir":  {0, 0, 0, 200, 200},
	"fish":   {0, 0, 0, 30, 60},
	"bread":  {0, 0, 0, 5, 10},
}

// масло: растительное — в овощное пюре, сливочное — в кашу (таблица 5.1, те же возрастные столбцы)
var (
	weaningOil    = [5]float64{3, 5, 5, 6, 6}
	weaningButter = [5]float64{3, 4, 4, 5, 5}
)

func weaningBand(month int) int {
	switch {
	case month < 6:
		return 0
	case month < 9:
		return month - 5
	default:
		return 4
	}
}

// WeaningGrams — суточный объём группы в этом возрасте.
func WeaningGrams(group string, month int) float64 { return weaningGrams[group][weaningBand(month)] }

func weaningFood(id string) (WeaningFood, bool) {
	for _, f := range WeaningFoods {
		if f.ID == id {
			return f, true
		}
	}
	return WeaningFood{}, false
}

// WeaningNext — следующий продукт для введения: первый по порядку, который уже можно по возрасту и ещё не
// введён. Фрукты — только когда есть хотя бы одна каша или овощ: по программе они не первый прикорм.
func WeaningNext(month int, introduced []string) (WeaningFood, bool) {
	hasBase := false
	for _, id := range introduced {
		if f, ok := weaningFood(id); ok && (f.Group == "veg" || f.Group == "cereal") {
			hasBase = true
		}
	}
	for _, f := range WeaningFoods {
		if f.From > month || slices.Contains(introduced, f.ID) {
			continue
		}
		if f.Group == "fruit" && !hasBase {
			continue
		}
		return f, true
	}
	return WeaningFood{}, false
}

// WeaningRamp — сколько давать нового продукта по дням недели: с половины чайной ложки до объёма по
// возрасту за 7 дней (программа: за 5–7 дней).
func WeaningRamp(full float64) []float64 {
	steps := []float64{0.03, 0.1, 0.2, 0.35, 0.55, 0.8, 1}
	out := make([]float64, len(steps))
	for i, s := range steps {
		v := full * s
		if full >= 1 && v < 5 && i == 0 {
			v = math.Min(5, full)
		}
		out[i] = roundWeaning(v)
	}
	return out
}

func roundWeaning(v float64) float64 {
	if v < 1 {
		return math.Round(v*4) / 4 // желток: четверть, половина
	}
	if v < 20 {
		return math.Round(v)
	}
	return math.Round(v/5) * 5
}

// WeaningItem — что дать в кормление.
type WeaningItem struct {
	Food   string  `json:"food"`             // id продукта базы; пусто — грудь или смесь
	Recipe string  `json:"recipe,omitempty"` // как приготовить
	Name   string  `json:"name"`             // название на языке недели
	Group  string  `json:"group"`            // группа прикорма или "milk"
	Grams  float64 `json:"grams"`            // г, для кефира мл, для желтка штуки
	Unit   string  `json:"unit"`             // g, ml, pcs
	New    bool    `json:"new"`              // продукт вводится на этой неделе
}

// WeaningFeed — кормление: время по примерному режиму и что в нём.
type WeaningFeed struct {
	Time string `json:"time"`
	Milk bool   `json:"milk"` // грудь или смесь (после прикорма — докармливание)
	// MilkKind — что в молочном кормлении у ребёнка на смеси: formula | breast; пусто — не на смеси
	MilkKind string        `json:"milkKind,omitempty"`
	MilkMl   int           `json:"milkMl,omitempty"` // смесь в это кормление, мл
	Items    []WeaningItem `json:"items"`
}

type WeaningDay struct {
	Index int           `json:"index"`
	Label string        `json:"label"`
	Feeds []WeaningFeed `json:"feeds"`
}

// Weaning — неделя прикорма одного ребёнка.
type Weaning struct {
	Month      int          `json:"month"`
	Days       []WeaningDay `json:"days"`
	New        *WeaningItem `json:"new,omitempty"`  // продукт недели
	Ramp       []float64    `json:"ramp,omitempty"` // его количество по дням
	Next       []string     `json:"next"`           // что вводить потом, по порядку (названия)
	Introduced []string     `json:"introduced"`     // введённые продукты базы (id)
	Texture    string       `json:"texture"`        // puree | mashed
	MilkMl     int          `json:"milkMl"`         // смесь в сутки по норме, 0 — грудь
}

// buildWeaning — неделя прикорма: примерный режим дня (5 кормлений), введённые продукты по кругу внутри
// группы, один новый продукт в первой половине дня с постепенным увеличением.
func (c *Catalog) buildWeaning(k Child, l i18n.Lang) *Weaning {
	m := k.AgeMonths
	w := &Weaning{Month: m, Texture: "puree", Next: []string{}, Introduced: []string{}}
	if m >= 9 {
		w.Texture = "mashed"
	}
	if k.Formula {
		w.MilkMl = k.FormulaMlPerDay()
	}
	name := func(id string) string { return c.WeaningName(id, l) }
	byGroup := map[string][]string{}
	for _, f := range WeaningFoods {
		if slices.Contains(k.Introduced, f.ID) && f.From <= m {
			byGroup[f.Group] = append(byGroup[f.Group], f.ID)
			w.Introduced = append(w.Introduced, f.ID)
		}
	}
	// в режиме «комбинирую» новый продукт вводят в первое домашнее кормление; если дома не кормят, его нет
	feedSlots := []string{"breakfast", "lunch", "dinner"} // 10:00, 14:00, 18:00
	newAt := -1
	for i, s := range feedSlots {
		if k.Feeding != FeedMix || k.MealSource(s) == MealHome {
			newAt = i
			break
		}
	}
	newFood, hasNew := WeaningNext(m, k.Introduced)
	hasNew = hasNew && newAt >= 0
	if hasNew {
		full := WeaningGrams(newFood.Group, m)
		if full == 0 { // по таблице группа ещё не положена — значит, возраст на границе, берём следующий столбец
			full = WeaningGrams(newFood.Group, m+1)
		}
		w.Ramp = WeaningRamp(full)
		it := c.weaningItem(newFood, full, l)
		it.New = true
		w.New = &it
		rest := append(slices.Clone(k.Introduced), newFood.ID)
		for len(w.Next) < 4 {
			nf, ok := WeaningNext(m+1, rest)
			if !ok {
				break
			}
			w.Next = append(w.Next, name(nf.ID))
			rest = append(rest, nf.ID)
		}
	}
	pick := func(group string, d int) (WeaningFood, bool) {
		ids := byGroup[group]
		if len(ids) == 0 {
			return WeaningFood{}, false
		}
		f, _ := weaningFood(ids[d%len(ids)])
		return f, true
	}
	band := weaningBand(m)
	for d := 0; d < 7; d++ {
		day := WeaningDay{Index: d, Label: DayLabel(l, d)}
		milk := func(t string) WeaningFeed { return WeaningFeed{Time: t, Milk: true, Items: []WeaningItem{}} }
		morning := WeaningFeed{Time: "10:00", Items: []WeaningItem{}}
		midday := WeaningFeed{Time: "14:00", Items: []WeaningItem{}}
		evening := WeaningFeed{Time: "18:00", Items: []WeaningItem{}}
		// утро: каша со сливочным маслом и фрукты
		if f, ok := pick("cereal", d); ok {
			morning.Items = append(morning.Items, c.weaningItem(f, WeaningGrams("cereal", m), l))
			morning.Items = append(morning.Items, c.weaningExtra("butter", weaningButter[band], l))
		}
		if f, ok := pick("fruit", d); ok {
			morning.Items = append(morning.Items, c.weaningItem(f, WeaningGrams("fruit", m), l))
		}
		// обед: овощи с растительным маслом, мясо (рыба дважды в неделю с 8 мес), желток через день
		if f, ok := pick("veg", d); ok {
			midday.Items = append(midday.Items, c.weaningItem(f, WeaningGrams("veg", m), l))
			midday.Items = append(midday.Items, c.weaningExtra("oil", weaningOil[band], l))
		}
		fishDay := d == 2 || d == 5
		if f, ok := pick("fish", d); ok && fishDay && m >= 8 {
			midday.Items = append(midday.Items, c.weaningItem(f, WeaningGrams("fish", m), l))
		} else if f, ok := pick("meat", d); ok {
			midday.Items = append(midday.Items, c.weaningItem(f, WeaningGrams("meat", m), l))
		}
		if f, ok := pick("yolk", d); ok && d%2 == 0 {
			midday.Items = append(midday.Items, c.weaningItem(f, WeaningGrams("yolk", m), l))
		}
		// ужин с 8 мес: творог или кефир с хлебом
		if f, ok := pick("curd", d); ok {
			evening.Items = append(evening.Items, c.weaningItem(f, WeaningGrams("curd", m), l))
		}
		if f, ok := pick("kefir", d); ok {
			evening.Items = append(evening.Items, c.weaningItem(f, WeaningGrams("kefir", m), l))
		}
		if f, ok := pick("bread", d); ok {
			evening.Items = append(evening.Items, c.weaningItem(f, WeaningGrams("bread", m), l))
		}
		// новый продукт — в первой половине дня, в утреннее кормление; если в этот день уже есть блюдо его
		// группы, новое не добавляется сверху, а постепенно занимает часть порции
		meals := []*WeaningFeed{&morning, &midday, &evening}
		if k.Feeding == FeedMix {
			for i, s := range feedSlots {
				switch k.MealSource(s) {
				case MealJars:
					meals[i].Items = c.weaningJars(meals[i].Items, l)
				case MealShared:
					if len(meals[i].Items) > 0 || m >= 8 { // с 8 мес и ужин — уже еда, не молоко
						meals[i].Items = []WeaningItem{{Group: "shared", Name: i18n.T(l, "weaning.shared")}}
					}
				}
			}
		}
		if hasNew {
			it := *w.New
			it.Grams = w.Ramp[d]
			for _, feed := range meals {
				for j := range feed.Items {
					if feed.Items[j].Group == it.Group && !feed.Items[j].New {
						feed.Items[j].Grams = roundWeaning(math.Max(0, feed.Items[j].Grams-it.Grams))
					}
				}
				feed.Items = slices.DeleteFunc(feed.Items, func(x WeaningItem) bool { return x.Grams == 0 && x.Group != "oil" && x.Group != "butter" && x.Group != "shared" })
			}
			meals[newAt].Items = append([]WeaningItem{it}, meals[newAt].Items...)
		}
		feeds := []WeaningFeed{milk("06:00")}
		for _, f := range []WeaningFeed{morning, midday, evening} {
			if len(f.Items) == 0 {
				f.Milk = true // прикорма в это кормление ещё нет — грудь или смесь
			}
			feeds = append(feeds, f)
		}
		feeds = append(feeds, milk("22:00"))
		if k.GivesFormula("night") && len(k.FormulaFeeds) > 0 {
			feeds = append(feeds, milk("02:00"))
		}
		markFormula(k, feeds)
		day.Feeds = feeds
		w.Days = append(w.Days, day)
	}
	return w
}

func (c *Catalog) weaningItem(f WeaningFood, grams float64, l i18n.Lang) WeaningItem {
	unit := "g"
	switch f.Group {
	case "kefir":
		unit = "ml"
	case "yolk":
		unit = "pcs"
	}
	n := c.WeaningName(f.ID, l)
	switch f.Group {
	case "yolk":
		n = i18n.T(l, "weaning.yolk")
	case "veg", "fruit", "meat", "fish":
		n = i18n.T(l, "weaning.form.puree", lowerFirst(l, n)) // «Пюре: кабачок» — граммы готового пюре, не сырого продукта
	case "cereal":
		n = i18n.T(l, "weaning.form.cereal", lowerFirst(l, n))
	}
	return WeaningItem{Food: f.ID, Recipe: f.Recipe, Name: n, Group: f.Group, Grams: roundWeaning(grams), Unit: unit}
}

// weaningExtra — масло к пюре и каше.
func (c *Catalog) weaningExtra(kind string, grams float64, l i18n.Lang) WeaningItem {
	id, unit := "sunflower_oil", "ml"
	if kind == "butter" {
		id, unit = "butter", "g"
	}
	return WeaningItem{Food: id, Name: c.WeaningName(id, l), Group: kind, Grams: grams, Unit: unit}
}

// weaningNeed — сколько продуктов купить на неделю прикорма (в единицах продукта базы).
func weaningNeed(w *Weaning) map[string]float64 {
	need := map[string]float64{}
	for _, d := range w.Days {
		for _, f := range d.Feeds {
			for _, it := range f.Items {
				if it.Food == "" {
					continue
				}
				raw := 1.0
				if wf, ok := weaningFood(it.Food); ok {
					raw = wf.RawPerUnit
				}
				need[it.Food] += it.Grams * raw
			}
		}
	}
	return need
}

// WeaningRow — строка таблицы 5.1 для страницы «Прикорм по месяцам»: объёмы как в программе, диапазонами.
type WeaningRow struct {
	Group string
	Cells [5]string // 4–5, 6, 7, 8, 9–12 мес
}

// WeaningTable — таблица 5.1 программы вскармливания (без соков и печенья, их планировщик не ставит).
// Мясо и рыба — отварные домашнего приготовления.
func WeaningTable() []WeaningRow {
	return []WeaningRow{
		{"veg", [5]string{"10–150", "150", "150", "150", "150"}},
		{"cereal", [5]string{"10–150", "150", "150", "180", "200"}},
		{"meat", [5]string{"—", "3–15", "20–30", "30–35", "40–50"}},
		{"fruit", [5]string{"5–50", "60", "70", "80", "90–100"}},
		{"yolk", [5]string{"—", "—", "¼", "½", "½"}},
		{"curd", [5]string{"—", "—", "—", "10–40", "50"}},
		{"fish", [5]string{"—", "—", "—", "5–30", "30–60"}},
		{"kefir", [5]string{"—", "—", "—", "200", "200"}},
		{"bread", [5]string{"—", "—", "—", "5", "10"}},
		{"oil", [5]string{"1–3", "5", "5", "6", "6"}},
		{"butter", [5]string{"1–3", "4", "4", "5", "5"}},
	}
}

// WeaningSample — примерный день в этом возрасте, когда введено всё, что положено к этому месяцу.
func (c *Catalog) WeaningSample(month int, l i18n.Lang) *Weaning {
	var intro []string
	for _, f := range WeaningFoods {
		if f.From <= month {
			intro = append(intro, f.ID)
		}
	}
	return c.buildWeaning(Child{AgeMonths: month, Feeding: FeedWeaning, Introduced: intro}, l)
}

// WeaningName — название продукта для прикорма: без пометок каталога в скобках («Брокколи (заморозка)» → «Брокколи»).
func (c *Catalog) WeaningName(id string, l i18n.Lang) string {
	n := id
	if ing, ok := c.Ingredients[id]; ok {
		n = ing.LocalName(l)
	}
	if i := strings.Index(n, " ("); i > 0 {
		n = n[:i]
	}
	return n
}

// lowerFirst — строчная первая буква для подстановки в «Пюре: {0}»; в немецком существительные с заглавной.
func lowerFirst(l i18n.Lang, s string) string {
	if l == i18n.DE {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}

// feedSlot — часть суток кормления по примерному режиму.
func feedSlot(t string) string {
	switch t {
	case "06:00":
		return "morning"
	case "22:00":
		return "bedtime"
	case "02:00":
		return "night"
	}
	return "day"
}

// markFormula — у ребёнка на смеси подписывает молочные кормления: смесь (сколько мл) или грудь.
// Объём одного кормления — по возрасту, а если родители указали мл в день, он делится на кормления смесью.
func markFormula(k Child, feeds []WeaningFeed) {
	if !k.Formula {
		return
	}
	var idx []int
	for i := range feeds {
		if !feeds[i].Milk || len(feeds[i].Items) > 0 {
			continue
		}
		if k.GivesFormula(feedSlot(feeds[i].Time)) {
			idx = append(idx, i)
		} else {
			feeds[i].MilkKind = "breast"
		}
	}
	if len(idx) == 0 {
		return
	}
	ml := formulaPerFeed(k.AgeMonths)
	if k.FormulaMl > 0 {
		ml = int(math.Round(float64(k.FormulaMl)/float64(len(idx))/10) * 10)
	}
	for _, i := range idx {
		feeds[i].MilkKind, feeds[i].MilkMl = "formula", ml
	}
}

// weaningJars — кормление из баночек вместо домашнего: те же группы и граммы, но покупная баночка
// (её считает babyItems), поэтому Food пустой и в список продуктов прикорма она не попадает.
// Желток, хлеб и масло — домашние, в баночное кормление их не ставим.
func (c *Catalog) weaningJars(items []WeaningItem, l i18n.Lang) []WeaningItem {
	out := []WeaningItem{}
	for _, it := range items {
		switch it.Group {
		case "veg", "fruit", "meat", "fish", "cereal", "curd", "kefir":
			out = append(out, WeaningItem{Group: "jar", Name: i18n.T(l, "weaning.jar."+it.Group), Grams: it.Grams, Unit: it.Unit})
		}
	}
	return out
}
