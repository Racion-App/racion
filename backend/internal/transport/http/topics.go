package http

import (
	"bytes"
	"encoding/json"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Страницы «Что приготовить из …»: /cook/{slug} и /{lang}/cook/{slug}. Собираются из каталога по продуктам темы:
// группы по приёмам пищи, разрезы по способу готовки, факты, текст и FAQ из данных. Языки: ru, en, de —
// у остальных нет своих названий продуктов, и страница вышла бы наполовину английской.

type topic struct {
	Slug string
	IDs  []string          // продукты темы
	Of   map[string]string // «из курицы» | «with chicken» | «mit Hähnchen»
	Name map[string]string // «Курица» для хаба и крошек
}

var topicLangs = []i18n.Lang{i18n.RU, i18n.EN, i18n.DE}

var topics = []topic{
	{"chicken", []string{"chicken_breast", "chicken_thigh", "chicken_drumstick", "chicken_whole", "chicken_wings", "chicken_legs", "ground_chicken", "chicken_mince_breast", "chicken_smoked"}, ofs("из курицы", "with chicken", "mit Hähnchen"), ofs("Курица", "Chicken", "Hähnchen")},
	{"chicken-breast", []string{"chicken_breast", "chicken_mince_breast"}, ofs("из куриной грудки", "with chicken breast", "mit Hähnchenbrust"), ofs("Куриная грудка", "Chicken breast", "Hähnchenbrust")},
	{"mince", []string{"ground_beef", "ground_pork", "ground_mixed", "ground_chicken", "turkey_ground", "beef_ground_lean", "ground_lamb", "chicken_mince_breast"}, ofs("из фарша", "with mince", "mit Hackfleisch"), ofs("Фарш", "Mince", "Hackfleisch")},
	{"potato", []string{"potato", "potato_baby"}, ofs("из картошки", "with potatoes", "mit Kartoffeln"), ofs("Картошка", "Potatoes", "Kartoffeln")},
	{"zucchini", []string{"zucchini"}, ofs("из кабачков", "with zucchini", "mit Zucchini"), ofs("Кабачки", "Zucchini", "Zucchini")},
	{"cottage-cheese", []string{"cottage_cheese", "cottage_cheese_9", "cottage_cheese_0", "cottage_soft"}, ofs("из творога", "with cottage cheese", "mit Quark"), ofs("Творог", "Cottage cheese", "Quark")},
	{"pumpkin", []string{"pumpkin"}, ofs("из тыквы", "with pumpkin", "mit Kürbis"), ofs("Тыква", "Pumpkin", "Kürbis")},
	{"apples", []string{"apple"}, ofs("из яблок", "with apples", "mit Äpfeln"), ofs("Яблоки", "Apples", "Äpfel")},
	{"eggplant", []string{"eggplant"}, ofs("из баклажанов", "with eggplant", "mit Auberginen"), ofs("Баклажаны", "Eggplant", "Auberginen")},
	{"bananas", []string{"banana"}, ofs("из бананов", "with bananas", "mit Bananen"), ofs("Бананы", "Bananas", "Bananen")},
	{"avocado", []string{"avocado"}, ofs("из авокадо", "with avocado", "mit Avocado"), ofs("Авокадо", "Avocado", "Avocado")},
	{"cabbage", []string{"cabbage", "cabbage_peking", "cabbage_red"}, ofs("из капусты", "with cabbage", "mit Kohl"), ofs("Капуста", "Cabbage", "Kohl")},
	{"cauliflower", []string{"cauliflower", "cauliflower_fresh"}, ofs("из цветной капусты", "with cauliflower", "mit Blumenkohl"), ofs("Цветная капуста", "Cauliflower", "Blumenkohl")},
	{"broccoli", []string{"broccoli", "broccoli_fresh"}, ofs("из брокколи", "with broccoli", "mit Brokkoli"), ofs("Брокколи", "Broccoli", "Brokkoli")},
	{"mushrooms", []string{"mushrooms", "mushrooms_forest", "mushrooms_oyster", "mushrooms_can", "mushrooms_dried"}, ofs("из грибов", "with mushrooms", "mit Pilzen"), ofs("Грибы", "Mushrooms", "Pilze")},
	{"pork", []string{"pork_neck", "pork_loin", "pork_shoulder", "pork_tenderloin", "pork_belly", "pork_chops", "pork_ribs", "pork_shashlik"}, ofs("из свинины", "with pork", "mit Schweinefleisch"), ofs("Свинина", "Pork", "Schweinefleisch")},
	{"beef", []string{"beef_stew", "beef_tenderloin", "beef_ribs", "beef_brisket", "beef_shank", "ribeye", "veal"}, ofs("из говядины", "with beef", "mit Rindfleisch"), ofs("Говядина", "Beef", "Rindfleisch")},
	{"turkey", []string{"turkey_fillet", "turkey_thigh", "turkey_drumstick", "turkey_ground"}, ofs("из индейки", "with turkey", "mit Pute"), ofs("Индейка", "Turkey", "Pute")},
	{"fish", []string{"salmon_fillet", "pollock_fillet", "hake_fillet", "pink_salmon", "cod_fillet", "tilapia", "pangasius", "trout", "perch_sea", "carp", "pike_perch", "flounder", "halibut", "dorado", "seabass", "mackerel", "tuna_steak", "salmon_steak"}, ofs("из рыбы", "with fish", "mit Fisch"), ofs("Рыба", "Fish", "Fisch")},
	{"salmon", []string{"salmon_fillet", "salmon_steak", "salmon_salted", "trout", "trout_salted", "pink_salmon"}, ofs("из лосося", "with salmon", "mit Lachs"), ofs("Лосось", "Salmon", "Lachs")},
	{"pollock-cod", []string{"pollock_fillet", "hake_fillet", "cod_fillet"}, ofs("из минтая и трески", "with pollock and cod", "mit Seelachs und Kabeljau"), ofs("Минтай и треска", "Pollock and cod", "Seelachs und Kabeljau")},
	{"shrimp", []string{"shrimp", "king_prawns"}, ofs("из креветок", "with shrimp", "mit Garnelen"), ofs("Креветки", "Shrimp", "Garnelen")},
	{"squid", []string{"squid", "squid_rings"}, ofs("из кальмаров", "with squid", "mit Tintenfisch"), ofs("Кальмары", "Squid", "Tintenfisch")},
	{"tuna", []string{"tuna_can"}, ofs("из консервированного тунца", "with canned tuna", "mit Thunfisch aus der Dose"), ofs("Консервированный тунец", "Canned tuna", "Thunfisch")},
	{"eggs", []string{"eggs"}, ofs("из яиц", "with eggs", "mit Eiern"), ofs("Яйца", "Eggs", "Eier")},
	{"rice", []string{"rice", "rice_round", "rice_basmati", "rice_brown", "arborio"}, ofs("из риса", "with rice", "mit Reis"), ofs("Рис", "Rice", "Reis")},
	{"buckwheat", []string{"buckwheat"}, ofs("из гречки", "with buckwheat", "mit Buchweizen"), ofs("Гречка", "Buckwheat", "Buchweizen")},
	{"pasta", []string{"pasta", "spaghetti", "wholegrain_pasta", "lasagna_sheets"}, ofs("из макарон", "with pasta", "mit Nudeln"), ofs("Макароны", "Pasta", "Nudeln")},
	{"lavash", []string{"lavash"}, ofs("из лаваша", "with lavash", "mit Fladenbrot"), ofs("Лаваш", "Lavash", "Fladenbrot")},
	{"cheese", []string{"cheese_hard", "mozzarella", "cheese_feta", "parmesan", "cheese_suluguni", "cheese_cheddar", "cheese_brie", "blue_cheese", "cheese_goat", "cheese_adygei", "halloumi", "cheese_processed", "cheese_russian", "cheese_gouda", "cheese_smoked"}, ofs("из сыра", "with cheese", "mit Käse"), ofs("Сыр", "Cheese", "Käse")},
	{"kefir", []string{"kefir", "kefir_1"}, ofs("из кефира", "with kefir", "mit Kefir"), ofs("Кефир", "Kefir", "Kefir")},
	{"oats", []string{"oats", "oat_flour", "oat_bran"}, ofs("из овсянки", "with oats", "mit Haferflocken"), ofs("Овсянка", "Oats", "Haferflocken")},
	{"lentils", []string{"lentils", "lentils_green", "lentils_can"}, ofs("из чечевицы", "with lentils", "mit Linsen"), ofs("Чечевица", "Lentils", "Linsen")},
	{"chickpeas", []string{"chickpeas_can", "chickpeas_dry", "hummus"}, ofs("из нута", "with chickpeas", "mit Kichererbsen"), ofs("Нут", "Chickpeas", "Kichererbsen")},
	{"beans", []string{"beans_can", "beans_dry", "beans_white_can", "beans_tomato_can", "green_beans", "green_beans_fresh"}, ofs("из фасоли", "with beans", "mit Bohnen"), ofs("Фасоль", "Beans", "Bohnen")},
	{"tofu", []string{"tofu"}, ofs("из тофу", "with tofu", "mit Tofu"), ofs("Тофу", "Tofu", "Tofu")},
	{"liver", []string{"liver_chicken", "beef_liver"}, ofs("из печени", "with liver", "mit Leber"), ofs("Печень", "Liver", "Leber")},
	{"sausages", []string{"sausages", "sardelki", "chicken_sausages", "hunting_sausages"}, ofs("из сосисок", "with sausages", "mit Würstchen"), ofs("Сосиски", "Sausages", "Würstchen")},
	{"tomatoes", []string{"tomato", "cherry_tomatoes", "tomatoes_can"}, ofs("из помидоров", "with tomatoes", "mit Tomaten"), ofs("Помидоры", "Tomatoes", "Tomaten")},
	{"peppers", []string{"bell_pepper"}, ofs("из болгарского перца", "with bell peppers", "mit Paprika"), ofs("Болгарский перец", "Bell peppers", "Paprika")},
	{"beets", []string{"beet", "beet_boiled"}, ofs("из свёклы", "with beets", "mit Roter Bete"), ofs("Свёкла", "Beets", "Rote Bete")},
	{"spinach", []string{"spinach_fresh", "spinach_frozen"}, ofs("из шпината", "with spinach", "mit Spinat"), ofs("Шпинат", "Spinach", "Spinat")},
	{"berries", []string{"berries_frozen", "berries_fresh_mix", "strawberry", "blueberry", "raspberry", "cranberry_frozen", "cherries_frozen"}, ofs("из ягод", "with berries", "mit Beeren"), ofs("Ягоды", "Berries", "Beeren")},
	{"chocolate", []string{"dark_chocolate", "chocolate_milk", "cocoa"}, ofs("из шоколада", "with chocolate", "mit Schokolade"), ofs("Шоколад", "Chocolate", "Schokolade")},
	{"puff-pastry", []string{"puff_pastry"}, ofs("из слоёного теста", "with puff pastry", "mit Blätterteig"), ofs("Слоёное тесто", "Puff pastry", "Blätterteig")},
}

func ofs(ru, en, de string) map[string]string { return map[string]string{"ru": ru, "en": en, "de": de} }

func topicBySlug(slug string) (topic, bool) {
	for _, t := range topics {
		if t.Slug == slug {
			return t, true
		}
	}
	return topic{}, false
}

// topicAmount — сколько продуктов темы в рецепте на порцию, в граммах (штуки считаем по 50 г):
// четверть яйца в тесте или ложка сыра сверху не делают блюдо «из яиц» или «из сыра».
func (s *Server) topicAmount(t topic, rc planner.Recipe) float64 {
	var sum float64
	for _, ri := range rc.Ingredients {
		if !slices.Contains(t.IDs, ri.IngredientID) {
			continue
		}
		if ing, ok := s.catalog.Ingredients[ri.IngredientID]; ok && ing.Unit == "pcs" {
			sum += ri.Amount * 50
		} else {
			sum += ri.Amount
		}
	}
	return sum
}

// topicRecipes — видимые рецепты базы, где продукта темы не меньше 40 г на порцию; сортировка по времени.
func (s *Server) topicRecipes(t topic) []planner.Recipe {
	var out []planner.Recipe
	for _, rc := range s.catalog.Recipes {
		if rc.Hidden || s.topicAmount(t, rc) < 40 {
			continue
		}
		out = append(out, rc)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TimeMin < out[j].TimeMin })
	return out
}

