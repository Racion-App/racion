package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Catalog — справочники для квиза и каталога: страны, магазины, продукты, фильтры, поиск (search.go).
type Catalog struct {
	catalog *planner.CatalogRef
	docs    sync.Map // i18n.Lang → []searchDoc
}

func (c *Catalog) Base() *planner.Catalog { return c.catalog.Load() }

// HasTag — есть ли у рецепта тег.
func HasTag(r planner.Recipe, t string) bool {
	for _, x := range r.Tags {
		if x == t {
			return true
		}
	}
	return false
}

type Labeled struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ExcludePreset struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // ingredient | tag
}

// Часто исключаемые продукты — плитки на шаге «Что не готовим?». Подпись берётся из базы продуктов
// (короткое имя) или из i18n для тегов.
var excludePresets = []struct{ ID, Kind string }{
	{"liver_chicken", "ingredient"}, {"cottage_cheese", "ingredient"}, {"eggplant", "ingredient"}, {"olives", "ingredient"},
	{"pearl_barley", "ingredient"}, {"beef_stew", "ingredient"}, {"pork_neck", "ingredient"}, {"mushrooms", "ingredient"},
	{"spicy", "tag"}, {"offal", "tag"}, {"sausages", "ingredient"}, {"shrimp", "ingredient"}, {"pumpkin", "ingredient"}, {"beet", "ingredient"},
	// целые группы: вегетарианцу хватает четырёх отметок вместо поиска по продуктам
	{"meat", "tag"}, {"poultry", "tag"}, {"fish", "tag"}, {"seafood", "tag"},
}

// ShortLabel — «Свинина (шея)» → «Свинина», «Творог 5%» → «Творог».
func ShortLabel(name string) string {
	if i := strings.Index(name, " ("); i > 0 {
		name = name[:i]
	}
	fields := strings.Fields(name)
	out := fields[:0]
	for _, f := range fields {
		if strings.HasSuffix(f, "%") {
			continue
		}
		out = append(out, f)
	}
	return strings.Join(out, " ")
}

// Regions — регионы и города Росстата (без федеральных округов) для таргетинга и геоподбора.
func (c *Catalog) Regions() []planner.Region {
	out := []planner.Region{}
	if pb := c.catalog.Load().PriceBook(); pb != nil {
		for _, rg := range pb.Regions {
			if rg.Kind != "district" {
				out = append(out, rg)
			}
		}
	}
	return out
}

// Meta — всё, что нужно квизу, на языке; geoCountry — страна посетителя, если известна.
type Meta struct {
	Lang           i18n.Lang              `json:"lang"`
	Countries      []map[string]any       `json:"countries"`
	Stores         []planner.Store        `json:"stores"`
	Regions        []planner.Region       `json:"regions"`
	Prices         map[string]any         `json:"prices"`
	LocalPrices    map[string]any         `json:"localPrices"`
	Allergens      []Labeled              `json:"allergens"`
	Equipment      []Labeled              `json:"equipment"`
	Slots          []Labeled              `json:"slots"`
	Goals          []map[string]any       `json:"goals"`
	Feeding        []Labeled              `json:"feeding"`
	BudgetPresets  []planner.BudgetPreset `json:"budgetPresets"`
	FormulaBrands  []planner.FormulaBrand `json:"formulaBrands"`
	ExcludePresets []ExcludePreset        `json:"excludePresets"`
	Ingredients    []Labeled              `json:"ingredients"`
	Recipes        int                    `json:"recipes"`
	GeoCountry     string                 `json:"geoCountry"`
	GeoLang        string                 `json:"geoLang"` // язык по стране посетителя
	GeoRegion      string                 `json:"geoRegion,omitempty"` // регион или город Росстата по IP: квиз подставит его сам
	AI             bool                   `json:"ai"`      // помощник для своих рецептов включён
	Photos         bool                   `json:"photos"`  // загрузка фото включена (есть S3)
}

