package planner

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"racion/internal/i18n"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Доля дневного бюджета и калорий на приём пищи; нормируется по выбранным приёмам.
var slotShare = map[string]float64{
	"breakfast": 0.25,
	"lunch":     0.35,
	"dinner":    0.30,
	"snack":     0.10,
}

// cart — что уже «в корзине» на момент выбора блюда: нужно для бонуса за доедание.
type cart struct {
	need   map[string]float64 // ингредиент → сколько уже требуется на неделю
	wanted map[string]int     // продукт из «хочется» → сколько раз уже в меню
}

// homeLeft — сколько домашнего запаса продукта ещё не расписано по блюдам (в единицах продукта).
func (c *cart) homeLeft(ing Ingredient, have []string) float64 {
	if !slices.Contains(have, ing.ID) {
		return 0
	}
	return ing.Pack - c.need[ing.ID]
}

func (c *cart) leftover(ing Ingredient) float64 {
	need := c.need[ing.ID]
	if need == 0 {
		return 0
	}
	if ing.Loose {
		return roundUp(need, looseStep(ing)) - need
	}
	packs := math.Ceil(need / ing.Pack)
	return packs*ing.Pack - need
}

func looseStep(ing Ingredient) float64 {
	if ing.Pack <= 200 {
		return 50
	}
	return 100
}

func roundUp(v, step float64) float64 {
	return math.Ceil(v/step-1e-9) * step
}

// Seed делает план воспроизводимым: одинаковые ответы → одинаковая неделя.
func Seed(p Params) int64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%s|%d|%v|%s|%.0f|%s|%.0f|%s|%s|%s|%s|%s|%s|%s",
		p.Store, p.Region, p.Adults, p.Kids, p.Goal, p.KcalTarget, p.BudgetMode, p.BudgetValue,
		strings.Join(sorted(p.Allergens), ","),
		strings.Join(sorted(p.Exclude), ","),
		strings.Join(sorted(p.ExcludeTags), ","),
		strings.Join(sorted(p.Equipment), ","),
		strings.Join(sorted(p.Slots), ","),
		p.StartDate,
		strings.Join(sorted(p.ExcludeRecipes), ","))
	fmt.Fprintf(h, "|%v|%s|%s|%s", p.Members, strings.Join(sorted(p.Wants), ","), strings.Join(sorted(p.Favorites), ","), strings.Join(sorted(p.Have), ","))
	fmt.Fprintf(h, "|%s|%s|%s|%v", strings.Join(sorted(p.Liked), ","), strings.Join(sorted(p.Meh), ","), strings.Join(sorted(p.CollectionIDs), ","), p.Compact)
	return int64(h.Sum64() & math.MaxInt64)
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	sort.Strings(out)
	return out
}

// Normalize приводит параметры к допустимым значениям.
func (c *Catalog) Normalize(p Params) Params {
	if _, ok := i18n.Valid(p.Lang); !ok {
		p.Lang = string(i18n.RU)
	}
	if _, ok := countryBy[p.Country]; !ok {
		p.Country = "RU"
	}
	if st, ok := c.Stores[p.Store]; !ok || st.Country != p.Country {
		p.Store = ""
		if list := c.StoresOf(p.Country); len(list) > 0 {
			p.Store = list[0].Code
		}
	}
	if p.Country != "RU" {
		p.Region = ""
	} else if pb := c.PriceBook(); pb != nil {
		if _, ok := pb.RegionBy[p.Region]; !ok {
			p.Region = "643"
		}
	}
	if p.Adults < 1 {
		p.Adults = 1
	}
	if p.Adults > 8 {
		p.Adults = 8
	}
	if len(p.Kids) > 8 {
		p.Kids = p.Kids[:8]
	}
	for i := range p.Kids {
		k := &p.Kids[i]
		if k.AgeMonths < 0 {
			k.AgeMonths = 0
		}
		if k.AgeMonths > 17*12 {
			k.AgeMonths = 17 * 12
		}
		*k = k.normalized()
	}
	if !slices.Contains(Goals, p.Goal) {
		p.Goal = "none"
	}
	if p.KcalTarget < 0 || p.KcalTarget > 6000 {
		p.KcalTarget = 0
	}
	if p.KcalTarget > 0 && p.KcalTarget < 1000 {
		p.KcalTarget = 1000
	}
	if p.BudgetMode != "week" {
		p.BudgetMode = "perPersonDay"
	}
	if p.Prep != PrepOne && p.Prep != PrepTwo {
		p.Prep = PrepNone
	}
	if p.BudgetValue < 0 || p.BudgetValue > 1_000_000 {
		p.BudgetValue = 0
	}
	slots := make([]string, 0, 4)
	for _, s := range SlotOrder {
		if slices.Contains(p.Slots, s) {
			slots = append(slots, s)
		}
	}
	if len(slots) == 0 {
		slots = []string{"breakfast", "lunch", "dinner"}
	}
	p.Slots = slots
	if len(p.Equipment) == 0 {
		p.Equipment = []string{"stove"}
	}
	p = normalizeMembers(p)
	wants := []string{}
	for _, id := range p.Wants {
		if _, ok := c.Ingredients[id]; ok && !slices.Contains(wants, id) && len(wants) < 10 {
			wants = append(wants, id)
		}
	}
	p.Wants = wants
	have := []string{}
	for _, id := range p.Have {
		if _, ok := c.Ingredients[id]; ok && !slices.Contains(have, id) && len(have) < 40 {
			have = append(have, id)
		}
	}
	p.Have = have
	if p.StartDate == "" {
		p.StartDate = nextMonday(time.Now()).Format("2006-01-02")
	} else if t, err := time.Parse("2006-01-02", p.StartDate); err != nil {
		p.StartDate = nextMonday(time.Now()).Format("2006-01-02")
	} else {
		p.StartDate = t.Format("2006-01-02")
	}
	if p.Allergens == nil {
		p.Allergens = []string{}
	}
	if p.Exclude == nil {
		p.Exclude = []string{}
	}
	if p.ExcludeTags == nil {
		p.ExcludeTags = []string{}
	}
	if p.Kids == nil {
		p.Kids = []Child{}
	}
	return p
}

// NextMonday — ближайший понедельник (сегодня, если понедельник).
func NextMonday(t time.Time) time.Time { return nextMonday(t) }

