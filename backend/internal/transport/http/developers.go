package http

import (
	"net/http"

	"racion/internal/i18n"
	"racion/internal/planner"
)

// devPage — /developers: страница для тех, кто хочет встроить «Рацион» в своё приложение или ассистента.
// Машиночитаемое описание лежит в /openapi.json, здесь то же самое человеческими словами: ключ не нужен,
// какие лимиты, откуда берутся цены, как подключить MCP.
func (s *Server) devPage(w http.ResponseWriter, r *http.Request) {
	pl, _ := s.localeFromPath(r)
	base := s.baseURL(r)
	t := devTextsEN
	// юридические и технические страницы у нас двуязычные: понятный английский лучше машинного
	// перевода на двенадцать языков
	if pl.L == i18n.RU || pl.L == "uk" || pl.L == "kk" {
		t = devTextsRU
	}
	email := legalEmail
	if email == "" {
		email = "info@racion.app"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = pageTpl.ExecuteTemplate(w, "developers.html", map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: t.Title + " — " + i18n.T(pl.L, "page.brand"), Description: t.Lead,
			Canonical: base + pl.P + "/developers", OGImage: brandOG(base, pl.L), OGWide: true, Alternates: s.alternates(r, "/developers")},
		"L": pl.L, "P": pl.P, "Country": pl.Country,
		"Title": t.Title, "Lead": t.Lead, "T": t, "BaseURL": base, "Email": email,
		"Recipes": len(s.catalog.Recipes), "Countries": len(planner.Countries),
	})
}

// devTexts — тексты страницы для разработчиков.
type devTexts struct {
	Title, Lead                                                 string
	NoKey, NoKeyVal, Limit, Recipes, Countries                  string
	KeyTitle, KeyBody                                           string
	StartTitle, StartBody, PlanBody, PlanAfter                  string
	MethodsTitle, ColMethod, ColWhat, SpecNote                  string
	MMeta, MRecipes, MRecipe, MCollections, MOccasions          string
	MIngredients, MPlan, MGetPlan, MBasket, MOccasionPlan       string
	LimitsTitle, LimitsBody, PricesTitle, PricesBody            string
	McpTitle, McpBody, McpTools, TermsTitle, TermsBody, Contact string
}

var devTextsRU = devTexts{
	Title:         "API и MCP",
	Lead:          "Меню на неделю с ценами магазина и список покупок — из вашего приложения или из ассистента. Ключ не нужен, регистрация тоже.",
	NoKey:         "Ключ",
	NoKeyVal:      "не нужен",
	Limit:         "Запросов в минуту",
	Recipes:       "Рецептов",
	Countries:     "Стран",
	KeyTitle:      "Где взять ключ",
	KeyBody:       "Нигде: публичное API работает без ключа и без аккаунта, просто отправьте запрос. Ключи вида rk_ существуют, но они нужны для записи рецептов в базу и выдаются только администраторам сервиса. Для чтения и сборки планов они не нужны, и лимиты для всех остальных считаются одинаково.",
	StartTitle:    "С чего начать",
	StartBody:     "Найти рецепты по словам в названии и составе. У каждого сразу время, калорийность и цена порции в валюте выбранной страны.",
	PlanBody:      "Главный вызов — собрать неделю. Отправьте, кто ест, где покупает и чего не ест, получите семь дней блюд и один список покупок, округлённый до упаковок.",
	PlanAfter:     "В ответе есть id: по адресу /plan/<id> открывается та же неделя страницей для человека — её можно распечатать, отметить купленное в магазине и отправить ссылкой. Показывайте ссылку, а не пересказ списка.",
	MethodsTitle:  "Что умеет",
	ColMethod:     "Метод",
	ColWhat:       "Что делает",
	MMeta:         "Страны, магазины с индексом цен, аллергены и пресеты ограничений",
	MRecipes:      "Каталог с поиском и постраничностью",
	MRecipe:       "Рецепт целиком: граммовки на порцию, шаги, КБЖУ, цена",
	MCollections:  "Подборки и списки их рецептов",
	MOccasions:    "Праздничные столы, которые можно собрать на число гостей",
	MIngredients:  "Справочник продуктов: единицы, размеры упаковок, КБЖУ на 100 г",
	MPlan:         "Собрать неделю со списком покупок",
	MGetPlan:      "Прочитать собранный ранее план",
	MBasket:       "Собрать стол из блюд, которые вы выбрали сами, у каждого своё число порций",
	MOccasionPlan: "Собрать праздничный стол на число гостей",
	SpecNote:      "полное описание в формате OpenAPI 3.1, его понимают генераторы клиентов и ассистенты.",
	LimitsTitle:   "Ограничения",
	LimitsBody:    "На адрес: 600 запросов в минуту ко всему /api/, из них 60 в минуту на сборку планов. Разрешены всплески до 100 и 20 запросов подряд. При превышении приходит 429 и заголовок Retry-After. CORS открыт, поэтому вызывать можно прямо из браузера. Нужно больше — напишите, договоримся.",
	PricesTitle:   "Откуда цены",
	PricesBody:    "Это оценка, а не ценник магазина. Основа — официальная статистика: Росстат в России, уровни цен Евростата с индексом HICP в Европе, BLS в США. Сверху множитель сети, поэтому дискаунтер и премиальный супермаркет дают разные суммы. В каждом ответе есть поле priceSource с источником и периодом — показывайте его рядом с цифрой, если публикуете сумму.",
	McpTitle:      "MCP-сервер",
	McpBody:       "Если вы работаете в Claude, ChatGPT или другом клиенте с поддержкой MCP, подключите сервер racion-mcp — и ассистент сможет планировать неделю сам. Ключ ему тоже не нужен.",
	McpTools:      "Шесть инструментов: собрать неделю, собрать стол из выбранных блюд, поиск рецептов, рецепт целиком, подборки, магазины со страновым индексом цен.",
	TermsTitle:    "Условия",
	TermsBody:     "Делать продукт поверх API можно. Перепубликовывать базу рецептов и фотографии целиком или большими кусками — нет. Подробности в разделе о правах:",
	Contact:       "Вопросы, увеличение лимитов, партнёрство —",
}

