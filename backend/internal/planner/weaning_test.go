package planner

import (
	"slices"
	"strings"
	"testing"

	"racion/internal/i18n"
)

// Сроки по таблице 5.1: мясо с 6 мес, желток с 7, творог, кефир и рыба с 8; фрукты не первыми.
func TestWeaningNext(t *testing.T) {
	if f, _ := WeaningNext(6, nil, nil); f.Group != "veg" {
		t.Fatalf("первый прикорм — овощи, а не %s", f.ID)
	}
	if f, ok := WeaningNext(4, []string{"zucchini", "cauliflower", "broccoli", "buckwheat", "rice_round", "cornmeal"}, nil); ok && f.From > 4 {
		t.Fatalf("в 4 мес нельзя %s", f.ID)
	}
	intro := []string{"zucchini", "cauliflower", "broccoli", "buckwheat", "rice_round", "cornmeal", "pumpkin", "carrot", "potato"}
	if f, _ := WeaningNext(6, intro, nil); f.Group != "meat" {
		t.Fatalf("после овощей и каш в 6 мес — мясо, а не %s", f.ID)
	}
	all := []string{}
	for _, f := range WeaningFoods {
		if f.From <= 7 {
			all = append(all, f.ID)
		}
	}
	if f, ok := WeaningNext(7, all, nil); ok {
		t.Fatalf("в 7 мес после всего до 7 мес ничего нового, а предлагается %s", f.ID)
	}
	if f, _ := WeaningNext(8, all, nil); f.From != 8 {
		t.Fatalf("в 8 мес — творог, кефир или рыба, а не %s", f.ID)
	}
}

// Поздний старт: группы, которые по возрасту уже должны быть, вводятся раньше новых овощей и круп.
func TestWeaningCatchUp(t *testing.T) {
	if f, _ := WeaningNext(5, []string{"zucchini"}, nil); f.ID != "cauliflower" {
		t.Fatalf("в 5 мес по порядку — цветная капуста, а не %s", f.ID)
	}
	if f, _ := WeaningNext(7, []string{"zucchini"}, nil); f.Group != "cereal" {
		t.Fatalf("в 7 мес с одним кабачком — каша, а не %s", f.ID)
	}
	if f, _ := WeaningNext(7, []string{"zucchini", "buckwheat"}, nil); f.Group != "meat" {
		t.Fatalf("в 7 мес без мяса — мясо, а не %s", f.ID)
	}
	if f, _ := WeaningNext(9, []string{"zucchini", "buckwheat", "turkey_fillet", "apple"}, nil); f.Group != "yolk" {
		t.Fatalf("в 9 мес без желтка — желток, а не %s", f.ID)
	}
	noDairy := func(f WeaningFood) bool { return f.Group == "curd" || f.Group == "kefir" }
	if f, _ := WeaningNext(9, []string{"zucchini", "buckwheat", "turkey_fillet", "apple", "eggs"}, noDairy); f.Group == "curd" || f.Group == "kefir" {
		t.Fatalf("при аллергии на молоко предложен %s", f.ID)
	}
}

