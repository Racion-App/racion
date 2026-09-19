package service

import (
	"context"
	"math"

	"racion/internal/i18n"
	"racion/internal/planner"
)

// Замены продуктов: «нет сметаны — что вместо?». Таблица из seed/data/substitutes.json (нейросеть + правки),
// пересчёт калорий и цены порции — честный: подменяем продукт в копии рецепта и считаем заново.

type SubOption struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Amount    float64 `json:"amount"` // сколько заменителя на порцию (с коэффициентом)
	Unit      string  `json:"unit"`
	Note      string  `json:"note"`
	KcalDelta float64 `json:"kcalDelta"` // порция: ккал после замены минус до
	CostDelta float64 `json:"costDelta"` // порция: цена после минус до (валюта страны)
	Symbol    string  `json:"symbol"`    // знак валюты страны для дельты
}

type SubRow struct {
	IngredientID string      `json:"ingredientId"`
	Options      []SubOption `json:"options"`
}

type SubEntry struct {
	ID     string   `json:"id"`
	Ratio  float64  `json:"ratio"`
	Note   string   `json:"note"`
	NoteEn string   `json:"noteEn"` // для всех языков, кроме русского
	Not    []string // теги рецепта, при которых замена неуместна
}

type Substitutes struct {
	table   map[string][]SubEntry
	recipes *Recipes
}

func NewSubstitutes(table map[string][]SubEntry, recipes *Recipes) *Substitutes {
	return &Substitutes{table: table, recipes: recipes}
}

// For — замены для каждого продукта рецепта, у которого они есть, с дельтами на порцию.
func (s *Substitutes) For(ctx context.Context, recipeID, country string, lang i18n.Lang) ([]SubRow, error) {
	rc, err := s.recipes.Find(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	c := s.recipes.CatalogFor(ctx, nil)
	if rc.Own {
		c = s.recipes.CatalogFor(ctx, &rc.OwnerID)
	}
	kcal0, _, _, _ := c.Nutrition(rc)
	cost0, _ := c.PortionCost(rc, country)
	out := []SubRow{}
	for i, ri := range rc.Ingredients {
		entries := s.table[ri.IngredientID]
		if len(entries) == 0 {
			continue
		}
		row := SubRow{IngredientID: ri.IngredientID}
		for _, e := range entries {
			ing, ok := c.Ingredients[e.ID]
			if !ok || unfit(e.Not, rc.Tags) {
				continue
			}
			alt := rc
			alt.Ingredients = append([]planner.RecipeIngredient{}, rc.Ingredients...)
			amount := math.Round(ri.Amount*e.Ratio*10) / 10
			alt.Ingredients[i] = planner.RecipeIngredient{IngredientID: e.ID, Amount: amount}
			kcal1, _, _, _ := c.Nutrition(alt)
			cost1, _ := c.PortionCost(alt, country)
			note := e.Note
			if lang != i18n.RU && e.NoteEn != "" {
				note = e.NoteEn
			}
			row.Options = append(row.Options, SubOption{ID: e.ID, Name: ing.LocalName(lang), Amount: amount, Unit: ing.Unit, Note: note, Symbol: planner.CountryOf(country).Symbol,
				KcalDelta: math.Round(kcal1 - kcal0), CostDelta: math.Round((cost1-cost0)*100) / 100})
		}
		if len(row.Options) > 0 {
			out = append(out, row)
		}
	}
	return out, nil
}

// unfit — у рецепта есть тег, при котором эта замена не годится.
func unfit(not, tags []string) bool {
	for _, n := range not {
		for _, t := range tags {
			if n == t {
				return true
			}
		}
	}
	return false
}
