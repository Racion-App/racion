package planner

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"
	"time"

	"racion/internal/i18n"
)

// Режимы кормления ребёнка.
const (
	FeedShared   = "shared"   // ест с общего стола (порция по возрасту, общие блюда фильтруются)
	FeedSeparate = "separate" // готовим отдельно — своё детское меню на неделю
	FeedJars     = "jars"     // баночки и детские каши по возрастным нормам
	FeedMilk     = "milk"     // только смесь или грудное молоко
	FeedWeaning  = "weaning"  // прикорм по месяцам: режим дня и новый продукт недели (weaning.go)
	FeedMix      = "mix"      // комбинирую: у каждого приёма свой источник (Child.Meals)
)

var FeedingModes = []string{FeedShared, FeedSeparate, FeedJars, FeedMilk, FeedWeaning, FeedMix}

func FeedingLabel(l i18n.Lang, mode string) string { return i18n.T(l, "feeding."+mode) }

// Child — ребёнок в семье. Возраст в месяцах, чтобы одной шкалой покрыть и грудничков, и школьников.
type Child struct {
	Name         string `json:"name,omitempty"` // имя в семье; планировщику не нужно
	AgeMonths    int    `json:"ageMonths"`
	Feeding      string `json:"feeding"`      // shared | separate | jars | milk
	SharesMeals  bool   `json:"sharesMeals"`  // устаревшее поле; читается, если Feeding пуст
	Formula      bool   `json:"formula"`      // на смеси
	FormulaBrand string `json:"formulaBrand"` // код из FormulaBrands
	FormulaMl    int    `json:"formulaMl"`    // мл в день; 0 — по возрастной норме
	// FormulaFeeds — когда даёт смесь (FormulaSlots): только утром, на ночь и т. п.; пусто — во все кормления.
	// До года отметки нужны только при смешанном вскармливании (Breast), с года — смесь как напиток.
	FormulaFeeds []string `json:"formulaFeeds,omitempty"`
	// Breast — ещё и грудное молоко: смешанное вскармливание («грудное молоко в любом сочетании с
	// адаптированной смесью», Программа вскармливания 2019). Тогда отмеченные кормления — смесь, остальные —
	// грудь. Без этого флага ребёнок на смеси получает её во все молочные кормления.
	Breast bool `json:"breast,omitempty"`
	// Meals — в режиме «комбинирую»: приём (MealSlots) → home | jars | shared
	Meals map[string]string `json:"meals,omitempty"`
	// Introduced — продукты прикорма, которые ребёнок уже ест (id продуктов базы из WeaningFoods)
	Introduced []string `json:"introduced,omitempty"`
	// Avoid — продукты, на которые была реакция («не подошло» в дневнике прикорма): в прикорм, детское
	// меню и общие блюда ребёнка не ставятся, пока родители не вернут
	Avoid []string `json:"avoid,omitempty"`
	// Allergens — аллергии самого ребёнка (коды как у семьи: dairy, eggs, gluten…): прикорм, детское меню
	// и, если ребёнок ест с общего стола, общие блюда их учитывают
	Allergens []string `json:"allergens,omitempty"`
	// AgeAt — месяц (YYYY-MM), когда указан возраст: в плане на более поздний месяц возраст растёт сам,
	// родителям не нужно каждый месяц его править
	AgeAt string `json:"ageAt,omitempty"`
	// Away — где ребёнок ест по будням: "" — дома, kindergarten — в саду (завтрак, обед, полдник),
	// school — в школе (обед). Порции и детское меню на будни это учитывают.
	Away string `json:"away,omitempty"`
}

// Где ребёнок ест по будням.
const (
	AwayKindergarten = "kindergarten"
	AwaySchool       = "school"
)

// AwaySlots — приёмы, которые ребёнок по будням ест не дома: в детском саду (10,5–12 ч) завтрак, второй
// завтрак, обед и полдник, дома ужин; у школьника обед в школе.
func (c Child) AwaySlots() []string {
	switch c.Away {
	case AwayKindergarten:
		return []string{"breakfast", "lunch", "snack"}
	case AwaySchool:
		return []string{"lunch"}
	}
	return nil
}

// AwayOn — ест ли ребёнок этот приём в этот день не дома (дни плана с понедельника: 0–4 — будни).
func (c Child) AwayOn(day int, slot string) bool {
	return day < 5 && slices.Contains(c.AwaySlots(), slot)
}

// grow — возраст на дату начала плана: «8 мес» в октябре — это 10 мес в плане на декабрь.
func (c *Child) grow(start time.Time) {
	at, err := time.Parse("2006-01", c.AgeAt)
	if err != nil {
		c.AgeAt = ""
		return
	}
	months := (start.Year()-at.Year())*12 + int(start.Month()) - int(at.Month())
	if months <= 0 {
		return
	}
	c.AgeMonths = min(c.AgeMonths+months, 17*12)
	c.AgeAt = start.Format("2006-01")
}

// FeedingOptions — какие режимы доступны в этом возрасте (первый — рекомендуемый по умолчанию).
func FeedingOptions(ageMonths int) []string {
	switch {
	case ageMonths < 4:
		return []string{FeedMilk}
	case ageMonths < 6: // прикорм с 4–6 мес по решению педиатра; по умолчанию пока молоко
		return []string{FeedMilk, FeedWeaning}
	case ageMonths < 12:
		return []string{FeedWeaning, FeedJars, FeedSeparate, FeedShared, FeedMix}
	case ageMonths < 36:
		return []string{FeedShared, FeedSeparate, FeedJars, FeedMix}
	default:
		return []string{FeedShared, FeedSeparate, FeedMix}
	}
}

