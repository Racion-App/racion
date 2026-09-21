// Package planner — детерминированный подбор недели: фильтры, скоринг, список покупок.
package planner

import (
	"sync"
	"sync/atomic"

	"racion/internal/i18n"
)

// Ingredient — продукт из базы. Для g/ml КБЖУ на 100, для pcs — на штуку.
type Ingredient struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Category      string             `json:"category"`
	Unit          string             `json:"unit"`
	Pack          float64            `json:"pack"`
	Price         float64            `json:"price"` // ручная цена упаковки — запасной вариант
	Kcal          float64            `json:"kcal"`
	Protein       float64            `json:"protein"`
	Fat           float64            `json:"fat"`
	Carb          float64            `json:"carb"`
	Allergens     []string           `json:"allergens"`
	Perishable    bool               `json:"perishable"`
	Pantry        bool               `json:"pantry"`
	Loose         bool               `json:"loose"`
	RosstatItem   int                `json:"rosstatItem"`   // код товара Росстата, 0 — нет соответствия
	RosstatFactor float64            `json:"rosstatFactor"` // множитель к цене Росстата (филе дороже тушки и т.п.)
	RosstatNote   string             `json:"rosstatNote"`
	Names         map[string]string  `json:"names,omitempty"`  // название на en/de; ru — в Name
	Prices        map[string]float64 `json:"prices,omitempty"` // ручная цена упаковки по странам (BY, KZ, DE, US) в местной валюте
	Image         string             `json:"image,omitempty"`  // картинка для подсказки при наведении
	Tier          string             `json:"tier,omitempty"`   // где продаётся: "" везде, super — супермаркеты и крупнее, premium — гипермаркеты, оптовики, премиум-сети
}

// StoreLevel — размер ассортимента по виду магазина: у дома и дискаунтеры (0–1), супермаркет (2), гипермаркет,
// оптовик и премиум-сеть (3). Продукт с Tier доступен, если TierLevel ≤ StoreLevel.
func StoreLevel(kind string) int {
	switch kind {
	case "discounter":
		return 0
	case "convenience":
		return 1
	case "supermarket":
		return 2
	default:
		return 3
	}
}

// TierLevel — какой магазин нужен продукту в стране. Разметка premium сделана по российским сетям (утка,
// мидии, вырезка есть только в гипермаркетах); в Европе и США это обычный супермаркетный ассортимент.
func TierLevel(tier, country string) int {
	switch tier {
	case "super":
		return 2
	case "premium":
		switch country {
		case "RU", "BY", "KZ", "":
			return 3
		}
		return 2
	}
	return 0
}

// Unavailable — продукты рецепта, которых в магазине такого вида обычно нет.
func (c *Catalog) Unavailable(r Recipe, storeKind, country string) []Ingredient {
	lvl := StoreLevel(storeKind)
	var out []Ingredient
	for _, ri := range r.Ingredients {
		if ing, ok := c.Ingredients[ri.IngredientID]; ok && TierLevel(ing.Tier, country) > lvl {
			out = append(out, ing)
		}
	}
	return out
}

// LocalName — название продукта на языке.
func (i Ingredient) LocalName(l i18n.Lang) string {
	// нет перевода на язык — английский, потом русский (основное поле)
	for _, code := range []string{string(l), "en"} {
		if code == "ru" {
			break
		}
		if n, ok := i.Names[code]; ok && n != "" {
			return n
		}
	}
	return i.Name
}

// RecipeText — перевод названия, подводки и шагов.
type RecipeText struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
}

// Notes — короткие заметки к рецепту: по одному-двум предложениям на тему. Пустые поля не показываются.
type Notes struct {
	Why      string `json:"why,omitempty"`      // почему готовим так, а не иначе
	Swaps    string `json:"swaps,omitempty"`    // чем заменить продукты
	Mistakes string `json:"mistakes,omitempty"` // где чаще всего ошибаются
	Keep     string `json:"keep,omitempty"`     // как хранить и разогреть
	Serve    string `json:"serve,omitempty"`    // с чем подать
}

func (n Notes) Empty() bool { return n.Why == "" && n.Swaps == "" && n.Mistakes == "" && n.Keep == "" && n.Serve == "" }

type RecipeIngredient struct {
	IngredientID string  `json:"ingredientId"`
	Amount       float64 `json:"amount"` // на 1 порцию
}