func nextMonday(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	wd := int(t.Weekday()) // Sunday = 0
	if wd == 1 {
		return t
	}
	delta := (8 - wd) % 7
	if delta == 0 {
		delta = 7
	}
	return t.AddDate(0, 0, delta)
}

// Portions — сколько взрослых порций готовим: взрослые + дети по возрастным долям.
func Portions(p Params) float64 {
	v := float64(p.Adults)
	for _, k := range p.Kids {
		v += k.PortionFactor()
	}
	return math.Round(v*100) / 100
}

func hasEquipment(have []string, need string) bool {
	if slices.Contains(have, need) {
		return true
	}
	for _, alt := range EquipmentAlternatives[need] {
		if slices.Contains(have, alt) {
			return true
		}
	}
	return false
}

// effective — параметры с учётом детских ограничений (жёстко, как просили).
type effective struct {
	Params
	kidExclude []string
	kidTags    []string
	storeLevel int // ассортимент выбранного магазина; без магазина — всё доступно
}

func (c *Catalog) effective(p Params) effective {
	e := effective{Params: p, storeLevel: 3}
	e.kidExclude, e.kidTags, _ = kidsRestrictions(p.Kids)
	if st, ok := c.Stores[p.Store]; ok {
		e.storeLevel = StoreLevel(st.Kind)
	}
	return e
}

// allowed — проходит ли рецепт фильтры пользователя.
func (c *Catalog) allowed(r Recipe, e effective) bool {
	if r.Hidden {
		return false
	}
	for _, eq := range r.Equipment {
		if !hasEquipment(e.Equipment, eq) {
			return false
		}
	}
	if slices.Contains(e.ExcludeRecipes, r.ID) {
		return false
	}
	for _, t := range r.Tags {
		if slices.Contains(e.ExcludeTags, t) || slices.Contains(e.kidTags, t) {
			return false
		}
	}
	for _, ri := range r.Ingredients {
		if slices.Contains(e.Exclude, ri.IngredientID) || slices.Contains(e.kidExclude, ri.IngredientID) {
			return false
		}
		ing, ok := c.Ingredients[ri.IngredientID]
		if !ok {
			return false
		}
		if TierLevel(ing.Tier, e.Country) > e.storeLevel && !slices.Contains(e.Have, ri.IngredientID) {
			return false // в «Пятёрочке» утки нет: не предлагать, пока не выбран магазин побольше
		}
		for _, a := range ing.Allergens {
			if slices.Contains(e.Allergens, a) {
				return false
			}
		}
	}
	return true
}

// Allowed — публичная версия для тестов и API.
func (c *Catalog) Allowed(r Recipe, p Params) bool { return c.allowed(r, c.effective(p)) }

// Nutrition — КБЖУ на одну порцию.
func (c *Catalog) Nutrition(r Recipe) (kcal, prot, fat, carb float64) {
	for _, ri := range r.Ingredients {
		ing := c.Ingredients[ri.IngredientID]
		k := ri.Amount
		if ing.Unit != "pcs" {
			k = ri.Amount / 100
		}
		kcal += ing.Kcal * k
		prot += ing.Protein * k
		fat += ing.Fat * k
		carb += ing.Carb * k
	}
	return
}

// costPerPortion — стоимость порции без «домашних» продуктов, по ценнику плана.
func (c *Catalog) costPerPortion(r Recipe, pr pricer) float64 {
	var sum float64
	for _, ri := range r.Ingredients {
		ing := c.Ingredients[ri.IngredientID]
		if ing.Pantry {
			continue
		}
		sum += ri.Amount * pr.unitPrice(ing)
	}
	return sum
}

func mainIngredient(r Recipe) string {
	if len(r.Ingredients) == 0 {
		return ""
	}
	return r.Ingredients[0].IngredientID
}

type scored struct {
	r       Recipe
	score   float64
	reused  []string
	over    float64 // насколько дороже цели (доля)
	kcal    float64
	wanted  string // продукт из «хочется», за который дан бонус
	fav     bool
	compact bool // почти из тех же продуктов, что уже в списке (режим «меньше разных продуктов»)
}

// targets — цели ячейки: деньги и калории на порцию.
type targets struct {
	cost    float64
	kcal    float64
	goal    string
	economy bool // бюджет на уровне «экономно»: дешевле — лучше
	premium bool // бюджет на уровне «премиум»: дорогие блюда получают бонус
	wants   []string
	favs    []string
	have    []string // что есть дома: блюда с этими продуктами ставим раньше, пока запас не израсходован
	liked   []string // «понравилось» после ужина
	coll    []string // рецепты коллекции, из которой собираем неделю
	meh     []string // «не зашло» один раз: штраф, но не запрет
	compact bool     // «меньше разных продуктов»: блюда из уже выбранных продуктов, новые позиции — штраф
}

// budgetMode — режим по бюджету на человека в день относительно пресетов страны.
func budgetMode(perDay float64, cy Country) (economy, premium bool) {
	return perDay <= cy.Presets[0]*1.1, perDay >= cy.Presets[3]*0.9
}

