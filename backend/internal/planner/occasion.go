package planner

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"
	"time"

	"racion/internal/fasts"
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
	ID        string                      `json:"id"`
	Icon      string                      `json:"icon"`
	Months    []int                       `json:"months"`    // когда актуально (1–12); пусто — всегда
	Countries []string                    `json:"countries"` // где отмечают (коды стран); пусто — везде
	Guests    int                         `json:"guests"`    // по умолчанию
	Kind      string                      `json:"kind"`      // menu (по умолчанию) | week — пресет квиза
	Preset    *OccasionPreset             `json:"preset,omitempty"`
	Extra     []string                    `json:"equipment"` // техника, которую событие добавляет к анкете (мангал на пикнике)
	Courses   []OccasionCourse            `json:"courses"`
	Styles    map[string][]OccasionCourse `json:"styles,omitempty"`
	January   [][]OccasionCourse          `json:"january,omitempty"`
}

type NewYearOptions struct {
	Style     string `json:"style,omitempty"`
	January   [3]int `json:"january,omitempty"`
	Alcohol   bool   `json:"alcohol,omitempty"`
	TableWine bool   `json:"tableWine,omitempty"`
	Cocktail  string `json:"cocktail,omitempty"`
	Glasses   int    `json:"glasses,omitempty"`
}