func (c Child) normalized() Child {
	if c.Feeding == "" {
		if c.SharesMeals {
			c.Feeding = FeedShared
		} else if c.AgeMonths < 6 {
			c.Feeding = FeedMilk
		} else if c.AgeMonths < 12 {
			c.Feeding = FeedWeaning
		} else {
			c.Feeding = FeedShared
		}
	}
	opts := FeedingOptions(c.AgeMonths)
	if !slices.Contains(opts, c.Feeding) {
		c.Feeding = opts[0]
	}
	c.SharesMeals = c.Feeding == FeedShared
	if c.AgeMonths >= 36 {
		c.Formula = false
	}
	if c.Formula && c.FormulaBrand == "" {
		c.FormulaBrand = "other"
	}
	if c.FormulaMl < 0 || c.FormulaMl > 1500 {
		c.FormulaMl = 0
	}
	var feeds []string
	if c.Formula {
		for _, f := range c.FormulaFeeds {
			if slices.Contains(FormulaSlots, f) && !slices.Contains(feeds, f) {
				feeds = append(feeds, f)
			}
		}
	}
	if !c.Formula || c.AgeMonths >= 24 {
		c.Breast = false
	}
	if c.Formula && !c.Breast && c.AgeMonths < 12 {
		feeds = nil // только смесь — во все молочные кормления, отметки «когда» не нужны
	}
	c.FormulaFeeds = feeds
	var meals map[string]string
	if c.Feeding == FeedMix {
		meals = map[string]string{}
		for _, s := range MealSlots(c.AgeMonths) {
			v := c.Meals[s]
			if !slices.Contains([]string{MealHome, MealJars, MealShared}, v) || (v == MealJars && c.AgeMonths >= 36) {
				v = MealHome
			}
			meals[s] = v
		}
	}
	c.Meals = meals
	// введённые продукты — только из списка прикорма, без повторов
	var intro []string
	for _, id := range c.Introduced {
		if _, ok := weaningFood(id); ok && !slices.Contains(intro, id) {
			intro = append(intro, id)
		}
	}
	c.Introduced = intro
	var avoid []string
	for _, id := range c.Avoid {
		if id != "" && !slices.Contains(avoid, id) && !slices.Contains(c.Introduced, id) && len(avoid) < 40 {
			avoid = append(avoid, id)
		}
	}
	c.Avoid = avoid
	var al []string
	for _, a := range c.Allergens {
		if slices.Contains(Allergens, a) && !slices.Contains(al, a) {
			al = append(al, a)
		}
	}
	c.Allergens = al
	switch {
	case c.Feeding == FeedMilk || c.Feeding == FeedWeaning || c.AgeMonths < 12:
		c.Away = ""
	case c.Away == AwaySchool && c.AgeMonths < 72:
		c.Away = AwayKindergarten
	case c.Away == AwayKindergarten && c.AgeMonths >= 96:
		c.Away = AwaySchool
	case c.Away != AwayKindergarten && c.Away != AwaySchool:
		c.Away = ""
	}
	return c
}

// avoids — нельзя ли продукт: исключён семьёй или в нём аллерген из списка.
func (c *Catalog) avoids(id string, allergens, exclude []string) bool {
	if slices.Contains(exclude, id) {
		return true
	}
	if ing, ok := c.Ingredients[id]; ok {
		for _, a := range ing.Allergens {
			if slices.Contains(allergens, a) {
				return true
			}
		}
	}
	return false
}

// recipeAvoids — есть ли в рецепте продукт, который нельзя.
func (c *Catalog) recipeAvoids(id string, allergens, exclude []string) bool {
	r, ok := c.RecipeByID[id]
	if !ok {
		return true
	}
	for _, ri := range r.Ingredients {
		if c.avoids(ri.IngredientID, allergens, exclude) {
			return true
		}
	}
	return false
}

// kidAvoid — аллергии и исключения для блюд ребёнка: семейные плюс его собственные (аллергии и продукты,
// на которые была реакция).
func kidAvoid(k Child, p Params) (allergens, exclude []string) {
	allergens = slices.Clone(p.Allergens)
	for _, a := range k.Allergens {
		if !slices.Contains(allergens, a) {
			allergens = append(allergens, a)
		}
	}
	return allergens, append(slices.Clone(p.Exclude), k.Avoid...)
}

// Источники приёма пищи в режиме «комбинирую».
const (
	MealHome   = "home"   // готовим сами: прикорм по программе или детское меню
	MealJars   = "jars"   // баночки, детские каши и творожки
	MealShared = "shared" // с общего стола, порция по возрасту
)

// MealSlots — приёмы, для которых в режиме «комбинирую» выбирают источник: до года без полдника.
func MealSlots(ageMonths int) []string {
	if ageMonths < 12 {
		return []string{"breakfast", "lunch", "dinner"}
	}
	return []string{"breakfast", "lunch", "dinner", "snack"}
}

// MealSource — откуда ребёнок ест этот приём: home | jars | shared; пусто — не ест (только молоко).
func (c Child) MealSource(slot string) string {
	switch c.Feeding {
	case FeedMix:
		return c.Meals[slot]
	case FeedShared:
		return MealShared
	case FeedJars:
		return MealJars
	case FeedSeparate, FeedWeaning:
		return MealHome
	}
	return ""
}

