package localprices

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Eurostat — цены для стран Европы. Официальных средних цен по товарам Eurostat не публикует,
// зато публикует три вещи, из которых цена собирается честно и обновляется каждый месяц:
//   - HICP по группам продуктов (prc_hicp_minr, ECOICOP v2, 2015 = 100) — как менялась цена яиц,
//     сыра, птицы, овощей в этой стране с базового месяца;
//   - индексы уровня цен по категориям еды (prc_ppp_ind, PLI, ЕС-27 = 100) — насколько мясо
//     или молочное в стране дороже, чем в Германии;
//   - месячные курсы к евро (ert_bil_eur_m) — для злотых, крон, франков, фунтов.
//
// Цена = ориентир Германии в евро (ingredients_i18n.json, уровень декабря 2025)
//
//	× PLI(страна, категория) / PLI(Германия, категория)
//	× HICP(страна, группа, последний месяц) / HICP(страна, группа, декабрь 2025)
//	× курс евро → валюта страны.
type Eurostat struct {
	Code string // код страны в приложении
	Geo  string // geo-код Eurostat
	Cur  string // валюта; пусто или EUR — без пересчёта курса
}

func (e Eurostat) Country() string { return e.Code }
func (Eurostat) Key() string       { return "price.eurostat" }

const (
	eurostatAPI  = "https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data/"
	eurostatBase = "2025-12" // месяц, к которому привязан ориентир Германии
)

// pliCats — категории индекса уровня цен по группам HICP.
var pliCats = map[string]string{
	"cereals": "A01010101", "meat": "A01010102", "fish": "A01010103", "dairy": "A01010104",
	"oils": "A01010105", "produce": "A01010106", "other": "A01010199",
}

// coicopOf — группа HICP и категория PLI для продукта. Сначала точечные исключения, потом по категории базы.
func coicopOf(id, category string) (coicop, pli string) {
	if c, ok := coicopByID[id]; ok {
		return c[0], c[1]
	}
	switch category {
	case "grains":
		return "CP01111", "cereals"
	case "bakery":
		return "CP01113", "cereals"
	case "meat":
		return "CP01122", "meat"
	case "fish":
		return "CP01131", "fish"
	case "eggs":
		return "CP01148", "dairy"
	case "dairy":
		return "CP0114", "dairy"
	case "vegetables":
		return "CP0117", "produce"
	case "fruits":
		return "CP0116", "produce"
	case "frozen":
		return "CP01178", "produce"
	case "canned":
		return "CP01179", "produce"
	case "nuts":
		return "CP01168", "produce"
	case "spices":
		return "CP01194", "other"
	default:
		return "CP0119", "other"
	}
}

