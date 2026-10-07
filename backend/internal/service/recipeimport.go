package service

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
	"racion/internal/recipeimport"
)

// Импорт рецепта по ссылке: сервер скачивает страницу, достаёт рецепт из разметки schema.org (нет разметки —
// просит нейросеть найти рецепт в тексте), сопоставляет строки ингредиентов с базой продуктов и пересчитывает
// количество на одну порцию. Ничего не сохраняет: черновик открывается в форме своего рецепта, человек
// проверяет продукты и сохраняет сам.

const (
	importsPerHour  = 20
	importParallel  = 4 // одновременных скачиваний на весь сервер
	importTextLimit = 14000
)

type RecipeImport struct {
	fetch   *recipeimport.Fetcher
	ai      recipeimport.JSONer // nil — без нейросети: только разметка и правила
	catalog *planner.CatalogRef
	sem     chan struct{}
	mu      sync.Mutex
	usage   map[string][]time.Time
}

func NewRecipeImport(catalog *planner.CatalogRef, client recipeimport.JSONer) *RecipeImport {
	return &RecipeImport{fetch: recipeimport.NewFetcher(), ai: client, catalog: catalog, sem: make(chan struct{}, importParallel), usage: map[string][]time.Time{}}
}

// SetAI подключает нейросеть: поиск рецепта в тексте страницы без разметки и сопоставление продуктов.
func (s *RecipeImport) SetAI(client recipeimport.JSONer) { s.ai = client }

// ImportDraft — черновик для формы своего рецепта.
type ImportDraft struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Slot        string              `json:"slot"`
	TimeMin     int                 `json:"timeMin"`
	Equipment   []string            `json:"equipment"`
	Steps       []string            `json:"steps"`
	Ingredients []recipeimport.Item `json:"ingredients"`
	Unmatched   []string            `json:"unmatched"` // строки, для которых продукт в базе не нашёлся
	Portions    int                 `json:"portions"`  // на сколько порций рецепт на сайте
	Guessed     bool                `json:"portionsGuessed,omitempty"`
	Lang        string              `json:"lang,omitempty"` // язык страницы, если известен
	Source      string              `json:"source"`
	Host        string              `json:"host"`
	Found       string              `json:"found"`   // markup — из разметки, ai — нейросеть по тексту
	Matched     string              `json:"matched"` // ai | rules
}

func (s *RecipeImport) Import(ctx context.Context, userID, rawURL string, lang i18n.Lang) (ImportDraft, error) {
	if _, err := recipeimport.ParseURL(rawURL); err != nil {
		return ImportDraft{}, domain.Invalid("import.err.url")
	}
	if !s.allow(userID) {
		return ImportDraft{}, domain.Invalid("import.err.limit")
	}
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return ImportDraft{}, domain.Invalid("import.err.fetch")
	}
	page, err := s.fetch.Fetch(ctx, rawURL)
	switch {
	case errors.Is(err, recipeimport.ErrURL), errors.Is(err, recipeimport.ErrBlocked):
		return ImportDraft{}, domain.Invalid("import.err.url")
	case errors.Is(err, recipeimport.ErrNotHTML):
		return ImportDraft{}, domain.Invalid("import.err.norecipe")
	case err != nil:
		return ImportDraft{}, domain.Invalid("import.err.fetch")
	}
	doc, err := recipeimport.ParseDoc(page.HTML)
	if err != nil {
		return ImportDraft{}, domain.Invalid("import.err.norecipe")
	}
	r := doc.Recipe()
	found := "markup"
	if !r.Complete() {
		if s.ai == nil {
			return ImportDraft{}, domain.Invalid("import.err.norecipe")
		}
		actx, cancel := context.WithTimeout(ctx, 25*time.Second)
		got, err := recipeimport.ExtractAI(actx, s.ai, r.Title, doc.Text(importTextLimit))
		cancel()
		if err != nil {
			return ImportDraft{}, domain.Invalid("api.ai.busy")
		}
		if !got.Complete() {
			return ImportDraft{}, domain.Invalid("import.err.norecipe")
		}
		got.Lang = doc.Lang()
		r, found = got, "ai"
	}

	source := page.URL
	if c := doc.Canonical(); c != "" {
		source = c
	}
	d := ImportDraft{Title: r.Title, Description: r.Description, TimeMin: r.TimeMin, Steps: r.Steps, Portions: r.Portions,
		Lang: r.Lang, Source: source, Found: found, Matched: "ai"}
	if u, err := url.Parse(source); err == nil {
		d.Host = strings.TrimPrefix(u.Hostname(), "www.")
	}
	if d.Portions == 0 {
		d.Portions, d.Guessed = recipeimport.DefaultPortions, true
	}
	if d.TimeMin == 0 {
		d.TimeMin = 30
	}
	d.Slot = guessSlot(r)
	d.Equipment = guessEquipment(r.Steps)

	products := s.products(r.Lang, lang)
	if s.ai != nil {
		actx, cancel := context.WithTimeout(ctx, 30*time.Second)
		d.Ingredients, d.Unmatched, err = recipeimport.MatchAI(actx, s.ai, r.Ingredients, d.Portions, products)
		cancel()
	}
	if s.ai == nil || err != nil { // нейросеть занята или не ответила — подбираем по правилам
		d.Ingredients, d.Unmatched = recipeimport.MatchRules(r.Ingredients, d.Portions, products)
		d.Matched = "rules"
	}
	if d.Unmatched == nil {
		d.Unmatched = []string{}
	}
	return d, nil
}

