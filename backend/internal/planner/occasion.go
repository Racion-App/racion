package planner

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"
	"time"

	"racion/internal/i18n"
)

// Событие вместо недели: «Новый год на 8 человек», «День рождения ребёнка», «Пикник». Меню — набор блюд
// по курсам (салаты, закуски, горячее, десерт), порции — на гостей, список покупок — тот же finish.
// Описания событий лежат в seed/data/occasions.json (SetOccasions при старте).

type OccasionCourse struct {
	Key   string   `json:"key"`   // salads | starters | mains | desserts | bliny …
	Base  int      `json:"base"`  // сколько блюд при любом числе гостей
	Per   int      `json:"per"`   // плюс одно блюдо на каждые Per гостей
	Any   []string `json:"any"`   // хотя бы один из тегов
	All   []string `json:"all"`   // все теги
	None  []string `json:"none"`  // ни одного из тегов
	IDs   []string `json:"ids"`   // префиксы id рецептов (bliny…)
	Pool  []string `json:"pool"`  // рецепты курса по id: из них собирается стол; теги — запасной вариант
	Must  []string `json:"must"`  // то, без чего стола не бывает (оливье на Новый год): берутся первыми
	Slots []string `json:"slots"` // приёмы пищи, из которых брать (пусто — любой)
}

type Occasion struct {
	ID        string           `json:"id"`
	Icon      string           `json:"icon"`
	Months    []int            `json:"months"`    // когда актуально (1–12); пусто — всегда
	Countries []string         `json:"countries"` // где отмечают (коды стран); пусто — везде
	Guests    int              `json:"guests"`    // по умолчанию
	Kind      string           `json:"kind"`      // menu (по умолчанию) | week — пресет квиза
	Preset    *OccasionPreset  `json:"preset,omitempty"`
	Extra     []string         `json:"equipment"` // техника, которую событие добавляет к анкете (мангал на пикнике)
	Courses   []OccasionCourse `json:"courses"`
}

// OccasionPreset — для событий-недель (Пост): что подставить в квиз.
type OccasionPreset struct {
	ExcludeTags []string `json:"excludeTags"`
	Allergens   []string `json:"allergens"`
	Goal        string   `json:"goal"`
}

// OccasionInfo — событие в плане: чтобы чек показывал «Новый год на 8 человек» и курсы вместо приёмов.
type OccasionInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Guests int    `json:"guests"`
}

var occasions []Occasion

func SetOccasions(list []Occasion) { occasions = list }

// Occasions — события в порядке актуальности: сначала те, чей сезон сейчас или в ближайшие два месяца.
func Occasions(now time.Time) []Occasion {
	m := int(now.Month())
	score := func(o Occasion) int {
		if len(o.Months) == 0 {
			return 1
		}
		for _, om := range o.Months {
			if om == m {
				return 0
			}
			if om == m%12+1 || om == (m+1)%12+1 {
				return 1
			}
		}
		return 2
	}
	out := slices.Clone(occasions)
	slices.SortStableFunc(out, func(a, b Occasion) int { return score(a) - score(b) })
	return out
}

func OccasionByID(id string) (Occasion, bool) {
	for _, o := range occasions {
		if o.ID == id {
			return o, true
		}
	}
	return Occasion{}, false
}

func (oc OccasionCourse) count(guests int) int {
	n := oc.Base
	if oc.Per > 0 {
		n += guests / oc.Per
	}
	return max(1, n)
}

// hasRules — есть ли у курса правила по тегам (иначе только pool).
func (oc OccasionCourse) hasRules() bool {
	return len(oc.IDs) > 0 || len(oc.Any) > 0 || len(oc.All) > 0
}