var coicopByID = map[string][2]string{
	"flour": {"CP01112", "cereals"}, "semolina": {"CP01112", "cereals"}, "cornmeal": {"CP01112", "cereals"},
	"pasta": {"CP01115", "cereals"}, "rice_noodles": {"CP01115", "cereals"}, "buckwheat_noodles": {"CP01115", "cereals"}, "funchoza": {"CP01115", "cereals"}, "couscous": {"CP01115", "cereals"},
	"oats": {"CP01114", "cereals"}, "oat_bran": {"CP01114", "cereals"}, "corn_flakes": {"CP01114", "cereals"},
	"bread_white": {"CP011131", "cereals"}, "bread_rye": {"CP011131", "cereals"}, "bread_wholegrain": {"CP011131", "cereals"},
	"lavash": {"CP011139", "cereals"}, "tortilla": {"CP011139", "cereals"}, "crispbread": {"CP011139", "cereals"}, "breadcrumbs": {"CP011139", "cereals"},
	"ground_beef": {"CP011221", "meat"}, "beef_stew": {"CP011221", "meat"}, "ground_mixed": {"CP011222", "meat"},
	"pork_neck": {"CP011222", "meat"}, "pork_loin": {"CP011222", "meat"}, "lamb": {"CP011223", "meat"},
	"chicken_breast": {"CP011224", "meat"}, "chicken_thigh": {"CP011224", "meat"}, "chicken_drumstick": {"CP011224", "meat"}, "chicken_whole": {"CP011224", "meat"},
	"chicken_wings": {"CP011224", "meat"}, "turkey_fillet": {"CP011224", "meat"}, "ground_chicken": {"CP011224", "meat"}, "turkey_ground": {"CP011224", "meat"},
	"bacon": {"CP01123", "meat"}, "ham": {"CP01123", "meat"}, "sausage_boiled": {"CP01125", "meat"}, "sausages": {"CP01125", "meat"}, "dumplings": {"CP01125", "meat"},
	"liver_chicken": {"CP01124", "meat"},
	"ribeye":        {"CP011221", "meat"}, "beef_tenderloin": {"CP011221", "meat"}, "beef_ribs": {"CP011221", "meat"}, "veal": {"CP011221", "meat"},
	"pork_ribs": {"CP011222", "meat"}, "lamb_rack": {"CP011223", "meat"}, "duck_breast": {"CP011224", "meat"}, "prosciutto": {"CP01123", "meat"}, "pancetta": {"CP01123", "meat"},
	"oysters": {"CP01134", "fish"}, "scallops": {"CP01134", "fish"}, "king_prawns": {"CP01134", "fish"}, "mussels": {"CP01134", "fish"}, "salmon_steak": {"CP01131", "fish"}, "red_caviar": {"CP01137", "fish"},
	"parmesan": {"CP01145", "dairy"}, "burrata": {"CP01145", "dairy"}, "blue_cheese": {"CP01145", "dairy"}, "heavy_cream": {"CP01143", "dairy"},
	"asparagus": {"CP01171", "produce"}, "cherry_tomatoes": {"CP01172", "produce"}, "grapes": {"CP01165", "produce"}, "rosemary": {"CP01171", "produce"}, "shallot": {"CP01174", "produce"},
	"arborio": {"CP01111", "cereals"}, "white_wine": {"CP0212", "other"}, "red_wine": {"CP0212", "other"}, "truffle_oil": {"CP01194", "other"}, "capers": {"CP01179", "produce"},
	"herring": {"CP01132", "fish"}, "tuna_can": {"CP01133", "fish"}, "saury_can": {"CP01133", "fish"}, "shrimp": {"CP01134", "fish"}, "squid": {"CP01134", "fish"}, "crab_sticks": {"CP01136", "fish"},
	"milk": {"CP01141", "dairy"}, "kefir": {"CP01143", "dairy"}, "ryazhenka": {"CP01143", "dairy"}, "cream": {"CP01143", "dairy"}, "condensed_milk": {"CP01143", "dairy"},
	"cottage_cheese": {"CP01145", "dairy"}, "cottage_cheese_9": {"CP01145", "dairy"}, "cheese_hard": {"CP01145", "dairy"}, "cheese_feta": {"CP01145", "dairy"}, "cream_cheese": {"CP01145", "dairy"},
	"mozzarella": {"CP01145", "dairy"}, "cheese_suluguni": {"CP01145", "dairy"}, "cheese_cheddar": {"CP01145", "dairy"},
	"yogurt_plain": {"CP01146", "dairy"}, "sour_cream": {"CP01143", "dairy"}, "sour_cream_20": {"CP01143", "dairy"}, "butter": {"CP01152", "oils"}, "tofu": {"CP01144", "other"},
	"sunflower_oil": {"CP01151", "oils"}, "olive_oil": {"CP011513", "oils"},
	"banana": {"CP01161", "produce"}, "avocado": {"CP01161", "produce"}, "orange": {"CP01162", "produce"}, "lemon": {"CP01162", "produce"},
	"apple": {"CP01163", "produce"}, "pear": {"CP01163", "produce"}, "berries_frozen": {"CP01166", "produce"},
	"raisins": {"CP01167", "produce"}, "dried_apricots": {"CP01167", "produce"}, "dates": {"CP01167", "produce"},
	"walnuts": {"CP01168", "produce"}, "peanuts": {"CP01168", "produce"}, "sunflower_seeds": {"CP01168", "produce"}, "sesame": {"CP01168", "produce"}, "coconut_flakes": {"CP01169", "produce"},
	"lettuce": {"CP01171", "produce"}, "dill": {"CP01171", "produce"}, "parsley": {"CP01171", "produce"}, "green_onion": {"CP01171", "produce"}, "cilantro": {"CP01171", "produce"},
	"basil": {"CP01171", "produce"}, "mint": {"CP01171", "produce"}, "arugula": {"CP01171", "produce"}, "celery": {"CP01171", "produce"}, "cabbage_peking": {"CP01171", "produce"},
	"tomato": {"CP01172", "produce"}, "cucumber": {"CP01172", "produce"}, "bell_pepper": {"CP01172", "produce"}, "zucchini": {"CP01172", "produce"}, "eggplant": {"CP01172", "produce"}, "pumpkin": {"CP01172", "produce"},
	"onion": {"CP01174", "produce"}, "carrot": {"CP01174", "produce"}, "beet": {"CP01174", "produce"}, "cabbage": {"CP01174", "produce"}, "garlic": {"CP01174", "produce"},
	"mushrooms": {"CP01174", "produce"}, "radish": {"CP01174", "produce"}, "ginger": {"CP01174", "produce"},
	"potato": {"CP01175", "produce"}, "sweet_potato": {"CP01175", "produce"},
	"lentils": {"CP01176", "produce"}, "lentils_green": {"CP01176", "produce"}, "peas_dry": {"CP01176", "produce"}, "chickpeas_can": {"CP01176", "produce"}, "beans_can": {"CP01176", "produce"},
	"broccoli": {"CP01178", "produce"}, "cauliflower": {"CP01178", "produce"}, "green_beans": {"CP01178", "produce"}, "mixed_veg": {"CP01178", "produce"},
	"spinach_frozen": {"CP01178", "produce"}, "peas_frozen": {"CP01178", "produce"}, "brussels": {"CP01178", "produce"},
	"corn_can": {"CP01179", "produce"}, "green_peas_can": {"CP01179", "produce"}, "tomato_paste": {"CP01179", "produce"}, "tomatoes_can": {"CP01179", "produce"},
	"pickles": {"CP01179", "produce"}, "pickles_gherkin": {"CP01179", "produce"}, "sauerkraut": {"CP01179", "produce"}, "olives": {"CP01179", "produce"}, "pineapple_can": {"CP01169", "produce"},
	"sugar": {"CP01181", "other"}, "vanilla_sugar": {"CP01181", "other"}, "honey": {"CP01183", "other"}, "peanut_butter": {"CP01184", "other"},
	"dark_chocolate": {"CP01185", "other"}, "cocoa": {"CP01185", "other"},
	"salt": {"CP01193", "other"}, "soy_sauce": {"CP01193", "other"}, "mayo": {"CP01193", "other"}, "mustard": {"CP01193", "other"}, "ketchup": {"CP01193", "other"}, "vinegar": {"CP01193", "other"},
	"black_pepper": {"CP01194", "other"}, "paprika": {"CP01194", "other"}, "curry": {"CP01194", "other"}, "cumin": {"CP01194", "other"}, "bay_leaf": {"CP01194", "other"},
	"cinnamon": {"CP01194", "other"}, "chili_flakes": {"CP01194", "other"}, "garlic_powder": {"CP01194", "other"}, "oregano": {"CP01194", "other"},
	"baking_powder": {"CP01199", "other"}, "starch": {"CP01199", "other"}, "yeast": {"CP01199", "other"}, "chicken_broth_cube": {"CP01193", "other"},
	"tea": {"CP0121", "other"}, "apple_juice": {"CP0122", "other"}, "grape_juice": {"CP0122", "other"}, "coconut_milk": {"CP01144", "other"}, "seaweed_nori": {"CP01199", "other"}, "puff_pastry": {"CP01191", "cereals"},
}

