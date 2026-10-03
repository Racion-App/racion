package planner

import (
	"math"
	"testing"
)

// Цены из каталога сети заменяют оценку «Росстат × индекс» и честно подписываются как цены сети.
func TestStorePricesReplaceEstimate(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.Store = "vkusvill"
	before := c.Build(p)

	// Цены сети, равные оценке: состав недели тот же, меняется только источник цены.
	pr := c.pricerFor(c.Normalize(p))
	sp := &StorePrices{Store: "vkusvill", Date: "2026-10-03", Items: map[string]StoreItem{}}
	for id, ing := range c.Ingredients {
		pp, _ := pr.packPrice(ing)
		sp.Items[id] = StoreItem{Unit: "шт", Pack: ing.Pack, Price: pp}
	}
	c.SetStorePrices(sp)
	same := c.Build(p)
	if math.Abs(same.Totals.Cost-before.Totals.Cost) > 1 {
		t.Fatalf("равные цены сети изменили сумму: %.2f → %.2f", before.Totals.Cost, same.Totals.Cost)
	}
	var pick ShopItem
	for _, g := range same.Shopping {
		for _, it := range g.Items {
			if !it.Store || it.Rosstat {
				t.Errorf("%s: store=%v rosstat=%v, ждали цену сети", it.IngredientID, it.Store, it.Rosstat)
			}
			if !it.Pantry && !it.AtHome && it.Packs > 0 && pick.IngredientID == "" {
				pick = it
			}
		}
	}
	if same.PriceSource.Store == "" || same.PriceSource.StoreCoverage < 0.99 || same.PriceSource.StoreDate == "" {
		t.Errorf("источник: %+v, ждали сеть, дату и долю ≈1", same.PriceSource)
	}

	// Одна цена вдвое выше: если продукт остался в неделе, его строка дорожает ровно вдвое.
	it := sp.Items[pick.IngredientID]
	it.Price *= 2
	sp.Items[pick.IngredientID] = it
	c.SetStorePrices(sp)
	double := c.Build(p)
	for _, g := range double.Shopping {
		for _, x := range g.Items {
			if x.IngredientID == pick.IngredientID && x.Packs == pick.Packs {
				if math.Abs(x.Cost-2*pick.Cost) > 1 {
					t.Errorf("%s: %.2f при двойной цене, ждали %.2f", x.IngredientID, x.Cost, 2*pick.Cost)
				}
			}
		}
	}

	// Сеть без своего каталога считается как раньше.
	q := baseParams()
	if other := c.Build(q); other.PriceSource.Store != "" {
		t.Errorf("у «%s» нет каталога, а источник %q", q.Store, other.PriceSource.Store)
	}
}

// Весовой товар: цена за кг переводится в цену нашего грамма.
func TestStoreItemPerUnit(t *testing.T) {
	cases := []struct {
		it   StoreItem
		want float64
	}{
		{StoreItem{Unit: "кг", Price: 168}, 0.168},
		{StoreItem{Unit: "шт", Pack: 200, Price: 304}, 1.52},
		{StoreItem{Unit: "шт", Pack: 10, Price: 115}, 11.5},
		{StoreItem{Unit: "шт", Pack: 0, Price: 100}, 0},
	}
	for _, c := range cases {
		if got := c.it.PerUnit(); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%+v: %.4f, ждали %.4f", c.it, got, c.want)
		}
	}
}
