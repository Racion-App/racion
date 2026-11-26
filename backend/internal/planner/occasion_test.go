package planner

import "testing"

func TestNewYearTableShopping(t *testing.T) {
	c := testCatalog(t)
	course := []OccasionCourse{{Key: "salads", Base: 2, Pool: []string{"olivier", "crab_salad"}}}
	o := Occasion{ID: "newyear", Courses: course, January: [][]OccasionCourse{course}}
	p := Params{Country: "RU", Lang: "ru", Equipment: []string{"stove"}, NewYear: &NewYearOptions{January: [3]int{4}, TableWine: true}}
	for _, wine := range []bool{true, false} {
		p.NewYear.TableWine = wine
		plan, err := c.BuildOccasion(o, p, 8)
		if err != nil {
			t.Fatal(err)
		}
		c.finish(&plan) // a recalculation must not add another round of groceries
		need := map[string]float64{}
		for _, group := range plan.Shopping {
			for _, item := range group.Items {
				need[item.IngredientID] = item.Needed
			}
		}
		if need["mandarin"] != 1600 {
			t.Fatalf("expected mandarins for December 31 only, got %v", need["mandarin"])
		}
		if wine && (need["sparkling_wine"] != 1500 || need["red_wine"] != 750) {
			t.Fatal("expected two bottles of sparkling wine and one of red")
		}
		if !wine && (need["sparkling_wine"] != 0 || need["red_wine"] != 0) {
			t.Fatal("bottles present with table wine turned off")
		}
	}
}

func TestNewYearOptionalDatesAndCocktail(t *testing.T) {
	c := testCatalog(t)
	cocktail := Recipe{ID: "ny_kir_royal", Title: "Кир рояль", Slot: "snack", Tags: []string{"drink", "alcohol", "bar"}, Ingredients: []RecipeIngredient{{"blackcurrant_liqueur", 10}}}
	c.Recipes = append(c.Recipes, cocktail)
	c.RecipeByID[cocktail.ID] = cocktail
	course := []OccasionCourse{{Key: "salads", Base: 1, Pool: []string{"olivier"}}}
	o := Occasion{ID: "newyear", Courses: course, January: [][]OccasionCourse{course, course, course}}
	p := Params{Country: "RU", Lang: "ru", Equipment: []string{"stove", "oven"}, NewYear: &NewYearOptions{January: [3]int{2, 0, 4}, Cocktail: cocktail.ID, Glasses: 3}}
	plan, err := c.BuildOccasion(o, p, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Days) != 3 || plan.Days[1].Date[5:] != "01-01" || plan.Days[2].Date[5:] != "01-03" {
		t.Fatalf("wrong dates: %+v", plan.Days)
	}
	for i, n := range []int{8, 2, 4} {
		if plan.Days[i].Dishes[0].Servings != n {
			t.Fatalf("day %d expected %d portions", i, n)
		}
	}
	if len(plan.Days[0].Dishes) != 2 {
		t.Fatalf("cocktail missing: %+v", plan.Days[0].Dishes)
	}
	d := plan.Days[0].Dishes[1]
	if d.RecipeID != cocktail.ID || d.Servings != 3 || d.DrinkML != 360 {
		t.Fatalf("wrong cocktail yield: %+v", d)
	}
	found := false
	for _, g := range plan.Shopping {
		for _, item := range g.Items {
			if item.IngredientID == "blackcurrant_liqueur" && item.Needed == 30 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("shopping list missing 30 ml for three cocktails")
	}
	p.NewYear = nil
	basic, err := c.BuildOccasion(o, p, 8)
	if err != nil || len(basic.Days) != 1 || len(basic.Days[0].Dishes) != 1 {
		t.Fatalf("default table: %v, error %v", basic.Days, err)
	}
}

func TestCocktailSwitchAndReplacement(t *testing.T) {
	c := testCatalog(t)
	cocktail := Recipe{ID: "bar_test_one", Slot: "snack", Tags: []string{"drink", "alcohol", "bar"}, Ingredients: []RecipeIngredient{{"sparkling_wine", 80}}}
	other := cocktail
	other.ID = "bar_test_two"
	other.Ingredients = []RecipeIngredient{{"sparkling_wine", 60}}
	for _, r := range []Recipe{cocktail, other} {
		c.Recipes = append(c.Recipes, r)
		c.RecipeByID[r.ID] = r
	}
	regular := []OccasionCourse{{Key: "salads", Base: 1, Pool: []string{"olivier"}}}
	o := Occasion{ID: "newyear", Courses: regular}
	previous := occasions
	SetOccasions([]Occasion{o})
	t.Cleanup(func() { SetOccasions(previous) })
	p := Params{Country: "RU", Lang: "ru", Equipment: []string{"stove", "oven"}, NewYear: &NewYearOptions{}}
	off, err := c.BuildOccasion(o, p, 6)
	if err != nil || len(off.Days[0].Dishes) != 1 {
		t.Fatalf("off: %v %v", off.Days, err)
	}
	if c.Allowed(cocktail, p) {
		t.Fatal("bar recipe eligible for a regular meal")
	}
	p.NewYear.Alcohol = true
	on, err := c.BuildOccasion(o, p, 6)
	if err != nil || len(on.Days[0].Dishes) != 2 {
		t.Fatalf("on: %v %v", on.Days, err)
	}
	first := on.Days[0].Dishes[1]
	if first.Course != "cocktails" || first.Servings != 6 || first.DrinkML != 6*c.GlassML(c.RecipeByID[first.RecipeID]) {
		t.Fatal(first)
	}
	swapped, err := c.swapOccasion(on, 0, "dinner#1", c.pricerFor(on.Params), targets{})
	if err != nil || first.RecipeID == swapped.Days[0].Dishes[1].RecipeID || first.DrinkML == swapped.Days[0].Dishes[1].DrinkML {
		t.Fatalf("replacement: %v %v", swapped.Days, err)
	}
}