type Recipe struct {
	ID          string                `json:"id"`
	Title       string                `json:"title"`
	Slot        string                `json:"slot"`
	TimeMin     int                   `json:"timeMin"`
	Equipment   []string              `json:"equipment"`
	Tags        []string              `json:"tags"`
	Batch       bool                  `json:"batch"`
	KeepDays    *int                  `json:"keep,omitempty"`   // сколько дней стоит в холодильнике готовым; nil — по правилам prep.go, 0 — есть свежим
	Freeze      bool                  `json:"freeze,omitempty"` // можно заморозить готовым
	Steps       []string              `json:"steps"`
	Ingredients []RecipeIngredient    `json:"ingredients"`
	Image       string                `json:"image"`            // URL картинки, пусто — нет фото
	Description string                `json:"description"`      // 1–2 предложения для страницы рецепта
	I18n        map[string]RecipeText `json:"i18n,omitempty"`   // en, de
	Notes       map[string]Notes      `json:"notes,omitempty"`  // заметки по языкам
	Own         bool                  `json:"own,omitempty"`    // рецепт пользователя, не из базы
	Public      bool                  `json:"public,omitempty"` // свой рецепт открыт для всех по ссылке
	OwnerID     string                `json:"-"`
	Author      string                `json:"author,omitempty"` // ник автора своего рецепта
	Status      string                `json:"status,omitempty"` // свой рецепт: private | checking | review | approved | rejected
	Note        string                `json:"note,omitempty"`   // причина от нейросети или модератора
	SubmittedAt string                `json:"submittedAt,omitempty"`
	Suggestion  *RecipeText           `json:"suggestion,omitempty"` // подробная версия от нейросети (status = improve)
	Views       int                   `json:"views,omitempty"`      // просмотры страницы своего рецепта
	Lang        string                `json:"lang,omitempty"`       // язык оригинала своего рецепта (ru по умолчанию)
}

// Text — название, подводка и шаги на языке (ru — из основных полей).
func (r Recipe) Text(l i18n.Lang) RecipeText {
	// нет перевода на язык — английский, потом русский (как у строк интерфейса)
	for _, code := range []string{string(l), "en"} {
		if code == "ru" {
			break // русский — это основные поля рецепта, не перевод
		}
		if t, ok := r.I18n[code]; ok && t.Title != "" {
			if len(t.Steps) == 0 {
				t.Steps = r.Steps
			}
			return t
		}
	}
	return RecipeText{Title: r.Title, Description: r.Description, Steps: r.Steps}
}

// LocalTitle — название на языке.
func (r Recipe) LocalTitle(l i18n.Lang) string { return r.Text(l).Title }

type Store struct {
	Code       string  `json:"code"`
	Country    string  `json:"country"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"` // convenience | supermarket | hypermarket | discounter | wholesale | premium
	PriceIndex float64 `json:"priceIndex"`
	Note       string  `json:"note"` // подпись на языке плана
	Sort       int     `json:"sort"`
}

// Catalog — вся база в памяти. Она маленькая, планировщик работает без запросов к БД.
// Ценник Росстата подменяется атомарно после каждой синхронизации.
type Catalog struct {
	Ingredients map[string]Ingredient
	Recipes     []Recipe
	RecipeByID  map[string]Recipe
	Stores      map[string]Store
	StoreList   []Store
	prices      *priceStore // общий для каталога и его копий с рецептами пользователя
}

// priceStore — ценники, которые подменяются на лету. Вынесены в указатель, чтобы копия каталога
// с рецептами пользователя (WithRecipes) видела те же обновления.
type priceStore struct {
	book  atomic.Pointer[PriceBook]
	local sync.Map // код страны → *LocalPrices
}

// NewCatalog — пустой каталог с готовыми картами и хранилищем цен.
func NewCatalog() *Catalog {
	return &Catalog{Ingredients: map[string]Ingredient{}, RecipeByID: map[string]Recipe{}, Stores: map[string]Store{}, prices: &priceStore{}}
}

func (c *Catalog) SetPriceBook(pb *PriceBook) { c.prices.book.Store(pb) }
func (c *Catalog) PriceBook() *PriceBook      { return c.prices.book.Load() }

// SetLocalPrices подменяет живой ценник страны (BY, KZ, DE, US) после синхронизации.
func (c *Catalog) SetLocalPrices(lp *LocalPrices) { c.prices.local.Store(lp.Country, lp) }

// LocalPrices — живой ценник страны или nil.
func (c *Catalog) LocalPrices(country string) *LocalPrices {
	if v, ok := c.prices.local.Load(country); ok {
		return v.(*LocalPrices)
	}
	return nil
}

// CatalogRef — атомарная ссылка на текущий каталог: сервисы берут Load() на каждый запрос,
// админка после правки рецептов кладёт новый каталог через Store().
type CatalogRef struct{ p atomic.Pointer[Catalog] }