// products — база для сопоставления: название на языке рецепта (или интерфейса) и все переводы.
func (s *RecipeImport) products(recipeLang string, ui i18n.Lang) []recipeimport.Product {
	l, ok := i18n.Valid(recipeLang)
	if !ok {
		l = ui
	}
	c := s.catalog.Load()
	ids := make([]string, 0, len(c.Ingredients))
	for id := range c.Ingredients {
		ids = append(ids, id)
	}
	slices.Sort(ids) // порядок влияет на выбор при равных совпадениях — пусть он будет один и тот же
	out := make([]recipeimport.Product, 0, len(ids))
	for _, id := range ids {
		ing := c.Ingredients[id]
		names := []string{ing.Name}
		for _, n := range ing.Names {
			names = append(names, n)
		}
		out = append(out, recipeimport.Product{ID: ing.ID, Name: ing.LocalName(l), Names: names, Unit: ing.Unit, Pantry: ing.Pantry})
	}
	return out
}

func (s *RecipeImport) allow(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	keep := s.usage[userID][:0]
	for _, t := range s.usage[userID] {
		if now.Sub(t) < time.Hour {
			keep = append(keep, t)
		}
	}
	if len(keep) >= importsPerHour {
		s.usage[userID] = keep
		return false
	}
	s.usage[userID] = append(keep, now)
	return true
}

// ── Догадки по тексту ──────────────────────────────────────────────────────

var slotWords = []struct {
	slot  string
	words []string
}{
	{"breakfast", []string{"завтрак", "breakfast", "frühstück", "petit-déjeuner", "desayuno", "colazione", "śniadanie", "ontbijt", "snídaně", "каша", "омлет", "сырник", "oatmeal", "porridge", "granola", "pancake", "оладь", "гренк"}},
	// ужин раньше обеда: сайты ставят основным блюдам обе метки сразу («Dinner», «Lunch»)
	{"dinner", []string{"ужин", "dinner", "supper", "abendessen", "dîner", "cena", "kolacja", "avondeten", "večeře", "akşam yemeği"}},
	{"lunch", []string{"суп", "soup", "suppe", "soupe", "sopa", "zuppa", "zupa", "soep", "polévka", "çorba", "борщ", "щи", "солянк", "бульон", "рассольник", "уха", "харчо", "окрошк"}},
	{"snack", []string{"десерт", "dessert", "выпечк", "baking", "перекус", "snack", "торт", "cake", "печенье", "cookie", "кекс", "маффин", "muffin", "пирожн", "смузи", "smoothie", "коктейль", "мороженое", "кукис", "брауни", "brownie", "чизкейк", "cheesecake"}},
}

// guessSlot — приём пищи по категории сайта и названию; по умолчанию ужин.
func guessSlot(r recipeimport.Recipe) string {
	hay := strings.ToLower(strings.Join(append(append([]string{}, r.Category...), r.Title), " "))
	for _, sw := range slotWords {
		for _, w := range sw.words {
			if strings.Contains(hay, w) {
				return sw.slot
			}
		}
	}
	return "dinner"
}

var equipmentWords = []struct {
	id    string
	words []string
}{
	{"oven", []string{"духовк", "запек", "запеч", "oven", "bake", "backofen", "horno", "forno", "piekarnik", "trouba", "fırın"}},
	{"microwave", []string{"микроволн", "свч", "microwave", "mikrowelle", "micro-ondes"}},
	{"airfryer", []string{"аэрогрил", "аэрофритюр", "air fryer", "airfryer", "heißluftfritteuse"}},
	{"multicooker", []string{"мультиварк", "multicooker", "slow cooker", "instant pot", "скороварк"}},
	{"blender", []string{"блендер", "blender", "пюрир"}},
	{"mixer", []string{"миксер", "mixer", "hand mixer", "stand mixer"}},
	{"grill", []string{"гриль", "мангал", "grill", "barbecue", "bbq"}},
	{"steamer", []string{"пароварк", "на пару", "steamer", "dampfgarer"}},
	{"meatgrinder", []string{"мясорубк", "meat grinder", "fleischwolf"}},
	{"stove", []string{"сковород", "кастрюл", "сотейник", "плит", "обжар", "жарьте", "варите", "отвар", "туш", "пассер", "frying pan", "skillet", "saucepan", "stove", "simmer", "boil", "fry", "pfanne", "topf", "poêle", "casserole", "sartén", "padella"}},
}

// guessEquipment — техника по шагам: «запекайте в духовке» → духовка. Без подсказок — плита.
func guessEquipment(steps []string) []string {
	hay := strings.ToLower(strings.Join(steps, " "))
	out := []string{}
	for _, e := range equipmentWords {
		for _, w := range e.words {
			if strings.Contains(hay, w) {
				out = append(out, e.id)
				break
			}
		}
	}
	if len(out) == 0 {
		out = append(out, "stove")
	}
	return out
}
