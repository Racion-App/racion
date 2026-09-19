package planner

import (
	"slices"
	"strings"
	"time"

	"racion/internal/i18n"
)

// Режим заготовок: человек готовит один раз в неделю (в воскресенье перед началом недели) или два раза
// (ещё в среду вечером), а в остальные дни только разогревает. Для этого у каждого рецепта есть срок
// хранения в холодильнике и признак «можно заморозить»; всё, что не дотянет до своего дня, планировщик
// либо берёт из морозилки, либо заменяет блюдом на 15 минут, которое несложно сделать свежим.

const (
	PrepNone = ""    // готовим каждый день
	PrepOne  = "one" // одна заготовка: воскресенье перед неделей
	PrepTwo  = "two" // две: воскресенье и среда вечером
)

// PrepSessions — дни заготовок как индексы недели: -1 — воскресенье до понедельника, 2 — среда.
func PrepSessions(mode string) []int {
	switch mode {
	case PrepOne:
		return []int{-1}
	case PrepTwo:
		return []int{-1, 2}
	}
	return nil
}

// prepSession — ближайшая заготовка до дня d: воскресная кормит с понедельника, вечерняя среды — с четверга.
func prepSession(mode string, d int) int {
	s := -1
	for _, x := range PrepSessions(mode) {
		if x < d {
			s = x
		}
	}
	return s
}

// PrepMode — как блюдо попадает на стол в режиме заготовок.
type PrepInfo struct {
	Session int    `json:"session"` // индекс дня заготовки (-1 — воскресенье до недели)
	Mode    string `json:"mode"`    // fridge | freezer | fresh
}

// Keep — срок хранения готового блюда в холодильнике (дней) и можно ли его заморозить.
// Явные поля рецепта важнее правил; правила — по тегам и названию.
func Keep(r Recipe) (days int, freeze bool) {
	if r.KeepDays > 0 || r.Freeze {
		return r.KeepDays, r.Freeze
	}
	t := strings.ToLower(r.Title)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(t, w) {
				return true
			}
		}
		return false
	}
	tag := func(x string) bool { return slices.Contains(r.Tags, x) }
	switch {
	case has("окрошк", "свекольник холод", "гаспачо", "таратор"):
		return 2, false
	case tag("soup") || has("суп", "борщ", "щи", "уха", "солянк", "рассольник", "харчо", "бульон"):
		return 3, true
	case has("оливье", "под шубой", "винегрет", "крабовый салат", "мимоза"):
		return 2, false
	case tag("nocook") || tag("salad") || has("салат", "тост", "бутерброд", "сэндвич", "смузи", "яичниц", "омлет", "глазунь", "каша", "овсянк", "гречка с молоком", "манн"):
		return 0, false
	case has("гранол", "печенье", "сухар", "крекер"):
		return 7, false
	case has("пельмен", "варени", "мант", "хинкал"):
		return 3, true
	case has("котлет", "тефтел", "фрикадел", "наггетс", "биточк", "голубц", "зраз", "ёжик", "ежик", "митбол", "люля"):
		return 3, true
	case has("сырник", "оладь", "блин", "панкейк", "вафл"):
		return 2, true
	case has("запеканк", "лазань", "пирог", "пицц", "шарлотк", "мусак", "гратен"):
		return 3, true
	case has("плов", "рагу", "гуляш", "тушён", "тушен", "жаркое", "чили", "карри", "дал ", "лобио", "бефстроган", "чечевиц", "фасол", "нут", "перлов", "чахохбил", "азу"):
		return 3, true
	case has("паст", "макарон", "спагетти", "лапш", "удон", "рамен", "ризотто", "ньокк"):
		return 2, false
	case has("креветк", "мидии", "кальмар", "морепрод"):
		return 1, false
	case tag("fish") || has("рыб", "лосос", "форел", "треск", "минтай", "горбуш", "тунец", "скумбри", "судак"):
		return 2, false
	case has("стейк", "шашлык", "гриль"):
		return 2, false
	case has("буженин", "запечён", "запечен", "отбивн", "курица", "курин", "индейк", "утк", "свинин", "говядин", "баранин", "рулет"):
		return 3, false
	case has("пюре", "картоф", "рис", "булгур", "кус-кус", "кускус", "киноа", "гречк"):
		return 3, false
	case tag("dessert") || tag("sweet") || has("чизкейк", "десерт", "тирамису", "панна", "мусс"):
		return 3, false
	}
	return 2, false
}

