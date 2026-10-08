package http

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Две страницы-подборки банок: /preserves — закрутки на зиму, /quick-pickles — быстрые маринады без
// закатки. Каждый рецепт в них рассчитан на одну банку (Recipe.Jar литров), в неделю и общий каталог
// банки не попадают — их дом здесь. Разделы и порядок рецептов — jars_sections.json, его пишет
// scripts/recipes/preserves_list.py. Языки — как у тематических страниц (ru, en, de).

//go:embed jars_sections.json
var jarSectionsJSON []byte

type jarSectionDef struct {
	ID      string   `json:"id"`
	Recipes []string `json:"recipes"`
}

var jarSections = func() map[string][]jarSectionDef {
	var m map[string][]jarSectionDef
	if err := json.Unmarshal(jarSectionsJSON, &m); err != nil {
		panic("jars_sections.json: " + err.Error())
	}
	return m
}()

// jarHub — одна из двух страниц. Key — раздел в jars_sections.json и префикс текстов jars.<key>.*.
type jarHub struct{ Key, Tag, Path string }

var (
	hubPreserves = jarHub{"preserves", "preserve", "/preserves"}
	hubQuick     = jarHub{"quick", "quickpickle", "/quick-pickles"}
	jarHubs      = []jarHub{hubPreserves, hubQuick}
)

// seasonRow — строка календаря заготовок: когда продукт в сезоне в средней полосе. Ings — продукты строки:
// по первому продукту рецепта собирается «что закрыть сейчас».
type seasonRow struct {
	Key, Image, Section string
	Months              []int
	Ings                []string
}

// Календарь — с июня по ноябрь: в остальные месяцы закрывают из замороженного, и столбцы были бы пустыми.
const seasonFrom, seasonTo = 6, 11

var seasonRows = []seasonRow{
	{"greens", "dill", "other", []int{6, 7, 8}, []string{"dill", "parsley", "green_onion"}},
	{"strawberry", "strawberry", "jam", []int{6, 7}, []string{"strawberry"}},
	{"cherry", "cherry_fresh", "jam", []int{7}, []string{"cherry_fresh"}},
	{"berries", "currant_black", "jam", []int{7, 8}, []string{"currant_black", "currant_red", "raspberry", "gooseberry"}},
	{"cucumbers", "cucumber", "cucumbers", []int{7, 8}, []string{"cucumber"}},
	{"zucchini", "zucchini", "salads", []int{7, 8, 9}, []string{"zucchini", "eggplant"}},
	{"stone", "apricot", "jam", []int{7, 8, 9}, []string{"apricot", "plum"}},
	{"tomatoes", "tomato", "tomatoes", []int{8, 9}, []string{"tomato"}},
	{"pepper", "bell_pepper", "salads", []int{8, 9}, []string{"bell_pepper"}},
	{"apples", "apple", "jam", []int{8, 9, 10}, []string{"apple", "pear", "grapes"}},
	{"mushrooms", "mushrooms_forest", "mushrooms", []int{8, 9, 10}, []string{"mushrooms_forest"}},
	{"pumpkin", "pumpkin", "vegetables", []int{9, 10}, []string{"pumpkin"}},
	{"forest", "lingonberry_frozen", "sauces", []int{9, 10}, []string{"lingonberry_frozen", "cranberry_frozen"}},
	{"late", "quince", "confiture", []int{9, 10}, []string{"quince", "chokeberry", "sea_buckthorn_frozen"}},
	{"cabbage", "cabbage", "ferment", []int{9, 10, 11}, []string{"cabbage"}},
}

// sterilRows — сколько минут стерилизовать: пустую банку над паром и банку с заготовкой в кастрюле
// (от закипания воды). Те же цифры стоят в шагах рецептов.
var sterilRows = []struct {
	Litres      float64
	Steam, Full string
}{{0.5, "10", "10–15"}, {1, "15", "15–20"}, {2, "20", "25"}, {3, "25", "30"}}

type jarSectionView struct {
	ID, Title, Lead string
	Cards           []recipeCard
}

type jarNorm struct {
	Value, Unit, Label string
	Rub                bool
}

type seasonView struct {
	Label, Image, Href string
	Cells              []seasonCell
	Now                bool
}