var devTextsEN = devTexts{
	Title:         "API and MCP",
	Lead:          "Weekly meal plans with real store prices and one shopping list, from your own app or from an assistant. No key, no account.",
	NoKey:         "Key",
	NoKeyVal:      "not needed",
	Limit:         "Requests a minute",
	Recipes:       "Recipes",
	Countries:     "Countries",
	KeyTitle:      "Where to get a key",
	KeyBody:       "Nowhere: the public API works without a key and without an account, just send the request. Keys starting with rk_ do exist, but they write recipes into the database and are issued to service administrators only. Reading and building plans needs none, and the limits are the same for everybody else.",
	StartTitle:    "Getting started",
	StartBody:     "Search recipes by words in the title and the ingredients. Each result already carries the time, the calories and the cost of one portion in the currency of the country you asked for.",
	PlanBody:      "The main call builds a week. Send who eats, where they shop and what they will not eat; get seven days of dishes and one shopping list rounded up to whole packs.",
	PlanAfter:     "The answer carries an id, and /plan/<id> opens the same week as a page for a person — printable, tickable in the shop, shareable by link. Give the link rather than retelling the list.",
	MethodsTitle:  "What it does",
	ColMethod:     "Method",
	ColWhat:       "What it does",
	MMeta:         "Countries, stores with their price index, allergens and exclusion presets",
	MRecipes:      "Catalogue with search and paging",
	MRecipe:       "Full recipe: grams per portion, steps, macros, cost",
	MCollections:  "Curated collections and their recipe ids",
	MOccasions:    "Holiday tables that can be built for a number of guests",
	MIngredients:  "Ingredient reference: units, pack sizes, nutrition per 100 g",
	MPlan:         "Build a week with a shopping list",
	MGetPlan:      "Read a plan built earlier",
	MBasket:       "Build a table from dishes you picked, each with its own number of servings",
	MOccasionPlan: "Build a holiday table for a number of guests",
	SpecNote:      "the full description in OpenAPI 3.1, understood by client generators and assistants.",
	LimitsTitle:   "Limits",
	LimitsBody:    "Per address: 600 requests a minute to /api/ overall, of which 60 a minute may build plans. Bursts of 100 and 20 are allowed. Over the limit the answer is 429 with a Retry-After header. CORS is open, so the API can be called straight from a browser. Need more? Write to us.",
	PricesTitle:   "Where the prices come from",
	PricesBody:    "They are estimates, not shelf prices. The base is official statistics: Rosstat in Russia, Eurostat price levels with HICP in Europe, BLS in the United States. A per-chain index is applied on top, so a discounter and a premium supermarket give different totals. Every answer carries a priceSource field with the source and the period — show it next to the figure if you publish one.",
	McpTitle:      "MCP server",
	McpBody:       "If you work in Claude, ChatGPT or another MCP client, connect the racion-mcp server and the assistant can plan the week itself. It needs no key either.",
	McpTools:      "Six tools: build a week, build a table from chosen dishes, search recipes, get a recipe, list collections, list stores with the country price index.",
	TermsTitle:    "Terms",
	TermsBody:     "Building a product on the API is fine. Republishing the recipe corpus or the photos, whole or in large parts, is not. Details in the rights section:",
	Contact:       "Questions, higher limits, partnership —",
}