// quickFresh — блюдо, которое несложно сделать в день еды даже в режиме заготовок.
func quickFresh(r Recipe) bool { return r.TimeMin <= 15 && !slices.Contains(r.Equipment, "oven") }

// prepAllowed — можно ли поставить блюдо на день d в режиме заготовок, и каким способом.
func prepAllowed(mode string, r Recipe, d int) (PrepInfo, bool) {
	if mode == PrepNone {
		return PrepInfo{}, true
	}
	s := prepSession(mode, d)
	keep, freeze := Keep(r)
	dist := d - s
	switch {
	case quickFresh(r):
		// 15 минут проще сделать в день еды, чем хранить
		return PrepInfo{Session: s, Mode: "fresh"}, true
	case keep > 0 && keep >= dist:
		return PrepInfo{Session: s, Mode: "fridge"}, true
	case freeze:
		return PrepInfo{Session: s, Mode: "freezer"}, true
	}
	return PrepInfo{}, false
}

// prepPool — кандидаты на день d в режиме заготовок; без режима — исходный список.
func prepPool(mode string, pool []Recipe, d int) []Recipe {
	if mode == PrepNone {
		return pool
	}
	out := make([]Recipe, 0, len(pool))
	for _, r := range pool {
		if _, ok := prepAllowed(mode, r, d); ok {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return pool
	}
	return out
}

// PrepDay — что готовить в день заготовки и для каких дней.
type PrepDay struct {
	Index    int        `json:"index"` // -1 — воскресенье до недели
	Date     string     `json:"date"`
	Label    string     `json:"label"`
	Items    []PrepItem `json:"items"`
	TotalMin int        `json:"totalMin"`
}

type PrepItem struct {
	RecipeID string  `json:"recipeId"`
	Title    string  `json:"title"`
	TimeMin  int     `json:"timeMin"`
	Mode     string  `json:"mode"`    // fridge | freezer
	ForDays  []int   `json:"forDays"` // индексы дней, когда это едят
	Portions float64 `json:"portions"`
	Side     string  `json:"side,omitempty"` // гарнир к блюду, готовится тогда же
}

// prepDays собирает список заготовок по дням: свежие блюда (fresh) в него не входят, они готовятся в день еды.
func (c *Catalog) prepDays(plan *Plan) []PrepDay {
	mode := plan.Params.Prep
	if mode == PrepNone {
		return nil
	}
	lang := i18n.Lang(plan.Lang)
	start, _ := time.Parse("2006-01-02", plan.Params.StartDate)
	out := []PrepDay{}
	for _, s := range PrepSessions(mode) {
		pd := PrepDay{Index: s, Date: start.AddDate(0, 0, s).Format("2006-01-02"), Items: []PrepItem{}}
		if s < 0 {
			pd.Label = i18n.T(lang, "prep.day.before")
		} else {
			pd.Label = DayLabel(lang, s)
		}
		byRecipe := map[string]int{}
		for _, day := range plan.Days {
			if day.Skipped {
				continue
			}
			for _, dish := range day.Dishes {
				if dish.Prep == nil || dish.Prep.Session != s || dish.Prep.Mode == "fresh" || dish.Leftover {
					continue
				}
				if i, ok := byRecipe[dish.RecipeID]; ok {
					pd.Items[i].ForDays = append(pd.Items[i].ForDays, day.Index)
					pd.Items[i].Portions += plan.SlotPortions[dish.Slot]
					continue
				}
				byRecipe[dish.RecipeID] = len(pd.Items)
				forDays := []int{day.Index}
				if dish.Batch && day.Index+1 < 7 {
					forDays = append(forDays, day.Index+1)
				}
				portions := plan.SlotPortions[dish.Slot]
				if portions == 0 {
					portions = plan.Portions
				}
				if dish.Batch {
					portions *= 2
				}
				it := PrepItem{RecipeID: dish.RecipeID, Title: dish.Title, TimeMin: dish.TimeMin, Mode: dish.Prep.Mode, ForDays: forDays, Portions: portions}
				if dish.Side != nil {
					it.Side = dish.Side.Title
					pd.TotalMin += dish.Side.TimeMin
				}
				pd.Items = append(pd.Items, it)
				pd.TotalMin += dish.TimeMin
			}
		}
		// параллельная готовка: на плите и в духовке идёт сразу несколько блюд, поэтому у плиты не сумма, а около 60 %
		pd.TotalMin = pd.TotalMin * 6 / 10
		out = append(out, pd)
	}
	return out
}