func (c *Catalog) Meta(lang i18n.Lang, country planner.Country, geoCountry string) Meta {
	m := Meta{Lang: lang, GeoCountry: geoCountry, Recipes: len(c.catalog.Load().Recipes)}
	for _, id := range planner.Allergens {
		m.Allergens = append(m.Allergens, Labeled{id, planner.AllergenLabel(lang, id)})
	}
	for _, id := range planner.EquipmentOrder {
		m.Equipment = append(m.Equipment, Labeled{id, planner.EquipmentLabel(lang, id)})
	}
	for _, id := range planner.SlotOrder {
		m.Slots = append(m.Slots, Labeled{id, planner.SlotLabel(lang, id)})
	}
	for _, id := range planner.Goals {
		m.Goals = append(m.Goals, map[string]any{"id": id, "label": planner.GoalLabel(lang, id), "kcal": planner.GoalKcal[id]})
	}
	for _, id := range planner.FeedingModes {
		m.Feeding = append(m.Feeding, Labeled{id, planner.FeedingLabel(lang, id)})
	}
	for _, cy := range planner.Countries {
		m.Countries = append(m.Countries, map[string]any{"code": cy.Code, "label": i18n.T(lang, "country."+cy.Code), "currency": cy.Currency, "symbol": cy.Symbol,
			"decimals": cy.Decimals, "locale": cy.Locale, "hasRegions": cy.HasRegions, "presets": planner.BudgetPresetsFor(lang, cy), "default": cy.Default})
	}
	m.Stores = make([]planner.Store, 0, len(c.catalog.Load().StoreList))
	for _, st := range c.catalog.Load().StoreList {
		st.Note = planner.StoreKindLabel(lang, st.Kind)
		m.Stores = append(m.Stores, st)
	}
	for _, p := range excludePresets {
		label := i18n.T(lang, "tag."+p.ID)
		if p.Kind == "ingredient" {
			label = ShortLabel(c.catalog.Load().Ingredients[p.ID].LocalName(lang))
		}
		m.ExcludePresets = append(m.ExcludePresets, ExcludePreset{p.ID, label, p.Kind})
	}
	// продукты для поиска в стоп-продуктах: только то, что реально встречается в рецептах и не «домашнее»
	used := map[string]bool{}
	for _, rc := range c.catalog.Load().Recipes {
		for _, ri := range rc.Ingredients {
			used[ri.IngredientID] = true
		}
	}
	m.Ingredients = make([]Labeled, 0, len(used))
	for id := range used {
		if ing := c.catalog.Load().Ingredients[id]; !ing.Pantry {
			m.Ingredients = append(m.Ingredients, Labeled{id, ing.LocalName(lang)})
		}
	}
	sortLabeled(m.Ingredients)
	// регионы: РФ, регионы и города (без федеральных округов — их никто не выбирает)
	m.Regions = []planner.Region{}
	if pb := c.catalog.Load().PriceBook(); pb != nil {
		for _, rg := range pb.Regions {
			if rg.Kind != "district" {
				m.Regions = append(m.Regions, rg)
			}
		}
		m.Prices = map[string]any{"source": i18n.T(lang, "price.rosstat"), "period": pb.PeriodLabel(lang), "weeklyDate": pb.WeeklyLabel(lang)}
	}
	m.LocalPrices = map[string]any{}
	for _, cy := range planner.Countries {
		if lp := c.catalog.Load().LocalPrices(cy.Code); lp != nil {
			m.LocalPrices[cy.Code] = map[string]any{"source": i18n.T(lang, lp.Source), "period": lp.PeriodLabel(lang)}
		}
	}
	m.BudgetPresets = planner.BudgetPresetsFor(lang, country)
	m.FormulaBrands = planner.FormulaBrandsFor(lang)
	return m
}

// IngredientRef — продукт для формы своего рецепта.
type IngredientRef struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Unit     string `json:"unit"`
	Pantry   bool   `json:"pantry"`
	Category string `json:"category"`
}

// Ingredients — вся база продуктов по алфавиту на языке.
func (c *Catalog) Ingredients(lang i18n.Lang) []IngredientRef {
	out := make([]IngredientRef, 0, len(c.catalog.Load().Ingredients))
	for _, ing := range c.catalog.Load().Ingredients {
		out = append(out, IngredientRef{ing.ID, ing.LocalName(lang), ing.Unit, ing.Pantry, ing.Category})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && strings.ToLower(out[j].Label) < strings.ToLower(out[j-1].Label); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func sortLabeled(v []Labeled) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && strings.ToLower(v[j].Label) < strings.ToLower(v[j-1].Label); j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// ── Аналитика ──────────────────────────────────────────────────────────────

type Events struct {
	repo EventRepo
}

type EventIn struct {
	Name  string          `json:"name"`
	Props json.RawMessage `json:"props"`
	Ts    int64           `json:"ts"`
}

// Track принимает пачку событий как есть; пустые имена и раздутые props отбрасываются.
func (e *Events) Track(ctx context.Context, sid string, in []EventIn) error {
	if sid == "" || len(sid) > 64 || len(in) == 0 || len(in) > 50 {
		return domain.ErrBadInput
	}
	events := make([]domain.Event, 0, len(in))
	for _, x := range in {
		if x.Name == "" || len(x.Name) > 64 {
			continue
		}
		props := x.Props
		if len(props) == 0 || len(props) > 4<<10 {
			props = json.RawMessage(`{}`)
		}
		at := time.Now()
		if x.Ts > 0 {
			at = time.UnixMilli(x.Ts)
		}
		events = append(events, domain.Event{Name: x.Name, Props: props, At: at})
	}
	if len(events) == 0 {
		return nil
	}
	return e.repo.AddBatch(ctx, sid, events)
}
