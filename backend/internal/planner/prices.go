package planner

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/i18n"
)

// Region — территория Росстата.
type Region struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // rf | district | region | city
	Parent string `json:"parent"`
}

// PriceSource — что именно легло в основу рублёвых цифр плана. Показывается пользователю.
type PriceSource struct {
	Name       string  `json:"name"`       // «Росстат» или «оценка»
	Period     string  `json:"period"`     // «август 2026»
	WeeklyDate string  `json:"weeklyDate"` // «14 сентября 2026» — недельная коррекция по РФ
	Region     string  `json:"region"`     // название территории
	RegionCode string  `json:"regionCode"`
	Coverage   float64 `json:"coverage"` // доля стоимости корзины, посчитанная по Росстату, 0..1
}

// PriceBook — цены Росстата в памяти: регион → товар → цена за кг/л/10 шт с недельной коррекцией.
type PriceBook struct {
	Period     string
	PeriodTime time.Time
	WeeklyDate time.Time
	Regions    []Region
	RegionBy   map[string]Region
	monthly    map[string]map[int]float64
	ratio      map[int]float64 // недельная коррекция по товару: цена сейчас / цена на конец месяца (РФ)
	itemName   map[int]string
}

func (pb *PriceBook) PeriodLabel(l i18n.Lang) string {
	if pb == nil || pb.PeriodTime.IsZero() {
		return ""
	}
	return i18n.MonthYear(l, int(pb.PeriodTime.Month()), pb.PeriodTime.Year())
}

func (pb *PriceBook) WeeklyLabel(l i18n.Lang) string {
	if pb == nil || pb.WeeklyDate.IsZero() {
		return ""
	}
	return i18n.DayMonthYear(l, pb.WeeklyDate.Day(), int(pb.WeeklyDate.Month()), pb.WeeklyDate.Year())
}

// LocalPrices — живой источник цен другой страны (аналог Росстата): цена типовой упаковки
// продукта в местной валюте. Адаптер источника сам приводит единицы и сопоставляет товары.
type LocalPrices struct {
	Country string
	Source  string // ключ i18n названия источника, например price.bls
	Period  time.Time
	Pack    map[string]float64 // id продукта → цена упаковки
}

func (lp *LocalPrices) PeriodLabel(l i18n.Lang) string {
	if lp == nil || lp.Period.IsZero() {
		return ""
	}
	return i18n.MonthYear(l, int(lp.Period.Month()), lp.Period.Year())
}

// Price возвращает цену товара в регионе; если по региону данных нет — по родительскому региону, потом по РФ.
func (pb *PriceBook) Price(region string, item int) (float64, bool) {
	if pb == nil {
		return 0, false
	}
	for _, code := range []string{region, pb.RegionBy[region].Parent, "643"} {
		if code == "" {
			continue
		}
		if p, ok := pb.monthly[code][item]; ok {
			if r, ok := pb.ratio[item]; ok && r > 0.5 && r < 2 {
				p *= r
			}
			return p, true
		}
	}
	return 0, false
}

// LoadPriceBook собирает ценник из БД. Возвращает nil, если синхронизации ещё не было.
func LoadPriceBook(ctx context.Context, pool *pgxpool.Pool) (*PriceBook, error) {
	var period string
	var weekly *time.Time
	if err := pool.QueryRow(ctx, `SELECT monthly_period, weekly_date FROM rosstat_sync WHERE id = 1`).Scan(&period, &weekly); err != nil {
		return nil, nil // нет данных — не ошибка
	}
	pb := &PriceBook{Period: period, RegionBy: map[string]Region{}, monthly: map[string]map[int]float64{}, ratio: map[int]float64{}, itemName: map[int]string{}}
	pb.PeriodTime, _ = time.Parse("2006-01", period)
	if weekly != nil {
		pb.WeeklyDate = *weekly
	}

	rows, err := pool.Query(ctx, `SELECT code, name, kind, parent FROM regions ORDER BY sort`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Region
		if err := rows.Scan(&r.Code, &r.Name, &r.Kind, &r.Parent); err != nil {
			rows.Close()
			return nil, err
		}
		pb.Regions = append(pb.Regions, r)
		pb.RegionBy[r.Code] = r
	}
	rows.Close()

	rows, err = pool.Query(ctx, `SELECT region, item, price FROM rosstat_prices WHERE period = $1`, period)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var region string
		var item int
		var price float64
		if err := rows.Scan(&region, &item, &price); err != nil {
			rows.Close()
			return nil, err
		}
		if pb.monthly[region] == nil {
			pb.monthly[region] = map[int]float64{}
		}
		pb.monthly[region][item] = price
	}
	rows.Close()

	rows, err = pool.Query(ctx, `SELECT item, name FROM rosstat_items`)
	if err != nil {
		return nil, err
	}
	byName := map[string]int{}
	for rows.Next() {
		var item int
		var name string
		if err := rows.Scan(&item, &name); err != nil {
			rows.Close()
			return nil, err
		}
		pb.itemName[item] = name
		byName[normalizeName(name)] = item
	}
	rows.Close()

	// Недельная коррекция: последняя неделя / последняя неделя внутри отчётного месяца.
	if !pb.WeeklyDate.IsZero() && !pb.PeriodTime.IsZero() {
		monthEnd := pb.PeriodTime.AddDate(0, 1, 0)
		rows, err = pool.Query(ctx, `SELECT item_name, week_date, price FROM rosstat_weekly ORDER BY item_name, week_date`)
		if err != nil {
			return nil, err
		}
		type pt struct {
			d time.Time
			p float64
		}
		series := map[string][]pt{}
		for rows.Next() {
			var name string
			var d time.Time
			var p float64
			if err := rows.Scan(&name, &d, &p); err != nil {
				rows.Close()
				return nil, err
			}
			series[name] = append(series[name], pt{d, p})
		}
		rows.Close()
		for name, pts := range series {
			item, ok := byName[name]
			if !ok {
				continue
			}
			var base, latest float64
			for _, x := range pts {
				if !x.d.After(monthEnd.AddDate(0, 0, 3)) {
					base = x.p
				}
				latest = x.p
			}
			if base > 0 && latest > 0 {
				pb.ratio[item] = latest / base
			}
		}
	}
	return pb, nil
}

func normalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	return strings.Join(strings.Fields(s), " ")
}

// pricer считает цену упаковки продукта для конкретного плана.
// Россия: Росстат по региону × индекс сети, иначе ручная цена. Другие страны: живой источник
// страны, иначе ручной ориентир в местной валюте, иначе рублёвая цена × курс-множитель.
type pricer struct {
	pb      *PriceBook
	local   *LocalPrices
	region  string
	idx     float64
	country Country
	lang    i18n.Lang
}

// round — деньги с точностью страны.
func (p pricer) round(v float64) float64 { return p.country.RoundMoney(v) }

// packPrice — цена типовой упаковки в валюте страны. Второе значение — из официального источника ли цена.
func (p pricer) packPrice(ing Ingredient) (float64, bool) {
	if p.country.Code == "RU" || p.country.Code == "" {
		if p.pb != nil && ing.RosstatItem > 0 {
			if v, ok := p.pb.Price(p.region, ing.RosstatItem); ok {
				f := ing.RosstatFactor
				if f <= 0 {
					f = 1
				}
				var price float64
				switch ing.Unit {
				case "pcs": // Росстат: за 10 шт
					price = v / 10 * ing.Pack * f
				default: // за кг или л
					price = v / 1000 * ing.Pack * f
				}
				return math.Round(price*100) / 100 * p.idx, true
			}
		}
		return ing.Price * p.idx, false
	}
	if p.local != nil {
		if v, ok := p.local.Pack[ing.ID]; ok && v > 0 {
			return p.round(v * p.idx), true
		}
	}
	if v, ok := ing.Prices[p.country.Code]; ok && v > 0 {
		return p.round(v * p.idx), false
	}
	return p.round(ing.Price * p.country.Scale * p.idx), false
}

// kgFallback — рублёвая цена за кг в валюту страны (для детского питания без местной цены).
func (p pricer) kgFallback(rub float64) float64 {
	if p.country.Code == "RU" || p.country.Code == "" {
		return rub
	}
	return rub * p.country.Scale
}

// unitPrice — цена за 1 g/ml/pcs.
func (p pricer) unitPrice(ing Ingredient) float64 {
	pp, _ := p.packPrice(ing)
	return pp / ing.Pack
}

// referencePricer — цены страны без индекса сети: Росстат по РФ или источник страны (для страниц рецептов).
func (c *Catalog) referencePricer(country string, l i18n.Lang) pricer {
	return pricer{pb: c.PriceBook(), local: c.LocalPrices(country), region: "643", idx: 1, country: CountryOf(country), lang: l}
}

// PortionCost — цена порции по справочным ценам страны без индекса сети. Второе значение — есть ли ценник вообще.
func (c *Catalog) PortionCost(r Recipe, country string) (float64, bool) {
	pr := c.referencePricer(country, i18n.RU)
	priced := pr.country.Code != "RU" || pr.pb != nil
	return pr.round(c.costPerPortion(r, pr)), priced
}

// IngredientPrice — цена упаковки по справочным ценам страны без индекса сети. Второе значение — из официального источника ли.
func (c *Catalog) IngredientPrice(ing Ingredient, country string) (float64, bool) {
	return c.referencePricer(country, i18n.RU).packPrice(ing)
}
