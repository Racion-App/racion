package service

import (
	"context"
	"math"
	"sort"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
	"racion/internal/vkusvill"
)

// CartLinker — сеть, у которой корзину можно собрать ссылкой. Сейчас это ВкусВилл через их MCP.
type CartLinker interface {
	CartLink(ctx context.Context, items []vkusvill.CartItem) (string, error)
}

// StoreCart — что положили в корзину сети и что осталось искать руками.
type StoreCart struct {
	Link    string   `json:"link"`
	Added   int      `json:"added"`   // позиций в корзине
	Matched int      `json:"matched"` // из списка нашлось в каталоге сети
	Wanted  int      `json:"wanted"`  // позиций в списке
	Left    []string `json:"left"`    // не нашлось в каталоге или не влезло в корзину
}

// SetCart подключает сборку корзины ссылкой.
func (p *Plans) SetCart(c CartLinker) { p.cart = c }

// StoreCart кладёт в корзину сети то, что ещё не куплено. ids — продукты, которые человек ещё не
// отметил в списке. Ссылка заменяет корзину и вмещает CartMax товаров, поэтому кладём самые дорогие:
// мясо, сыр и рыбу искать вручную дольше всего, а мелочь остаётся в списке со ссылками на поиск.
func (p *Plans) StoreCart(ctx context.Context, planID string, ids []string, lang i18n.Lang) (StoreCart, error) {
	var out StoreCart
	if p.cart == nil {
		return out, domain.Invalid("cart.unsupported")
	}
	l, cat, err := p.load(ctx, planID)
	if err != nil {
		return out, err
	}
	plan := cat.Localize(l.Plan, lang)
	sp := cat.StorePrices(plan.Store.Code)
	if sp == nil {
		return out, domain.Invalid("cart.unsupported")
	}
	items, out := buildStoreCart(plan, sp, ids)
	if len(items) == 0 {
		return out, domain.Invalid("cart.nothing")
	}
	link, err := p.cart.CartLink(ctx, items)
	if err != nil {
		return out, err
	}
	out.Link = link
	return out, nil
}

// buildStoreCart — что положить в корзину сети: количество в упаковках сети (не наших) или в
// килограммах для весового, самые дорогие позиции первыми, не больше CartMax.
func buildStoreCart(plan planner.Plan, sp *planner.StorePrices, ids []string) ([]vkusvill.CartItem, StoreCart) {
	var out StoreCart
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	type cand struct {
		item vkusvill.CartItem
		cost float64
		name string
	}
	var cands []cand
	for _, g := range plan.Shopping {
		for _, it := range g.Items {
			if !want[it.IngredientID] || it.AtHome {
				continue
			}
			out.Wanted++
			si, ok := sp.Items[it.IngredientID]
			need := it.Needed - it.Home
			if !ok || si.XMLID == 0 || need <= 0 {
				out.Left = append(out.Left, it.Name)
				continue
			}
			var q float64
			if si.Weighed() {
				q = math.Max(0.01, math.Ceil(need/1000*100-1e-9)/100) // килограммы, с точностью до 10 г
			} else {
				q = math.Max(1, math.Ceil(need/si.Pack-1e-9))
			}
			q = math.Min(q, 40) // больше их инструмент не принимает
			cands = append(cands, cand{vkusvill.CartItem{XMLID: si.XMLID, Q: q}, q * si.Price, it.Name})
		}
	}
	out.Matched = len(cands)
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].cost > cands[j].cost })
	items := make([]vkusvill.CartItem, 0, vkusvill.CartMax)
	for i, c := range cands {
		if i < vkusvill.CartMax {
			items = append(items, c.item)
		} else {
			out.Left = append(out.Left, c.name)
		}
	}
	out.Added = len(items)
	return items, out
}
