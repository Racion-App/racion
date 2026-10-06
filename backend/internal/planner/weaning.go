package planner

import (
	"math"
	"slices"

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
}

// WeaningFoods — продукты в порядке, в каком их обычно вводят: внутри группы сначала самые мягкие для
// пищеварения (кабачок, цветная капуста, брокколи; рис и гречка без глютена; индейка и кролик).
var WeaningFoods = []WeaningFood{
	{"zucchini", "veg", 4, 1.15},
	{"cauliflower", "veg", 4, 1.1},
	{"broccoli", "veg", 4, 1.1},
	{"buckwheat", "cereal", 4, 0.12},
	{"rice_round", "cereal", 4, 0.12},
	{"cornmeal", "cereal", 4, 0.12},
	{"pumpkin", "veg", 5, 1.25},
	{"carrot", "veg", 6, 1.15},
	{"potato", "veg", 6, 1.2},
	{"turkey_fillet", "meat", 6, 1.4},
	{"rabbit", "meat", 6, 1.6},
	{"oats", "cereal", 6, 0.12},
	{"apple", "fruit", 6, 1.2},
	{"pear", "fruit", 6, 1.2},
	{"veal", "meat", 6, 1.4},
	{"chicken_breast", "meat", 6, 1.4},
	{"banana", "fruit", 7, 1.3},
	{"plum", "fruit", 7, 1.2},
	{"eggs", "yolk", 7, 1},
	{"cottage_soft", "curd", 8, 1},
	{"kefir", "kefir", 8, 1},
	{"cod_fillet", "fish", 8, 1.3},
	{"hake_fillet", "fish", 8, 1.3},
	{"bread_white", "bread", 8, 1},
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
	Food  string  `json:"food"`  // id продукта базы; пусто — грудь или смесь
	Name  string  `json:"name"`  // название на языке недели
	Group string  `json:"group"` // группа прикорма или "milk"
	Grams float64 `json:"grams"` // г, для кефира мл, для желтка штуки
	Unit  string  `json:"unit"`  // g, ml, pcs
	New   bool    `json:"new"`   // продукт вводится на этой неделе
}

// WeaningFeed — кормление: время по примерному режиму и что в нём.
type WeaningFeed struct {
	Time  string        `json:"time"`
	Milk  bool          `json:"milk"` // грудь или смесь (после прикорма — докармливание)
	Items []WeaningItem `json:"items"`
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
	name := func(id string) string {
		if ing, ok := c.Ingredients[id]; ok {
			return ing.LocalName(l)
		}
		return id
	}
	byGroup := map[string][]string{}
	for _, f := range WeaningFoods {
		if slices.Contains(k.Introduced, f.ID) && f.From <= m {
			byGroup[f.Group] = append(byGroup[f.Group], f.ID)
			w.Introduced = append(w.Introduced, f.ID)
		}
	}
	newFood, hasNew := WeaningNext(m, k.Introduced)
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
		if hasNew {
			it := *w.New
			it.Grams = w.Ramp[d]
			for _, feed := range []*WeaningFeed{&morning, &midday, &evening} {
				for j := range feed.Items {
					if feed.Items[j].Group == it.Group && !feed.Items[j].New {
						feed.Items[j].Grams = roundWeaning(math.Max(0, feed.Items[j].Grams-it.Grams))
					}
				}
				feed.Items = slices.DeleteFunc(feed.Items, func(x WeaningItem) bool { return x.Grams == 0 && x.Group != "oil" && x.Group != "butter" })
			}
			morning.Items = append([]WeaningItem{it}, morning.Items...)
		}
		feeds := []WeaningFeed{milk("06:00")}
		for _, f := range []WeaningFeed{morning, midday, evening} {
			if len(f.Items) == 0 {
				f.Milk = true // прикорма в это кормление ещё нет — грудь или смесь
			}
			feeds = append(feeds, f)
		}
		feeds = append(feeds, milk("22:00"))
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
	n := f.ID
	if ing, ok := c.Ingredients[f.ID]; ok {
		n = ing.LocalName(l)
	}
	switch f.Group {
	case "yolk":
		n = i18n.T(l, "weaning.yolk")
	case "veg", "fruit", "meat", "fish":
		n = i18n.T(l, "weaning.form.puree", n) // «Кабачок: пюре» — граммы готового пюре, не сырого продукта
	case "cereal":
		n = i18n.T(l, "weaning.form.cereal", n)
	}
	return WeaningItem{Food: f.ID, Name: n, Group: f.Group, Grams: roundWeaning(grams), Unit: unit}
}

// weaningExtra — масло к пюре и каше.
func (c *Catalog) weaningExtra(kind string, grams float64, l i18n.Lang) WeaningItem {
	id, unit := "sunflower_oil", "ml"
	if kind == "butter" {
		id, unit = "butter", "g"
	}
	n := id
	if ing, ok := c.Ingredients[id]; ok {
		n = ing.LocalName(l)
	}
	return WeaningItem{Food: id, Name: n, Group: kind, Grams: grams, Unit: unit}
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