// eatsShared — ест ли хоть один приём с общего стола (тогда общие блюда фильтруются под возраст).
func (c Child) eatsShared() bool {
	for _, s := range SlotOrder {
		if c.MealSource(s) == MealShared {
			return true
		}
	}
	return false
}

// SlotPortionFactor — доля взрослой порции в этом приёме, если ребёнок ест его с общего стола.
func (c Child) SlotPortionFactor(slot string) float64 {
	if c.MealSource(slot) != MealShared {
		return 0
	}
	return c.ageFactor()
}

// PortionFactor — доля взрослой порции по возрасту в среднем за день (для тех, кто ест с общего стола).
func (c Child) PortionFactor() float64 {
	switch c.Feeding {
	case FeedShared:
		return c.ageFactor()
	case FeedMix:
		n := 0
		for _, s := range MealSlots(c.AgeMonths) {
			if c.Meals[s] == MealShared {
				n++
			}
		}
		return math.Round(c.ageFactor()*float64(n)/float64(len(MealSlots(c.AgeMonths)))*100) / 100
	}
	return 0
}

func (c Child) ageFactor() float64 {
	// МР 2.3.1.0253-21, табл. 21: 1–2 года 1300 ккал, 3–6 лет 1800, 7–10 лет 2100, 11–14 лет 2300–2500,
	// 15–17 лет 2500–2900. Общий стол даёт до 7 лет около 3/4 дневной нормы, дальше около 85%; взрослый — 2100.
	switch {
	case c.AgeMonths < 6:
		return 0
	case c.AgeMonths < 12:
		return 0.15 // до года со стола — несколько ложек, остальное грудь, смесь и прикорм
	case c.AgeMonths < 36:
		return 0.45
	case c.AgeMonths < 84:
		return 0.65
	case c.AgeMonths < 132:
		return 0.85
	case c.AgeMonths < 180:
		return 1
	default:
		return 1.1
	}
}

// MenuFactor — множитель к детскому рецепту (рецепты написаны на ребёнка 1–3 лет).
func (c Child) MenuFactor() float64 {
	switch {
	case c.AgeMonths < 9:
		return 0.5
	case c.AgeMonths < 12:
		return 0.7
	case c.AgeMonths < 36:
		return 1
	case c.AgeMonths < 84:
		return 1.3
	case c.AgeMonths < 132:
		return 1.6
	default:
		return 1.8 // СанПиН 2.3/2.4.4282-26, прил. 9: порции 12+ почти вдвое больше, чем в 1–3 года
	}
}

// FormulaMlPerDay — ориентировочная суточная норма смеси по возрасту (усреднённые рекомендации педиатров).
func (c Child) FormulaMlPerDay() int {
	if c.FormulaMl > 0 {
		return c.FormulaMl
	}
	if len(c.FormulaFeeds) > 0 {
		n := 0
		for _, f := range c.FormulaFeeds {
			n += formulaFeedsIn(f, c.AgeMonths)
		}
		return n * formulaPerFeed(c.AgeMonths)
	}
	switch {
	case c.AgeMonths < 1:
		return 600
	case c.AgeMonths < 2:
		return 750
	case c.AgeMonths < 4:
		return 850
	case c.AgeMonths < 6:
		return 900
	case c.AgeMonths < 9:
		return 700
	case c.AgeMonths < 12:
		return 500
	case c.AgeMonths < 24:
		return 350
	case c.AgeMonths < 36:
		return 250
	default:
		return 200
	}
}

// FormulaSlots — части суток, когда ребёнок может получать смесь: утром после сна, днём, на ночь, ночью.
var FormulaSlots = []string{"morning", "day", "bedtime", "night"}

// formulaPerFeed — смесь за одно кормление по возрасту, мл (таблицы на банках смесей, усреднённо).
func formulaPerFeed(m int) int {
	switch {
	case m < 1:
		return 80
	case m < 2:
		return 110
	case m < 4:
		return 140
	case m < 6:
		return 180
	default:
		return 200
	}
}

// formulaFeedsIn — сколько молочных кормлений приходится на часть суток в этом возрасте. С 6 месяцев
// дневные кормления занимает прикорм: в 6–7 месяцев остаётся одно в 18:00, с 8 — ни одного.
func formulaFeedsIn(slot string, m int) int {
	switch slot {
	case "morning", "bedtime":
		return 1
	case "night":
		switch {
		case m < 2:
			return 2
		default:
			return 1
		}
	case "day":
		switch {
		case m < 1:
			return 4
		case m < 4:
			return 3
		case m < 6:
			return 2
		case m < 8:
			return 1
		}
	}
	return 0
}

// GivesFormula — получает ли ребёнок смесь в эту часть суток.
func (c Child) GivesFormula(slot string) bool {
	return c.Formula && (len(c.FormulaFeeds) == 0 || slices.Contains(c.FormulaFeeds, slot))
}

// FormulaStage — ступень смеси по возрасту: 1 (0–6 мес), 2 (6–12), 3 (12+).
func (c Child) FormulaStage() int {
	switch {
	case c.AgeMonths < 6:
		return 1
	case c.AgeMonths < 12:
		return 2
	default:
		return 3
	}
}