func NewCatalogRef(c *Catalog) *CatalogRef {
	r := &CatalogRef{}
	r.p.Store(c)
	return r
}

func (r *CatalogRef) Load() *Catalog   { return r.p.Load() }
func (r *CatalogRef) Store(c *Catalog) { r.p.Store(c) }

// WithRecipes — копия каталога с добавленными рецептами пользователя. Базовые продукты, магазины
// и ценники общие; свои рецепты видны только в этой копии, поэтому в общий каталог они не попадают.
func (c *Catalog) WithRecipes(extra []Recipe) *Catalog {
	if len(extra) == 0 {
		return c
	}
	cp := *c
	cp.Recipes = make([]Recipe, 0, len(c.Recipes)+len(extra))
	cp.Recipes = append(cp.Recipes, c.Recipes...)
	cp.RecipeByID = make(map[string]Recipe, len(c.RecipeByID)+len(extra))
	for k, v := range c.RecipeByID {
		cp.RecipeByID[k] = v
	}
	for _, r := range extra {
		r.Own = true
		cp.Recipes = append(cp.Recipes, r)
		cp.RecipeByID[r.ID] = r
	}
	return &cp
}

// Params — ответы квиза.
type Params struct {
	Lang        string   `json:"lang"`    // ru | en | de — сервер подставляет из запроса
	Country     string   `json:"country"` // RU | BY | KZ | DE | US
	Store       string   `json:"store"`
	Region      string   `json:"region"` // код территории Росстата; пусто = РФ
	Adults      int      `json:"adults"`
	Members     []Member `json:"members,omitempty"` // семья: цель и аппетит каждого; пусто — Adults одинаковых
	Kids        []Child  `json:"kids"`
	Goal        string   `json:"goal"`        // lose | healthy | gain | none
	KcalTarget  float64  `json:"kcalTarget"`  // ккал в день на взрослого; 0 — по цели
	BudgetMode  string   `json:"budgetMode"`  // perPersonDay | week
	BudgetValue float64  `json:"budgetValue"` // в валюте страны
	Allergens   []string `json:"allergens"`
	Exclude     []string `json:"exclude"`     // id ингредиентов
	ExcludeTags []string `json:"excludeTags"` // spicy, offal, ...
	Equipment   []string `json:"equipment"`
	Slots       []string `json:"slots"`
	StartDate   string   `json:"startDate"`       // YYYY-MM-DD, понедельник; пусто = ближайший понедельник
	Prep        string   `json:"prep,omitempty"`  // заготовки: "" каждый день | one раз в неделю | two два раза
	Wants       []string `json:"wants,omitempty"` // продукты, которые хочется на этой неделе (id)
	// Have — что уже есть дома (id продуктов): планировщик ставит блюда с ними раньше, список покупок
	// вычитает одну типовую упаковку каждого.
	Have []string `json:"have,omitempty"`
	// Compact — «меньше разных продуктов»: неделя из повторяющихся продуктов, чтобы покупать меньше позиций
	Compact bool `json:"compact,omitempty"`
	// ExcludeRecipes — нелюбимые рецепты из личного кабинета; сервер подставляет сам, из квиза не приходит.
	ExcludeRecipes []string `json:"excludeRecipes,omitempty"`
	// Favorites — избранные рецепты из кабинета; тоже подставляет сервер. Получают бонус в подборе.
	Favorites []string `json:"favorites,omitempty"`
	// Collection — id коллекции из кабинета: неделю собираем из неё; CollectionIDs подставляет сервер.
	Collection    string   `json:"collection,omitempty"`
	CollectionIDs []string `json:"collectionIds,omitempty"`
	// Liked / Meh — ответы «как было?» после ужина (сервер): понравившееся ставим чаще, «не зашло» — реже.
	Liked []string `json:"liked,omitempty"`
	Meh   []string `json:"meh,omitempty"`
}

// Dish — одна ячейка недели.
type Dish struct {
	Slot     string  `json:"slot"`
	RecipeID string  `json:"recipeId"`
	Title    string  `json:"title"`
	TimeMin  int     `json:"timeMin"`
	Leftover bool    `json:"leftover"`         // доедаем вчерашнее
	Batch    bool    `json:"batch"`            // готовим сразу на два дня
	Own      bool    `json:"own,omitempty"`    // рецепт пользователя
	Course   string  `json:"course,omitempty"` // событие: курс (salads, mains…) вместо приёма пищи
	Kcal     float64 `json:"kcal"`             // на порцию взрослого
	Protein  float64 `json:"protein"`
	Fat      float64 `json:"fat"`
	Carb     float64 `json:"carb"`
	Cost     float64 `json:"cost"` // на порцию, без «домашних» продуктов, с индексом магазина
	Why      string  `json:"why"`  // почему выбрано: коротко и честно, на языке плана
	WhyCode  WhyCode `json:"whyCode"`
	Side     *Side   `json:"side,omitempty"` // гарнир к основному (итоги выше — суммой с ним)
	Prep     *PrepInfo `json:"prep,omitempty"` // режим заготовок: когда и как приготовлено
}

