package planner

import (
	"strings"
	"time"

	"racion/internal/i18n"
)

// StorePrices — цены одной сети из её собственного каталога. Пока такой источник один: ВкусВилл
// открыл каталог через MCP, и для сопоставленных продуктов мы берём его цену вместо оценки
// «Росстат × индекс сети». Не сопоставленное по-прежнему считается по индексу, а доля корзины
// по настоящим ценам видна в подписи источника.
type StorePrices struct {
	Store string               `json:"store"`
	Date  string               `json:"date"`  // когда сняты цены, YYYY-MM-DD
	Items map[string]StoreItem `json:"items"` // продукт базы → товар сети
}

// StoreItem — товар сети, которым закрываем наш продукт.
type StoreItem struct {
	XMLID int     `json:"xml_id"`
	Name  string  `json:"name"`
	URL   string  `json:"url"`
	Unit  string  `json:"unit"`  // «шт» — упаковка, «кг» — на развес
	Pack  float64 `json:"pack"`  // упаковка сети в наших единицах (г, мл, шт); у весового 0
	Price float64 `json:"price"` // цена упаковки, у весового — цена за кг
}

// Weighed — товар продаётся на развес.
func (s StoreItem) Weighed() bool { return s.Unit == "кг" }

// PerUnit — цена за наш грамм, миллилитр или штуку; 0 — посчитать нельзя.
func (s StoreItem) PerUnit() float64 {
	switch {
	case s.Price <= 0:
		return 0
	case s.Weighed():
		return s.Price / 1000
	case s.Pack > 0:
		return s.Price / s.Pack
	}
	return 0
}

// SetStorePrices подменяет цены сети целиком (после выгрузки каталога сети).
func (c *Catalog) SetStorePrices(sp *StorePrices) {
	if sp != nil && sp.Store != "" {
		c.prices.stores.Store(strings.ToLower(sp.Store), sp)
	}
}

// StorePrices — цены сети или nil, если своего каталога у сети нет.
func (c *Catalog) StorePrices(store string) *StorePrices {
	if v, ok := c.prices.stores.Load(strings.ToLower(store)); ok {
		return v.(*StorePrices)
	}
	return nil
}

// storeItem — товар сети для продукта, если цена у него есть.
func (p pricer) storeItem(ing Ingredient) (StoreItem, bool) {
	if p.store == nil {
		return StoreItem{}, false
	}
	it, ok := p.store.Items[ing.ID]
	return it, ok && it.PerUnit() > 0
}

// DateLabel — «3 октября 2026» на языке плана; пусто, если дата не задана.
func (sp *StorePrices) DateLabel(l i18n.Lang) string {
	t, err := time.Parse("2006-01-02", sp.Date)
	if err != nil {
		return ""
	}
	return i18n.DayMonthYear(l, t.Day(), int(t.Month()), t.Year())
}