// topicCover — фото блюда, где продукта темы больше всего: обложка страницы и хаба.
func (s *Server) topicCover(t topic, rs []planner.Recipe) string {
	best, cover := 0.0, ""
	for _, rc := range rs {
		if a := s.topicAmount(t, rc); rc.Image != "" && a > best {
			best, cover = a, rc.Image
		}
	}
	return cover
}

func topicLang(l i18n.Lang) bool { return slices.Contains(topicLangs, l) }

// cutList — разрез списка карточек: «в духовке», «пп», «быстро»…
type cutList struct {
	Key, Title string
	Cards      []recipeCard
	Total      int
}

func (s *Server) cookPage(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.localeFromPath(r)
	if !ok || !topicLang(pl.L) {
		s.notFoundPage(w, r)
		return
	}
	t, ok := topicBySlug(r.PathValue("slug"))
	if !ok {
		s.notFoundPage(w, r)
		return
	}
	rememberCountry(w, r)
	l := pl.L
	recipes := s.topicRecipes(t)
	if len(recipes) < 4 {
		s.notFoundPage(w, r)
		return
	}
	cards := make([]recipeCard, 0, len(recipes))
	for _, rc := range recipes {
		cards = append(cards, s.card(rc, pl))
	}
	of := t.Of[string(l)]
	name := t.Name[string(l)]
	h1 := i18n.T(l, "cook.h1", of)
	base := s.baseURL(r)

	// группы по приёмам пищи, в каждой не больше 40 карточек: у курицы рецептов сотни
	const capPerSlot = 40
	type group struct {
		Key, Title string
		Cards      []recipeCard
		Total      int
	}
	var groups []group
	bySlot := map[string]int{}
	for _, slot := range planner.SlotOrder {
		var g []recipeCard
		for _, c := range cards {
			if c.Slot == slot {
				g = append(g, c)
			}
		}
		bySlot[slot] = len(g)
		if len(g) == 0 {
			continue
		}
		total := len(g)
		if len(g) > capPerSlot {
			g = g[:capPerSlot]
		}
		groups = append(groups, group{slot, planner.SlotLabel(l, slot), g, total})
	}

	// разрезы по способу: духовка, плита без духовки, аэрогриль, пп, быстро, детям
	byID := map[string]planner.Recipe{}
	for _, rc := range recipes {
		byID[rc.ID] = rc
	}
	needs := func(rc planner.Recipe, eq string) bool { return slices.Contains(rc.Equipment, eq) }
	cutDefs := []struct {
		key  string
		pick func(c recipeCard) bool
	}{
		{"oven", func(c recipeCard) bool { return needs(byID[c.ID], "oven") }},
		{"stove", func(c recipeCard) bool { rc := byID[c.ID]; return !needs(rc, "oven") && !needs(rc, "airfryer") }},
		{"airfryer", func(c recipeCard) bool { return needs(byID[c.ID], "airfryer") || slices.Contains(c.Tags, "airfryer") }},
		{"pp", func(c recipeCard) bool { return slices.Contains(c.Tags, "pp") }},
		{"quick", func(c recipeCard) bool { return c.TimeMin <= 20 }},
		{"kids", func(c recipeCard) bool { return c.Kid || slices.Contains(c.Tags, "kids") }},
	}
	var cuts []cutList
	cutCount := map[string]int{}
	for _, d := range cutDefs {
		var g []recipeCard
		for _, c := range cards {
			if d.pick(c) {
				g = append(g, c)
			}
		}
		cutCount[d.key] = len(g)
		if len(g) < 3 {
			continue
		}
		total := len(g)
		if len(g) > 12 {
			g = g[:12]
		}
		cuts = append(cuts, cutList{d.key, i18n.T(l, "cook.cut."+d.key), g, total})
	}

	// факты и текст из данных
	minC, maxC, minT, maxT := cards[0].Cost, cards[0].Cost, cards[0].TimeMin, cards[0].TimeMin
	cheapest, quickest := cards[0], cards[0]
	for _, c := range cards {
		if c.Cost > 0 && (minC == 0 || c.Cost < minC) {
			minC = c.Cost
			cheapest = c
		}
		maxC = math.Max(maxC, c.Cost)
		if c.TimeMin < minT {
			minT = c.TimeMin
			quickest = c
		}
		maxT = max(maxT, c.TimeMin)
	}
	type fact struct{ Value, Label string }
	facts := []fact{{strconv.Itoa(len(cards)), i18n.Plural(l, len(cards), "catalog.recipe")}}
	if maxC > 0 {
		facts = append(facts, fact{rangeLabel(formatMoney(pl.Country, minC), formatMoney(pl.Country, maxC)), i18n.T(l, "coll.fact.cost")})
	}
	facts = append(facts, fact{i18n.Minutes(l, minT) + " – " + i18n.Minutes(l, maxT), i18n.T(l, "coll.fact.time")})

	n := len(cards)
	intro := []string{
		i18n.T(l, "cook.intro1", n, i18n.Plural(l, n, "catalog.recipe"), of, bySlot["breakfast"], bySlot["lunch"], bySlot["dinner"], bySlot["snack"]),
		i18n.T(l, "cook.intro2", quickest.Title, i18n.Minutes(l, quickest.TimeMin), cheapest.Title, formatMoney(pl.Country, cheapest.Cost), cutCount["oven"], cutCount["airfryer"], cutCount["pp"], cutCount["quick"]),
	}
	// FAQ: только те разрезы, где есть из чего выбирать
	var faq []domain.QA
	names := func(cs []recipeCard, k int) string {
		var parts []string
		for i, c := range cs {
			if i >= k {
				break
			}
			parts = append(parts, c.Title)
		}
		return strings.Join(parts, ", ")
	}
	for _, c := range cuts {
		if c.Key == "kids" || c.Key == "airfryer" && c.Total < 5 {
			continue
		}
		faq = append(faq, domain.QA{Q: i18n.T(l, "cook.faq."+c.Key+".q", of), A: i18n.T(l, "cook.faq."+c.Key+".a", names(c.Cards, 5), c.Total)})
	}
	for _, slot := range []string{"dinner", "breakfast"} {
		var g []recipeCard
		for _, c := range cards {
			if c.Slot == slot {
				g = append(g, c)
			}
		}
		if len(g) >= 3 {
			faq = append(faq, domain.QA{Q: i18n.T(l, "cook.faq."+slot+".q", of), A: i18n.T(l, "cook.faq."+slot+".a", names(g, 5), len(g))})
		}
	}

	// другие темы, кроме текущей, с числом рецептов
	type topicLink struct {
		Name, Href string
		Count      int
	}
	var others []topicLink
	for _, o := range topics {
		if o.Slug == t.Slug {
			continue
		}
		if c := len(s.topicRecipes(o)); c >= 4 {
			others = append(others, topicLink{o.Name[string(l)], pl.P + "/cook/" + o.Slug, c})
		}
	}
	var alts []altLink
	for _, al := range topicLangs {
		m := i18n.Meta(al)
		alts = append(alts, altLink{Lang: string(al), Href: base + prefix(al) + "/cook/" + t.Slug, Name: m.Name, English: m.English, Flag: m.Flag})
	}
	title := i18n.T(l, "cook.title", of, n, i18n.Plural(l, n, "catalog.recipe"))
	desc := i18n.T(l, "cook.desc", n, i18n.Plural(l, n, "catalog.recipe"), of, bySlot["breakfast"], bySlot["lunch"], bySlot["dinner"], bySlot["snack"])
	cover := s.topicCover(t, recipes)
	ldCards := cards
	if len(ldCards) > 100 {
		ldCards = ldCards[:100]
	}
	data := map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: title + " — " + i18n.T(l, "page.brand"), Description: desc, Canonical: base + pl.P + "/cook/" + t.Slug,
			OGImage: topicOG(base, cover, l), OGType: "article", OGWide: cover == "", Alternates: alts, JSONLD: topicLD(base, pl, h1, desc, t.Slug, ldCards, faq)},
		"L": l, "P": pl.P, "Country": pl.Country, "NavRecipes": true,
		"H1": h1, "Name": name, "Cover": cover, "Facts": facts, "Intro": intro, "Groups": groups, "Cuts": cuts, "FAQ": faq, "Others": others,
		"CatalogHref": pl.P + "/recipes?q=" + url.QueryEscape(name),
	}
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "topic.html", data); err != nil {
		s.log.Error("topic page", zap.Error(err))
		http.Error(w, "template error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = buf.WriteTo(w)
}

// topicOG — превью: фото первого рецепта или брендовая картинка.
func topicOG(base, cover string, l i18n.Lang) string {
	if cover == "" {
		return brandOG(base, l)
	}
	return ogImage(base, cover)
}

// cookHubPage — /cook: все темы с числом рецептов.
func (s *Server) cookHubPage(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.localeFromPath(r)
	if !ok || !topicLang(pl.L) {
		s.notFoundPage(w, r)
		return
	}
	l := pl.L
	base := s.baseURL(r)
	type item struct {
		Name, Of, Href, Cover string
		Count                 int
	}
	var items []item
	total := 0
	for _, t := range topics {
		rs := s.topicRecipes(t)
		if len(rs) < 4 {
			continue
		}
		cover := s.topicCover(t, rs)
		items = append(items, item{t.Name[string(l)], t.Of[string(l)], pl.P + "/cook/" + t.Slug, cover, len(rs)})
		total += len(rs)
	}
	var alts []altLink
	for _, al := range topicLangs {
		m := i18n.Meta(al)
		alts = append(alts, altLink{Lang: string(al), Href: base + prefix(al) + "/cook", Name: m.Name, English: m.English, Flag: m.Flag})
	}
	data := map[string]any{
		"Base": pageBase{User: currentUser(r) != nil, Title: i18n.T(l, "cook.hub.title") + " — " + i18n.T(l, "page.brand"), Description: i18n.T(l, "cook.hub.desc", len(items)), Canonical: base + pl.P + "/cook", OGImage: brandOG(base, l), OGWide: true, Alternates: alts},
		"L":    l, "P": pl.P, "Country": pl.Country, "NavRecipes": true, "Items": items, "Total": len(items),
	}
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "cook.html", data); err != nil {
		s.log.Error("cook hub", zap.Error(err))
		http.Error(w, "template error", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = buf.WriteTo(w)
}

func topicLD(base string, pl pageLocale, name, desc, slug string, cards []recipeCard, faq []domain.QA) template.JS {
	items := make([]map[string]any, 0, len(cards))
	for i, c := range cards {
		items = append(items, map[string]any{"@type": "ListItem", "position": i + 1, "url": base + pl.P + "/recipe/" + c.ID, "name": c.Title})
	}
	u := base + pl.P + "/cook/" + slug
	list := map[string]any{"@type": "ItemList", "@id": u + "#list", "url": u, "name": name, "description": desc, "numberOfItems": len(cards), "itemListElement": items, "inLanguage": string(pl.L)}
	crumbs := breadcrumbLD([][2]string{{i18n.T(pl.L, "page.brand"), base + "/"}, {i18n.T(pl.L, "cook.hub.title"), base + pl.P + "/cook"}, {name, ""}})
	graph := []any{list, crumbs}
	if len(faq) > 0 {
		var qs []map[string]any
		for _, x := range faq {
			qs = append(qs, map[string]any{"@type": "Question", "name": x.Q, "acceptedAnswer": map[string]any{"@type": "Answer", "text": x.A}})
		}
		graph = append(graph, map[string]any{"@type": "FAQPage", "mainEntity": qs})
	}
	b, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph})
	return template.JS(b)
}

// topicsFor — темы, в которые попадает рецепт: для блока «Что приготовить из …» на странице рецепта.
func (s *Server) topicsFor(rc planner.Recipe, l i18n.Lang, p string) []FootLink {
	if !topicLang(l) {
		return nil
	}
	var out []FootLink
	for _, t := range topics {
		if s.topicAmount(t, rc) >= 40 {
			out = append(out, FootLink{Name: i18n.T(l, "cook.h1", t.Of[string(l)]), Href: p + "/cook/" + t.Slug})
		}
		if len(out) >= 4 {
			break
		}
	}
	return out
}
