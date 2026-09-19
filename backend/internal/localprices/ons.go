package localprices

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ONS — Office for National Statistics (Британия). Средние цены по товарам (серии RPI «Ave price»)
// ONS перестал публиковать после января 2025, поэтому берём их как якорь и двигаем по CPI
// группы товара (01.1.1 хлеб и крупы … 01.1.9 прочее), который выходит каждый месяц.
// Для продуктов без якоря — ориентир Германии × курс евро → фунт × тот же CPI-дрейф.
type ONS struct{}

func (ONS) Country() string { return "GB" }
func (ONS) Key() string     { return "price.ons" }

const (
	onsAPI    = "https://www.ons.gov.uk/economy/inflationandpriceindices/timeseries/%s/mm23/data"
	onsAnchor = "2025 JAN" // последний месяц средних цен ONS
)

// onsItems — серия средних цен → продукт; единица серии в граммах/мл/штуках и множитель.
var onsItems = []struct {
	Series     string
	Ingredient string
	PerUnit    float64 // сколько г/мл/шт в единице цены
	Factor     float64
}{
	{"czoh", "bread_white", 800, 1}, {"vkyu", "bread_wholegrain", 800, 1}, {"vkyu", "bread_rye", 800, 1.1},
	{"czoc", "flour", 1500, 1},
	{"czpi", "ground_beef", 1000, 1}, {"czpi", "ground_mixed", 1000, 0.85}, {"jtta", "beef_stew", 1000, 1},
	{"doif", "bacon", 1000, 1}, {"czor", "ham", 113, 1}, {"doln", "pork_neck", 1000, 1}, {"czox", "pork_loin", 1000, 1},
	{"czoq", "sausages", 1000, 1}, {"czpd", "lamb", 1000, 1},
	{"zptx", "salmon_fillet", 1000, 1},
	{"cznt", "milk", 568, 1}, {"cznt", "kefir", 568, 2.2}, {"cznw", "cheese_cheddar", 1000, 1}, {"cznw", "cheese_hard", 1000, 1},
	{"j428", "eggs", 12, 1}, {"kw9b", "butter", 250, 1},
	{"czmy", "apple", 1000, 1}, {"czmx", "pear", 1000, 1}, {"czmv", "banana", 1000, 1}, {"doht", "avocado", 1, 1}, {"czmw", "orange", 1, 5}, // апельсин: за штуку, ≈5 шт в кг
	{"vkyy", "potato", 1000, 1}, {"cznd", "onion", 1000, 1}, {"czne", "carrot", 1000, 1}, {"cznh", "cabbage", 1000, 1},
	{"cznc", "mushrooms", 1000, 1}, {"cznj", "tomato", 1000, 1}, {"cznb", "cucumber", 1, 2.5}, {"czna", "lettuce", 1, 4},
	{"gk8e", "broccoli", 1000, 1}, {"cznn", "sugar", 1000, 1}, {"cznq", "tea", 250, 1},
}

// onsCPI — CPI по группам еды (2015 = 100): категория PLI → серия.
var onsCPI = map[string]string{
	"cereals": "d7d5", "meat": "d7d6", "fish": "d7d7", "dairy": "d7d8", "oils": "d7d9", "produce": "d7db", "other": "d7dd", "food": "d7c8",
}

type onsSeries struct {
	Description struct {
		Title string `json:"title"`
		Unit  string `json:"unit"`
	} `json:"description"`
	Months []struct {
		Date  string `json:"date"`
		Value string `json:"value"`
	} `json:"months"`
}

func fetchONS(ctx context.Context, id string) (*onsSeries, error) {
	data, err := get(ctx, fmt.Sprintf(onsAPI, id), "application/json")
	if err != nil {
		return nil, err
	}
	var s onsSeries
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("ons %s: %w", id, err)
	}
	return &s, nil
}

func (s *onsSeries) at(month string) (float64, bool) {
	for _, m := range s.Months {
		if m.Date == month {
			v, err := strconv.ParseFloat(m.Value, 64)
			return v, err == nil
		}
	}
	return 0, false
}

func (s *onsSeries) last() (string, float64) {
	for i := len(s.Months) - 1; i >= 0; i-- {
		if v, err := strconv.ParseFloat(s.Months[i].Value, 64); err == nil {
			return s.Months[i].Date, v
		}
	}
	return "", 0
}

func (o ONS) Fetch(ctx context.Context, packs map[string]PackInfo) (*Result, error) {
	// CPI-дрейф по группам с января 2025
	drift := map[string]float64{}
	latest := ""
	for cat, id := range onsCPI {
		s, err := fetchONS(ctx, id)
		if err != nil {
			return nil, err
		}
		base, ok := s.at(onsAnchor)
		date, now := s.last()
		if !ok || now <= 0 {
			continue
		}
		drift[cat] = now / base
		if date > latest || latest == "" {
			latest = date
		}
	}
	if drift["food"] == 0 {
		return nil, fmt.Errorf("ons: CPI food index missing")
	}
	period, err := time.Parse("2006 Jan", strings.Title(strings.ToLower(latest)))
	if err != nil {
		period = time.Now().UTC()
	}
	driftOf := func(cat string) float64 {
		if d, ok := drift[cat]; ok && d > 0 {
			return d
		}
		return drift["food"]
	}

	r := &Result{Period: period, Pack: map[string]float64{}}
	// якорные цены ONS: пенсы за единицу на январь 2025
	cache := map[string]*onsSeries{}
	for _, it := range onsItems {
		p, ok := packs[it.Ingredient]
		if !ok {
			continue
		}
		s, ok := cache[it.Series]
		if !ok {
			var err error
			s, err = fetchONS(ctx, it.Series)
			if err != nil {
				return nil, err
			}
			cache[it.Series] = s
		}
		pence, ok := s.at(onsAnchor)
		if !ok || pence <= 0 {
			continue
		}
		_, cat := coicopOf(it.Ingredient, p.Category)
		r.Pack[it.Ingredient] = packPrice(pence/100, it.PerUnit, p, it.Factor) * driftOf(cat)
	}
	// остальное — ориентир Германии в фунтах с тем же дрейфом
	fx, err := loadFX(ctx, "GBP")
	if err != nil {
		return r, nil // якорные цены есть, курса нет — отдаём что есть
	}
	for id, p := range packs {
		if _, done := r.Pack[id]; done {
			continue
		}
		bench, ok := p.Local["DE"]
		if !ok || bench <= 0 {
			continue
		}
		_, cat := coicopOf(id, p.Category)
		r.Pack[id] = bench * fx * driftOf(cat)
	}
	return r, nil
}