// Кэш общих таблиц на один прогон синхронизации: PLI и курсы одинаковы для всех стран.
var eurostatCache struct {
	mu   sync.Mutex
	at   time.Time
	pli  map[string]map[string]float64 // geo → категория → PLI
	fx   map[string]float64            // валюта → курс за 1 евро
	fxAt time.Time
}

type jsonStat struct {
	Label   string             `json:"label"`
	Value   map[string]float64 `json:"value"`
	ID      []string           `json:"id"`
	Size    []int              `json:"size"`
	Dimensn map[string]struct {
		Category struct {
			Index map[string]int `json:"index"`
		} `json:"category"`
	} `json:"dimension"`
}

// cells разворачивает JSON-stat в карту «значения измерений → число».
func (j *jsonStat) cells() []map[string]any {
	// множители позиций: последняя размерность меняется быстрее всех
	names := j.ID
	mult := make([]int, len(names))
	m := 1
	for i := len(names) - 1; i >= 0; i-- {
		mult[i] = m
		m *= j.Size[i]
	}
	rev := make([]map[int]string, len(names))
	for i, n := range names {
		rev[i] = map[int]string{}
		for k, idx := range j.Dimensn[n].Category.Index {
			rev[i][idx] = k
		}
	}
	var out []map[string]any
	for pos, v := range j.Value {
		p, _ := strconv.Atoi(pos)
		cell := map[string]any{"value": v}
		for i, n := range names {
			cell[n] = rev[i][(p/mult[i])%j.Size[i]]
		}
		out = append(out, cell)
	}
	return out
}

func fetchStat(ctx context.Context, dataset string, params string) (*jsonStat, error) {
	data, err := get(ctx, eurostatAPI+dataset+"?"+params+"&format=JSON&lang=EN", "application/json")
	if err != nil {
		return nil, err
	}
	var j jsonStat
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("eurostat %s: %w", dataset, err)
	}
	return &j, nil
}