// pick выбирает лучший рецепт для ячейки.
func (c *Catalog) pick(pool []Recipe, day int, slot string, portions float64, pr pricer,
	tg targets, used map[string]int, dayMains map[int][]string, ct *cart, rng *rand.Rand) (scored, bool) {

	weekday := day < 5
	best := scored{score: math.Inf(1)}
	found := false
	for _, r := range pool {
		s := scored{r: r}
		// повтор блюда на неделе — почти запрет
		if n := used[r.ID]; n > 0 {
			s.score += 6 * float64(n)
		}
		// «хочется на неделе»: блюдо с таким продуктом получает бонус, пока продукт не подан дважды;
		// человек попросил его сам, так что перерасход по бюджету прощается наполовину
		for _, ri := range r.Ingredients {
			if slices.Contains(tg.wants, ri.IngredientID) && ct.wanted[ri.IngredientID] < 2 {
				s.score -= 2.0
				s.wanted = ri.IngredientID
				break
			}
		}
		// «есть дома»: пока домашний запас не расписан, блюдо с ним получает бонус; скоропортящееся — больше,
		// его надо использовать первым
		for _, ri := range r.Ingredients {
			if ing, ok := c.Ingredients[ri.IngredientID]; ok && ct.homeLeft(ing, tg.have) > 0 {
				if ing.Perishable {
					s.score -= 1.2
				} else {
					s.score -= 0.7
				}
				break
			}
		}
		// бюджет
		cost := c.costPerPortion(r, pr)
		if tg.cost > 0 {
			over := (cost - tg.cost) / tg.cost
			s.over = over
			if over > 0 && s.wanted != "" {
				s.score += 1.5 * math.Pow(over, 1.4)
			} else if over > 0 {
				s.score += 3 * math.Pow(over, 1.4)
			} else if tg.economy {
				s.score += 0.3 * over // экономный режим: дешевле — лучше
			} else if tg.premium && over < -0.4 {
				s.score += 1.5 * (-over - 0.4) // щедрый бюджет: сильно дешевле цели — тоже мимо
			}
		}
		// «премиум»: при щедром бюджете дорогие блюда идут чаще, при обычном — реже
		if slices.Contains(r.Tags, "premium") {
			if tg.premium {
				s.score -= 1.0
			} else {
				s.score += 0.4
			}
		}
		// праздничное (торт, кулич, начинки к блинам) и гарниры (запечённая картошка, пюре, соусы): сами по себе
		// это не приём пищи — в обычную неделю не берём, только события, коллекции и «хочется»
		if (slices.Contains(r.Tags, "festive") || slices.Contains(r.Tags, "filling") || slices.Contains(r.Tags, "side") || slices.Contains(r.Tags, "sauce")) && !slices.Contains(tg.coll, r.ID) && s.wanted == "" {
			continue
		}
		// своё блюдо: человек его добавил сам, значит хочет видеть в неделе
		if r.Own {
			s.score -= 2.0
		}
		// избранное: заметный, но не решающий бонус — неделя не должна состоять из одного любимого
		if slices.Contains(tg.favs, r.ID) {
			s.score -= 0.9
			s.fav = true
		}
		// неделя из коллекции: её рецепты идут первыми, остальное добирается по правилам
		if slices.Contains(tg.coll, r.ID) {
			s.score -= 2.5
		}
		// ответы после ужина: семья сказала сама, без анкет
		if slices.Contains(tg.liked, r.ID) {
			s.score -= 0.6
		}
		if slices.Contains(tg.meh, r.ID) {
			s.score += 1.5
		}

		// калории под цель; блюдо без гарнира оцениваем вместе с типичным гарниром
		kcal, _, _, _ := c.Nutrition(r)
		if NeedsSide(r) {
			kcal += sideKcalTypical
		}
		s.kcal = kcal
		if tg.kcal > 0 {
			dev := (kcal - tg.kcal) / tg.kcal
			switch tg.goal {
			case "lose":
				if dev > 0 {
					s.score += 2.5 * math.Pow(dev, 1.3)
				} else {
					s.score += 0.8 * -dev
				}
			case "gain":
				if dev < 0 {
					s.score += 2.0 * math.Pow(-dev, 1.3)
				} else {
					s.score += 0.5 * dev
				}
			default:
				s.score += 1.5 * math.Pow(math.Abs(dev), 1.3)
			}
		}
		// теги под цель
		switch tg.goal {
		case "lose", "healthy":
			if slices.Contains(r.Tags, "pp") {
				s.score -= 0.35
			}
			if slices.Contains(r.Tags, "heavy") {
				s.score += 0.5
			}
			if slices.Contains(r.Tags, "sweet") {
				if tg.goal == "lose" {
					s.score += 0.4
				} else {
					s.score += 0.15
				}
			}
		case "gain":
			if slices.Contains(r.Tags, "protein") || slices.Contains(r.Tags, "hearty") {
				s.score -= 0.3
			}
		}
		// разнообразие по главному ингредиенту
		main := mainIngredient(r)
		if slices.Contains(dayMains[day], main) {
			s.score += 1.0
		}
		if day > 0 && slices.Contains(dayMains[day-1], main) {
			s.score += 0.8
		}
		if day > 1 && slices.Contains(dayMains[day-2], main) {
			s.score += 0.3
		}
		// доедание: уже купленное с остатком — бонус; новая скоропортящаяся упаковка ради малой доли — штраф
		var reuse float64
		for _, ri := range r.Ingredients {
			ing := c.Ingredients[ri.IngredientID]
			if ing.Pantry {
				continue
			}
			needNow := ri.Amount * portions
			if r.Batch && slot == "lunch" {
				needNow *= 2
			}
			left := ct.leftover(ing)
			if left > 0 && left >= needNow*0.8 {
				b := 0.2
				if ing.Perishable {
					b = 0.35
				}
				if left >= needNow*3 && !ing.Pantry {
					b += 0.15 // кочан капусты или тыква куском: остаток большой, доедать его важнее
				}
				reuse -= b
				s.reused = append(s.reused, ing.ID)
			} else if ct.need[ing.ID] == 0 && ing.Perishable && !ing.Loose && needNow < 0.4*ing.Pack {
				reuse += 0.25
			}
		}
		s.score += math.Max(-1.2, math.Min(1.0, reuse))
		// «меньше разных продуктов»: каждый новый продукт в списке — минус, каждый уже лежащий в корзине — плюс;
		// специи и масло не считаются, они и так дома
		if tg.compact {
			newOnes, known := 0, 0
			for _, ri := range r.Ingredients {
				ing, ok := c.Ingredients[ri.IngredientID]
				if !ok || ing.Pantry {
					continue
				}
				if ct.need[ing.ID] > 0 {
					known++
				} else {
					newOnes++
				}
			}
			s.score += 0.3*float64(newOnes) - 0.2*float64(known)
			s.compact = known >= 2 && newOnes <= 1
		}
		// время
		if weekday && slot == "dinner" && r.TimeMin > 45 {
			s.score += 0.4
		}
		if slot == "breakfast" && r.TimeMin > 20 {
			s.score += 0.3
		}
		if weekday && slot == "lunch" && r.TimeMin > 75 {
			s.score += 0.3
		}
		// детерминированный «шум», чтобы неделя не была одинаковой у всех
		s.score += rng.Float64() * 0.35
		if s.score < best.score {
			best = s
			found = true
		}
	}
	return best, found
}

