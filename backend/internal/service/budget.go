package service

import (
	"context"
	"math"
	"time"
)

// Динамика бюджета: по неделям «план / факт покупок», по месяцам — факт и сравнение с прошлым.
// План — стоимость недель пользователя (свои и семейные), факт — отмеченные покупки.

type BudgetWeek struct {
	Start   string  `json:"start"`   // понедельник
	Planned float64 `json:"planned"` // сумма недель, начинающихся в эту неделю
	Bought  float64 `json:"bought"`  // покупки за неделю
}

type BudgetMonth struct {
	Month  string  `json:"month"` // YYYY-MM
	Bought float64 `json:"bought"`
}

type BudgetReport struct {
	Weeks    []BudgetWeek  `json:"weeks"`
	Months   []BudgetMonth `json:"months"`
	DeltaPct *float64      `json:"deltaPct,omitempty"` // этот месяц к прошлому по факту: −12 = тратите на 12% меньше
	Currency string        `json:"currency"`           // код страны последней недели, для форматирования
}

// Budget — последние weeks недель (≤ 12) по данным пользователя.
func (a *Accounts) Budget(ctx context.Context, userID string, weeks int, now time.Time) (BudgetReport, error) {
	if weeks <= 0 || weeks > 12 {
		weeks = 8
	}
	plans, err := a.plans.ByUser(ctx, userID)
	if err != nil {
		return BudgetReport{}, err
	}
	purchases, err := a.purchases.Recent(ctx, userID, weeks*7+31)
	if err != nil {
		return BudgetReport{}, err
	}
	monday := func(t time.Time) time.Time {
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		wd := (int(t.Weekday()) + 6) % 7 // пн = 0
		return t.AddDate(0, 0, -wd)
	}
	first := monday(now).AddDate(0, 0, -7*(weeks-1))
	report := BudgetReport{Weeks: make([]BudgetWeek, weeks)}
	for i := range report.Weeks {
		report.Weeks[i].Start = first.AddDate(0, 0, 7*i).Format("2006-01-02")
	}
	idx := func(day string) int {
		t, err := time.Parse("2006-01-02", day)
		if err != nil {
			return -1
		}
		i := int(monday(t).Sub(first).Hours() / 24 / 7)
		if i < 0 || i >= weeks {
			return -1
		}
		return i
	}
	for _, p := range plans {
		if p.Shared {
			continue
		}
		if i := idx(p.StartDate); i >= 0 {
			report.Weeks[i].Planned += p.Cost
		}
		if report.Currency == "" {
			report.Currency = p.Country.Code
		}
	}
	months := map[string]float64{}
	for _, pu := range purchases {
		t, err := time.Parse(time.RFC3339, pu.BoughtAt)
		if err != nil {
			continue
		}
		if i := idx(t.Format("2006-01-02")); i >= 0 {
			report.Weeks[i].Bought += pu.Cost
		}
		months[t.Format("2006-01")] += pu.Cost
	}
	cur, prev := now.Format("2006-01"), now.AddDate(0, -1, 0).Format("2006-01")
	report.Months = []BudgetMonth{{Month: prev, Bought: round2(months[prev])}, {Month: cur, Bought: round2(months[cur])}}
	// сравнение по среднему дневному расходу: текущий месяц ещё не кончился
	if months[prev] > 0 && months[cur] > 0 {
		daysCur := float64(now.Day())
		daysPrev := float64(time.Date(now.Year(), now.Month(), 0, 0, 0, 0, 0, time.UTC).Day())
		d := math.Round((months[cur]/daysCur/(months[prev]/daysPrev) - 1) * 100)
		report.DeltaPct = &d
	}
	for i := range report.Weeks {
		report.Weeks[i].Planned = round2(report.Weeks[i].Planned)
		report.Weeks[i].Bought = round2(report.Weeks[i].Bought)
	}
	return report, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