// loadPLI — индексы уровня цен по категориям еды для всех стран за последний год.
func loadPLI(ctx context.Context) (map[string]map[string]float64, error) {
	eurostatCache.mu.Lock()
	defer eurostatCache.mu.Unlock()
	if eurostatCache.pli != nil && time.Since(eurostatCache.at) < time.Hour {
		return eurostatCache.pli, nil
	}
	var cats []string
	for _, c := range pliCats {
		cats = append(cats, "ppp_cat="+c)
	}
	j, err := fetchStat(ctx, "prc_ppp_ind", "na_item=PLI_EU27_2020&lastTimePeriod=1&"+strings.Join(cats, "&"))
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]float64{}
	for _, c := range j.cells() {
		geo, _ := c["geo"].(string)
		cat, _ := c["ppp_cat"].(string)
		if out[geo] == nil {
			out[geo] = map[string]float64{}
		}
		out[geo][cat] = c["value"].(float64)
	}
	eurostatCache.pli, eurostatCache.at = out, time.Now()
	return out, nil
}

// loadFX — курс евро к валюте (сколько единиц валюты за 1 евро), среднее за последний месяц.
func loadFX(ctx context.Context, cur string) (float64, error) {
	if cur == "" || cur == "EUR" {
		return 1, nil
	}
	eurostatCache.mu.Lock()
	if eurostatCache.fx != nil && time.Since(eurostatCache.fxAt) < time.Hour {
		if v, ok := eurostatCache.fx[cur]; ok {
			eurostatCache.mu.Unlock()
			return v, nil
		}
	}
	eurostatCache.mu.Unlock()
	j, err := fetchStat(ctx, "ert_bil_eur_m", "currency="+cur+"&statinfo=AVG&unit=NAC&lastTimePeriod=1")
	if err != nil {
		return 0, err
	}
	for _, c := range j.cells() {
		v := c["value"].(float64)
		eurostatCache.mu.Lock()
		if eurostatCache.fx == nil {
			eurostatCache.fx = map[string]float64{}
		}
		eurostatCache.fx[cur], eurostatCache.fxAt = v, time.Now()
		eurostatCache.mu.Unlock()
		return v, nil
	}
	return 0, fmt.Errorf("eurostat: no rate for %s", cur)
}

// hicp — индексы групп для страны: группа → месяц → индекс, и последний месяц с данными.
func loadHICP(ctx context.Context, geo string, groups []string) (map[string]map[string]float64, string, error) {
	var q []string
	for _, g := range groups {
		q = append(q, "coicop18="+g)
	}
	j, err := fetchStat(ctx, "prc_hicp_minr", "geo="+geo+"&unit=I15&sinceTimePeriod="+eurostatBase+"&"+strings.Join(q, "&"))
	if err != nil {
		return nil, "", err
	}
	out := map[string]map[string]float64{}
	latest := ""
	for _, c := range j.cells() {
		g, _ := c["coicop18"].(string)
		t, _ := c["time"].(string)
		if out[g] == nil {
			out[g] = map[string]float64{}
		}
		out[g][t] = c["value"].(float64)
		if t > latest {
			latest = t
		}
	}
	if latest == "" {
		return nil, "", fmt.Errorf("eurostat: no HICP data for %s", geo)
	}
	return out, latest, nil
}

// drift — рост группы с базового месяца; если у группы нет данных, берём родительскую (CP0114 для CP01145 и т.д.).
func drift(h map[string]map[string]float64, group, latest string) float64 {
	for g := group; len(g) >= 4; g = g[:len(g)-1] {
		if s, ok := h[g]; ok {
			base, okb := s[eurostatBase]
			now, okn := s[latest]
			if okb && okn && base > 0 {
				return now / base
			}
		}
	}
	return 1
}

func (e Eurostat) Fetch(ctx context.Context, packs map[string]PackInfo) (*Result, error) {
	// набор групп: все, что встречаются у продуктов, плюс родительские для запасного варианта
	set := map[string]bool{"CP011": true, "CP0111": true, "CP0112": true, "CP0113": true, "CP0114": true, "CP0115": true, "CP0116": true, "CP0117": true, "CP0118": true, "CP0119": true}
	for id, p := range packs {
		g, _ := coicopOf(id, p.Category)
		set[g] = true
	}
	groups := make([]string, 0, len(set))
	for g := range set {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	hicp, latest, err := loadHICP(ctx, e.Geo, groups)
	if err != nil {
		return nil, err
	}
	pli, err := loadPLI(ctx)
	if err != nil {
		return nil, err
	}
	fx, err := loadFX(ctx, e.Cur)
	if err != nil {
		return nil, err
	}
	period, _ := time.Parse("2006-01", latest)
	r := &Result{Period: period, Pack: map[string]float64{}}
	for id, p := range packs {
		bench, ok := p.Local["DE"]
		if !ok || bench <= 0 {
			continue
		}
		group, cat := coicopOf(id, p.Category)
		level := 1.0
		if e.Geo != "DE" {
			if de, ok := pli["DE"][pliCats[cat]]; ok && de > 0 {
				if here, ok := pli[e.Geo][pliCats[cat]]; ok && here > 0 {
					level = here / de
				}
			}
		}
		r.Pack[id] = bench * level * drift(hicp, group, latest) * fx
	}
	return r, nil
}
