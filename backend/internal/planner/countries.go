package planner

import "math"

// Country — страна с валютой, магазинами и ориентиром бюджета.
// Цены: Россия — Росстат по регионам; Беларусь, Казахстан, США — свои официальные источники;
// Европа — ориентир Германии × разница уровня цен страны (Eurostat PLI) × месячный HICP группы товара × курс;
// Британия — средние цены ONS (январь 2025) × CPI группы. Ручные ориентиры — запасной вариант.
type Country struct {
	Code        string     `json:"code"`
	Currency    string     `json:"currency"` // ISO 4217
	Symbol      string     `json:"symbol"`
	Decimals    int        `json:"decimals"`    // знаков после запятой в ценах
	Scale       float64    `json:"-"`           // RUB → местная валюта для того, у чего нет местной цены (детское питание, смесь)
	Presets     [4]float64 `json:"presets"`     // бюджет на человека в день: экономно / средне / свободно / премиум
	Default     float64    `json:"default"`     // бюджет по умолчанию
	HasRegions  bool       `json:"hasRegions"`  // есть выбор региона (Росстат)
	Locale      string     `json:"locale"`      // для форматирования чисел на клиенте
	PriceLevels [3]float64 `json:"priceLevels"` // пороги фильтра «цена порции» в каталоге
	// Формат суммы на серверных страницах: «12,50 €», «£12.50», «45 kr».
	SymbolBefore bool   `json:"-"`
	DecSep       string `json:"-"`
	ThouSep      string `json:"-"`
	// Geo-код Eurostat (HICP, индексы уровня цен, курсы). Пусто — у страны свой источник.
	Eurostat string `json:"-"`
	Sort     int    `json:"-"`
}

// euro — страна еврозоны: «12,50 €», цены через Eurostat.
func euro(code, locale string, presets [4]float64, def float64, sort int) Country {
	return Country{Code: code, Currency: "EUR", Symbol: "€", Decimals: 2, Scale: 0.019, Presets: presets, Default: def, Locale: locale,
		PriceLevels: [3]float64{1, 2, 3.5}, DecSep: ",", ThouSep: " ", Eurostat: code, Sort: sort}
}

