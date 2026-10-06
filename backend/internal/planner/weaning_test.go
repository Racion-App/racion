package planner

import (
	"slices"
	"testing"

	"racion/internal/i18n"
)

// Сроки по таблице 5.1: мясо с 6 мес, желток с 7, творог, кефир и рыба с 8; фрукты не первыми.
func TestWeaningNext(t *testing.T) {
	if f, _ := WeaningNext(6, nil); f.Group != "veg" {
		t.Fatalf("первый прикорм — овощи, а не %s", f.ID)
	}
	if f, ok := WeaningNext(4, []string{"zucchini", "cauliflower", "broccoli", "buckwheat", "rice_round", "cornmeal"}); ok && f.From > 4 {
		t.Fatalf("в 4 мес нельзя %s", f.ID)
	}
	intro := []string{"zucchini", "cauliflower", "broccoli", "buckwheat", "rice_round", "cornmeal", "pumpkin", "carrot", "potato"}
	if f, _ := WeaningNext(6, intro); f.Group != "meat" {
		t.Fatalf("после овощей и каш в 6 мес — мясо, а не %s", f.ID)
	}
	all := []string{}
	for _, f := range WeaningFoods {
		if f.From <= 7 {
			all = append(all, f.ID)
		}
	}
	if f, ok := WeaningNext(7, all); ok {
		t.Fatalf("в 7 мес после всего до 7 мес ничего нового, а предлагается %s", f.ID)
	}
	if f, _ := WeaningNext(8, all); f.From != 8 {
		t.Fatalf("в 8 мес — творог, кефир или рыба, а не %s", f.ID)
	}
}

func TestWeaningGrams(t *testing.T) {
	cases := []struct {
		group string
		month int
		want  float64
	}{{"veg", 6, 150}, {"cereal", 8, 180}, {"cereal", 10, 200}, {"meat", 6, 15}, {"meat", 11, 50}, {"curd", 7, 0}, {"curd", 8, 40}, {"kefir", 9, 200}, {"yolk", 7, 0.25}}
	for _, c := range cases {
		if got := WeaningGrams(c.group, c.month); got != c.want {
			t.Errorf("%s в %d мес: %v, по таблице %v", c.group, c.month, got, c.want)
		}
	}
	r := WeaningRamp(150)
	if r[0] > 10 || r[6] != 150 || !slices.IsSorted(r) {
		t.Fatalf("наращивание за неделю: %v", r)
	}
}

