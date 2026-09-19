package planner

import (
	"encoding/json"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// testCatalog собирает каталог прямо из seed-JSON, без БД.
func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	c := NewCatalog()

	var ings struct {
		Items []struct {
			Ingredient
			Rosstat *struct {
				Item   int
				Factor float64
			}
		}
	}
	mustLoad(t, "../seed/data/ingredients.json", &ings)
	for _, x := range ings.Items {
		i := x.Ingredient
		if i.Allergens == nil {
			i.Allergens = []string{}
		}
		if x.Rosstat != nil {
			i.RosstatItem = x.Rosstat.Item
			i.RosstatFactor = x.Rosstat.Factor
		}
		c.Ingredients[i.ID] = i
	}

	type recJSON struct {
		ID          string
		Title       string
		Slot        string
		Time        int
		Equipment   []string
		Tags        []string
		Batch       bool
		Ingredients [][]json.RawMessage
		Steps       []string
	}
	var all []recJSON
	for _, f := range []string{"recipes.json", "recipes_lunch.json", "recipes_dinner.json", "recipes_breakfast.json", "recipes_kids.json", "recipes_trendy.json", "recipes_pp.json", "recipes_premium.json", "recipes_more.json"} {
		var recs struct{ Items []recJSON }
		mustLoad(t, "../seed/data/"+f, &recs)
		all = append(all, recs.Items...)
	}
	for _, r := range all {
		rc := Recipe{ID: r.ID, Title: r.Title, Slot: r.Slot, TimeMin: r.Time, Equipment: r.Equipment, Tags: r.Tags, Batch: r.Batch, Steps: r.Steps}
		for _, pair := range r.Ingredients {
			var id string
			var amt float64
			_ = json.Unmarshal(pair[0], &id)
			_ = json.Unmarshal(pair[1], &amt)
			if _, ok := c.Ingredients[id]; !ok {
				t.Fatalf("recipe %s: unknown ingredient %s", r.ID, id)
			}
			rc.Ingredients = append(rc.Ingredients, RecipeIngredient{id, amt})
		}
		c.Recipes = append(c.Recipes, rc)
		c.RecipeByID[rc.ID] = rc
	}

	var stores struct {
		Items []struct {
			Code       string
			Country    string
			Name       string
			Kind       string
			PriceIndex float64 `json:"price_index"`
			Sort       int
		}
	}
	mustLoad(t, "../seed/data/stores.json", &stores)
	for _, s := range stores.Items {
		st := Store{Code: s.Code, Country: s.Country, Name: s.Name, Kind: s.Kind, PriceIndex: s.PriceIndex, Sort: s.Sort}
		c.Stores[st.Code] = st
		c.StoreList = append(c.StoreList, st)
	}
	return c
}