func (o Occasion) menuCourses(options *NewYearOptions, january int) []OccasionCourse {
	if o.ID != "newyear" {
		return o.Courses
	}
	if january > 0 && january <= len(o.January) {
		return o.January[january-1]
	}
	courses := o.Courses
	if options != nil {
		if themed := o.Styles[options.Style]; len(themed) > 0 {
			courses = themed
		}
		if options.Alcohol {
			courses = append(slices.Clone(courses), OccasionCourse{Key: "cocktails", Base: 1, All: []string{"drink", "alcohol"}})
		} else if options.Glasses > 0 && (options.Cocktail == "ny_kir_royal" || options.Cocktail == "ny_mandarin_bellini") {
			courses = append(slices.Clone(courses), OccasionCourse{Key: "cocktails", Base: 1, Pool: []string{options.Cocktail}})
		}
	}
	return courses
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

// InSeason — событие актуально сейчас. «Постная неделя» — когда пост идёт или начнётся в ближайшие три
// недели (даты постов считает пакет fasts, Великий и Петров — от Пасхи); остальные — по месяцам.
func (o Occasion) InSeason(now time.Time) bool {
	if o.ID == "lent" {
		return fasts.Soon(now, 21)
	}
	return slices.Contains(o.Months, int(now.Month()))
}

// Occasions — события в порядке актуальности: сначала те, чей сезон сейчас или в ближайшие два месяца.
func Occasions(now time.Time) []Occasion {
	m := int(now.Month())
	score := func(o Occasion) int {
		if o.ID == "lent" {
			if o.InSeason(now) {
				return 0
			}
			return 2
		}
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
	if o.ID == "newyear" {
		return c.BuildOccasionWithSeed(o, p, guests, int64(guests))
	}
	return c.BuildOccasionWithSeed(o, p, guests, time.Now().UnixNano())
}

// BuildOccasionWithSeed keeps the published menu reproducible when a recipient opens a shared link.
func (c *Catalog) BuildOccasionWithSeed(o Occasion, p Params, guests int, seed int64) (Plan, error) {
	if guests < 1 {
		guests = o.Guests
	}
	if guests > 40 {
		guests = 40
	}
	if o.ID == "newyear" {
		household := c.effective(p)
		p.Exclude = append(slices.Clone(p.Exclude), household.kidExclude...)
		p.Exclude = append(p.Exclude, household.momExclude...)
		p.ExcludeTags = append(slices.Clone(p.ExcludeTags), household.kidTags...)
		if household.momRaw {
			p.ExcludeTags = append(p.ExcludeTags, "raw", "alcohol")
		}
		p.Allergens = append(slices.Clone(p.Allergens), household.kidAllergens...)
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
	rng := rand.New(rand.NewSource(seed))
	dates := []time.Time{}
	covers := []int{guests}
	january := []int{0}
	if o.ID == "newyear" {
		now := time.Now()
		year := now.Year()
		if now.Month() == time.January && now.Day() <= 3 {
			year--
		}
		start := time.Date(year, time.December, 31, 12, 0, 0, 0, time.UTC)
		dates = append(dates, start)
		p.StartDate = start.Format("2006-01-02")
		if p.NewYear != nil {
			for i, n := range p.NewYear.January {
				if n < 0 || n > 40 {
					return Plan{}, fmt.Errorf("january guests out of range")
				}
				if n == 0 {
					continue
				}
				covers = append(covers, n)
				january = append(january, i+1)
				dates = append(dates, start.AddDate(0, 0, i+1))
			}
		}
	}
	var days []Day
	guestVisits := 0
	for i, n := range covers {
		day := Day{Index: i, Date: p.StartDate, Label: i18n.T(lang, "occasion."+o.ID+".title")}
		if len(dates) > 0 {
			day.Date = dates[i].Format("2006-01-02")
		}
		if i > 0 {
			day.Label = i18n.T(lang, "feast.january.day", january[i], n)
		}
		day = c.fillOccasionDay(day, o, o.menuCourses(p.NewYear, january[i]), p, n, e, pr, rng)
		if len(day.Dishes) == 0 {
			return Plan{}, fmt.Errorf("occasion: no dishes")
		}
		days = append(days, day)
		guestVisits += n
	}
	plan := Plan{
		Params: p, Lang: p.Lang, Country: CountryOf(p.Country), Store: c.Stores[p.Store],
		Portions: float64(guests), SlotPortions: map[string]float64{"dinner": float64(guests)},
		Days: days,
		Goal: Goal{Level: "none", Label: GoalLabel(lang, "none")},
		Seed: seed, Rejected: map[string][]string{}, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Notes: []string{}, Warnings: []string{},
		Occasion: &OccasionInfo{ID: o.ID, Title: i18n.T(lang, "occasion."+o.ID+".title"), Guests: guests},
	}
	c.finish(&plan)
	plan.Warnings = occasionWarnings(plan, lang) // недельные предупреждения («ужин: 6 вариантов») к столу не относятся
	plan.Budget = Budget{Mode: "week", Value: plan.Totals.Cost, PerDay: plan.Totals.Cost / float64(guestVisits), TargetWeek: plan.Totals.Cost}
	return plan, nil
}

// Volume of one finished serving, in ml, for the published New Year drinks.
var holidayGlassML = map[string]int{"ny_kir_royal": 120, "ny_mandarin_bellini": 150, "ny_pomegranate_spritz": 200, "ny_pear_punch": 250, "ny_mandarin_lemonade": 250, "tg_apple_cider_hot": 250, "ny_mulled_wine_nonalco": 200, "vd_berry_mojito_nonalco": 350, "tg_cranberry_lemonade": 250}

func (c *Catalog) GlassML(r Recipe) int {
	if ml := holidayGlassML[r.ID]; ml > 0 {
		return ml
	}
	if !slices.Contains(r.Tags, "bar") {
		return 0
	}
	// Estimate the undiluted volume. Pourable yogurt in lassis is sold by weight.
	var ml float64
	for _, item := range r.Ingredients {
		if c.Ingredients[item.IngredientID].Unit == "ml" || item.IngredientID == "yogurt_plain" {
			ml += item.Amount
		}
	}
	return int(math.Round(ml))
}

// NewYearTableExtras are groceries served as bought on December 31.
func (c *Catalog) NewYearTableExtras(plan *Plan) []RecipeIngredient {
	if plan.Occasion == nil || plan.Occasion.ID != "newyear" || plan.Params.NewYear == nil || len(plan.Days) == 0 || plan.Days[0].Skipped {
		return nil
	}
	guests := plan.Occasion.Guests
	items := []RecipeIngredient{{"mandarin", float64(guests * 200)}}
	if plan.Params.NewYear.TableWine {
		items = append(items, RecipeIngredient{"sparkling_wine", float64((guests+5)/6) * 750}, RecipeIngredient{"red_wine", float64((guests+7)/8) * 750})
	}
	e := c.effective(plan.Params)
	e.adultCocktail = true
	return slices.DeleteFunc(items, func(item RecipeIngredient) bool {
		r := Recipe{Ingredients: []RecipeIngredient{item}}
		if item.IngredientID != "mandarin" {
			r.Tags = []string{"alcohol"}
		}
		return !c.allowed(r, e)
	})
}

func (c *Catalog) fillOccasionDay(day Day, o Occasion, courses []OccasionCourse, p Params, guests int, e effective, pr pricer, rng *rand.Rand) Day {
	lang := i18n.Lang(p.Lang)
	usedMain := map[string]bool{}
	for _, course := range courses {
		e.adultCocktail = course.Key == "cocktails"
		e.bar = e.adultCocktail || course.Key == "drinks"
		start := len(day.Dishes)
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
			slices.SortFunc(list, func(a, b Recipe) int { return strings.Compare(a.ID, b.ID) })
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
		// недорогой стол: в каждом курсе сначала то, что дешевле на порцию; фото уже не главное
		if p.Thrifty {
			cheap := func(list []Recipe) {
				slices.SortStableFunc(list, func(a, b Recipe) int {
					return int(math.Round(c.costPerPortion(a, pr) - c.costPerPortion(b, pr)))
				})
			}
			cheap(primary)
			cheap(backup)
		}
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
				if premium >= 1 || p.Thrifty {
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
		if o.ID == "newyear" {
			dishes := day.Dishes[start:]
			for i := range dishes {
				d := &dishes[i]
				// Split one course among the dishes being served, rounding up to whole recipe portions.
				d.Servings = (guests + len(dishes) - 1) / len(dishes)
				if course.Key == "salads" && day.Index == 0 {
					d.Servings = max(d.Servings, (guests+1)/2)
				}
				if course.Key == "cocktails" && !p.NewYear.Alcohol {
					d.Servings = min(guests, max(1, p.NewYear.Glasses))
				}
				d.DrinkML = c.GlassML(c.RecipeByID[d.RecipeID]) * d.Servings
			}
		}
	}
	return day
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
	for _, course := range o.menuCourses(plan.Params.NewYear, 0) {
		if !slices.ContainsFunc(plan.Days[0].Dishes, func(d Dish) bool { return d.Course == course.Key }) {
			empty = append(empty, i18n.T(l, "course."+course.Key))
		}
	}
	if len(empty) > 0 {
		out = append(out, i18n.T(l, "warn.course", strings.Join(empty, "», «")))
	}
	return out
}
