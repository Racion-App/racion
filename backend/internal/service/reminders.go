package service

import (
	"strings"
	"time"

	"racion/internal/domain"
)

// Напоминания: что пора сказать человеку в этот час по его местному времени. Считает одна функция,
// а доставляют двое — веб-пуш и боты в мессенджерах, поэтому правила у них не расходятся.

// Reminder — одно напоминание. Key — запись в журнале отправленного: одно и то же не приходит дважды.
// У аккаунта, привязанного к боту, ключи общие с веб-пушем: напоминание приходит в одно место.
type Reminder struct {
	Kind   string // shop, today, prepday-eve, prepday, prep, ask, digest, week
	Key    string
	PlanID string
	Date   string             // день, о котором напоминание
	Left   int                // shop: сколько позиций ещё купить
	Dishes []string           // today, prep: что готовим
	Prep   domain.PrepDayInfo // prepday-eve, prepday
	Dinner domain.DishRef     // ask: ужин, о котором спрашиваем
}

// dueReminders — напоминания на этот час. local — время человека (UTC со сдвигом из настроек),
// планы — последние сначала, как их отдаёт хранилище.
func dueReminders(local time.Time, s domain.NotifySettings, plans []domain.PlanReminderInfo) []Reminder {
	today := local.Format("2006-01-02")
	tomorrow := local.AddDate(0, 0, 1).Format("2006-01-02")
	hour := local.Hour()
	var out []Reminder
	// магазин: в выбранный день и час, для ближайшей недели, где ещё есть что купить
	if s.ShopHour >= 0 && int(local.Weekday()) == s.ShopDay && hour == s.ShopHour {
		from, to := local.AddDate(0, 0, -6).Format("2006-01-02"), local.AddDate(0, 0, 7).Format("2006-01-02")
		for _, p := range plans {
			if p.StartDate < from || p.StartDate > to {
				continue
			}
			if left := p.Items - p.Checked; left > 0 {
				out = append(out, Reminder{Kind: "shop", Key: "shop:" + p.ID + ":" + today, PlanID: p.ID, Date: today, Left: left})
				break
			}
		}
	}
	// утром: что готовим сегодня
	if s.Today && hour == s.TodayHour {
		for _, p := range plans {
			if dishes := p.Dishes[today]; len(dishes) > 0 {
				out = append(out, Reminder{Kind: "today", Key: "today:" + p.ID + ":" + today, PlanID: p.ID, Date: today, Dishes: dishes})
				break
			}
		}
	}
	// заготовки: накануне вечером и утром в день заготовок
	if s.PrepDay {
		for _, p := range plans {
			for _, pd := range p.PrepDays {
				switch {
				case pd.Date == tomorrow && hour == s.PrepHour:
					out = append(out, Reminder{Kind: "prepday-eve", Key: "prepday-eve:" + p.ID + ":" + pd.Date, PlanID: p.ID, Date: pd.Date, Prep: pd})
				case pd.Date == today && hour == s.TodayHour:
					out = append(out, Reminder{Kind: "prepday", Key: "prepday:" + p.ID + ":" + pd.Date, PlanID: p.ID, Date: pd.Date, Prep: pd})
				}
			}
		}
	}
	// вечером: что готовим завтра
	if s.Prep && hour == s.PrepHour {
		for _, p := range plans {
			if dishes := p.Dishes[tomorrow]; len(dishes) > 0 {
				out = append(out, Reminder{Kind: "prep", Key: "prep:" + p.ID + ":" + tomorrow, PlanID: p.ID, Date: tomorrow, Dishes: dishes})
				break
			}
		}
	}
	// после ужина: «как было?» — ответ учит планировщик; свои рецепты не спрашиваем
	if !s.NoAsk && hour == 20 {
		for _, p := range plans {
			if d, ok := p.Dinner[today]; ok && !strings.HasPrefix(d.RecipeID, "u_") {
				out = append(out, Reminder{Kind: "ask", Key: "ask:" + p.ID + ":" + today, PlanID: p.ID, Date: today, Dinner: d})
				break
			}
		}
	}
	// пятница вечером: новые рецепты и подборки за неделю
	if s.Digest && local.Weekday() == time.Friday && hour == 18 {
		out = append(out, Reminder{Kind: "digest", Key: "digest:" + today, Date: today})
	}
	// воскресенье в полдень: на следующую неделю плана нет
	if s.Week && local.Weekday() == time.Sunday && hour == 12 {
		monday := tomorrow
		has := false
		for _, p := range plans {
			if p.StartDate == monday {
				has = true
			}
		}
		if !has {
			out = append(out, Reminder{Kind: "week", Key: "week:" + monday, Date: monday})
		}
	}
	return out
}