func mustLoad(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func baseParams() Params {
	return Params{
		Store: "pyaterochka", Adults: 2,
		Kids:       []Child{{AgeMonths: 60, SharesMeals: true}},
		Equipment:  []string{"stove", "oven", "blender"},
		Slots:      []string{"breakfast", "lunch", "dinner", "snack"},
		BudgetMode: "perPersonDay", BudgetValue: 400, StartDate: "2026-09-21",
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	c := testCatalog(t)
	a := c.Build(baseParams())
	b := c.Build(baseParams())
	if a.Seed != b.Seed {
		t.Fatalf("seed differs")
	}
	for d := range a.Days {
		for i := range a.Days[d].Dishes {
			if a.Days[d].Dishes[i].RecipeID != b.Days[d].Dishes[i].RecipeID {
				t.Fatalf("day %d slot %d differs", d, i)
			}
		}
	}
}

func TestEveryCellFilledAndNoRepeats(t *testing.T) {
	c := testCatalog(t)
	p := c.Build(baseParams())
	seen := map[string]int{}
	for _, d := range p.Days {
		if len(d.Dishes) != 4 {
			t.Fatalf("day %s has %d dishes", d.Label, len(d.Dishes))
		}
		for _, dish := range d.Dishes {
			if !dish.Leftover {
				seen[dish.RecipeID]++
			}
			if dish.Kcal <= 0 || dish.Cost < 0 {
				t.Fatalf("bad numbers for %s", dish.Title)
			}
		}
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("recipe %s cooked %d times", id, n)
		}
	}
}

func TestAllergenFilterIsHard(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.Allergens = []string{"dairy", "eggs"}
	plan := c.Build(p)
	for _, d := range plan.Days {
		for _, dish := range d.Dishes {
			r := c.RecipeByID[dish.RecipeID]
			for _, ri := range r.Ingredients {
				for _, a := range c.Ingredients[ri.IngredientID].Allergens {
					if a == "dairy" || a == "eggs" {
						t.Fatalf("%s contains %s (%s)", r.Title, a, ri.IngredientID)
					}
				}
			}
		}
	}
	if len(plan.Warnings) == 0 {
		t.Log("no warnings — fine if pools are still big enough")
	}
}

func TestExcludeAndEquipment(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.Equipment = []string{"stove"} // без духовки и блендера
	p.Exclude = []string{"liver_chicken", "eggplant"}
	p.ExcludeTags = []string{"spicy"}
	plan := c.Build(p)
	for _, d := range plan.Days {
		for _, dish := range d.Dishes {
			r := c.RecipeByID[dish.RecipeID]
			if slices.Contains(r.Equipment, "oven") || slices.Contains(r.Equipment, "blender") {
				t.Fatalf("%s needs equipment the user lacks", r.Title)
			}
			if slices.Contains(r.Tags, "spicy") {
				t.Fatalf("%s is spicy", r.Title)
			}
			for _, ri := range r.Ingredients {
				if ri.IngredientID == "liver_chicken" || ri.IngredientID == "eggplant" {
					t.Fatalf("%s uses excluded %s", r.Title, ri.IngredientID)
				}
			}
		}
	}
}

func TestShoppingListCoversNeeds(t *testing.T) {
	c := testCatalog(t)
	plan := c.Build(baseParams())
	need := map[string]float64{}
	for _, d := range plan.Days {
		for _, dish := range d.Dishes {
			if dish.Leftover {
				continue
			}
			mult := 1.0
			if dish.Batch {
				mult = 2
			}
			for _, ri := range c.RecipeByID[dish.RecipeID].Ingredients {
				need[ri.IngredientID] += ri.Amount * plan.Portions * mult
			}
		}
	}
	listed := map[string]ShopItem{}
	for _, g := range plan.Shopping {
		for _, it := range g.Items {
			listed[it.IngredientID] = it
		}
	}
	for id, n := range need {
		it, ok := listed[id]
		if !ok {
			t.Fatalf("%s missing from shopping list", id)
		}
		if it.Buy+1e-6 < n {
			t.Fatalf("%s: buy %.1f < need %.1f", id, it.Buy, n)
		}
	}
	if plan.Totals.Cost <= 0 || plan.Totals.Items != len(listed) {
		t.Fatalf("totals: %+v", plan.Totals)
	}
}

func TestBudgetLevelsOrdered(t *testing.T) {
	c := testCatalog(t)
	costs := map[float64]float64{}
	for _, b := range []float64{250, 400, 700} {
		p := baseParams()
		p.BudgetValue = b
		costs[b] = c.Build(p).Totals.Cost
	}
	if costs[250] > costs[700] {
		t.Errorf("economy %.0f > free %.0f", costs[250], costs[700])
	}
	t.Logf("250: %.0f · 400: %.0f · 700: %.0f", costs[250], costs[400], costs[700])
}

func TestBudgetWeekMode(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.BudgetMode = "week"
	p.BudgetValue = 5000
	plan := c.Build(p)
	if plan.Budget.TargetWeek != 5000 || plan.Budget.PerDay <= 0 {
		t.Fatalf("budget %+v", plan.Budget)
	}
	t.Logf("week 5000 → per day %.0f, actual %.0f", plan.Budget.PerDay, plan.Totals.Cost)
}

func TestGoalLoseLowersKcal(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.Goal = "lose"
	lose := c.Build(p)
	p.Goal = "gain"
	gain := c.Build(p)
	if lose.Totals.KcalPerDay >= gain.Totals.KcalPerDay {
		t.Errorf("lose %.0f should be below gain %.0f", lose.Totals.KcalPerDay, gain.Totals.KcalPerDay)
	}
	if lose.Goal.KcalTarget != 1500 || gain.Goal.KcalTarget != 2600 {
		t.Errorf("targets %v %v", lose.Goal, gain.Goal)
	}
	t.Logf("lose %.0f kcal/day · gain %.0f kcal/day", lose.Totals.KcalPerDay, gain.Totals.KcalPerDay)
}

func TestKidsPortionsFormulaAndRestrictions(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.Kids = []Child{
		{AgeMonths: 4, Formula: true, FormulaBrand: "nan"},
		{AgeMonths: 8, SharesMeals: true, Formula: true, FormulaBrand: "malyutka"},
		{AgeMonths: 24, SharesMeals: true},
	}
	plan := c.Build(p)
	want := 2 + 0.15 + 0.4
	if plan.Portions != want {
		t.Fatalf("portions %.2f, want %.2f", plan.Portions, want)
	}
	var baby *ShopGroup
	for i := range plan.Shopping {
		if plan.Shopping[i].Category == "baby" {
			baby = &plan.Shopping[i]
		}
	}
	if baby == nil || len(baby.Items) != 4 { // две смеси + овощное и фруктовое пюре для 8 мес
		t.Fatalf("baby group: %+v", baby)
	}
	if plan.Totals.BabyCost <= 0 {
		t.Fatalf("baby cost %v", plan.Totals.BabyCost)
	}
	// ребёнок 2 лет за столом: грибов, колбасы и острого быть не должно
	for _, d := range plan.Days {
		for _, dish := range d.Dishes {
			r := c.RecipeByID[dish.RecipeID]
			for _, ri := range r.Ingredients {
				if ri.IngredientID == "mushrooms" || ri.IngredientID == "sausages" || ri.IngredientID == "mayo" {
					t.Fatalf("%s uses %s with a 2-year-old at the table", r.Title, ri.IngredientID)
				}
			}
			if slices.Contains(r.Tags, "spicy") {
				t.Fatalf("%s is spicy", r.Title)
			}
		}
	}
	t.Logf("baby: %d items, %.0f ₽; warnings: %v", len(baby.Items), plan.Totals.BabyCost, plan.Warnings)
}

func TestAirfryerReplacesOven(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.Equipment = []string{"stove", "airfryer"}
	ovenOK := 0
	for _, r := range c.Recipes {
		if slices.Contains(r.Equipment, "oven") && c.Allowed(r, p) {
			ovenOK++
		}
	}
	if ovenOK == 0 {
		t.Fatal("oven recipes should be allowed with an airfryer")
	}
}

func TestPriceBookOverridesManualPrices(t *testing.T) {
	c := testCatalog(t)
	pb := &PriceBook{Period: "2026-08", RegionBy: map[string]Region{"643": {Code: "643", Name: "РФ", Kind: "rf"}}, monthly: map[string]map[int]float64{"643": {1501: 97.4, 1111: 97.88}}, ratio: map[int]float64{}, itemName: map[int]string{}}
	pb.PeriodTime, _ = time.Parse("2006-01", "2026-08")
	c.SetPriceBook(pb)
	defer c.SetPriceBook(nil)
	p := baseParams()
	p.Region = "643"
	plan := c.Build(p)
	if plan.PriceSource.Name != "Росстат" || plan.PriceSource.Coverage <= 0 {
		t.Fatalf("source %+v", plan.PriceSource)
	}
	pr := c.pricerFor(plan.Params)
	if v, ok := pr.packPrice(c.Ingredients["eggs"]); !ok || math.Abs(v-97.4*0.95) > 0.5 {
		t.Fatalf("eggs pack price %.2f ok=%v", v, ok)
	}
}

func TestStoreIndexScalesCost(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	cheap := c.Build(p)
	// Тот же набор блюд, пересчитанный по индексу ВкусВилла: планировщик при дорогом магазине
	// сам выберет блюда дешевле, поэтому сравниваем одинаковую корзину.
	dear := cheap
	dear.Params.Store = "vkusvill"
	dear.Store = c.Stores["vkusvill"]
	c.finish(&dear)
	if dear.Totals.Cost <= cheap.Totals.Cost {
		t.Errorf("vkusvill %.0f should cost more than pyaterochka %.0f", dear.Totals.Cost, cheap.Totals.Cost)
	}
}

func TestSwapChangesOnlyThatCell(t *testing.T) {
	c := testCatalog(t)
	plan := c.Build(baseParams())
	before := plan.Days[3].Dishes[2] // четверг, ужин
	after, err := c.Swap(plan, 3, "dinner")
	if err != nil {
		t.Fatal(err)
	}
	if after.Days[3].Dishes[2].RecipeID == before.RecipeID {
		t.Fatalf("swap returned the same recipe")
	}
	for d := range plan.Days {
		for i := range plan.Days[d].Dishes {
			if d == 3 && i == 2 {
				continue
			}
			if plan.Days[d].Dishes[i].RecipeID != after.Days[d].Dishes[i].RecipeID {
				t.Fatalf("swap touched day %d dish %d", d, i)
			}
		}
	}
	if after.Swaps != 1 {
		t.Fatalf("swaps counter %d", after.Swaps)
	}
	// повторный swap той же ячейки даёт третий вариант
	third, err := c.Swap(after, 3, "dinner")
	if err != nil {
		t.Fatal(err)
	}
	if third.Days[3].Dishes[2].RecipeID == after.Days[3].Dishes[2].RecipeID {
		t.Fatalf("second swap repeated")
	}
}

func TestBatchLunchPairs(t *testing.T) {
	c := testCatalog(t)
	plan := c.Build(baseParams())
	pairs := 0
	for d := 0; d < 6; d++ {
		src := plan.Days[d].Dishes[1]
		if src.Batch {
			nxt := plan.Days[d+1].Dishes[1]
			if !nxt.Leftover || nxt.RecipeID != src.RecipeID {
				t.Fatalf("day %d batch lunch has no leftover next day", d)
			}
			pairs++
		}
	}
	t.Logf("batch pairs: %d, week cost %.0f ₽, kcal/day %.0f", pairs, plan.Totals.Cost, plan.Totals.KcalPerDay)
	if plan.Totals.KcalPerDay < 1200 || plan.Totals.KcalPerDay > 3200 {
		t.Errorf("kcal/day looks off: %.0f", plan.Totals.KcalPerDay)
	}
}

func TestNextMonday(t *testing.T) {
	c := testCatalog(t)
	p := c.Normalize(Params{})
	if p.StartDate == "" {
		t.Fatal("no start date")
	}
	if math.IsNaN(float64(Seed(p))) {
		t.Fatal("seed")
	}
}

func TestFeedingModes(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.Kids = []Child{
		{AgeMonths: 20, Feeding: FeedSeparate},
		{AgeMonths: 9, Feeding: FeedJars, Formula: true, FormulaBrand: "nan"},
		{AgeMonths: 3, Feeding: FeedMilk},
	}
	plan := c.Build(p)
	if plan.Portions != 2 {
		t.Fatalf("portions %.2f: separate/jars/milk kids must not add adult portions", plan.Portions)
	}
	if len(plan.KidsMenus) != 1 || len(plan.KidsMenus[0].Days) != 7 {
		t.Fatalf("kids menus: %+v", plan.KidsMenus)
	}
	for _, d := range plan.KidsMenus[0].Days {
		if len(d.Dishes) != 4 {
			t.Fatalf("kid day %s has %d dishes", d.Label, len(d.Dishes))
		}
		for _, x := range d.Dishes {
			r := c.RecipeByID[x.RecipeID]
			if !isKidRecipe(r) || kidRecipeMinAge(r) > 20 {
				t.Fatalf("%s is not a kid recipe for 20 months", r.Title)
			}
		}
	}
	if plan.Totals.KidsMenuCost <= 0 {
		t.Fatalf("kids menu cost %v", plan.Totals.KidsMenuCost)
	}
	var baby []string
	for _, g := range plan.Shopping {
		if g.Category == "baby" {
			for _, it := range g.Items {
				baby = append(baby, it.Name)
			}
		}
	}
	// 9 мес на баночках + смесь: смесь, овощное, фруктовое, мясное пюре, каша, творожок
	if len(baby) != 6 {
		t.Fatalf("baby items: %v", baby)
	}
	// взрослые блюда не должны содержать детских рецептов, и наоборот ограничения общего стола не применяются
	for _, d := range plan.Days {
		for _, dish := range d.Dishes {
			if isKidRecipe(c.RecipeByID[dish.RecipeID]) {
				t.Fatalf("kid recipe %s in adult plan", dish.Title)
			}
		}
	}
	for _, w := range plan.Warnings {
		if strings.Contains(w, "общего стола") {
			t.Fatalf("no shared kids, but warning: %s", w)
		}
	}
	t.Logf("kid menu: %s; baby items %d; menu cost %.0f", plan.KidsMenus[0].Days[0].Dishes[1].Title, len(baby), plan.Totals.KidsMenuCost)
}

func TestKidMenuRespectsAgeAndAllergens(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.Allergens = []string{"dairy"}
	p.Kids = []Child{{AgeMonths: 7, Feeding: FeedSeparate}}
	plan := c.Build(p)
	if len(plan.KidsMenus) != 1 {
		t.Fatal("no kid menu")
	}
	for _, d := range plan.KidsMenus[0].Days {
		for _, x := range d.Dishes {
			r := c.RecipeByID[x.RecipeID]
			if kidRecipeMinAge(r) > 7 {
				t.Fatalf("%s needs %d months", r.Title, kidRecipeMinAge(r))
			}
			for _, ri := range r.Ingredients {
				if slices.Contains(c.Ingredients[ri.IngredientID].Allergens, "dairy") {
					t.Fatalf("%s has dairy", r.Title)
				}
			}
		}
	}
}

// Другая страна: магазин берётся из её списка, цены — в местной валюте (ручной ориентир
// или рублёвая цена × курс-множитель), подписи — на языке плана.
func TestCountryAndLanguage(t *testing.T) {
	c := testCatalog(t)
	c.Ingredients["chicken_breast"] = withPrice(c.Ingredients["chicken_breast"], "US", 7.2)
	p := baseParams()
	p.Country, p.Store, p.Lang, p.BudgetValue = "US", "pyaterochka", "en", 9
	plan := c.Build(p)
	if plan.Country.Currency != "USD" || plan.Store.Country != "US" {
		t.Fatalf("expected a US store in USD, got %s / %s", plan.Store.Code, plan.Country.Currency)
	}
	if plan.Params.Region != "" {
		t.Fatalf("region must be empty outside Russia")
	}
	if plan.Totals.Cost <= 0 || plan.Totals.Cost > 400 {
		t.Fatalf("US weekly cost out of range: %.2f", plan.Totals.Cost)
	}
	if plan.Days[0].Label != "Mon" || plan.Shopping[0].Label == "" {
		t.Fatalf("labels must be English, got %q", plan.Days[0].Label)
	}
	for _, g := range plan.Shopping {
		for _, it := range g.Items {
			if it.IngredientID == "chicken_breast" && it.Name != "Chicken breast fillet" && it.Name != "Куриное филе" {
				t.Fatalf("unexpected name %q", it.Name)
			}
		}
	}
	// Тот же план по-немецки: числа те же, подписи другие.
	de := c.Localize(plan, "de")
	if de.Totals.Cost != plan.Totals.Cost || de.Days[0].Label != "Mo" || de.Days[0].Dishes[0].Why == plan.Days[0].Dishes[0].Why {
		t.Fatalf("localize changed numbers or kept labels: %v %q", de.Totals.Cost, de.Days[0].Label)
	}
}

func withPrice(ing Ingredient, country string, v float64) Ingredient {
	if ing.Prices == nil {
		ing.Prices = map[string]float64{}
	}
	ing.Prices[country] = v
	return ing
}