// whyCode — из чего собирается подпись «почему выбрано».
func whyCode(s scored, tg targets) WhyCode {
	wc := WhyCode{Batch: s.r.Batch, Wanted: s.wanted, Favorite: s.fav, Compact: s.compact}
	if len(s.reused) > 0 {
		wc.Reused = s.reused
		if len(wc.Reused) > 2 {
			wc.Reused = wc.Reused[:2]
		}
	}
	if tg.cost > 0 {
		switch {
		case s.over <= -0.25:
			wc.Cost = "cheaper"
		case s.over <= 0.15:
			wc.Cost = "inbudget"
		default:
			wc.Cost = "pricier"
		}
	}
	if tg.kcal > 0 {
		dev := (s.kcal - tg.kcal) / tg.kcal
		switch {
		case math.Abs(dev) <= 0.15:
			wc.Kcal = "kcalok"
		case dev < 0:
			wc.Kcal = "lighter"
		default:
			wc.Kcal = "denser"
		}
	}
	return wc
}

// renderWhy — подпись «почему» на языке.
func (c *Catalog) renderWhy(wc WhyCode, l i18n.Lang) string {
	if wc.Leftover {
		return i18n.T(l, "why.leftover")
	}
	parts := []string{}
	if wc.Compact {
		parts = append(parts, i18n.T(l, "why.compact"))
	}
	if len(wc.Reused) > 0 {
		short := make([]string, 0, len(wc.Reused))
		for _, id := range wc.Reused {
			short = append(short, shortName(c.Ingredients[id].LocalName(l)))
		}
		parts = append(parts, i18n.T(l, "why.eating", strings.ToLower(strings.Join(short, i18n.T(l, "why.and")))))
	}
	if wc.Cost != "" {
		parts = append(parts, i18n.T(l, "why."+wc.Cost))
	}
	if wc.Kcal != "" {
		parts = append(parts, i18n.T(l, "why."+wc.Kcal))
	}
	if wc.Batch {
		parts = append(parts, i18n.T(l, "why.batch"))
	}
	if wc.Wanted != "" {
		parts = append([]string{i18n.T(l, "why.wanted", strings.ToLower(shortName(c.Ingredients[wc.Wanted].LocalName(l))))}, parts...)
	}
	if wc.Favorite {
		parts = append([]string{i18n.T(l, "why.favorite")}, parts...)
	}
	return strings.Join(parts, " · ")
}

func (c *Catalog) why(s scored, tg targets, l i18n.Lang) (string, WhyCode) {
	wc := whyCode(s, tg)
	return c.renderWhy(wc, l), wc
}

// Localize пересобирает подписи плана на другом языке: названия блюд и продуктов, дни, группы,
// «почему», цель, источник цен. Числа не меняются.
func (c *Catalog) Localize(plan Plan, l i18n.Lang) Plan {
	// подписи пересобираем всегда, даже если язык совпадает: в сохранённом плане могли остаться
	// старые названия (перевод рецепта поправили, или план собрали при ошибке фолбэка)
	plan.Lang = string(l)
	plan.Params.Lang = string(l)
	days := make([]Day, len(plan.Days))
	for i, d := range plan.Days {
		d.Label = DayLabel(l, d.Index)
		if plan.Occasion != nil {
			d.Label = i18n.T(l, "occasion."+plan.Occasion.ID+".title")
			plan.Occasion.Title = d.Label
		}
		d.Dishes = slices.Clone(d.Dishes)
		for j := range d.Dishes {
			dish := &d.Dishes[j]
			dish.Title = c.RecipeByID[dish.RecipeID].LocalTitle(l)
			if dish.Side != nil {
				side := *dish.Side
				side.Title = c.RecipeByID[side.RecipeID].LocalTitle(l)
				dish.Side = &side
			}
			dish.Why = c.renderWhy(dish.WhyCode, l)
			if dish.Course != "" {
				dish.Why = i18n.T(l, "course."+dish.Course)
			}
		}
		days[i] = d
	}
	plan.Days = days
	menus := make([]KidMenu, len(plan.KidsMenus))
	for i, km := range plan.KidsMenus {
		if km.Child < len(plan.Params.Kids) {
			km.AgeLabel = plan.Params.Kids[km.Child].AgeLabel(l)
		}
		km.Days = slices.Clone(km.Days)
		for j := range km.Days {
			km.Days[j].Label = DayLabel(l, km.Days[j].Index)
			km.Days[j].Dishes = slices.Clone(km.Days[j].Dishes)
			for k := range km.Days[j].Dishes {
				km.Days[j].Dishes[k].Title = c.RecipeByID[km.Days[j].Dishes[k].RecipeID].LocalTitle(l)
			}
		}
		puree := false
		for _, x := range i18n.Langs {
			if km.Note == i18n.T(x, "kidmenu.note.puree") {
				puree = true
			}
		}
		if puree {
			km.Note = i18n.T(l, "kidmenu.note.puree")
		} else if km.Note != "" {
			km.Note = i18n.T(l, "kidmenu.note.general")
		}
		menus[i] = km
	}
	plan.KidsMenus = menus
	plan.Goal.Label = GoalLabel(l, plan.Goal.Level)
	plan.Store.Note = StoreKindLabel(l, plan.Store.Kind)
	// Список покупок, предупреждения и источник цен проще пересчитать: числа те же, подписи новые.
	c.finish(&plan)
	if plan.Occasion != nil {
		plan.Warnings = occasionWarnings(plan, l)
	} else {
		plan.Warnings = c.warnings(plan.Params, l)
	}
	return plan
}

// Build собирает неделю.
func (c *Catalog) Build(p Params) Plan {
	p = c.Normalize(p)
	seed := Seed(p)
	return c.build(p, seed, 0)
}

func (c *Catalog) budget(p Params, portions float64) Budget {
	cy := CountryOf(p.Country)
	b := Budget{Mode: p.BudgetMode, Value: p.BudgetValue}
	switch {
	case p.BudgetMode == "week" && p.BudgetValue > 0:
		b.TargetWeek = p.BudgetValue
		b.PerDay = cy.RoundMoney(p.BudgetValue / (portions * 7))
	case p.BudgetValue > 0:
		b.PerDay = p.BudgetValue
		b.TargetWeek = cy.RoundMoney(p.BudgetValue * portions * 7)
	default:
		b.PerDay = cy.Default
		b.TargetWeek = cy.RoundMoney(cy.Default * portions * 7)
	}
	return b
}

func goalOf(p Params) Goal {
	g := Goal{Level: p.Goal, Label: GoalLabel(i18n.Lang(p.Lang), p.Goal), KcalTarget: p.KcalTarget}
	if g.KcalTarget == 0 {
		_, g.KcalTarget = memberFactors(p)
	}
	if g.KcalTarget == 0 {
		g.KcalTarget = GoalKcal[p.Goal]
	}
	return g
}