// seasonCell — клетка календаря; Start и End скругляют полосу сезона с краёв, внутри она сплошная.
type seasonCell struct{ On, Now, Start, End bool }

type monthHead struct {
	Label string
	Now   bool
}

type amountRow struct{ Name, Qty string }

type keepRow struct{ Label, Href, Range string }

func (s *Server) jarsPage(h jarHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pl, ok := s.stableLocale(r)
		if !ok || !topicLang(pl.L) {
			s.notFoundPage(w, r)
			return
		}
		l := pl.L
		base := s.baseURL(r)
		cat := s.svc.Catalog.Base()
		pre := "jars." + h.Key + "."

		// разделы: рецепты по порядку из файла, скрытые и пропавшие из базы пропускаем
		var sections []jarSectionView
		var all []planner.Recipe
		bySection := map[string][]planner.Recipe{}
		for _, sd := range jarSections[h.Key] {
			sec := jarSectionView{ID: sd.ID, Title: i18n.T(l, "jars.sec."+sd.ID), Lead: i18n.T(l, "jars.sec."+sd.ID+".lead")}
			for _, id := range sd.Recipes {
				rc, ok := cat.RecipeByID[id]
				if !ok || rc.Hidden || !rc.IsJar() {
					continue
				}
				sec.Cards = append(sec.Cards, s.card(rc, pl))
				bySection[sd.ID] = append(bySection[sd.ID], rc)
				all = append(all, rc)
			}
			if len(sec.Cards) > 0 {
				sections = append(sections, sec)
			}
		}
		if len(all) == 0 {
			s.notFoundPage(w, r)
			return
		}

		// плитки: сколько рецептов, обычная цена банки и срок хранения
		var costs []float64
		var keeps []int
		for _, rc := range all {
			if c, priced := s.catalog.PortionCost(rc, pl.Country.Code); priced && c > 0 {
				costs = append(costs, c)
			}
			if rc.KeepDays != nil {
				keeps = append(keeps, *rc.KeepDays)
			}
		}
		norms := []jarNorm{{Value: strconv.Itoa(len(all)), Label: i18n.Plural(l, len(all), "jars.n")}}
		if len(costs) > 0 {
			sort.Float64s(costs)
			norms = append(norms, jarNorm{Value: "≈ " + formatMoney(pl.Country, costs[len(costs)/2]), Label: i18n.T(l, "jars.norm.cost"), Rub: true})
		}
		if h == hubPreserves {
			maxShelf := 0
			for _, rc := range all {
				maxShelf = max(maxShelf, rc.Shelf)
			}
			norms = append(norms, jarNorm{Value: strconv.Itoa(maxShelf), Unit: i18n.T(l, "jars.unit.months"), Label: i18n.T(l, "jars.norm.shelf")})
		} else if len(keeps) > 0 {
			sort.Ints(keeps)
			norms = append(norms, jarNorm{Value: strconv.Itoa(keeps[len(keeps)/2]), Unit: i18n.T(l, "jars.unit.days"), Label: i18n.T(l, "jars.norm.keep")})
		}
		month := int(time.Now().In(moscow).Month())
		data := map[string]any{
			"L": l, "P": pl.P, "Country": pl.Country, "NavRecipes": true, "Key": h.Key, "Path": h.Path,
			"H1":       i18n.T(l, pre+"h1"),
			"Intro":    []string{i18n.T(l, pre+"intro1"), i18n.T(l, pre+"intro2")},
			"Switch":   jarSwitch(pl, h),
			"Norms":    norms,
			"Sections": sections,
			"Rules":    jarTexts(l, pre+"rule", 6),
			"Safety":   jarTexts(l, pre+"bad", 4),
		}

		if h == hubPreserves {
			// календарь: столбцы июнь–ноябрь, текущий месяц подсвечен; строка ведёт к своему разделу
			var heads []monthHead
			for m := seasonFrom; m <= seasonTo; m++ {
				heads = append(heads, monthHead{Label: shortMonth(l, m), Now: m == month})
			}
			var rows []seasonView
			inSeason := map[string]bool{}
			for _, sr := range seasonRows {
				v := seasonView{Label: i18n.T(l, "jars.season."+sr.Key), Image: cat.Ingredients[sr.Image].Image, Href: pl.P + h.Path + "#" + sr.Section}
				for m := seasonFrom; m <= seasonTo; m++ {
					on := slices.Contains(sr.Months, m)
					v.Cells = append(v.Cells, seasonCell{On: on, Now: m == month})
					if on && m == month {
						v.Now = true
						for _, id := range sr.Ings {
							inSeason[id] = true
						}
					}
				}
				for i := range v.Cells {
					c := &v.Cells[i]
					c.Start = c.On && (i == 0 || !v.Cells[i-1].On)
					c.End = c.On && (i == len(v.Cells)-1 || !v.Cells[i+1].On)
				}
				rows = append(rows, v)
			}
			data["Months"], data["Season"] = heads, rows
			// «что закрыть сейчас»: банки из продуктов текущего месяца, сначала с фото, не больше восьми
			if len(inSeason) > 0 {
				var now []planner.Recipe
				for _, rc := range all {
					if len(rc.Ingredients) > 0 && inSeason[rc.Ingredients[0].IngredientID] {
						now = append(now, rc)
					}
				}
				sort.SliceStable(now, func(i, j int) bool { return now[i].Image != "" && now[j].Image == "" })
				var cards []recipeCard
				for i, rc := range now {
					if i == 8 {
						break
					}
					cards = append(cards, s.card(rc, pl))
				}
				data["Now"], data["NowTitle"] = cards, i18n.T(l, "jars.now."+strconv.Itoa(month))
			}
			var steril [][3]string
			for _, sr := range sterilRows {
				steril = append(steril, [3]string{i18n.T(l, "recipe.jar.size", litres(l, sr.Litres)), sr.Steam, sr.Full})
			}
			data["Steril"] = steril
		} else {
			// маринад и рассол — те же пропорции, что в рецептах подборки (морковь с укропом, цветная капуста с куркумой, огурцы в рассоле)
			data["Marinade"] = []amountRow{
				{i18n.T(l, "jars.f.water"), "200 " + i18n.T(l, "unit.ml")}, {i18n.T(l, "jars.f.vinegar"), "50 " + i18n.T(l, "unit.ml")},
				{i18n.T(l, "jars.f.sugar"), "15 " + i18n.T(l, "unit.g")}, {i18n.T(l, "jars.f.salt"), "8 " + i18n.T(l, "unit.g")},
			}
			data["Brine"] = []amountRow{{i18n.T(l, "jars.f.water"), "1 " + i18n.T(l, "unit.l")}, {i18n.T(l, "jars.f.salt"), "25–35 " + i18n.T(l, "unit.g")}}
			// сколько хранится: по разделам, от самого короткого срока до самого длинного
			var keepRows []keepRow
			for _, sec := range sections {
				lo, hi := -1, 0
				for _, rc := range bySection[sec.ID] {
					if rc.KeepDays == nil {
						continue
					}
					k := *rc.KeepDays
					if lo < 0 || k < lo {
						lo = k
					}
					hi = max(hi, k)
				}
				if lo < 0 {
					continue
				}
				rng := strconv.Itoa(lo)
				if hi > lo {
					rng += "–" + strconv.Itoa(hi)
				}
				keepRows = append(keepRows, keepRow{sec.Title, "#" + sec.ID, rng + " " + i18n.T(l, "jars.unit.days")})
			}
			data["Keep"] = keepRows
		}

		var faq []domain.QA
		for i := 1; ; i++ {
			k := pre + "faq" + strconv.Itoa(i)
			q := i18n.T(l, k+".q")
			if q == k+".q" {
				break
			}
			faq = append(faq, domain.QA{Q: q, A: i18n.T(l, k+".a")})
		}
		data["FAQ"] = faq

		var alts []altLink
		for _, al := range topicLangs {
			m := i18n.Meta(al)
			alts = append(alts, altLink{Lang: string(al), Href: base + prefix(al) + h.Path, Name: m.Name, English: m.English, Flag: m.Flag})
		}
		title := i18n.T(l, pre+"title", strconv.Itoa(len(all))+" "+i18n.Plural(l, len(all), "jars.n"))
		desc := i18n.T(l, pre+"desc")
		data["Base"] = pageBase{User: currentUser(r) != nil, Title: title + " — " + i18n.T(l, "page.brand"), Description: desc, Canonical: base + pl.P + h.Path,
			OGImage: brandOG(base, l), OGType: "website", OGWide: true, Alternates: alts, JSONLD: jarsLD(base, pl, h, i18n.T(l, pre+"h1"), desc, all, faq)}

		var buf bytes.Buffer
		if err := pageTpl.ExecuteTemplate(&buf, "jars.html", data); err != nil {
			s.log.Error("jars page", zap.String("hub", h.Key), zap.Error(err))
			http.Error(w, "template error", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=600")
		_, _ = buf.WriteTo(w)
	}
}

// moscow — месяц календаря считаем по Москве: сервер живёт в UTC, а читатель в основном в России.
var moscow = func() *time.Location {
	if loc, err := time.LoadLocation("Europe/Moscow"); err == nil {
		return loc
	}
	return time.FixedZone("MSK", 3*3600)
}()

// jarSwitch — переключатель двух страниц сверху, как шкала возраста.
func jarSwitch(pl pageLocale, cur jarHub) []ageStep {
	var out []ageStep
	for _, h := range jarHubs {
		out = append(out, ageStep{Label: i18n.T(pl.L, "jars."+h.Key+".tab"), Href: pl.P + h.Path, Current: h == cur})
	}
	return out
}

// jarTexts — пункты списка key1…keyN; пустые (нет перевода) не выводим.
func jarTexts(l i18n.Lang, key string, n int) []string {
	var out []string
	for i := 1; i <= n; i++ {
		k := key + strconv.Itoa(i)
		if v := i18n.T(l, k); v != k {
			out = append(out, v)
		}
	}
	return out
}

// shortMonth — «окт», «Oct», «Okt»: три первые буквы названия месяца.
func shortMonth(l i18n.Lang, m int) string {
	name := []rune(i18n.Meta(l).Months[m-1])
	if len(name) > 3 {
		name = name[:3]
	}
	return string(name)
}

// jarsLD — разметка: страница-подборка со списком всех банок, вопросы-ответы и крошки.
func jarsLD(base string, pl pageLocale, h jarHub, title, desc string, recipes []planner.Recipe, faq []domain.QA) template.JS {
	url := base + pl.P + h.Path
	items := make([]map[string]any, 0, len(recipes))
	for i, rc := range recipes {
		items = append(items, map[string]any{"@type": "ListItem", "position": i + 1, "url": base + pl.P + "/recipe/" + rc.ID, "name": rc.Text(pl.L).Title})
	}
	var qa []map[string]any
	for _, f := range faq {
		qa = append(qa, map[string]any{"@type": "Question", "name": f.Q, "acceptedAnswer": map[string]any{"@type": "Answer", "text": f.A}})
	}
	graph := []any{
		map[string]any{"@type": "CollectionPage", "name": title, "description": desc, "url": url, "inLanguage": string(pl.L),
			"mainEntity": map[string]any{"@type": "ItemList", "numberOfItems": len(items), "itemListElement": items},
			"publisher":  map[string]any{"@type": "Organization", "name": i18n.T(pl.L, "page.brand"), "url": base + "/"}},
		breadcrumbLD([][2]string{{i18n.T(pl.L, "page.brand"), base + "/"}, {i18n.T(pl.L, "page.recipes"), base + pl.P + "/recipes"}, {title, url}}),
	}
	if len(qa) > 0 {
		graph = append(graph, map[string]any{"@type": "FAQPage", "mainEntity": qa})
	}
	b, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph})
	return template.JS(b)
}

// jarHubStat — сколько банок на странице и фото для карточки в каталоге: первое фото по порядку разделов.
func (s *Server) jarHubStat(h jarHub) (int, string) {
	cat := s.svc.Catalog.Base()
	n, cover := 0, ""
	for _, sd := range jarSections[h.Key] {
		for _, id := range sd.Recipes {
			if rc, ok := cat.RecipeByID[id]; ok && !rc.Hidden && rc.IsJar() {
				n++
				if cover == "" {
					cover = rc.Image
				}
			}
		}
	}
	return n, cover
}