func (oc OccasionCourse) fits(r Recipe) bool {
	if len(oc.IDs) > 0 {
		ok := false
		for _, p := range oc.IDs {
			if strings.HasPrefix(r.ID, p) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	if len(oc.Slots) > 0 && !slices.Contains(oc.Slots, r.Slot) {
		return false
	}
	for _, t := range oc.All {
		if !slices.Contains(r.Tags, t) {
			return false
		}
	}
	for _, t := range oc.None {
		if slices.Contains(r.Tags, t) {
			return false
		}
	}
	if len(oc.Any) > 0 && !slices.ContainsFunc(oc.Any, func(t string) bool { return slices.Contains(r.Tags, t) }) {
		return false
	}
	return true
}

// BuildOccasion — меню события: по каждому курсу count блюд, разных по главному продукту, с фото — чаще.
func (c *Catalog) BuildOccasion(o Occasion, p Params, guests int) (Plan, error) {
	if guests < 1 {
		guests = o.Guests
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
	for _, eq := range o.Extra {
		if !slices.Contains(p.Equipment, eq) {
			p.Equipment = append(p.Equipment, eq)
		}
	}
	p.BudgetValue = 0
	p = c.Normalize(p)
	lang := i18n.Lang(p.Lang)
	e := c.effective(p)
	pr := c.pricerFor(p)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	seed := rng.Int63()
	day := Day{Index: 0, Date: p.StartDate, Label: i18n.T(lang, "occasion."+o.ID+".title"), Dishes: []Dish{}}
	usedMain := map[string]bool{}
	for _, course := range o.Courses {
		// сначала рецепты курса (pool), затем — подходящие по тегам, если из pool не набралось
		var primary, backup []Recipe
		for _, r := range c.Recipes {
			if !c.allowed(r, e) {
				continue
			}
			if slices.Contains(course.Pool, r.ID) {
				primary = append(primary, r)
			} else if course.hasRules() && course.fits(r) {
				backup = append(backup, r)
			}
		}
		byPhoto := func(list []Recipe) {
			rng.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
			// с фото — вперёд, чтобы праздничный стол выглядел как стол
			slices.SortStableFunc(list, func(a, b Recipe) int {
				if (a.Image != "") == (b.Image != "") {
					return 0
				}
				if a.Image != "" {
					return -1
				}
				return 1
			})
		}
		byPhoto(primary)
		byPhoto(backup)
		// обязательные — вперёд в заданном порядке
		slices.SortStableFunc(primary, func(a, b Recipe) int {
			ia, ib := slices.Index(course.Must, a.ID), slices.Index(course.Must, b.ID)
			if ia < 0 {
				ia = len(course.Must)
			}
			if ib < 0 {
				ib = len(course.Must)
			}
			return ia - ib
		})
		pool := append(primary, backup...)
		n := course.count(guests)
		picked := 0
		premium := 0
		for _, r := range pool {
			if picked >= n {
				break
			}
			must := slices.Contains(course.Must, r.ID)
			mi := mainIngredient(r)
			if usedMain[mi] && picked < len(pool)-n && !must {
				continue
			}
			// дорогих блюд (буррата, икра, спаржа) — не больше одного на курс, иначе стол на 8 гостей уходит за 30 тысяч
			if slices.Contains(r.Tags, "premium") && !must {
				if premium >= 1 {
					continue
				}
				premium++
			}
			usedMain[mi] = true
			d := c.dish(r, "dinner", pr)
			d.Course = course.Key
			d.Why = i18n.T(lang, "course."+course.Key)
			day.Dishes = append(day.Dishes, d)
			picked++
		}
	}
	if len(day.Dishes) == 0 {
		return Plan{}, fmt.Errorf("occasion: no dishes")
	}
	plan := Plan{
		Params: p, Lang: p.Lang, Country: CountryOf(p.Country), Store: c.Stores[p.Store],
		Portions: float64(guests), SlotPortions: map[string]float64{"dinner": float64(guests)},
		Days: []Day{day},
		Goal: Goal{Level: "none", Label: GoalLabel(lang, "none")},
		Seed: seed, Rejected: map[string][]string{}, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Notes: []string{}, Warnings: []string{},
		Occasion: &OccasionInfo{ID: o.ID, Title: i18n.T(lang, "occasion."+o.ID+".title"), Guests: guests},
	}
	c.finish(&plan)
	plan.Warnings = occasionWarnings(plan, lang) // недельные предупреждения («ужин: 6 вариантов») к столу не относятся
	plan.Budget = Budget{Mode: "week", Value: plan.Totals.Cost, PerDay: plan.Totals.Cost / float64(guests), TargetWeek: plan.Totals.Cost}
	plan.Totals.KcalPerDay = math.Round(plan.Days[0].Kcal)
	return plan, nil
}

// occasionWarnings — курсы стола, в которые не попало ни одного блюда (аллергии, стоп-продукты, техника).
func occasionWarnings(plan Plan, l i18n.Lang) []string {
	out := []string{}
	if plan.Occasion == nil || len(plan.Days) == 0 {
		return out
	}
	o, ok := OccasionByID(plan.Occasion.ID)
	if !ok {
		return out
	}
	var empty []string
	for _, course := range o.Courses {
		if !slices.ContainsFunc(plan.Days[0].Dishes, func(d Dish) bool { return d.Course == course.Key }) {
			empty = append(empty, i18n.T(l, "course."+course.Key))
		}
	}
	if len(empty) > 0 {
		out = append(out, i18n.T(l, "warn.course", strings.Join(empty, "», «")))
	}
	return out
}