func (c *Catalog) pricerFor(p Params) pricer {
	idx := c.Stores[p.Store].PriceIndex
	if idx <= 0 {
		idx = 1
	}
	return pricer{pb: c.PriceBook(), local: c.LocalPrices(p.Country), region: p.Region, idx: idx, country: CountryOf(p.Country), lang: i18n.Lang(p.Lang)}
}

func (c *Catalog) build(p Params, seed int64, swaps int) Plan {
	rng := rand.New(rand.NewSource(seed + int64(swaps)*7919))
	e := c.effective(p)
	store := c.Stores[p.Store]
	pr := c.pricerFor(p)
	portions := Portions(p)
	perSlot := slotPortions(p)
	budget := c.budget(p, portions)
	goal := goalOf(p)

	var shareSum float64
	for _, s := range p.Slots {
		shareSum += slotShare[s]
	}

	lang := i18n.Lang(p.Lang)
	pools := map[string][]Recipe{}
	for _, s := range p.Slots {
		for _, r := range c.Recipes {
			if r.Slot == s && !isKidRecipe(r) && !IsSide(r) && c.allowed(r, e) {
				pools[s] = append(pools[s], r)
			}
		}
	}
	warnings := c.warnings(p, lang)

	start, _ := time.Parse("2006-01-02", p.StartDate)
	days := make([]Day, 7)
	for d := range days {
		days[d] = Day{Index: d, Date: start.AddDate(0, 0, d).Format("2006-01-02"), Label: DayLabel(lang, d), Dishes: []Dish{}}
	}

	used := map[string]int{}
	dayMains := map[int][]string{}
	ct := &cart{need: map[string]float64{}, wanted: map[string]int{}}
	leftover := map[int]map[string]Recipe{}

	for d := 0; d < 7; d++ {
		for _, slot := range p.Slots {
			if lr, ok := leftover[d][slot]; ok {
				dish := c.dish(lr, slot, pr)
				dish.Leftover = true
				dish.WhyCode = WhyCode{Leftover: true}
				dish.Why = i18n.T(lang, "why.leftover")
				days[d].Dishes = append(days[d].Dishes, dish)
				dayMains[d] = append(dayMains[d], mainIngredient(lr))
				continue
			}
			pool := prepPool(p.Prep, pools[slot], d)
			if len(pool) == 0 {
				continue
			}
			tg := targets{cost: budget.PerDay * slotShare[slot] / shareSum, kcal: goal.KcalTarget * slotShare[slot] / shareSum, goal: goal.Level}
			tg.economy, tg.premium = budgetMode(budget.PerDay, CountryOf(p.Country))
			tg.wants = p.Wants
			tg.favs = p.Favorites
			tg.compact = p.Compact
			tg.liked, tg.meh = p.Liked, p.Meh
			tg.coll = p.CollectionIDs
			// батч только для обедов и только в чётные дни (пн, ср, пт): суп на два дня
			batchDay := slot == "lunch" && d%2 == 0 && d < 6
			s, ok := c.pick(pool, d, slot, perSlot[slot], pr, tg, used, dayMains, ct, rng)
			if !ok {
				continue
			}
			r := s.r
			used[r.ID]++
			dayMains[d] = append(dayMains[d], mainIngredient(r))
			dish := c.dish(r, slot, pr)
			if pi, ok := prepAllowed(p.Prep, r, d); ok && p.Prep != PrepNone {
				dish.Prep = &pi
			}
			mult := 1.0
			if batchDay && r.Batch {
				dish.Batch = true
				mult = 2
				if leftover[d+1] == nil {
					leftover[d+1] = map[string]Recipe{}
				}
				leftover[d+1][slot] = r
				dayMains[d+1] = append(dayMains[d+1], mainIngredient(r))
			}
			sc := s
			if !dish.Batch {
				sc.r.Batch = false
			}
			dish.Why, dish.WhyCode = c.why(sc, tg, lang)
			if s.wanted != "" {
				ct.wanted[s.wanted]++
			}
			if NeedsSide(r) && (slot == "lunch" || slot == "dinner") {
				if side, ok := c.pickSide(r, e, tg.kcal-dish.Kcal, sideRecent(days, d), nil, pr, rng); ok {
					c.attachSide(&dish, side, pr)
					for _, ri := range side.Ingredients {
						ct.need[ri.IngredientID] += ri.Amount * perSlot[slot] * mult
					}
				}
			}
			days[d].Dishes = append(days[d].Dishes, dish)
			for _, ri := range r.Ingredients {
				ct.need[ri.IngredientID] += ri.Amount * perSlot[slot] * mult
			}
		}
	}

	kidsMenus := []KidMenu{}
	for i, k := range p.Kids {
		if k.Feeding == FeedSeparate {
			kidsMenus = append(kidsMenus, c.buildKidMenu(i, k, p, pr, rand.New(rand.NewSource(seed+int64(i)*101+int64(swaps)*7919))))
		}
	}

	store.Note = StoreKindLabel(lang, store.Kind)
	plan := Plan{
		Params:       p,
		Lang:         p.Lang,
		Country:      CountryOf(p.Country),
		Store:        store,
		Portions:     portions,
		SlotPortions: perSlot,
		Members:      memberViews(p, lang),
		Days:         days,
		KidsMenus:    kidsMenus,
		Seed:         seed,
		Swaps:        swaps,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Warnings:     warnings,
		Notes:        []string{},
		Budget:       budget,
		Goal:         goal,
	}
	c.finish(&plan)
	return plan
}

// warnings — предупреждения о тесных фильтрах и детских ограничениях на языке.
func (c *Catalog) warnings(p Params, lang i18n.Lang) []string {
	e := c.effective(p)
	warnings := []string{}
	for _, s := range p.Slots {
		n := 0
		for _, r := range c.Recipes {
			if r.Slot == s && !isKidRecipe(r) && c.allowed(r, e) {
				n++
			}
		}
		if n == 0 {
			warnings = append(warnings, i18n.T(lang, "warn.none", strings.ToLower(SlotLabel(lang, s))))
		} else if n < 7 {
			warnings = append(warnings, i18n.T(lang, "warn.few", SlotLabel(lang, s), n))
		}
	}
	if len(e.kidExclude) > 0 {
		warnings = append(warnings, i18n.T(lang, "warn.kid3"))
	} else if len(e.kidTags) > 0 {
		warnings = append(warnings, i18n.T(lang, "warn.kid7"))
	}
	return warnings
}

