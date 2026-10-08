package http

import (
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"racion/internal/i18n"
	"racion/internal/planner"
)

// Новогодний стол в подборке new-year-table: стол события newyear (occasions.json), собранный тем же
// планировщиком, что и в приложении, — на 4, 6 и 8 человек и недорогой на шестерых. У каждого варианта
// блюда по курсам с фото, цена стола и на гостя, список покупок; для стола на шестерых — тот же список по
// ценам разных сетей. Варианты — вкладки на радиокнопках, все в HTML.

type feastVariant struct {
	Key     string
	Guests  int
	Thrifty bool
}

var feastVariants = []feastVariant{{"4", 4, false}, {"6", 6, false}, {"8", 8, false}, {"cheap", 6, true}}

// кэш столов на час: вариант+язык+страна (сборка стола — проход по всему каталогу)
var feastCache sync.Map

func (s *Server) feastPlan(v feastVariant, pl pageLocale) (planner.Plan, bool) {
	key := v.Key + "|" + string(pl.L) + "|" + pl.Country.Code
	if c, ok := feastCache.Load(key); ok && time.Since(c.(menuCached).at) < time.Hour {
		return c.(menuCached).plan, true
	}
	o, ok := planner.OccasionByID("newyear")
	if !ok {
		return planner.Plan{}, false
	}
	p := planner.Params{Lang: string(pl.L), Country: pl.Country.Code, Equipment: []string{"stove", "oven", "microwave"}, Thrifty: v.Thrifty}
	plan, err := s.catalog.BuildOccasion(o, p, v.Guests)
	if err != nil || len(plan.Days) == 0 {
		return planner.Plan{}, false
	}
	feastCache.Store(key, menuCached{plan, time.Now()})
	return plan, true
}

type feastDish struct{ Title, Href, Image, Money string }

type feastCourse struct {
	Title  string
	Dishes []feastDish
}

type feastItem struct{ Name, Qty string }

type feastGroup struct {
	Label, Money string
	Items        []feastItem
}

type feastView struct {
	Key, Tab, Title, Total, PerGuest, Href string
	DishesWord                             string // «блюд», «блюдо», «блюда» под число
	Guests, Dishes, Items, Salads          int
	Courses                                []feastCourse
	Groups                                 []feastGroup
	Checked                                bool
}

// storeCost — стол в одной сети: цена и длина полоски относительно самой дорогой.
type storeCost struct {
	Name, Money string
	Pct         int
	Cheapest    bool
}

type prepStep struct{ Key, When, Text string }

// feastViews — варианты стола и стол по умолчанию (на шестерых) целиком: из него сравнение сетей.
func (s *Server) feastViews(pl pageLocale) ([]feastView, planner.Plan) {
	l := pl.L
	var views []feastView
	var def planner.Plan
	for _, v := range feastVariants {
		plan, ok := s.feastPlan(v, pl)
		if !ok {
			continue
		}
		fv := feastView{Key: v.Key, Guests: v.Guests, Checked: v.Key == "6", Items: plan.Totals.Items,
			Total: formatMoney(pl.Country, plan.Totals.Cost), PerGuest: formatMoney(pl.Country, plan.Totals.Cost/float64(v.Guests)),
			Href: pl.P + "/event/newyear?guests=" + strconv.Itoa(v.Guests)}
		if fv.Checked {
			def = plan
		}
		people := i18n.Plural(l, v.Guests, "feast.people")
		if v.Thrifty {
			fv.Tab, fv.Title = i18n.T(l, "feast.tab.cheap"), i18n.T(l, "feast.table.cheap", v.Guests, people)
		} else {
			fv.Tab, fv.Title = i18n.T(l, "feast.tab.guests", v.Guests), i18n.T(l, "feast.table", v.Guests, people)
		}
		for _, d := range plan.Days[0].Dishes {
			title := i18n.T(l, "course."+d.Course)
			if len(fv.Courses) == 0 || fv.Courses[len(fv.Courses)-1].Title != title {
				fv.Courses = append(fv.Courses, feastCourse{Title: title})
			}
			img := ""
			if rc, ok := s.catalog.RecipeByID[d.RecipeID]; ok {
				img = rc.Image
			}
			c := &fv.Courses[len(fv.Courses)-1]
			c.Dishes = append(c.Dishes, feastDish{Title: d.Title, Href: pl.P + "/recipe/" + d.RecipeID, Image: img, Money: formatMoney(pl.Country, d.Cost)})
			fv.Dishes++
			if d.Course == "salads" {
				fv.Salads++
			}
		}
		for _, g := range plan.Shopping {
			fg := feastGroup{Label: g.Label, Money: formatMoney(pl.Country, g.Cost)}
			for _, it := range g.Items {
				if it.Pantry {
					continue // соль, масло и специи обычно дома, в цене стола их нет
				}
				fg.Items = append(fg.Items, feastItem{Name: it.Name, Qty: formatQty(l, it.Buy, it.Unit)})
			}
			if len(fg.Items) > 0 {
				fv.Groups = append(fv.Groups, fg)
			}
		}
		fv.DishesWord = i18n.Plural(l, fv.Dishes, "feast.dish")
		views = append(views, fv)
	}
	return views, def
}

// feastStores — тот же список покупок стола по ценам сетей страны, от дешёвой к дорогой.
func (s *Server) feastStores(plan planner.Plan, pl pageLocale) []storeCost {
	var items []planner.ShopItem
	for _, g := range plan.Shopping {
		items = append(items, g.Items...)
	}
	stores := s.catalog.StoresOf(pl.Country.Code)
	if len(items) == 0 || len(stores) < 2 {
		return nil
	}
	type sc struct {
		name string
		cost float64
	}
	var list []sc
	maxC := 0.0
	for _, st := range stores {
		c := s.catalog.ListCost(items, pl.Country.Code, st.Code)
		if c <= 0 {
			continue
		}
		list = append(list, sc{st.Name, c})
		maxC = math.Max(maxC, c)
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].cost < list[j].cost })
	out := make([]storeCost, 0, len(list))
	for i, x := range list {
		out = append(out, storeCost{Name: x.name, Money: formatMoney(pl.Country, x.cost), Pct: max(8, int(math.Round(x.cost/maxC*100))), Cheapest: i == 0})
	}
	return out
}

// feastPrep — план на последние дни перед праздником: пункты prep1…N из локали, отметки хранит браузер.
func feastPrep(l i18n.Lang) []prepStep {
	var out []prepStep
	for i := 1; ; i++ {
		k := "feast.prep" + strconv.Itoa(i)
		w := i18n.T(l, k+".when")
		if w == k+".when" {
			break
		}
		out = append(out, prepStep{Key: strconv.Itoa(i), When: w, Text: i18n.T(l, k+".text")})
	}
	return out
}

// newYearYear — какой Новый год встречаем: в январе — только что наступивший, дальше — следующий.
func newYearYear(now time.Time) int {
	if now.Month() == time.January {
		return now.Year()
	}
	return now.Year() + 1
}

// newYearCountdown — «До Нового года 84 дня» с октября по 31 декабря; в остальное время пусто.
func newYearCountdown(l i18n.Lang, now time.Time) string {
	if now.Month() < time.October {
		return ""
	}
	ny := time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := int(ny.Sub(today).Hours() / 24)
	if days <= 1 {
		return i18n.T(l, "feast.countdown.today")
	}
	return i18n.T(l, "feast.countdown", days, i18n.Plural(l, days, "fast.days"))
}
