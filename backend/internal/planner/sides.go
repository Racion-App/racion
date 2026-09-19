package planner

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
)

// Гарниры. Блюдо с тегом needs_side (котлеты, шашлык, стейк, рыба без крупы) само по себе — не ужин:
// планировщик докладывает к нему гарнир с тегом side. Гарнир подбирается под остаток калорий приёма,
// под тип белка (for_meat / for_poultry / for_fish; без пометки — ко всему) и без повторов подряд.
// В Dish итоги (ккал, БЖУ, цена, время) — суммой основного и гарнира; сам гарнир — в Dish.Side.

// Side — гарнир внутри приёма пищи.
type Side struct {
	RecipeID string  `json:"recipeId"`
	Title    string  `json:"title"`
	TimeMin  int     `json:"timeMin"`
	Kcal     float64 `json:"kcal"`
	Protein  float64 `json:"protein"`
	Fat      float64 `json:"fat"`
	Carb     float64 `json:"carb"`
	Cost     float64 `json:"cost"`
}

// sideKcalTypical — сколько калорий обычно добавляет гарнир; так основное блюдо оценивается честно
// против цели приёма, а не как «слишком лёгкое».
const sideKcalTypical = 230

func NeedsSide(r Recipe) bool { return slices.Contains(r.Tags, "needs_side") }
func IsSide(r Recipe) bool    { return slices.Contains(r.Tags, "side") }

// proteinKind — к какому типу белка относится основное блюдо: meat | poultry | fish | "".
func proteinKind(r Recipe) string {
	for _, k := range []string{"poultry", "fish", "seafood", "meat"} {
		if slices.Contains(r.Tags, k) {
			if k == "seafood" {
				return "fish"
			}
			return k
		}
	}
	return ""
}

// SidePairs — гарнир подходит к блюду: у гарнира нет пометок for_* или есть пометка под белок блюда.
func SidePairs(side, main Recipe) bool {
	limited := false
	for _, t := range side.Tags {
		if t == "for_meat" || t == "for_poultry" || t == "for_fish" {
			limited = true
		}
	}
	if !limited {
		return true
	}
	return slices.Contains(side.Tags, "for_"+proteinKind(main))
}

// SidesFor — гарниры к блюду по сочетаемости (для страницы рецепта и подсказок).
func (c *Catalog) SidesFor(main Recipe) []Recipe {
	var out []Recipe
	for _, r := range c.Recipes {
		if IsSide(r) && SidePairs(r, main) {
			out = append(out, r)
		}
	}
	return out
}

// MainsFor — блюда, к которым подходит гарнир.
func (c *Catalog) MainsFor(side Recipe) []Recipe {
	var out []Recipe
	for _, r := range c.Recipes {
		if NeedsSide(r) && SidePairs(side, r) {
			out = append(out, r)
		}
	}
	return out
}

// pickSide — гарнир к основному блюду: разрешён по технике и ограничениям, сочетается, не тот же, что
// вчера и не из отклонённых; из подходящих — ближе всего к остатку калорий приёма, чуть случайно.
func (c *Catalog) pickSide(main Recipe, e effective, wantKcal float64, recent []string, reject []string, pr pricer, rng *rand.Rand) (Recipe, bool) {
	type cand struct {
		r     Recipe
		score float64
	}
	var cands []cand
	for _, r := range c.Recipes {
		if !IsSide(r) || !c.allowed(r, e) || !SidePairs(r, main) || slices.Contains(reject, r.ID) {
			continue
		}
		k, _, _, _ := c.Nutrition(r)
		score := math.Abs(k-wantKcal) / 100
		if slices.Contains(recent, r.ID) {
			score += 3
		}
		// гарнир не должен тянуть на второе блюдо: дороже основного — штраф
		if c.costPerPortion(r, pr) > c.costPerPortion(main, pr) {
			score += 1.5
		}
		score += rng.Float64() * 0.8
		cands = append(cands, cand{r, score})
	}
	if len(cands) == 0 {
		return Recipe{}, false
	}
	slices.SortFunc(cands, func(a, b cand) int {
		if a.score < b.score {
			return -1
		}
		if a.score > b.score {
			return 1
		}
		return 0
	})
	return cands[0].r, true
}