func (c *Catalog) dish(r Recipe, slot string, pr pricer) Dish {
	k, prot, f, cb := c.Nutrition(r)
	return Dish{
		Slot:     slot,
		RecipeID: r.ID,
		Title:    r.LocalTitle(pr.lang),
		TimeMin:  r.TimeMin,
		Kcal:     math.Round(k),
		Protein:  math.Round(prot),
		Fat:      math.Round(f),
		Carb:     math.Round(cb),
		Cost:     pr.round(c.costPerPortion(r, pr)),
		Own:      r.Own,
	}
}

// finish пересчитывает итоги дней, список покупок, детские строки и источник цен. Вызывается после Build и Swap.
func (c *Catalog) finish(plan *Plan) {
	p := plan.Params
	pr := c.pricerFor(p)
	lang := pr.lang
	need := map[string]float64{}
	usedIn := map[string][]string{}
	var kcal, prot, fat, carb, usedCost float64
	cookMin := 0
	factors, _ := memberFactors(p)
	eatenDays := 0.0
	for _, d := range plan.Days {
		if !d.Skipped {
			eatenDays++
		}
	}
	if eatenDays == 0 {
		eatenDays = 1
	}
	for d := range plan.Days {
		day := &plan.Days[d]
		day.Kcal, day.Protein, day.Fat, day.Carb, day.Cost, day.CookMin = 0, 0, 0, 0, 0, 0
		day.PerMember = make([]float64, len(p.Members))
		if day.Skipped {
			continue
		}
		for _, dish := range day.Dishes {
			day.Kcal += dish.Kcal
			day.Protein += dish.Protein
			day.Fat += dish.Fat
			day.Carb += dish.Carb
			day.Cost += dish.Cost * plan.portionsFor(dish.Slot)
			for i, m := range p.Members {
				if m.eats(dish.Slot) {
					day.PerMember[i] += math.Round(dish.Kcal * factors[i])
				}
			}
			if dish.Leftover {
				continue
			}
			day.CookMin += dish.TimeMin
			r := c.RecipeByID[dish.RecipeID]
			mult := 1.0
			if dish.Batch {
				mult = 2
			}
			title := r.LocalTitle(lang)
			for _, ri := range r.Ingredients {
				need[ri.IngredientID] += ri.Amount * plan.portionsFor(dish.Slot) * mult
				if !slices.Contains(usedIn[ri.IngredientID], title) {
					usedIn[ri.IngredientID] = append(usedIn[ri.IngredientID], title)
				}
			}
			if dish.Side != nil {
				if sr, ok := c.RecipeByID[dish.Side.RecipeID]; ok {
					st := sr.LocalTitle(lang)
					for _, ri := range sr.Ingredients {
						need[ri.IngredientID] += ri.Amount * plan.portionsFor(dish.Slot) * mult
						if !slices.Contains(usedIn[ri.IngredientID], st) {
							usedIn[ri.IngredientID] = append(usedIn[ri.IngredientID], st)
						}
					}
				}
			}
		}
		day.Cost = pr.round(day.Cost)
		usedCost += day.Cost
		kcal += day.Kcal
		prot += day.Protein
		fat += day.Fat
		carb += day.Carb
		cookMin += day.CookMin
	}

	// Детские меню: продукты идут в общий список, стоимость — отдельной строкой и в «съедите за неделю».
	var kidsMenuCost float64
	for _, km := range plan.KidsMenus {
		for _, d := range km.Days {
			for _, dish := range d.Dishes {
				r, ok := c.RecipeByID[dish.RecipeID]
				if !ok {
					continue
				}
				kidsMenuCost += dish.Cost
				for _, ri := range r.Ingredients {
					need[ri.IngredientID] += ri.Amount * km.Factor
					tag := i18n.T(lang, "kid.tag", r.LocalTitle(lang))
					if !slices.Contains(usedIn[ri.IngredientID], tag) {
						usedIn[ri.IngredientID] = append(usedIn[ri.IngredientID], tag)
					}
				}
			}
		}
	}
	usedCost += kidsMenuCost

	groups := map[string]*ShopGroup{}
	var total, pantryTotal, rosstatCost, pricedCost, babyTotal, homeSaved float64
	items := 0
	add := func(item ShopItem, pantry bool) {
		g, ok := groups[item.Category]
		if !ok {
			g = &ShopGroup{Category: item.Category, Label: CategoryLabel(lang, item.Category)}
			groups[item.Category] = g
		}
		g.Items = append(g.Items, item)
		if pantry {
			pantryTotal += item.Cost
		} else {
			g.Cost += item.Cost
			total += item.Cost
			pricedCost += item.Cost
			if item.Rosstat {
				rosstatCost += item.Cost
			}
		}
		items++
	}
	for id, n := range need {
		ing := c.Ingredients[id]
		packPrice, fromRosstat := pr.packPrice(ing)
		item := ShopItem{
			IngredientID: id, Name: ing.LocalName(lang), Category: ing.Category, Unit: ing.Unit,
			Needed: round1(n), Pack: ing.Pack, Loose: ing.Loose, Pantry: ing.Pantry, Rosstat: fromRosstat, UsedIn: usedIn[id], Image: ing.Image,
		}
		// дома есть одна типовая упаковка: покупаем только остаток
		if slices.Contains(p.Have, id) {
			item.Home = math.Min(ing.Pack, round1(n))
			n -= ing.Pack
		}
		switch {
		case n <= 1e-9:
			item.AtHome = true
			if ing.Loose {
				homeSaved += pr.round(roundUp(item.Needed, looseStep(ing)) * packPrice / ing.Pack)
			} else {
				homeSaved += pr.round(math.Max(1, math.Ceil(item.Needed/ing.Pack-1e-9)) * packPrice)
			}
		case ing.Loose:
			item.Buy = roundUp(n, looseStep(ing))
			item.Cost = pr.round(item.Buy * packPrice / ing.Pack)
			if item.Home > 0 {
				homeSaved += pr.round(roundUp(item.Home, looseStep(ing)) * packPrice / ing.Pack)
			}
		default:
			packs := int(math.Ceil(n/ing.Pack - 1e-9))
			if packs < 1 {
				packs = 1
			}
			item.Packs = packs
			item.Buy = float64(packs) * ing.Pack
			item.Cost = pr.round(float64(packs) * packPrice)
			if item.Home > 0 {
				homeSaved += pr.round(packPrice)
			}
		}
		add(item, ing.Pantry || item.AtHome)
	}
	baby, notes := c.babyItems(p.Kids, pr)
	for _, it := range baby {
		add(it, false)
		babyTotal += it.Cost
	}

	plan.Shopping = plan.Shopping[:0]
	for _, cat := range CategoryOrder {
		if g, ok := groups[cat]; ok {
			sort.Slice(g.Items, func(i, j int) bool { return g.Items[i].Name < g.Items[j].Name })
			g.Cost = pr.round(g.Cost)
			plan.Shopping = append(plan.Shopping, *g)
		}
	}
	plan.PrepDays = c.prepDays(plan)
	plan.Totals = Totals{
		Cost:          pr.round(total),
		PantryCost:    pr.round(pantryTotal),
		BabyCost:      pr.round(babyTotal),
		KidsMenuCost:  pr.round(kidsMenuCost),
		UsedCost:      pr.round(usedCost),
		HomeSaved:     pr.round(homeSaved),
		Items:         items,
		KcalPerDay:    math.Round(kcal / eatenDays),
		ProteinPerDay: math.Round(prot / eatenDays),
		FatPerDay:     math.Round(fat / eatenDays),
		CarbPerDay:    math.Round(carb / eatenDays),
		CookMin:       cookMin,
	}

	// Источник цен
	src := PriceSource{Name: i18n.T(lang, "price.estimate"), RegionCode: p.Region}
	switch {
	case pr.country.Code == "RU" && pr.pb != nil:
		src.Name = i18n.T(lang, "price.rosstat")
		src.Period = pr.pb.PeriodLabel(lang)
		src.WeeklyDate = pr.pb.WeeklyLabel(lang)
		if r, ok := pr.pb.RegionBy[p.Region]; ok {
			src.Region = r.Name
		}
	case pr.country.Code != "RU" && pr.local != nil:
		src.Name = i18n.T(lang, pr.local.Source)
		src.Period = pr.local.PeriodLabel(lang)
		src.Region = i18n.T(lang, "country."+pr.country.Code)
	case pr.country.Code != "RU":
		src.Name = i18n.T(lang, "price.curated")
		src.Region = i18n.T(lang, "country."+pr.country.Code)
	}
	if pricedCost > 0 {
		src.Coverage = math.Round(rosstatCost/pricedCost*100) / 100
	}
	plan.PriceSource = src
	plan.Notes = notes
	if plan.Notes == nil {
		plan.Notes = []string{}
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// Swap заменяет блюдо в ячейке (день, приём). Батч-пара (обед на два дня) меняется целиком.
func (c *Catalog) Swap(plan Plan, day int, slot string) (Plan, error) {
	if day < 0 || day > 6 {
		return plan, fmt.Errorf("day out of range")
	}
	if base, _, ok := strings.Cut(slot, "#"); ok && plan.Occasion != nil {
		if !slices.Contains(plan.Params.Slots, base) {
			return plan, fmt.Errorf("slot not in plan")
		}
	} else if !slices.Contains(plan.Params.Slots, slot) {
		return plan, fmt.Errorf("slot not in plan")
	}
	// План приходит по значению, но слайсы общие — копируем, чтобы не трогать оригинал.
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

	p := c.Normalize(plan.Params)
	plan.Params = p
	e := c.effective(p)
	pr := c.pricerFor(p)
	portions := Portions(p)
	plan.Portions = portions
	plan.SlotPortions = slotPortions(p)
	plan.Members = memberViews(p, pr.lang)
	portions = plan.SlotPortions[slot]
	budget := c.budget(p, Portions(p))
	goal := goalOf(p)
	var shareSum float64
	for _, s := range p.Slots {
		shareSum += slotShare[s]
	}
	tg := targets{cost: budget.PerDay * slotShare[slot] / shareSum, kcal: goal.KcalTarget * slotShare[slot] / shareSum, goal: goal.Level, wants: p.Wants, favs: p.Favorites, have: p.Have, liked: p.Liked, meh: p.Meh, coll: p.CollectionIDs, compact: p.Compact}
	tg.economy, tg.premium = budgetMode(budget.PerDay, CountryOf(p.Country))

	// событие: заменяем в рамках курса (салат на салат), приём один — dinner
	if plan.Occasion != nil {
		return c.swapOccasion(plan, day, slot, pr, tg)
	}
	// найти ячейку и её батч-пару
	di := dishIndex(plan.Days[day], slot)
	if di < 0 {
		return plan, fmt.Errorf("no dish in that cell")
	}
	cur := plan.Days[day].Dishes[di]
	pairDay := -1
	if cur.Batch && day < 6 {
		pairDay = day + 1
	} else if cur.Leftover && day > 0 {
		pairDay = day - 1
		day, pairDay = pairDay, day // источник — предыдущий день
		di = dishIndex(plan.Days[day], slot)
		cur = plan.Days[day].Dishes[di]
	}

	// корзина и использованные рецепты без этой ячейки (и пары)
	used := map[string]int{}
	dayMains := map[int][]string{}
	ct := &cart{need: map[string]float64{}, wanted: map[string]int{}}
	for d, dd := range plan.Days {
		for _, dish := range dd.Dishes {
			if (d == day || d == pairDay) && dish.Slot == slot {
				continue
			}
			r := c.RecipeByID[dish.RecipeID]
			dayMains[d] = append(dayMains[d], mainIngredient(r))
			if dish.Leftover {
				continue
			}
			used[r.ID]++
			mult := 1.0
			if dish.Batch {
				mult = 2
			}
			for _, ri := range r.Ingredients {
				ct.need[ri.IngredientID] += ri.Amount * plan.portionsFor(dish.Slot) * mult
			}
			if dish.Side != nil {
				if sr, ok := c.RecipeByID[dish.Side.RecipeID]; ok {
					for _, ri := range sr.Ingredients {
						ct.need[ri.IngredientID] += ri.Amount * plan.portionsFor(dish.Slot) * mult
					}
				}
			}
			if dish.WhyCode.Wanted != "" {
				ct.wanted[dish.WhyCode.Wanted]++
			}
		}
	}
	// Текущее и все ранее отклонённые в этой ячейке — не предлагаем снова.
	cellKey := fmt.Sprintf("%d:%s", day, slot)
	plan.Rejected[cellKey] = append(plan.Rejected[cellKey], cur.RecipeID)
	for _, id := range plan.Rejected[cellKey] {
		used[id] += 3
	}

	pool := []Recipe{}
	for _, r := range c.Recipes {
		if r.Slot == slot && !isKidRecipe(r) && !IsSide(r) && c.allowed(r, e) {
			pool = append(pool, r)
		}
	}
	pool = prepPool(plan.Params.Prep, pool, day)
	swaps := plan.Swaps + 1
	rng := rand.New(rand.NewSource(plan.Seed + int64(swaps)*7919 + int64(day*10) + int64(len(slot))))
	s, ok := c.pick(pool, day, slot, portions, pr, tg, used, dayMains, ct, rng)
	if !ok {
		return plan, fmt.Errorf("nothing to swap to")
	}
	newDish := c.dish(s.r, slot, pr)
	if pi, ok := prepAllowed(plan.Params.Prep, s.r, day); ok && plan.Params.Prep != PrepNone {
		newDish.Prep = &pi
	}
	batch := pairDay >= 0 && s.r.Batch && slot == "lunch"
	sc := s
	if !batch {
		sc.r.Batch = false
	}
	newDish.Batch = batch
	newDish.Why, newDish.WhyCode = c.why(sc, tg, pr.lang)
	if NeedsSide(s.r) && (slot == "lunch" || slot == "dinner") {
		if side, ok := c.pickSide(s.r, e, tg.kcal-newDish.Kcal, sideRecent(plan.Days, day), nil, pr, rng); ok {
			c.attachSide(&newDish, side, pr)
		}
	}
	plan.Days[day].Dishes[di] = newDish

	if pairDay >= 0 {
		pi := dishIndex(plan.Days[pairDay], slot)
		if batch {
			lo := c.dish(s.r, slot, pr)
			lo.Leftover = true
			lo.WhyCode = WhyCode{Leftover: true}
			lo.Why = i18n.T(pr.lang, "why.leftover")
			plan.Days[pairDay].Dishes[pi] = lo
		} else {
			// пара распалась: второму дню — своё блюдо
			used[s.r.ID]++
			dayMains[day] = append(dayMains[day], mainIngredient(s.r))
			for _, ri := range s.r.Ingredients {
				ct.need[ri.IngredientID] += ri.Amount * portions
			}
			s2, ok2 := c.pick(pool, pairDay, slot, portions, pr, tg, used, dayMains, ct, rng)
			if ok2 {
				d2 := c.dish(s2.r, slot, pr)
				sc2 := s2
				sc2.r.Batch = false
				d2.Why, d2.WhyCode = c.why(sc2, tg, pr.lang)
				plan.Days[pairDay].Dishes[pi] = d2
			}
		}
	}
	plan.Swaps = swaps
	plan.Budget = budget
	plan.Goal = goal
	c.finish(&plan)
	return plan, nil
}

func dishIndex(d Day, slot string) int {
	for i, dish := range d.Dishes {
		if dish.Slot == slot {
			return i
		}
	}
	return -1
}

// shortName убирает жирность и уточнения в скобках: «Молоко 2,5%» → «Молоко», «Свинина (шея)» → «Свинина».
func shortName(n string) string {
	if i := strings.Index(n, " ("); i > 0 {
		n = n[:i]
	}
	fields := strings.Fields(n)
	out := fields[:0]
	for _, f := range fields {
		if strings.HasSuffix(f, "%") {
			continue
		}
		out = append(out, f)
	}
	return strings.Join(out, " ")
}

// swapOccasion — замена блюда события: другой рецепт того же курса, не из уже стоящих на столе.
func (c *Catalog) swapOccasion(plan Plan, day int, slot string, pr pricer, tg targets) (Plan, error) {
	di := dishIndex(plan.Days[day], slot)
	// у события все блюда в одном приёме: конкретное указывается как "dinner#3"
	if base, idx, ok := strings.Cut(slot, "#"); ok {
		slot = base
		if n, err := strconv.Atoi(idx); err == nil && n >= 0 && n < len(plan.Days[day].Dishes) {
			di = n
		}
	}
	if di < 0 {
		return plan, fmt.Errorf("no dish in that cell")
	}
	cur := plan.Days[day].Dishes[di]
	o, ok := OccasionByID(plan.Occasion.ID)
	if !ok {
		return plan, fmt.Errorf("plan.readonly")
	}
	var course OccasionCourse
	for _, cs := range o.Courses {
		if cs.Key == cur.Course {
			course = cs
		}
	}
	onTable := map[string]bool{}
	for _, d := range plan.Days[day].Dishes {
		onTable[d.RecipeID] = true
	}
	key := fmt.Sprintf("%d:%s:%d", day, slot, di)
	rejected := append(slices.Clone(plan.Rejected[key]), cur.RecipeID)
	e := c.effective(plan.Params)
	// сначала рецепты курса (pool), запасные по тегам — только если из pool ничего не осталось;
	// без этого закуску 14 февраля меняло на «детский борщик»: у курса с pool нет правил по тегам, и fits пускал всё
	var pool, backup []Recipe
	for _, r := range c.Recipes {
		if !c.allowed(r, e) || onTable[r.ID] || slices.Contains(rejected, r.ID) {
			continue
		}
		if slices.Contains(course.Pool, r.ID) {
			pool = append(pool, r)
		} else if course.hasRules() && course.fits(r) {
			backup = append(backup, r)
		}
	}
	if len(pool) == 0 {
		pool = backup
	}
	if len(pool) == 0 {
		return plan, fmt.Errorf("plan.noalt")
	}
	rng := rand.New(rand.NewSource(plan.Seed + int64(len(rejected))*7919 + int64(di)))
	r := pool[rng.Intn(len(pool))]
	d := c.dish(r, "dinner", pr)
	d.Course = cur.Course
	d.Why = i18n.T(i18n.Lang(plan.Params.Lang), "course."+cur.Course)
	_ = tg
	plan.Days[day].Dishes[di] = d
	plan.Rejected[key] = rejected
	plan.Swaps++
	c.finish(&plan)
	return plan, nil
}