func (c Child) AgeLabel(l i18n.Lang) string {
	if c.AgeMonths == 0 {
		return i18n.T(l, "age.newborn")
	}
	if c.AgeMonths < 24 {
		return i18n.T(l, "age.months", c.AgeMonths)
	}
	y := c.AgeMonths / 12
	return fmt.Sprintf("%d %s", y, i18n.Plural(l, y, "age.year"))
}

func plural(n int, one, few, many string) string {
	m10, m100 := n%10, n%100
	switch {
	case m10 == 1 && m100 != 11:
		return one
	case m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20):
		return few
	default:
		return many
	}
}

// FormulaBrand — марка смеси: типовая банка и множитель к средней цене Росстата
// («Смеси сухие молочные для детского питания, кг», код 1123).
type FormulaBrand struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	PackG  float64 `json:"packG"`
	Factor float64 `json:"factor"`
	Note   string  `json:"note"`
}

var FormulaBrands = []FormulaBrand{
	{"nutrilon", "Nutrilon", 800, 1.25, ""},
	{"nan", "NAN (Nestlé)", 800, 1.2, ""},
	{"similac", "Similac", 600, 1.1, ""},
	{"friso", "Friso", 800, 1.3, ""},
	{"hipp", "HiPP", 600, 1.35, ""},
	{"nestogen", "Nestogen", 600, 0.85, ""},
	{"nutrilak", "Nutrilak", 600, 0.8, ""},
	{"malyutka", "formula.malyutka", 600, 0.7, ""},
	{"bellakt", "formula.bellakt", 400, 0.65, ""},
	{"kabrita", "formula.kabrita", 800, 2.2, "formula.note.goat"},
	{"other", "formula.other", 600, 1.0, ""},
}

