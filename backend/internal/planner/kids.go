package planner

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"

	"racion/internal/i18n"
)

// Режимы кормления ребёнка.
const (
	FeedShared   = "shared"   // ест с общего стола (порция по возрасту, общие блюда фильтруются)
	FeedSeparate = "separate" // готовим отдельно — своё детское меню на неделю
	FeedJars     = "jars"     // баночки и детские каши по возрастным нормам
	FeedMilk     = "milk"     // только смесь или грудное молоко
)

var FeedingModes = []string{FeedShared, FeedSeparate, FeedJars, FeedMilk}

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
}

// FeedingOptions — какие режимы доступны в этом возрасте (первый — рекомендуемый по умолчанию).
func FeedingOptions(ageMonths int) []string {
	switch {
	case ageMonths < 6:
		return []string{FeedMilk}
	case ageMonths < 12:
		return []string{FeedJars, FeedSeparate, FeedShared}
	case ageMonths < 36:
		return []string{FeedShared, FeedSeparate, FeedJars}
	default:
		return []string{FeedShared, FeedSeparate}
	}
}

func (c Child) normalized() Child {
	if c.Feeding == "" {
		if c.SharesMeals {
			c.Feeding = FeedShared
		} else if c.AgeMonths < 6 {
			c.Feeding = FeedMilk
		} else if c.AgeMonths < 12 {
			c.Feeding = FeedJars
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
	return c
}

// PortionFactor — доля взрослой порции по возрасту (для тех, кто ест с общего стола).
func (c Child) PortionFactor() float64 {
	if c.Feeding != FeedShared {
		return 0
	}
	switch {
	case c.AgeMonths < 6:
		return 0
	case c.AgeMonths < 12:
		return 0.15
	case c.AgeMonths < 36:
		return 0.4
	case c.AgeMonths < 84: // до 7 лет
		return 0.55
	case c.AgeMonths < 132: // до 11
		return 0.7
	case c.AgeMonths < 168: // до 14
		return 0.85
	default:
		return 1
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
	default:
		return 1.6
	}
}

// FormulaMlPerDay — ориентировочная суточная норма смеси по возрасту (усреднённые рекомендации педиатров).
func (c Child) FormulaMlPerDay() int {
	if c.FormulaMl > 0 {
		return c.FormulaMl
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
	{"nutrilon", "Nutrilon", 800, 1.25, "formula.note.premium"},
	{"nan", "NAN (Nestlé)", 800, 1.2, ""},
	{"similac", "Similac", 600, 1.1, ""},
	{"friso", "Friso", 800, 1.3, ""},
	{"hipp", "HiPP", 600, 1.35, "formula.note.organic"},
	{"nestogen", "Nestogen", 600, 0.85, ""},
	{"nutrilak", "Nutrilak", 600, 0.8, ""},
	{"malyutka", "formula.malyutka", 600, 0.7, ""},
	{"bellakt", "formula.bellakt", 400, 0.65, ""},
	{"kabrita", "formula.kabrita", 800, 2.2, "formula.note.goat"},
	{"other", "formula.other", 600, 1.0, ""},
}

// FormulaBrandsFor — марки с подписями на языке (названия-ключи переводятся, остальные как есть).
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
var kidsExcludeUnder3 = []string{"mushrooms", "sausages", "shrimp", "king_prawns", "mussels", "scallops", "oysters", "red_caviar", "blue_cheese", "mayo", "chili_flakes", "peanuts", "peanut_butter", "walnuts", "sunflower_seeds", "olives"}

func kidsRestrictions(kids []Child) (excludeIngredients []string, excludeTags []string, minAgeSharing int) {
	minAgeSharing = -1
	for _, k := range kids {
		if k.Feeding != FeedShared {
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
		return slices.Clone(kidsExcludeUnder3), []string{"spicy", "raw"}, minAgeSharing
	}
	if minAgeSharing < 84 {
		return nil, []string{"spicy", "raw"}, minAgeSharing
	}
	return nil, nil, minAgeSharing
}

// jarNorm — дневные нормы баночного питания по возрасту, г. Ориентировочно, по методичкам педиатров.
type jarNorm struct {
	veg, fruit, meat, kasha, curd float64
	milk                          float64 // мл детского молочка/кефира
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

// babyItems — строки списка покупок для детей: смесь, баночки, каши. Всё на неделю.
func (c *Catalog) babyItems(kids []Child, pr pricer) ([]ShopItem, []string) {
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
		case FeedJars:
			n := jarNormFor(k.AgeMonths)
			if n.veg > 0 {
				items = append(items, jar(i, "baby_veg", "baby.veg", n.veg, 100, rosstatBabyVeg, fallbackBabyVeg, who, "g"))
			}
			if n.fruit > 0 {
				items = append(items, jar(i, "baby_fruit", "baby.fruit", n.fruit, 100, rosstatBabyFruit, fallbackBabyFrt, who, "g"))
			}
			if n.meat > 0 {
				items = append(items, jar(i, "baby_meat", "baby.meat", n.meat, 80, rosstatBabyMeat, fallbackBabyMeat, who, "g"))
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
	return items, notes
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

// buildKidMenu собирает неделю для одного ребёнка из детской базы (тег kidmenu) с учётом возраста и аллергий.
func (c *Catalog) buildKidMenu(idx int, k Child, p Params, pr pricer, rng *rand.Rand) KidMenu {
	l := pr.lang
	menu := KidMenu{Child: idx, AgeLabel: k.AgeLabel(l), Factor: k.MenuFactor()}
	pools := map[string][]Recipe{}
	for _, r := range c.Recipes {
		if !isKidRecipe(r) || kidRecipeMinAge(r) > k.AgeMonths || slices.Contains(p.ExcludeRecipes, r.ID) {
			continue
		}
		ok := true
		for _, ri := range r.Ingredients {
			ing := c.Ingredients[ri.IngredientID]
			if slices.Contains(p.Exclude, ri.IngredientID) {
				ok = false
				break
			}
			for _, a := range ing.Allergens {
				if slices.Contains(p.Allergens, a) {
					ok = false
				}
			}
			if !ok {
				break
			}
		}
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
			pool := pools[slot]
			if len(pool) == 0 {
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
		menu.Days = append(menu.Days, day)
	}
	if k.AgeMonths < 12 {
		menu.Note = i18n.T(l, "kidmenu.note.puree")
	} else if k.AgeMonths < 36 {
		menu.Note = i18n.T(l, "kidmenu.note.general")
	}
	return menu
}
