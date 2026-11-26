package planner

import "testing"

func TestNewYearOptionalDatesAndCocktail(t *testing.T) {
	c := testCatalog(t)
	cocktail := Recipe{ID: "ny_kir_royal", Title: "Кир рояль", Slot: "snack", Tags: []string{"alcohol"}, Ingredients: []RecipeIngredient{{"blackcurrant_liqueur", 10}}}
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