// FormulaBrandsFor — марки с подписями на языке (названия-ключи переводятся, остальные как есть). Марки
// не ранжируем: никаких «премиум», только состав (козье молоко), по алфавиту, «любая» — первой: её
// предлагаем по умолчанию, цена по ней — средняя по Росстату.
func FormulaBrandsFor(l i18n.Lang) []FormulaBrand {
	out := make([]FormulaBrand, len(FormulaBrands))
	for i, b := range FormulaBrands {
		if strings.HasPrefix(b.Name, "formula.") {
			b.Name = i18n.T(l, b.Name)
		}
		if strings.HasPrefix(b.Note, "formula.") {
			b.Note = i18n.T(l, b.Note)
		}
		out[i] = b
	}
	slices.SortStableFunc(out, func(a, b FormulaBrand) int {
		if (a.ID == "other") != (b.ID == "other") {
			if a.ID == "other" {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}

func formulaBrand(id string) FormulaBrand {
	for _, b := range FormulaBrands {
		if b.ID == id {
			return b
		}
	}
	return FormulaBrands[len(FormulaBrands)-1]
}

// Коды Росстата для детского питания и ручные цены за кг на случай, если Росстат недоступен.
const (
	rosstatFormula    = 1123 // смеси сухие молочные, кг
	rosstatBabyVeg    = 1303 // консервы овощные для детского питания, кг
	rosstatBabyFruit  = 1402 // консервы фруктово-ягодные для детского питания, кг
	rosstatBabyMeat   = 302  // консервы мясные для детского питания, кг
	rosstatBabyCurd   = 1128 // творожок детский, кг
	rosstatBabyMilk   = 1129 // молоко для детей, л
	fallbackFormula   = 1400.0
	fallbackBabyVeg   = 760.0
	fallbackBabyFrt   = 630.0
	fallbackBabyMeat  = 1320.0
	fallbackBabyCurd  = 900.0
	fallbackBabyMilk  = 180.0
	fallbackBabyKasha = 950.0 // детская сухая каша, ₽/кг — Росстат её не считает
	powderPer100ml    = 13.5  // г сухой смеси на 100 мл готовой
)

// Ограничения для блюд с общего стола, если за столом маленький ребёнок.
// До 3 лет: острое, грибы, колбасные изделия, морепродукты, майонез, цельные орехи и семечки, сыры с плесенью.
// 3–7 лет: острое и сырое (тартар, устрицы).
var kidsExcludeUnder3 = []string{
	"honey", "mushrooms", // мёд — не раньше года (NHS), грибы — не раньше 3 лет (программа 1–3)
	"sausages", "sausage_boiled", "chicken_sausages", "hunting_sausages", "salami", "ham", "bacon", // колбасные изделия
	"shrimp", "king_prawns", "mussels", "scallops", "oysters", "red_caviar", // морепродукты и икра
	"herring", "anchovies", "mackerel_smoked", // солёная и копчёная рыба
	"blue_cheese", "mayo", "ketchup", "soy_sauce", "pickles", "capers", "carrot_korean", // соусы, маринады
	"vinegar", "apple_cider_vinegar", "rice_vinegar", "chili_flakes", "mustard", "horseradish", "adjika",
	"coffee", "coffee_instant", "dark_chocolate", "chocolate_milk", "chocolate_spread", // кофе, шоколад до 3 лет
	"peanuts", "peanut_butter", "walnuts", "almonds", "cashews", "hazelnuts", "pistachios", "pine_nuts", "nut_mix", // целые орехи
	"sunflower_seeds", "pumpkin_seeds", "olives", "olives_black",
}

// kidsExcludeAny — алкоголь в блюдах: если с общего стола ест ребёнок любого возраста, такие блюда не ставим.
var kidsExcludeAny = []string{"white_wine", "red_wine", "beer_light"}

func kidsRestrictions(kids []Child) (excludeIngredients []string, excludeTags []string, minAgeSharing int) {
	minAgeSharing = -1
	for _, k := range kids {
		if !k.eatsShared() {
			continue
		}
		if minAgeSharing < 0 || k.AgeMonths < minAgeSharing {
			minAgeSharing = k.AgeMonths
		}
	}
	if minAgeSharing < 0 {
		return nil, nil, minAgeSharing
	}
	if minAgeSharing < 36 {
		return append(slices.Clone(kidsExcludeUnder3), kidsExcludeAny...), []string{"spicy", "raw"}, minAgeSharing
	}
	if minAgeSharing < 84 {
		return slices.Clone(kidsExcludeAny), []string{"spicy", "raw"}, minAgeSharing
	}
	return slices.Clone(kidsExcludeAny), nil, minAgeSharing
}

// jarNorm — дневные нормы баночного питания по возрасту, г. Ориентировочно, по методичкам педиатров.
type jarNorm struct {
	veg, fruit, meat, fish, kasha, curd float64
	milk                                float64 // мл детского молочка/кефира
}

func jarNormFor(age int) jarNorm {
	switch {
	case age < 6:
		return jarNorm{}
	case age < 8:
		return jarNorm{veg: 120, fruit: 60, meat: 0, kasha: 25}
	case age < 10:
		return jarNorm{veg: 160, fruit: 80, meat: 50, kasha: 30, curd: 30}
	case age < 12:
		return jarNorm{veg: 180, fruit: 100, meat: 60, kasha: 30, curd: 40, milk: 150}
	case age < 24:
		return jarNorm{veg: 180, fruit: 100, meat: 70, kasha: 35, curd: 50, milk: 200}
	default:
		return jarNorm{veg: 150, fruit: 100, meat: 70, kasha: 30, curd: 50, milk: 200}
	}
}

// jarShare — баночки только на приёмы, отмеченные «баночки»: утром каша и фрукты, в обед овощи и мясо,
// вечером творожок и кефир, в полдник вторая половина фруктов.
func jarShare(n jarNorm, k Child) jarNorm {
	on := func(s string) float64 {
		if k.MealSource(s) == MealJars {
			return 1
		}
		return 0
	}
	fruit := on("breakfast")
	if slices.Contains(MealSlots(k.AgeMonths), "snack") {
		fruit = (on("breakfast") + on("snack")) / 2
	}
	return jarNorm{veg: n.veg * on("lunch"), meat: n.meat * on("lunch"), kasha: n.kasha * on("breakfast"),
		fruit: n.fruit * fruit, curd: n.curd * on("dinner"), milk: n.milk * on("dinner")}
}

// babyItems — строки списка покупок для детей: смесь, баночки, каши. Всё на неделю.
// jarsFromWeaning — баночки по расписанию прикорма: сколько в среднем в день по группам. Каша в расписании —
// готовая, в покупках — сухая: из 1 г сухой детской каши получается около 5 г готовой.
func jarsFromWeaning(w *Weaning) (jarNorm, bool) {
	if w == nil || len(w.Days) == 0 {
		return jarNorm{}, false
	}
	sum := map[string]float64{}
	for _, d := range w.Days {
		for _, f := range d.Feeds {
			for _, it := range f.Items {
				if it.Jar {
					sum[it.Group] += it.Grams
				}
			}
		}
	}
	n := float64(len(w.Days))
	return jarNorm{veg: sum["veg"] / n, fruit: sum["fruit"] / n, meat: sum["meat"] / n, fish: sum["fish"] / n,
		kasha: sum["cereal"] / n / 5, curd: sum["curd"] / n, milk: sum["kefir"] / n}, true
}

func (c *Catalog) babyItems(kids []Child, menus []KidMenu, pr pricer) ([]ShopItem, []string) {
	var items []ShopItem
	var notes []string
	l := pr.lang
	kgPrice := func(item int, fallback float64) (float64, bool) {
		if item > 0 && pr.pb != nil && pr.country.Code == "RU" {
			if v, ok := pr.pb.Price(pr.region, item); ok {
				return v, true
			}
		}
		return pr.kgFallback(fallback), false
	}
	jar := func(idx int, id, name string, perDay, pack float64, item int, fallback float64, who string, unit string) ShopItem {
		need := perDay * 7
		packs := int(math.Ceil(need/pack - 1e-9))
		perKg, fromRosstat := kgPrice(item, fallback)
		return ShopItem{
			IngredientID: fmt.Sprintf("%s_%d", id, idx), Name: i18n.T(l, name), Category: "baby", Unit: unit,
			Needed: need, Buy: float64(packs) * pack, Packs: packs, Pack: pack,
			Cost: pr.round(float64(packs) * pack / 1000 * perKg * pr.idx), Rosstat: fromRosstat,
			UsedIn: []string{i18n.T(l, "baby.perday", who, perDay, i18n.T(l, "unit."+unit))},
		}
	}
	for i, k := range kids {
		who := i18n.T(l, "baby.child", k.AgeLabel(l))
		if k.Formula && slices.Contains(k.Allergens, "dairy") {
			// КР «Пищевая аллергия», 2025: высокогидролизная или аминокислотная смесь, козья не подходит
			notes = append(notes, i18n.T(l, "note.formula.cmpa", k.AgeLabel(l)))
		}
		if k.Formula {
			b := formulaBrand(k.FormulaBrand)
			if strings.HasPrefix(b.Name, "formula.") {
				b.Name = i18n.T(l, b.Name)
			}
			ml := k.FormulaMlPerDay()
			grams := float64(ml) * 7 * powderPer100ml / 100
			packs := int(math.Ceil(grams/b.PackG - 1e-9))
			if packs < 1 {
				packs = 1
			}
			perKg, fromRosstat := kgPrice(rosstatFormula, fallbackFormula)
			items = append(items, ShopItem{
				IngredientID: fmt.Sprintf("formula_%d", i),
				Name:         i18n.T(l, "baby.formula", b.Name, k.FormulaStage()),
				Category:     "baby", Unit: "g",
				Needed: math.Round(grams), Buy: float64(packs) * b.PackG, Packs: packs, Pack: b.PackG,
				Cost: pr.round(float64(packs) * b.PackG / 1000 * perKg * b.Factor * pr.idx), Rosstat: fromRosstat,
				UsedIn: []string{i18n.T(l, "baby.perday", who, float64(ml), i18n.T(l, "unit.ml"))},
			})
		}
		switch k.Feeding {
		case FeedJars, FeedMix:
			n := jarNormFor(k.AgeMonths)
			if k.Feeding == FeedMix {
				n = jarShare(n, k)
			}
			// до года баночки — по расписанию прикорма: те же продукты и граммы, что в плане
			for _, km := range menus {
				if km.Child == i && km.Weaning != nil {
					if wn, ok := jarsFromWeaning(km.Weaning); ok {
						n = wn
					}
				}
			}
			if n.veg > 0 {
				items = append(items, jar(i, "baby_veg", "baby.veg", n.veg, 100, rosstatBabyVeg, fallbackBabyVeg, who, "g"))
			}
			if n.fruit > 0 {
				items = append(items, jar(i, "baby_fruit", "baby.fruit", n.fruit, 100, rosstatBabyFruit, fallbackBabyFrt, who, "g"))
			}
			if n.meat > 0 {
				items = append(items, jar(i, "baby_meat", "baby.meat", n.meat, 80, rosstatBabyMeat, fallbackBabyMeat, who, "g"))
			}
			if n.fish > 0 { // Росстат рыбные баночки отдельно не считает — цена как у мясных
				items = append(items, jar(i, "baby_fish", "baby.fish", n.fish, 100, rosstatBabyMeat, fallbackBabyMeat, who, "g"))
			}
			if n.kasha > 0 {
				items = append(items, jar(i, "baby_kasha", "baby.kasha", n.kasha, 200, 0, fallbackBabyKasha, who, "g"))
			}
			if n.curd > 0 {
				items = append(items, jar(i, "baby_curd", "baby.curd", n.curd, 100, rosstatBabyCurd, fallbackBabyCurd, who, "g"))
			}
			if n.milk > 0 {
				items = append(items, jar(i, "baby_milk", "baby.milk", n.milk, 200, rosstatBabyMilk, fallbackBabyMilk, who, "ml"))
			}
		case FeedShared:
			// С общего стола до года: порции крошечные, докармливаем пюре.
			if k.AgeMonths < 12 {
				n := jarNormFor(k.AgeMonths)
				items = append(items,
					jar(i, "baby_veg", "baby.veg.top", n.veg*0.6, 100, rosstatBabyVeg, fallbackBabyVeg, who, "g"),
					jar(i, "baby_fruit", "baby.fruit.top", n.fruit, 100, rosstatBabyFruit, fallbackBabyFrt, who, "g"),
				)
			}
		case FeedMilk:
			if !k.Formula {
				notes = append(notes, i18n.T(l, "note.breastfed", k.AgeLabel(l)))
			}
		}
	}
	return mergeBaby(items), notes
}

// mergeBaby — одинаковые строки детского питания (у близнецов одна смесь, одни баночки) сливаются в одну:
// упаковки считаются от общей потребности, а не округляются для каждого ребёнка отдельно.
func mergeBaby(items []ShopItem) []ShopItem {
	var out []ShopItem
	at := map[string]int{}
	for _, it := range items {
		key := it.Name + "|" + it.Unit + "|" + fmt.Sprint(it.Pack)
		i, ok := at[key]
		if !ok {
			at[key] = len(out)
			out = append(out, it)
			continue
		}
		m := &out[i]
		per := 0.0
		if m.Buy > 0 {
			per = m.Cost / m.Buy
		}
		m.Needed += it.Needed
		m.Packs = int(math.Ceil(m.Needed/m.Pack - 1e-9))
		m.Buy = float64(m.Packs) * m.Pack
		m.Cost = math.Round(per * m.Buy)
		m.UsedIn = append(m.UsedIn, it.UsedIn...)
	}
	return out
}

// ── Детское меню (режим «готовим отдельно») ────────────────────────────────

// KidDish — блюдо детского меню; количества уже умножены на возрастной множитель при подсчёте покупок.
type KidDish struct {
	Slot     string  `json:"slot"`
	RecipeID string  `json:"recipeId"`
	Title    string  `json:"title"`
	TimeMin  int     `json:"timeMin"`
	Kcal     float64 `json:"kcal"`
	Cost     float64 `json:"cost"`
	Kind     string  `json:"kind,omitempty"` // jars | shared — приём не из детского меню (режим «комбинирую»); none — подходящего рецепта нет
	Note     string  `json:"note,omitempty"` // как подать малышу: нарезка против удушья
	Ref      string  `json:"ref,omitempty"`  // shared: рецепт семьи в этот приём
}

// kidFallback — когда в детской базе на приём нет рецепта без аллергенов ребёнка (от 3 лет): обычные блюда
// этого приёма с теми же ограничениями, что у общего стола для его возраста (без острого и сырого),
// без его аллергенов и того, что исключила семья.
func (c *Catalog) kidFallback(slot string, k Child, p Params, allergens []string) []Recipe {
	q := p
	q.Kids = []Child{{AgeMonths: k.AgeMonths, Feeding: FeedShared, Allergens: k.Allergens}}
	q.Allergens = allergens
	e := c.effective(q)
	var out []Recipe
	for _, r := range c.Recipes {
		if r.Slot == slot && !isKidRecipe(r) && !IsSide(r) && c.allowed(r, e) {
			out = append(out, r)
		}
	}
	return out
}

// KidNone — приём, на который в детской базе нет рецепта под аллергии ребёнка.
const KidNone = "none"

// KidAway — приём в саду или школе (Kind: away.kindergarten, away.school).
const KidAway = "away"

// KidBedtime — кефир или молоко перед сном для детей 1–3 лет (пятый приём по программе питания 1–3 лет).
const KidBedtime = "bedtime"

// bedtimeKefirMl — сколько кефира перед сном, мл: дополнительный приём около 10% дневной калорийности.
const bedtimeKefirMl = 200

// kidDishTitle — подпись блюда детского меню на языке; для приёма-баночки и приёма с общего стола — своя.
func (c *Catalog) kidDishTitle(d KidDish, l i18n.Lang) string {
	if strings.HasPrefix(d.Kind, KidAway+".") {
		return i18n.T(l, "kid."+d.Kind)
	}
	switch d.Kind {
	case KidBedtime:
		return i18n.T(l, "kid.bedtime", bedtimeKefirMl)
	case KidNone:
		return i18n.T(l, "kid.none")
	case MealJars:
		return i18n.T(l, "kid.jars."+d.Slot)
	case MealShared:
		if r, ok := c.RecipeByID[d.Ref]; ok {
			return i18n.T(l, "kid.shared.dish", r.LocalTitle(l))
		}
		return i18n.T(l, "kid.shared")
	}
	return c.RecipeByID[d.RecipeID].LocalTitle(l)
}

type KidDay struct {
	Index  int       `json:"index"`
	Label  string    `json:"label"`
	Dishes []KidDish `json:"dishes"`
}

type KidMenu struct {
	Child    int      `json:"child"` // индекс в Params.Kids
	AgeLabel string   `json:"ageLabel"`
	Factor   float64  `json:"factor"` // множитель к рецепту
	Days     []KidDay `json:"days"`
	Note     string   `json:"note"`
	Norm     string   `json:"norm,omitempty"`    // норма ккал, белка и питья по возрасту (МР 2.3.1.0253-21)
	Weaning  *Weaning `json:"weaning,omitempty"` // прикорм по месяцам вместо меню из рецептов
}

var kidSlots = []string{"breakfast", "lunch", "dinner", "snack"}

// kidRecipeMinAge — с какого возраста (мес) рецепт подходит. Берётся из тега вида "age6" / "age12" / "age18" / "age36".
func kidRecipeMinAge(r Recipe) int {
	for _, t := range r.Tags {
		var m int
		if n, _ := fmt.Sscanf(t, "age%d", &m); n == 1 {
			return m
		}
	}
	return 12
}

func isKidRecipe(r Recipe) bool { return slices.Contains(r.Tags, "kidmenu") }

// kidRecipeFits — подходит ли детский рецепт по возрасту: не раньше своего тега ageN и не «малышовый» для
// старших. Блюда прикорма (wean_) — до 15 мес, протёртые блюда с тегом baby («говяжье пюре с овощами») —
// до 2 лет: пятилетке «треска, мелко измельчённая» ни к чему. Банан, каша и тефтели подходят всем.
func kidRecipeFits(r Recipe, age int) bool {
	if kidRecipeMinAge(r) > age {
		return false
	}
	if strings.HasPrefix(r.ID, "wean_") {
		return age < 15
	}
	if slices.Contains(r.Tags, "baby") {
		return age < 24
	}
	return true
}

// buildKidMenu собирает неделю для одного ребёнка из детской базы (тег kidmenu) с учётом возраста и аллергий.
func (c *Catalog) buildKidMenu(idx int, k Child, p Params, pr pricer, rng *rand.Rand) KidMenu {
	l := pr.lang
	menu := KidMenu{Child: idx, AgeLabel: k.AgeLabel(l), Factor: k.MenuFactor()}
	allergens, exclude := kidAvoid(k, p)
	pools := map[string][]Recipe{}
	for _, r := range c.Recipes {
		if !isKidRecipe(r) || r.Hidden || !kidRecipeFits(r, k.AgeMonths) || slices.Contains(p.ExcludeRecipes, r.ID) {
			continue
		}
		ok := !c.recipeAvoids(r.ID, allergens, exclude)
		for _, eq := range r.Equipment {
			if !hasEquipment(p.Equipment, eq) {
				ok = false
			}
		}
		if ok {
			pools[r.Slot] = append(pools[r.Slot], r)
		}
	}
	used := map[string]int{}
	prevMain := map[string]string{}
	for d := 0; d < 7; d++ {
		day := KidDay{Index: d, Label: DayLabel(l, d)}
		for _, slot := range kidSlots {
			if k.AwayOn(d, slot) {
				dish := KidDish{Slot: slot, Kind: KidAway + "." + k.Away}
				dish.Title = c.kidDishTitle(dish, l)
				day.Dishes = append(day.Dishes, dish)
				continue
			}
			if src := k.MealSource(slot); src == MealJars || src == MealShared {
				dish := KidDish{Slot: slot, Kind: src}
				dish.Title = c.kidDishTitle(dish, l)
				day.Dishes = append(day.Dishes, dish)
				continue
			} else if src != MealHome {
				continue
			}
			pool := pools[slot]
			if len(pool) == 0 && k.AgeMonths >= 36 {
				// с трёх лет можно взять обычное блюдо без острого и сырого; малышам — только детская база
				pool = c.kidFallback(slot, k, p, allergens)
			}
			if len(pool) == 0 {
				// детского рецепта без аллергенов ребёнка на этот приём нет — честно говорим, а не молчим
				dish := KidDish{Slot: slot, Kind: KidNone}
				dish.Title = c.kidDishTitle(dish, l)
				day.Dishes = append(day.Dishes, dish)
				continue
			}
			best, bestScore := Recipe{}, math.Inf(1)
			for _, r := range pool {
				s := float64(used[r.ID]) * 3
				if prevMain[slot] == mainIngredient(r) {
					s += 1
				}
				s += rng.Float64() * 0.5
				if s < bestScore {
					best, bestScore = r, s
				}
			}
			used[best.ID]++
			prevMain[slot] = mainIngredient(best)
			kcal, _, _, _ := c.Nutrition(best)
			day.Dishes = append(day.Dishes, KidDish{
				Slot: slot, RecipeID: best.ID, Title: best.LocalTitle(l), TimeMin: best.TimeMin,
				Kcal: math.Round(kcal * menu.Factor), Cost: pr.round(c.costPerPortion(best, pr) * menu.Factor),
			})
		}
		if k.AgeMonths >= 12 && k.AgeMonths < 36 && !c.avoids("kefir", allergens, exclude) {
			dish := KidDish{Slot: "bedtime", Kind: KidBedtime}
			dish.Title = c.kidDishTitle(dish, l)
			day.Dishes = append(day.Dishes, dish)
		}
		menu.Days = append(menu.Days, day)
	}
	if k.AgeMonths < 12 {
		menu.Note = i18n.T(l, "kidmenu.note.puree")
	} else if k.AgeMonths < 36 {
		menu.Note = i18n.T(l, "kidmenu.note.general")
	}
	return menu
}

// chokeRules — продукты, которыми маленькие дети чаще всего давятся, и как их подать (NHS «Preparing food
// safely», AAP «Choking Prevention», CDC «Choking Hazards»). До какого возраста (мес) показывать подсказку.
var chokeRules = []struct {
	Key   string
	Until int
	IDs   []string
}{
	{"choke.quarter", 60, []string{"grapes", "cherry_tomatoes", "cherry_sweet", "blueberry", "strawberry", "olives", "olives_black"}},
	{"choke.sausage", 60, []string{"sausages", "chicken_sausages", "hunting_sausages", "sausage_boiled"}},
	{"choke.nuts", 60, []string{"walnuts", "peanuts", "almonds", "cashews", "hazelnuts", "pistachios", "pine_nuts", "nut_mix", "sunflower_seeds", "pumpkin_seeds"}},
	{"choke.beans", 36, []string{"corn_can", "corn_cob", "chickpeas_can", "chickpeas_dry", "beans_can", "beans_dry", "beans_white_can", "beans_tomato_can", "green_peas"}},
	{"choke.dried", 36, []string{"raisins", "cranberry_dried", "prunes", "dried_apricots"}},
	{"choke.fish", 60, []string{"cod_fillet", "hake_fillet", "salmon_steak", "mackerel", "pike_perch", "trout", "pink_salmon", "tilapia", "flounder", "carp"}},
}

// ChokeNote — подсказка, как безопасно подать блюдо ребёнку этого возраста: «виноград — на четвертинки».
func (c *Catalog) ChokeNote(r Recipe, ageMonths int, l i18n.Lang) string {
	var out []string
	for _, rule := range chokeRules {
		if ageMonths >= rule.Until {
			continue
		}
		for _, ri := range r.Ingredients {
			if slices.Contains(rule.IDs, ri.IngredientID) {
				out = append(out, i18n.T(l, rule.Key))
				break
			}
		}
	}
	return strings.Join(out, " ")
}

// KidNorm — суточная норма ребёнка по МР 2.3.1.0253-21 (табл. 21 — энергия и белок, табл. 8 — вода и
// напитки) одной строкой для детского меню. До года норму считают на кг веса — тогда пусто.
func KidNorm(age int, l i18n.Lang) string {
	switch {
	case age < 12:
		return ""
	case age < 36:
		return i18n.T(l, "kid.norm", "1300", 39, "600–700")
	case age < 84:
		return i18n.T(l, "kid.norm", "1800", 54, "800–900")
	case age < 132:
		return i18n.T(l, "kid.norm", "2100", 63, "1100–1300")
	case age < 180:
		return i18n.T(l, "kid.norm.nowater", "2300–2500", "69–75")
	default:
		return i18n.T(l, "kid.norm.nowater", "2500–2900", "75–87")
	}
}