// attachSide — добавить гарнир к блюду: итоги суммой, время — большее из двух плюс пять минут на подачу.
func (c *Catalog) attachSide(d *Dish, side Recipe, pr pricer) {
	k, prot, f, cb := c.Nutrition(side)
	s := Side{RecipeID: side.ID, Title: side.LocalTitle(pr.lang), TimeMin: side.TimeMin,
		Kcal: math.Round(k), Protein: math.Round(prot), Fat: math.Round(f), Carb: math.Round(cb), Cost: pr.round(c.costPerPortion(side, pr))}
	d.Side = &s
	d.Kcal += s.Kcal
	d.Protein += s.Protein
	d.Fat += s.Fat
	d.Carb += s.Carb
	d.Cost = pr.round(d.Cost + s.Cost)
	if s.TimeMin > d.TimeMin {
		d.TimeMin = s.TimeMin
	}
	d.TimeMin += 5
}

// sideRecent — гарниры двух дней вокруг, чтобы не есть пюре три дня подряд.
func sideRecent(days []Day, day int) []string {
	var out []string
	for d := day - 2; d <= day+2; d++ {
		if d < 0 || d >= len(days) {
			continue
		}
		for _, dish := range days[d].Dishes {
			if dish.Side != nil {
				out = append(out, dish.Side.RecipeID)
			}
		}
	}
	return out
}

// SwapSide — другой гарнир к тому же блюду; прежние гарниры этой ячейки больше не предлагаются.
func (c *Catalog) SwapSide(plan Plan, day int, slot string) (Plan, error) {
	if day < 0 || day >= len(plan.Days) {
		return plan, fmt.Errorf("day out of range")
	}
	days := make([]Day, len(plan.Days))
	for i, d := range plan.Days {
		d.Dishes = slices.Clone(d.Dishes)
		days[i] = d
	}
	plan.Days = days
	rejected := map[string][]string{}
	for k, v := range plan.Rejected {
		rejected[k] = slices.Clone(v)
	}
	plan.Rejected = rejected

	di := dishIndex(plan.Days[day], slot)
	if di < 0 {
		return plan, fmt.Errorf("no dish in that cell")
	}
	dish := plan.Days[day].Dishes[di]
	main, ok := c.RecipeByID[dish.RecipeID]
	if !ok || !NeedsSide(main) {
		return plan, fmt.Errorf("dish has no side")
	}
	p := c.Normalize(plan.Params)
	e := c.effective(p)
	pr := c.pricerFor(p)
	key := fmt.Sprintf("side:%d:%s", day, slot)
	if dish.Side != nil {
		plan.Rejected[key] = append(plan.Rejected[key], dish.Side.RecipeID)
		// снять старый гарнир из итогов
		dish.Kcal -= dish.Side.Kcal
		dish.Protein -= dish.Side.Protein
		dish.Fat -= dish.Side.Fat
		dish.Carb -= dish.Side.Carb
		dish.Cost = pr.round(dish.Cost - dish.Side.Cost)
		dish.TimeMin = main.TimeMin
		dish.Side = nil
	}
	goal := goalOf(p)
	var shareSum float64
	for _, s := range p.Slots {
		shareSum += slotShare[s]
	}
	want := goal.KcalTarget*slotShare[slot]/shareSum - dish.Kcal
	swaps := plan.Swaps + 1
	rng := rand.New(rand.NewSource(plan.Seed + int64(swaps)*7919 + int64(day*10) + int64(len(slot)) + 17))
	side, ok := c.pickSide(main, e, want, sideRecent(plan.Days, day), plan.Rejected[key], pr, rng)
	if !ok {
		// все перебрали — начинаем круг заново
		plan.Rejected[key] = nil
		side, ok = c.pickSide(main, e, want, nil, nil, pr, rng)
		if !ok {
			return plan, fmt.Errorf("no side available")
		}
	}
	c.attachSide(&dish, side, pr)
	plan.Days[day].Dishes[di] = dish
	plan.Swaps = swaps
	c.finish(&plan)
	return plan, nil
}