// WhyCode — из чего собрана подпись «почему»; по нему подпись пересобирается на другом языке.
type WhyCode struct {
	Reused   []string `json:"reused,omitempty"` // id продуктов, которые доедаем
	Cost     string   `json:"cost,omitempty"`   // cheaper | inbudget | pricier
	Kcal     string   `json:"kcal,omitempty"`   // kcalok | lighter | denser
	Batch    bool     `json:"batch,omitempty"`
	Leftover bool     `json:"leftover,omitempty"`
	Wanted   string   `json:"wanted,omitempty"` // id продукта из «хочется на неделе»
	Favorite bool     `json:"favorite,omitempty"`
	Compact  bool     `json:"compact,omitempty"` // из тех же продуктов, что уже в списке
}

type Day struct {
	Index     int       `json:"index"`
	Date      string    `json:"date"`
	Label     string    `json:"label"` // ПН … ВС
	Dishes    []Dish    `json:"dishes"`
	Kcal      float64   `json:"kcal"`
	Protein   float64   `json:"protein"`
	Fat       float64   `json:"fat"`
	Carb      float64   `json:"carb"`
	Cost      float64   `json:"cost"`                // на всех едоков
	CookMin   int       `json:"cookMin"`             // суммарное время у плиты
	PerMember []float64 `json:"perMember,omitempty"` // ккал на каждого едока за день (порядок как в Members)
	Skipped   bool      `json:"skipped,omitempty"`   // «не дома»: блюда не считаются в покупках и итогах
}

