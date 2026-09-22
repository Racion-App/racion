package planner

import "testing"

// Корзина: порции у каждого блюда свои, и список покупок должен это учитывать —
// салат на восьмерых требует вдвое больше продуктов, чем тот же салат на четверых.
func TestBuildBasketServings(t *testing.T) {
	c := testCatalog(t)
	var ids []string
	for _, r := range c.Recipes {
		if !r.Hidden && len(r.Ingredients) > 0 {
			ids = append(ids, r.ID)
		}
		if len(ids) == 2 {
			break
		}
	}
	if len(ids) < 2 {
		t.Skip("в тестовом каталоге меньше двух рецептов")
	}
	p := Params{Country: "RU", Adults: 4, Lang: "ru"}

	small, err := c.BuildBasket([]BasketItem{{RecipeID: ids[0], Servings: 4}}, p, 4)
	if err != nil {
		t.Fatalf("BuildBasket: %v", err)
	}
	big, err := c.BuildBasket([]BasketItem{{RecipeID: ids[0], Servings: 8}}, p, 4)
	if err != nil {
		t.Fatalf("BuildBasket: %v", err)
	}
	if big.Totals.Cost <= small.Totals.Cost {
		t.Errorf("восемь порций должны стоить дороже четырёх: %.2f против %.2f", big.Totals.Cost, small.Totals.Cost)
	}
	if small.Basket == nil || small.Basket.Guests != 4 || small.Basket.Dishes != 1 {
		t.Errorf("шапка корзины неверна: %+v", small.Basket)
	}
	if len(small.Days) != 1 || len(small.Days[0].Dishes) != 1 {
		t.Fatalf("ожидался один день с одним блюдом, получено %d дней", len(small.Days))
	}
	if small.Days[0].Dishes[0].Servings != 4 {
		t.Errorf("порции блюда не сохранились: %d", small.Days[0].Dishes[0].Servings)
	}

	// два блюда и дубль одного из них: дубль отбрасывается, лишние блюда не задваивают покупки
	two, err := c.BuildBasket([]BasketItem{{RecipeID: ids[0], Servings: 4}, {RecipeID: ids[1], Servings: 4}, {RecipeID: ids[0], Servings: 4}}, p, 4)
	if err != nil {
		t.Fatalf("BuildBasket: %v", err)
	}
	if len(two.Days[0].Dishes) != 2 {
		t.Errorf("дубль не отброшен: блюд %d", len(two.Days[0].Dishes))
	}

	if _, err := c.BuildBasket(nil, p, 4); err == nil {
		t.Error("пустая корзина должна давать ошибку")
	}
	if _, err := c.BuildBasket([]BasketItem{{RecipeID: "нет-такого", Servings: 2}}, p, 4); err == nil {
		t.Error("корзина из несуществующих рецептов должна давать ошибку")
	}
}
