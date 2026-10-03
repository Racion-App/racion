package service

import (
	"fmt"
	"testing"

	"racion/internal/planner"
	"racion/internal/vkusvill"
)

func shopPlan(items ...planner.ShopItem) planner.Plan {
	return planner.Plan{Shopping: []planner.ShopGroup{{Category: "x", Items: items}}}
}

// Количество — в упаковках сети, а не наших; весовое — в килограммах; купленное и чужое не кладём.
func TestBuildStoreCartQuantities(t *testing.T) {
	sp := &planner.StorePrices{Store: "vkusvill", Items: map[string]planner.StoreItem{
		"cheese": {XMLID: 1, Unit: "шт", Pack: 150, Price: 337}, // наша упаковка 200 г, у них 150 г
		"banana": {XMLID: 2, Unit: "кг", Price: 168},
		"eggs":   {XMLID: 3, Unit: "шт", Pack: 10, Price: 115},
	}}
	plan := shopPlan(
		planner.ShopItem{IngredientID: "cheese", Name: "Сыр", Needed: 253},
		planner.ShopItem{IngredientID: "banana", Name: "Бананы", Needed: 1240},
		planner.ShopItem{IngredientID: "eggs", Name: "Яйца", Needed: 12, Home: 10}, // десяток дома: докупаем 2
		planner.ShopItem{IngredientID: "salt", Name: "Соль", Needed: 5},            // нет в каталоге сети
		planner.ShopItem{IngredientID: "milk", Name: "Молоко", Needed: 900},         // уже куплено: не в ids
	)
	items, out := buildStoreCart(plan, sp, []string{"cheese", "banana", "eggs", "salt"})
	q := map[int]float64{}
	for _, it := range items {
		q[it.XMLID] = it.Q
	}
	if q[1] != 2 {
		t.Errorf("сыр: %v упаковок по 150 г, ждали 2 (253 г)", q[1])
	}
	if q[2] != 1.24 {
		t.Errorf("бананы: %v кг, ждали 1.24", q[2])
	}
	if q[3] != 1 {
		t.Errorf("яйца: %v упаковок, ждали 1 (двух не хватает, десяток дома)", q[3])
	}
	if out.Wanted != 4 || out.Matched != 3 || out.Added != 3 || len(out.Left) != 1 || out.Left[0] != "Соль" {
		t.Errorf("итог %+v", out)
	}
}

// В ссылку влезает CartMax товаров: кладём самые дорогие, остальное — в «осталось».
func TestBuildStoreCartKeepsMostExpensive(t *testing.T) {
	sp := &planner.StorePrices{Store: "vkusvill", Items: map[string]planner.StoreItem{}}
	var shop []planner.ShopItem
	var ids []string
	for i := 1; i <= 25; i++ {
		id := fmt.Sprintf("p%d", i)
		sp.Items[id] = planner.StoreItem{XMLID: i, Unit: "шт", Pack: 100, Price: float64(i * 10)}
		shop = append(shop, planner.ShopItem{IngredientID: id, Name: id, Needed: 100})
		ids = append(ids, id)
	}
	items, out := buildStoreCart(shopPlan(shop...), sp, ids)
	if len(items) != vkusvill.CartMax || out.Added != vkusvill.CartMax || len(out.Left) != 5 {
		t.Fatalf("в корзине %d, осталось %d", len(items), len(out.Left))
	}
	for _, it := range items {
		if it.XMLID <= 5 {
			t.Errorf("дешёвый товар %d попал в корзину, а дорогие нет", it.XMLID)
		}
	}
}
