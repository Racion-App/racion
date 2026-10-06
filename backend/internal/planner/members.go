package planner

import (
	"math"
	"slices"
	"strings"

	"racion/internal/i18n"
)

// Семья: у каждого взрослого своя цель, аппетит и приёмы пищи дома. Меню одно на всех (продукты, которых
// кто-то не ест, исключаются целиком), а порции разные: коэффициент едока = его ккал / средние ккал семьи.
// Список покупок и цена считаются по сумме коэффициентов тех, кто ест этот приём.

type Member struct {
	Name     string   `json:"name"`
	Goal     string   `json:"goal"`            // lose | healthy | gain | none
	Appetite string   `json:"appetite"`        // small | normal | big
	Slots    []string `json:"slots,omitempty"` // приёмы дома; пусто — все
	// Mom — беременность или кормление грудью: pregnant1 | pregnant2 | pregnant3 | nursing. Прибавка к ккал
	// по МР 2.3.1.0253-21 (табл. 19) и блюда, которых при беременности избегают
	Mom string `json:"mom,omitempty"`
}

// MomStates — беременность по триместрам и кормление грудью.
var MomStates = []string{"pregnant1", "pregnant2", "pregnant3", "nursing"}

// momKcal — прибавка к дневной норме, ккал (МР 2.3.1.0253-21, табл. 19): 1-й триместр — без прибавки,
// 2-й +250, 3-й +350, кормление грудью +500 (в 1–6 мес; в 7–12 мес +450).
var momKcal = map[string]float64{"pregnant2": 250, "pregnant3": 350, "nursing": 500}

// momExclude — чего при беременности избегают (КР «Нормальная беременность», 2023; NHS): печень (витамин A),
// сыры с плесенью и бри, тунец, холодное копчение, алкоголь. Сырое мясо и рыба — тег raw.
var momExclude = []string{"beef_liver", "liver_chicken", "liver_pate", "cod_liver", "cheese_brie", "blue_cheese", "tuna_steak", "tuna_can", "mackerel_smoked", "white_wine", "red_wine", "beer_light"}

// nursingExclude — при кормлении грудью (Программа вскармливания 2019, гл. 2): без алкоголя; сырое — тег raw.
var nursingExclude = []string{"white_wine", "red_wine", "beer_light"}

// MemberView — едок на чеке: подписи, ккал и коэффициент порции.
type MemberView struct {
	Name      string   `json:"name"`
	Goal      string   `json:"goal"`
	GoalLabel string   `json:"goalLabel"`
	Kcal      float64  `json:"kcal"`   // ориентир в день
	Factor    float64  `json:"factor"` // доля от средней порции
	Slots     []string `json:"slots"`
}

// Anonymize стирает имена взрослых и детей. По ссылке план видят чужие люди, а «Маша, похудение,
// 1600 ккал» или имя ребёнка рядом с возрастом в месяцах — уже сведения о конкретном человеке.
// Цели, порции и возраст остаются без имён: они нужны тому, кто пойдёт в магазин по этому списку.
func (p *Plan) Anonymize() {
	for i := range p.Members {
		p.Members[i].Name = ""
	}
	for i := range p.Params.Members {
		p.Params.Members[i].Name = ""
	}
	for i := range p.Params.Kids {
		p.Params.Kids[i].Name = ""
	}
	p.Family = nil
}

var appetiteMult = map[string]float64{"small": 0.85, "normal": 1, "big": 1.2}

const refKcal = 2100 // ориентир для тех, у кого цели нет: только для расчёта долей

// kcalOf — дневной ориентир едока; для цели «без цели» — refKcal.
func (m Member) kcalOf() float64 {
	k := GoalKcal[m.Goal]
	if k == 0 {
		k = refKcal
	}
	return k*appetiteMult[m.Appetite] + momKcal[m.Mom]
}

func (m Member) eats(slot string) bool {
	return len(m.Slots) == 0 || slices.Contains(m.Slots, slot)
}

func (m Member) normalized(slots []string) Member {
	if !slices.Contains(Goals, m.Goal) {
		m.Goal = "none"
	}
	if !slices.Contains(MomStates, m.Mom) {
		m.Mom = ""
	}
	if strings.HasPrefix(m.Mom, "pregnant") && m.Goal == "lose" {
		m.Goal = "healthy" // худеть во время беременности не советуют — меню «правильное питание»
	}
	if _, ok := appetiteMult[m.Appetite]; !ok {
		m.Appetite = "normal"
	}
	if len([]rune(m.Name)) > 40 {
		m.Name = string([]rune(m.Name)[:40])
	}
	own := make([]string, 0, len(m.Slots))
	for _, s := range slots {
		if slices.Contains(m.Slots, s) {
			own = append(own, s)
		}
	}
	if len(own) == len(slots) {
		own = nil // ест всё — храним пусто
	}
	m.Slots = own
	return m
}

