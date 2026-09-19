package planner

import "testing"

// Свой рецепт попадает в неделю владельца, помечается как own и не трогает общий каталог.
func TestOwnRecipes(t *testing.T) {
	c := testCatalog(t)
	own := Recipe{ID: "u_test", Title: "Мамина запеканка", Slot: "dinner", TimeMin: 40, Equipment: []string{"oven"},
		Steps:       []string{"Смешать", "Запечь"},
		Ingredients: []RecipeIngredient{{"chicken_breast", 150}, {"potato", 200}}}
	uc := c.WithRecipes([]Recipe{own})
	if len(uc.Recipes) != len(c.Recipes)+1 || uc.RecipeByID["u_test"].ID == "" {
		t.Fatalf("copy must carry the own recipe")
	}
	if _, ok := c.RecipeByID["u_test"]; ok {
		t.Fatalf("shared catalog must not see own recipes")
	}
	if uc.PriceBook() != c.PriceBook() {
		t.Fatalf("copies must share the price store")
	}
	p := baseParams()
	p.Lang = "ru"
	plan := uc.Build(p)
	found := false
	for _, d := range plan.Days {
		for _, dish := range d.Dishes {
			if dish.RecipeID == "u_test" {
				found = true
				if !dish.Own || dish.Title != "Мамина запеканка" || dish.Kcal <= 0 || dish.Cost <= 0 {
					t.Fatalf("own dish must be flagged and priced: %+v", dish)
				}
			}
		}
	}
	if !found {
		t.Fatalf("own recipe should appear in the owner's week")
	}
	// Localize и Swap работают с копией, где рецепт есть.
	en := uc.Localize(plan, "en")
	for _, d := range en.Days {
		for _, dish := range d.Dishes {
			if dish.RecipeID == "u_test" && dish.Title != "Мамина запеканка" {
				t.Fatalf("own recipe keeps its title in other languages, got %q", dish.Title)
			}
		}
	}
}