// Аллергия на молоко: без творога, кефира и сливочного масла, каша с растительным маслом.
// Первые ложки нового продукта — с докормом грудью или смесью.
func TestWeaningAllergyAndTopUp(t *testing.T) {
	c := testCatalog(t)
	k := Child{AgeMonths: 9, Feeding: FeedWeaning, Introduced: []string{"zucchini", "buckwheat", "turkey_fillet", "apple", "eggs", "cottage_soft", "kefir"}}.normalized()
	w := c.buildWeaning(k, i18n.RU, []string{"dairy"}, nil)
	for _, d := range w.Days {
		for _, f := range d.Feeds {
			for _, it := range f.Items {
				if it.Food == "butter" || it.Food == "cottage_soft" || it.Food == "kefir" {
					t.Fatalf("при аллергии на молоко в меню %s", it.Food)
				}
			}
		}
	}
	k = Child{AgeMonths: 5, Feeding: FeedWeaning}.normalized()
	w = c.buildWeaning(k, i18n.RU, nil, nil)
	f := w.Days[0].Feeds[1]
	if f.Time != "10:00" || len(f.Items) != 1 || !f.Milk {
		t.Fatalf("первая ложка без докорма: %+v", f)
	}
	if last := w.Days[6].Feeds[1]; last.Milk {
		t.Fatalf("на седьмой день полная порция, докорм не нужен: %+v", last)
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
	w := c.buildWeaning(k, i18n.RU, nil, nil)
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
	w = c.buildWeaning(k, i18n.RU, nil, nil)
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
		if !it.Jar {
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

// Режим «баночки» до года: расписание как у прикорма, все приёмы — баночки, новая баночка недели,
// в покупках детское питание по расписанию, а не сырые продукты.
func TestJarsSchedule(t *testing.T) {
	c := testCatalog(t)
	k := Child{AgeMonths: 8, Feeding: FeedJars, Introduced: []string{"zucchini", "buckwheat", "turkey_fillet", "apple", "eggs"}}
	plan := c.Build(Params{Country: "RU", Adults: 0, Slots: []string{"breakfast", "lunch", "dinner"}, Goal: "none", Kids: []Child{k}})
	if len(plan.KidsMenus) != 1 || plan.KidsMenus[0].Weaning == nil {
		t.Fatal("у баночек нет расписания")
	}
	w := plan.KidsMenus[0].Weaning
	if w.New == nil || !w.New.Jar {
		t.Fatalf("новый продукт не баночкой: %+v", w.New)
	}
	for _, f := range w.Days[3].Feeds {
		for _, it := range f.Items {
			if !it.Jar && it.Group != "yolk" && it.Group != "bread" {
				t.Fatalf("домашнее в баночном режиме: %+v", it)
			}
		}
	}
	var baby, raw []string
	for _, g := range plan.Shopping {
		for _, it := range g.Items {
			if it.Category == "baby" {
				baby = append(baby, it.IngredientID)
			}
			if it.IngredientID == "zucchini" || it.IngredientID == "buckwheat" {
				raw = append(raw, it.IngredientID)
			}
		}
	}
	if !slices.Contains(baby, "baby_veg_0") || !slices.Contains(baby, "baby_kasha_0") || len(raw) > 0 {
		t.Fatalf("покупки: детское %v, сырое %v", baby, raw)
	}
	// после года — меню из баночек по приёмам
	k = Child{AgeMonths: 18, Feeding: FeedJars}
	plan = c.Build(Params{Country: "RU", Adults: 2, Slots: []string{"breakfast", "lunch", "dinner"}, Goal: "none", Kids: []Child{k}})
	if len(plan.KidsMenus) != 1 || len(plan.KidsMenus[0].Days[0].Dishes) == 0 || plan.KidsMenus[0].Days[0].Dishes[0].Kind != MealJars {
		t.Fatalf("после года баночки не по приёмам: %+v", plan.KidsMenus)
	}
}

// Возраст растёт сам: «8 мес» с отметкой октября в плане на декабрь — уже 10 мес.
func TestKidAgeGrows(t *testing.T) {
	c := testCatalog(t)
	p := c.Normalize(Params{Adults: 2, StartDate: "2026-12-07", Kids: []Child{{AgeMonths: 8, Feeding: FeedWeaning, AgeAt: "2026-10"}}})
	if k := p.Kids[0]; k.AgeMonths != 10 || k.AgeAt != "2026-12" {
		t.Fatalf("возраст %d, отметка %s", k.AgeMonths, k.AgeAt)
	}
	// в 12 мес прикорм по месяцам кончается: режим меняется на допустимый по возрасту
	p = c.Normalize(Params{Adults: 2, StartDate: "2027-02-01", Kids: []Child{{AgeMonths: 10, Feeding: FeedWeaning, AgeAt: "2026-12"}}})
	if k := p.Kids[0]; k.AgeMonths != 12 || k.Feeding == FeedWeaning {
		t.Fatalf("в год: %d мес, режим %s", k.AgeMonths, k.Feeding)
	}
}

// Ребёнок в саду: по будням дома только ужин — порции завтрака и обеда меньше, в детском меню «в саду».
func TestKindergartenWeekdays(t *testing.T) {
	c := testCatalog(t)
	p := Params{Country: "RU", Adults: 2, Slots: []string{"breakfast", "lunch", "dinner"}, Goal: "none",
		Kids: []Child{{AgeMonths: 48, Feeding: FeedShared, Away: AwayKindergarten}}}
	plan := c.Build(p)
	var mon, sat Dish
	for _, x := range plan.Days[0].Dishes {
		if x.Slot == "breakfast" {
			mon = x
		}
	}
	for _, x := range plan.Days[5].Dishes {
		if x.Slot == "breakfast" {
			sat = x
		}
	}
	if plan.portionsForDish(mon) >= plan.portionsForDish(sat) {
		t.Fatalf("завтрак в будни %.2f, в субботу %.2f", plan.portionsForDish(mon), plan.portionsForDish(sat))
	}
	for _, x := range plan.Days[0].Dishes {
		if x.Slot == "dinner" && x.Portions != 0 {
			t.Fatalf("ужин в будни дома — порции обычные, а не %.2f", x.Portions)
		}
	}
	// отдельное меню: в будни завтрак, обед и полдник — «в саду»
	p.Kids = []Child{{AgeMonths: 30, Feeding: FeedSeparate, Away: AwayKindergarten}}
	plan = c.Build(p)
	kinds := map[string]string{}
	for _, x := range plan.KidsMenus[0].Days[1].Dishes {
		kinds[x.Slot] = x.Kind
	}
	if kinds["breakfast"] != "away.kindergarten" || kinds["dinner"] != "" {
		t.Fatalf("вторник в саду: %v", kinds)
	}
	for _, x := range plan.KidsMenus[0].Days[6].Dishes {
		if x.Kind != "" && x.Kind != KidBedtime {
			t.Fatalf("в воскресенье всё дома, а %s — %s", x.Slot, x.Kind)
		}
	}
}

// Беременность: из семейного меню уходят печень, сыры с плесенью, тунец, копчёная рыба, алкоголь и сырое;
// ккал мамы с прибавкой по триместру.
func TestPregnancyMenu(t *testing.T) {
	c := testCatalog(t)
	p := Params{Country: "RU", Slots: []string{"breakfast", "lunch", "dinner"},
		Members: []Member{{Goal: "lose", Appetite: "normal", Mom: "pregnant3"}, {Goal: "none", Appetite: "normal"}}}
	n := c.Normalize(p)
	if n.Members[0].Goal != "healthy" {
		t.Fatalf("при беременности цель «похудение» меняется, а осталась %s", n.Members[0].Goal)
	}
	if k := n.Members[0].kcalOf(); k != GoalKcal["healthy"]+350 {
		t.Fatalf("ккал в 3-м триместре %.0f", k)
	}
	e := c.effective(n)
	for _, r := range c.Recipes {
		if !c.allowed(r, e) {
			continue
		}
		for _, ri := range r.Ingredients {
			if slices.Contains(momExclude, ri.IngredientID) {
				t.Fatalf("при беременности в меню %s (%s)", r.Title, ri.IngredientID)
			}
		}
		if slices.Contains(r.Tags, "raw") {
			t.Fatalf("при беременности сырое: %s", r.Title)
		}
	}
	plan := c.Build(p)
	found := false
	for _, note := range plan.Notes {
		if strings.Contains(note, "300 мг") {
			found = true
		}
	}
	if !found {
		t.Fatalf("нет заметки для беременной: %v", plan.Notes)
	}
}

// Нарезка против удушья: малышу 2 лет у блюда с виноградом — подсказка про четвертинки.
func TestChokeNote(t *testing.T) {
	c := testCatalog(t)
	r := Recipe{ID: "x", Ingredients: []RecipeIngredient{{IngredientID: "grapes", Amount: 50}, {IngredientID: "cod_fillet", Amount: 50}}}
	if n := c.ChokeNote(r, 24, i18n.RU); !strings.Contains(n, "четвертинки") || !strings.Contains(n, "кост") {
		t.Fatalf("подсказка для 2 лет: %q", n)
	}
	if n := c.ChokeNote(r, 72, i18n.RU); n != "" {
		t.Fatalf("в 6 лет подсказка не нужна, а %q", n)
	}
}