// normalizeMembers сводит семью и старые поля (Adults, Goal) к одному виду.
func normalizeMembers(p Params) Params {
	if len(p.Members) > 8 {
		p.Members = p.Members[:8]
	}
	if len(p.Members) == 0 {
		// старый квиз: N одинаковых взрослых с общей целью
		for i := 0; i < p.Adults; i++ {
			p.Members = append(p.Members, Member{Goal: p.Goal, Appetite: "normal"})
		}
	}
	for i := range p.Members {
		p.Members[i] = p.Members[i].normalized(p.Slots)
	}
	p.Adults = len(p.Members)
	// цель меню: все худеют — «похудение»; кто-то худеет или следит — «правильное»; все набирают — «набор»
	goals := map[string]int{}
	for _, m := range p.Members {
		goals[m.Goal]++
	}
	switch {
	case goals["lose"] == len(p.Members):
		p.Goal = "lose"
	case goals["lose"] > 0 || goals["healthy"] > 0:
		p.Goal = "healthy"
	case goals["gain"] == len(p.Members):
		p.Goal = "gain"
	case goals["gain"] > 0:
		p.Goal = "healthy"
	default:
		p.Goal = "none"
	}
	return p
}

// NormalizeFamily чистит состав семьи без каталога: для кабинета и сохранённого состава.
func NormalizeFamily(p Params) Params {
	if len(p.Slots) == 0 {
		p.Slots = SlotOrder
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
		if len([]rune(k.Name)) > 40 {
			k.Name = string([]rune(k.Name)[:40])
		}
		name := k.Name
		*k = k.normalized()
		k.Name = name
	}
	if p.Kids == nil {
		p.Kids = []Child{}
	}
	if len(p.Members) == 0 {
		p.Adults = 0
	}
	p = normalizeMembers(p)
	if p.Members == nil {
		p.Members = []Member{}
	}
	return p
}

// memberFactors — коэффициенты порций и средний ориентир ккал (0, если ни у кого нет цели).
func memberFactors(p Params) (factors []float64, meanKcal float64) {
	if len(p.Members) == 0 {
		return nil, 0
	}
	var sum float64
	withGoal := 0
	var goalSum float64
	for _, m := range p.Members {
		k := m.kcalOf()
		sum += k
		if GoalKcal[m.Goal] > 0 {
			withGoal++
			goalSum += k
		}
	}
	mean := sum / float64(len(p.Members))
	factors = make([]float64, len(p.Members))
	for i, m := range p.Members {
		factors[i] = math.Round(m.kcalOf()/mean*100) / 100
	}
	if withGoal > 0 {
		meanKcal = math.Round(goalSum / float64(withGoal))
	}
	return factors, meanKcal
}

// slotPortions — порции по приёмам: едоки, которые едят этот приём дома, плюс дети по возрастным долям.
func slotPortions(p Params) map[string]float64 {
	factors, _ := memberFactors(p)
	out := map[string]float64{}
	for _, s := range SlotOrder {
		v := 0.0
		for i, m := range p.Members {
			if m.eats(s) {
				v += factors[i]
			}
		}
		for _, k := range p.Kids {
			v += k.SlotPortionFactor(s)
		}
		out[s] = math.Round(v*100) / 100
	}
	return out
}

func memberViews(p Params, l i18n.Lang) []MemberView {
	factors, _ := memberFactors(p)
	out := make([]MemberView, 0, len(p.Members))
	for i, m := range p.Members {
		slots := m.Slots
		if len(slots) == 0 {
			slots = p.Slots
		}
		kcal := 0.0
		if GoalKcal[m.Goal] > 0 {
			kcal = math.Round(m.kcalOf())
		}
		out = append(out, MemberView{Name: m.Name, Goal: m.Goal, GoalLabel: GoalLabel(l, m.Goal), Kcal: kcal, Factor: factors[i], Slots: slots})
	}
	return out
}

// dayPortions — порции приёма в конкретный день: в будни дети в саду или школе едят часть приёмов не дома.
func dayPortions(p Params, perSlot map[string]float64, day int, slot string) float64 {
	v := perSlot[slot]
	for _, k := range p.Kids {
		if k.AwayOn(day, slot) {
			v -= k.SlotPortionFactor(slot)
		}
	}
	return math.Round(math.Max(v, 0)*100) / 100
}

// portionsFor — порций на приём; старые планы без SlotPortions считаются по общему числу.
// portionsForDish — сколько порций у конкретного блюда. В корзине человек задаёт это сам
// (салат на восьмерых, курица на четверых), в остальных планах порции общие для приёма пищи.
func (p Plan) portionsForDish(d Dish) float64 {
	if d.Servings > 0 {
		return float64(d.Servings)
	}
	if d.Portions > 0 {
		return d.Portions
	}
	return p.portionsFor(d.Slot)
}

func (p Plan) portionsFor(slot string) float64 {
	if v, ok := p.SlotPortions[slot]; ok && v > 0 {
		return v
	}
	return p.Portions
}