type ShopItem struct {
	IngredientID string   `json:"ingredientId"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Unit         string   `json:"unit"`
	Needed       float64  `json:"needed"` // сколько реально уйдёт в блюда
	Buy          float64  `json:"buy"`    // сколько купить (с округлением)
	Packs        int      `json:"packs"`  // упаковок (0 для весовых)
	Pack         float64  `json:"pack"`
	Loose        bool     `json:"loose"`
	Pantry       bool     `json:"pantry"`
	Cost         float64  `json:"cost"`
	Rosstat      bool     `json:"rosstat"`          // цена из Росстата, а не ручная
	Home         float64  `json:"home,omitempty"`   // сколько уже есть дома (вычтено из Buy)
	AtHome       bool     `json:"atHome,omitempty"` // дома хватает на всю неделю: покупать не нужно
	Image        string   `json:"image,omitempty"`
	UsedIn       []string `json:"usedIn"` // названия блюд
}

type ShopGroup struct {
	Category string     `json:"category"`
	Label    string     `json:"label"`
	Items    []ShopItem `json:"items"`
	Cost     float64    `json:"cost"`
}

type Totals struct {
	Cost          float64 `json:"cost"`         // покупки без «домашних»
	PantryCost    float64 `json:"pantryCost"`   // если покупать и специи/масло
	BabyCost      float64 `json:"babyCost"`     // смесь, баночки, каши (входит в Cost)
	KidsMenuCost  float64 `json:"kidsMenuCost"` // продукты детского меню (входят в Cost и UsedCost)
	UsedCost      float64 `json:"usedCost"`     // стоимость того, что реально съедается за неделю (без остатков упаковок)
	HomeSaved     float64 `json:"homeSaved"`    // сколько сэкономили продукты, которые уже есть дома
	Items         int     `json:"items"`
	KcalPerDay    float64 `json:"kcalPerDay"`
	ProteinPerDay float64 `json:"proteinPerDay"`
	FatPerDay     float64 `json:"fatPerDay"`
	CarbPerDay    float64 `json:"carbPerDay"`
	CookMin       int     `json:"cookMin"`
}

type Budget struct {
	Mode       string  `json:"mode"`       // perPersonDay | week
	Value      float64 `json:"value"`      // как ввёл пользователь
	PerDay     float64 `json:"perDay"`     // цель на человека в день
	TargetWeek float64 `json:"targetWeek"` // цель на всех на неделю
}

type Goal struct {
	Level      string  `json:"level"` // lose | healthy | gain | none
	Label      string  `json:"label"`
	KcalTarget float64 `json:"kcalTarget"` // на взрослого в день; 0 — без цели
}

type Plan struct {
	ID           string              `json:"id"`
	Params       Params              `json:"params"`
	Lang         string              `json:"lang"`
	Country      Country             `json:"country"`
	Store        Store               `json:"store"`
	Portions     float64             `json:"portions"`
	SlotPortions map[string]float64  `json:"slotPortions,omitempty"` // порций на приём: кто ест дома
	Members      []MemberView        `json:"members,omitempty"`
	Days         []Day               `json:"days"`
	Shopping     []ShopGroup         `json:"shopping"`
	Totals       Totals              `json:"totals"`
	Budget       Budget              `json:"budget"`
	Goal         Goal                `json:"goal"`
	PriceSource  PriceSource         `json:"priceSource"`
	KidsMenus    []KidMenu           `json:"kidsMenus"` // детские меню для детей в режиме «готовим отдельно»
	Seed         int64               `json:"seed"`
	Swaps        int                 `json:"swaps"`
	Rejected     map[string][]string `json:"rejected,omitempty"` // "день:приём" → что уже отклонили
	GeneratedAt  string              `json:"generatedAt"`
	Warnings     []string            `json:"warnings"`
	Notes        []string            `json:"notes"`              // пояснения (дети, источник цен)
	Family       []string            `json:"family,omitempty"`   // имена аккаунтов, присоединившихся к плану; ставит транспорт
	Occasion     *OccasionInfo       `json:"occasion,omitempty"` // событие вместо недели
	PrepDays     []PrepDay           `json:"prepDays,omitempty"` // режим заготовок: что готовить в дни заготовок
}

// Справочники для UI.
var SlotOrder = []string{"breakfast", "lunch", "dinner", "snack"}

// SlotLabel, CategoryLabel и остальные подписи — через i18n по ключу.
func SlotLabel(l i18n.Lang, slot string) string      { return i18n.T(l, "slot."+slot) }
func CategoryLabel(l i18n.Lang, cat string) string   { return i18n.T(l, "category."+cat) }
func DayLabel(l i18n.Lang, d int) string             { return i18n.T(l, "day."+string(rune('0'+d))) }
func AllergenLabel(l i18n.Lang, id string) string    { return i18n.T(l, "allergen."+id) }
func EquipmentLabel(l i18n.Lang, id string) string   { return i18n.T(l, "equipment."+id) }
func GoalLabel(l i18n.Lang, id string) string        { return i18n.T(l, "goal."+id) }
func StoreKindLabel(l i18n.Lang, kind string) string { return i18n.T(l, "store."+kind) }

var Allergens = []string{"gluten", "dairy", "eggs", "nuts", "peanuts", "fish", "seafood", "soy"}
var Goals = []string{"lose", "healthy", "gain", "none"}

var CategoryOrder = []string{"baby", "vegetables", "fruits", "meat", "fish", "eggs", "dairy", "grains", "bakery", "canned", "frozen", "nuts", "pantry", "spices"}

var EquipmentOrder = []string{"stove", "oven", "microwave", "airfryer", "multicooker", "blender", "mixer", "grill", "steamer", "meatgrinder"}

// Что чем можно заменить: рецепт «для духовки» готовится и в аэрогриле.
var EquipmentAlternatives = map[string][]string{
	"oven":    {"airfryer"},
	"blender": {"mixer"},
	"steamer": {"multicooker"},
}

// Ориентир ккал в день на взрослого по цели. Это стартовое значение — пользователь может вписать своё.
var GoalKcal = map[string]float64{
	"lose":    1500,
	"healthy": 1900,
	"gain":    2600,
	"none":    0,
}

// BudgetPreset — подсказка бюджета на человека в день в валюте страны.
type BudgetPreset struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	PerDay float64 `json:"perDay"`
}

func BudgetPresetsFor(l i18n.Lang, c Country) []BudgetPreset {
	ids := []string{"economy", "medium", "free", "premium"}
	out := make([]BudgetPreset, 4)
	for i, id := range ids {
		out[i] = BudgetPreset{ID: id, Label: i18n.T(l, "budget."+id), PerDay: c.Presets[i]}
	}
	return out
}

// NotesFor — заметки на языке с запасом en → ru (как Text).
func (r Recipe) NotesFor(l i18n.Lang) Notes {
	if n, ok := r.Notes[string(l)]; ok && !n.Empty() {
		return n
	}
	for _, fb := range []string{"en", "ru"} {
		if n, ok := r.Notes[fb]; ok && !n.Empty() {
			return n
		}
	}
	return Notes{}
}