var Countries = []Country{
	{Code: "RU", Currency: "RUB", Symbol: "₽", Decimals: 0, Scale: 1, Presets: [4]float64{250, 400, 700, 1200}, Default: 400, HasRegions: true, Locale: "ru-RU", PriceLevels: [3]float64{80, 150, 250}, DecSep: ",", ThouSep: " ", Sort: 1},
	{Code: "BY", Currency: "BYN", Symbol: "Br", Decimals: 2, Scale: 0.035, Presets: [4]float64{8, 12, 20, 36}, Default: 12, Locale: "ru-BY", PriceLevels: [3]float64{2, 4, 7}, DecSep: ",", ThouSep: " ", Sort: 2},
	{Code: "KZ", Currency: "KZT", Symbol: "₸", Decimals: 0, Scale: 5.5, Presets: [4]float64{1500, 2500, 4000, 7500}, Default: 2500, Locale: "ru-KZ", PriceLevels: [3]float64{400, 800, 1400}, DecSep: ",", ThouSep: " ", Sort: 3},
	{Code: "US", Currency: "USD", Symbol: "$", Decimals: 2, Scale: 0.023, Presets: [4]float64{6, 9, 15, 27}, Default: 9, Locale: "en-US", PriceLevels: [3]float64{1.5, 3, 5}, SymbolBefore: true, DecSep: ".", ThouSep: ",", Sort: 4},
	{Code: "GB", Currency: "GBP", Symbol: "£", Decimals: 2, Scale: 0.0165, Presets: [4]float64{3.5, 5.5, 9, 16}, Default: 5.5, Locale: "en-GB", PriceLevels: [3]float64{1, 2, 3.5}, SymbolBefore: true, DecSep: ".", ThouSep: ",", Sort: 5},
	euro("DE", "de-DE", [4]float64{4, 6, 10, 18}, 6, 10),
	euro("AT", "de-AT", [4]float64{4.5, 7, 11, 21}, 7, 11),
	euro("FR", "fr-FR", [4]float64{4.5, 7, 11, 21}, 7, 12),
	euro("IT", "it-IT", [4]float64{4, 6, 10, 18}, 6, 13),
	euro("ES", "es-ES", [4]float64{3.5, 5.5, 9, 16}, 5.5, 14),
	euro("PT", "pt-PT", [4]float64{3.5, 5.5, 9, 16}, 5.5, 15),
	euro("NL", "nl-NL", [4]float64{4, 6, 10, 18}, 6, 16),
	euro("BE", "nl-BE", [4]float64{4.5, 7, 11, 21}, 7, 17),
	euro("FI", "fi-FI", [4]float64{4.5, 7, 11, 21}, 7, 18),
	euro("LT", "lt-LT", [4]float64{3.5, 5.5, 9, 16}, 5.5, 19),
	euro("LV", "lv-LV", [4]float64{3.5, 5.5, 9, 16}, 5.5, 20),
	euro("EE", "et-EE", [4]float64{4, 6, 10, 18}, 6, 21),
	{Code: "PL", Currency: "PLN", Symbol: "zł", Decimals: 2, Scale: 0.08, Presets: [4]float64{15, 22, 35, 66}, Default: 22, Locale: "pl-PL", PriceLevels: [3]float64{4, 8, 14}, DecSep: ",", ThouSep: " ", Eurostat: "PL", Sort: 22},
	{Code: "CZ", Currency: "CZK", Symbol: "Kč", Decimals: 0, Scale: 0.45, Presets: [4]float64{90, 140, 220, 420}, Default: 140, Locale: "cs-CZ", PriceLevels: [3]float64{25, 50, 85}, DecSep: ",", ThouSep: " ", Eurostat: "CZ", Sort: 23},
	{Code: "SE", Currency: "SEK", Symbol: "kr", Decimals: 0, Scale: 0.21, Presets: [4]float64{45, 70, 110, 210}, Default: 70, Locale: "sv-SE", PriceLevels: [3]float64{10, 20, 35}, DecSep: ",", ThouSep: " ", Eurostat: "SE", Sort: 24},
	{Code: "NO", Currency: "NOK", Symbol: "kr", Decimals: 0, Scale: 0.26, Presets: [4]float64{55, 85, 130, 255}, Default: 85, Locale: "nb-NO", PriceLevels: [3]float64{12, 24, 40}, DecSep: ",", ThouSep: " ", Eurostat: "NO", Sort: 25},
	{Code: "CH", Currency: "CHF", Symbol: "CHF", Decimals: 2, Scale: 0.028, Presets: [4]float64{6, 9, 15, 27}, Default: 9, Locale: "de-CH", PriceLevels: [3]float64{1.5, 3, 5}, SymbolBefore: true, DecSep: ".", ThouSep: "’", Eurostat: "CH", Sort: 26},
}

var countryBy = func() map[string]Country {
	m := map[string]Country{}
	for _, c := range Countries {
		m[c.Code] = c
	}
	return m
}()

// CountryOf — страна по коду; неизвестный код → Россия.
func CountryOf(code string) Country {
	if c, ok := countryBy[code]; ok {
		return c
	}
	return countryBy["RU"]
}

// RoundMoney — округление суммы до принятого в стране числа знаков.
func (c Country) RoundMoney(v float64) float64 {
	p := math.Pow(10, float64(c.Decimals))
	return math.Round(v*p) / p
}

// StoresOf — магазины страны в порядке sort.
func (c *Catalog) StoresOf(country string) []Store {
	var out []Store
	for _, s := range c.StoreList {
		if s.Country == country {
			out = append(out, s)
		}
	}
	return out
}
