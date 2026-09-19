package planner

import (
	"fmt"
	"slices"
	"time"
)

// Правки готовой недели без пересборки: день «не дома», перестановка блюд между днями, повтор недели.
// Подбор не трогаем — меняем ячейки и пересчитываем список покупок и итоги (finish).

// cloneDays — копия дней, чтобы не трогать план, пришедший по значению со общими слайсами.
func cloneDays(plan Plan) Plan {
	days := make([]Day, len(plan.Days))
	for i, d := range plan.Days {
		d.Dishes = slices.Clone(d.Dishes)
		days[i] = d
	}
	plan.Days = days
	return plan
}

// SetSkip помечает день как «не дома»: блюда остаются, но не считаются ни в покупках, ни в итогах.
func (c *Catalog) SetSkip(plan Plan, day int, skip bool) (Plan, error) {
	if day < 0 || day >= len(plan.Days) {
		return plan, fmt.Errorf("day out of range")
	}
	plan = cloneDays(plan)
	plan.Days[day].Skipped = skip
	c.finish(&plan)
	return plan, nil
}

// Move меняет местами блюда одного приёма в двух днях. Суп на два дня (батч и его остаток) не двигаем:
// пара должна остаться соседями — такое проще заменить.
func (c *Catalog) Move(plan Plan, from, to int, slot string) (Plan, error) {
	if from < 0 || from >= len(plan.Days) || to < 0 || to >= len(plan.Days) || from == to {
		return plan, fmt.Errorf("plan.move.days")
	}
	fi, ti := dishIndex(plan.Days[from], slot), dishIndex(plan.Days[to], slot)
	if fi < 0 || ti < 0 {
		return plan, fmt.Errorf("no dish in that cell")
	}
	a, b := plan.Days[from].Dishes[fi], plan.Days[to].Dishes[ti]
	if a.Batch || a.Leftover || b.Batch || b.Leftover {
		return plan, fmt.Errorf("plan.move.batch")
	}
	plan = cloneDays(plan)
	plan.Days[from].Dishes[fi], plan.Days[to].Dishes[ti] = b, a
	c.finish(&plan)
	return plan, nil
}

// Repeat — та же неделя с новыми датами: любимую неделю можно «заморозить» и повторять.
// Отклонённые замены и пропуски дней сбрасываются; покупки у новой недели свои.
func (c *Catalog) Repeat(plan Plan, start string) (Plan, error) {
	t, err := time.Parse("2006-01-02", start)
	if err != nil || t.Weekday() != time.Monday {
		return plan, fmt.Errorf("plan.repeat.monday")
	}
	plan = cloneDays(plan)
	plan.ID = ""
	plan.Params.StartDate = start
	plan.Rejected = map[string][]string{}
	plan.Swaps = 0
	plan.Family = nil
	for d := range plan.Days {
		plan.Days[d].Date = t.AddDate(0, 0, d).Format("2006-01-02")
		plan.Days[d].Skipped = false
	}
	plan.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	c.finish(&plan)
	return plan, nil
}
