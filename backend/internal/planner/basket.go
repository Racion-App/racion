package planner

import (
	"fmt"
	"math"
	"slices"
	"time"

	"racion/internal/i18n"
)

// Корзина: человек сам набирает блюда и получает один список покупок на всех.
// От события отличается только тем, что состав задаёт он, а не курсы: цены, порции, округление
// до упаковок и страница плана — те же самые.

// BasketMax — сколько блюд можно положить в корзину. Больше тридцати на один стол не готовят,
// а список покупок перестаёт помещаться на экран.
const BasketMax = 30

// BasketItem — блюдо в корзине и на сколько человек его готовят. Salads на восьмерых и курица
// на четверых в одном столе — обычное дело, поэтому порции считаются по каждому блюду отдельно.
type BasketItem struct {
	RecipeID string `json:"recipeId"`
	Servings int    `json:"servings"`
}

// BuildBasket собирает стол из выбранных блюд. guests — сколько человек за столом: он идёт в шапку
// чека и в цену на человека, а количество продуктов считается по servings каждого блюда.
func (c *Catalog) BuildBasket(items []BasketItem, p Params, guests int) (Plan, error) {
	if len(items) == 0 {
		return Plan{}, fmt.Errorf("basket: empty")
	}
	if len(items) > BasketMax {
		items = items[:BasketMax]
	}
	if guests < 1 {
		guests = 1
	}
	if guests > 40 {
		guests = 40
	}
	p.Adults = guests
	p.Members = make([]Member, guests)
	for i := range p.Members {
		p.Members[i] = Member{Appetite: "normal"}
	}
	p.Kids = nil
	p.Slots = []string{"dinner"}
	p.BudgetValue = 0
	p = c.Normalize(p)
	lang := i18n.Lang(p.Lang)
	pr := c.pricerFor(p)

	byID := make(map[string]Recipe, len(c.Recipes))
	for _, r := range c.Recipes {
		byID[r.ID] = r
	}
	day := Day{Index: 0, Date: p.StartDate, Label: i18n.T(lang, "basket.title"), Dishes: []Dish{}}
	seen := map[string]bool{}
	for _, it := range items {
		r, ok := byID[it.RecipeID]
		if !ok || r.Hidden || seen[it.RecipeID] {
			continue
		}
		seen[it.RecipeID] = true
		n := it.Servings
		if n < 1 {
			n = guests
		}
		if n > 40 {
			n = 40
		}
		d := c.dish(r, slotOf(r), pr)
		d.Servings = n
		day.Dishes = append(day.Dishes, d)
	}
	if len(day.Dishes) == 0 {
		return Plan{}, fmt.Errorf("basket: no dishes")
	}
	plan := Plan{
		Params: p, Lang: p.Lang, Country: CountryOf(p.Country), Store: c.Stores[p.Store],
		Portions: float64(guests), SlotPortions: map[string]float64{"dinner": float64(guests)},
		Days: []Day{day},
		Goal: Goal{Level: "none", Label: GoalLabel(lang, "none")},
		Seed: time.Now().UnixNano(), Rejected: map[string][]string{}, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Notes: []string{}, Warnings: []string{},
		Basket: &BasketInfo{Guests: guests, Dishes: len(day.Dishes)},
		// шапка и итоги чека у стола такие же, как у события: пустой ID отличает его от настоящего события
		Occasion: &OccasionInfo{ID: "", Title: i18n.T(lang, "basket.title"), Guests: guests},
	}
	c.finish(&plan)
	plan.Budget = Budget{Mode: "week", Value: plan.Totals.Cost, PerDay: plan.Totals.Cost / float64(guests), TargetWeek: plan.Totals.Cost}
	plan.Totals.KcalPerDay = math.Round(plan.Days[0].Kcal)
	return plan, nil
}

// BasketInfo — шапка чека корзины вместо названия события.
type BasketInfo struct {
	Guests int `json:"guests"`
	Dishes int `json:"dishes"`
}

// slotOf — к какому приёму отнести блюдо на столе. Нужен только для подписи в чеке: завтраки и
// обеды в корзине встречаются редко, но если человек положил сырники, пусть они и называются завтраком.
func slotOf(r Recipe) string {
	if r.Slot != "" && slices.Contains([]string{"breakfast", "lunch", "dinner", "snack"}, r.Slot) {
		return r.Slot
	}
	return "dinner"
}