// Меню только для ребёнка на прикорме: взрослых нет, семейных блюд нет, в списке покупок — продукты прикорма.
func TestKidsOnlyWeaning(t *testing.T) {
	c := testCatalog(t)
	p := Params{Country: "RU", Adults: 0, Slots: []string{"breakfast", "lunch", "dinner"}, Goal: "none",
		Kids: []Child{{AgeMonths: 7, Feeding: FeedWeaning, Introduced: []string{"zucchini", "broccoli", "buckwheat", "rice_round", "turkey_fillet"}}}}
	plan := c.Build(p)
	if plan.Params.Adults != 0 {
		t.Fatalf("взрослых стало %d", plan.Params.Adults)
	}
	for _, d := range plan.Days {
		if len(d.Dishes) > 0 {
			t.Fatalf("семейные блюда в меню только для ребёнка: %d в %s", len(d.Dishes), d.Label)
		}
	}
	if len(plan.KidsMenus) != 1 || plan.KidsMenus[0].Weaning == nil {
		t.Fatal("нет недели прикорма")
	}
	w := plan.KidsMenus[0].Weaning
	if w.New == nil || len(w.Ramp) != 7 {
		t.Fatalf("нет продукта недели: %+v", w.New)
	}
	found := false
	for _, g := range plan.Shopping {
		for _, it := range g.Items {
			if it.IngredientID == "zucchini" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("кабачка нет в списке покупок")
	}
}

// Смесь только на ночь: в 22:00 смесь, остальные молочные кормления — грудь; норма в день — одно кормление.
func TestFormulaBedtimeOnly(t *testing.T) {
	c := testCatalog(t)
	k := Child{AgeMonths: 7, Feeding: FeedWeaning, Formula: true, FormulaBrand: "nan", FormulaFeeds: []string{"bedtime", "bogus"}}.normalized()
	if len(k.FormulaFeeds) != 1 || k.FormulaMlPerDay() != 200 {
		t.Fatalf("кормления %v, мл %d", k.FormulaFeeds, k.FormulaMlPerDay())
	}
	w := c.buildWeaning(k, i18n.RU)
	for _, f := range w.Days[0].Feeds {
		switch f.Time {
		case "22:00":
			if f.MilkKind != "formula" || f.MilkMl != 200 {
				t.Fatalf("22:00: %+v", f)
			}
		case "06:00":
			if f.MilkKind != "breast" {
				t.Fatalf("06:00: %+v", f)
			}
		case "02:00":
			t.Fatal("ночного кормления не просили")
		}
	}
	// утром и ночью, свой объём в день делится на кормления смесью
	k = Child{AgeMonths: 10, Feeding: FeedWeaning, Formula: true, FormulaMl: 500, FormulaFeeds: []string{"morning", "night"}}.normalized()
	w = c.buildWeaning(k, i18n.RU)
	got := map[string]int{}
	for _, f := range w.Days[0].Feeds {
		if f.MilkKind == "formula" {
			got[f.Time] = f.MilkMl
		}
	}
	if len(got) != 2 || got["06:00"] != 250 || got["02:00"] != 250 {
		t.Fatalf("смесь по кормлениям: %v", got)
	}
}

// Комбинирую в 8 мес: утром баночки, в обед дома (сюда же новый продукт), вечером с общего стола.
func TestMixWeaning(t *testing.T) {
	c := testCatalog(t)
	k := Child{AgeMonths: 8, Feeding: FeedMix, Introduced: []string{"zucchini", "broccoli", "buckwheat", "rice_round", "turkey_fillet", "apple", "carrot"},
		Meals: map[string]string{"breakfast": MealJars, "lunch": MealHome, "dinner": MealShared, "snack": MealJars}}
	p := Params{Country: "RU", Adults: 2, Slots: []string{"breakfast", "lunch", "dinner"}, Goal: "none", Kids: []Child{k}}
	plan := c.Build(p)
	k = plan.Params.Kids[0]
	if len(k.Meals) != 3 {
		t.Fatalf("приёмы до года без полдника: %v", k.Meals)
	}
	if len(plan.KidsMenus) != 1 || plan.KidsMenus[0].Weaning == nil {
		t.Fatal("нет недели прикорма")
	}
	feeds := plan.KidsMenus[0].Weaning.Days[0].Feeds
	byTime := map[string]WeaningFeed{}
	for _, f := range feeds {
		byTime[f.Time] = f
	}
	for _, it := range byTime["10:00"].Items {
		if it.Group != "jar" {
			t.Fatalf("утром не баночки: %+v", it)
		}
	}
	if ev := byTime["18:00"].Items; len(ev) != 1 || ev[0].Group != "shared" {
		t.Fatalf("вечер не с общего стола: %+v", ev)
	}
	if w := plan.KidsMenus[0].Weaning; w.New != nil && !byTime["14:00"].Items[0].New {
		t.Fatalf("новый продукт не в обед: %+v", byTime["14:00"].Items)
	}
	sp := slotPortions(plan.Params)
	if sp["dinner"] <= sp["breakfast"] {
		t.Fatalf("ужин с ребёнком должен быть больше завтрака: %v", sp)
	}
	var jars []string
	for _, g := range plan.Shopping {
		for _, it := range g.Items {
			if g.Category == "baby" || it.Category == "baby" {
				jars = append(jars, it.IngredientID)
			}
		}
	}
	if !slices.Contains(jars, "baby_kasha_0") || slices.Contains(jars, "baby_veg_0") {
		t.Fatalf("баночки не по приёмам: %v", jars)
	}
}

// Комбинирую в 1,5 года: завтрак с общего стола, обед дома, ужин баночки, полдник дома.
func TestMixToddler(t *testing.T) {
	c := testCatalog(t)
	k := Child{AgeMonths: 18, Feeding: FeedMix, Meals: map[string]string{"breakfast": MealShared, "lunch": MealHome, "dinner": MealJars, "snack": MealHome}}
	plan := c.Build(Params{Country: "RU", Adults: 2, Slots: []string{"breakfast", "lunch", "dinner"}, Goal: "none", Kids: []Child{k}})
	if len(plan.KidsMenus) != 1 {
		t.Fatal("нет детского меню")
	}
	kinds := map[string]KidDish{}
	for _, x := range plan.KidsMenus[0].Days[0].Dishes {
		kinds[x.Slot] = x
	}
	if kinds["breakfast"].Kind != MealShared || kinds["breakfast"].Ref == "" || kinds["breakfast"].Ref != plan.Days[0].Dishes[0].RecipeID {
		t.Fatalf("завтрак с общего стола: %+v, семье %s", kinds["breakfast"], plan.Days[0].Dishes[0].RecipeID)
	}
	if kinds["dinner"].Kind != MealJars || kinds["lunch"].RecipeID == "" {
		t.Fatalf("ужин %+v, обед %+v", kinds["dinner"], kinds["lunch"])
	}
}

// Без взрослых и с ужином «с общего стола» семье готовится только ужин.
func TestMixKidsOnlySharedDinner(t *testing.T) {
	c := testCatalog(t)
	k := Child{AgeMonths: 18, Feeding: FeedMix, Meals: map[string]string{"breakfast": MealHome, "lunch": MealJars, "dinner": MealShared, "snack": MealHome}}
	plan := c.Build(Params{Country: "RU", Adults: 0, Slots: []string{"breakfast", "lunch", "dinner"}, Goal: "none", Kids: []Child{k}})
	for _, d := range plan.Days {
		for _, x := range d.Dishes {
			if x.Slot != "dinner" {
				t.Fatalf("семье готовится %s, хотя ребёнок ест со стола только ужин", x.Slot)
			}
		}
	}
	if len(plan.Days[0].Dishes) == 0 {
		t.Fatal("нет ужина для ребёнка со стола")
	}
}
