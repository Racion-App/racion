package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
	"racion/internal/service"
)

// Страница рецепта: все блоки считаются из данных, ничего не сочиняется.

type ingredientLine struct {
	N        int
	ID       string
	Name     string
	Amount   float64
	Unit     string
	Pantry   bool
	ToTaste  bool   // щепотка: меньше грамма, число не показываем
	PackNote string // «упаковка 800 г ≈ 390 ₽» — по справочным ценам, если есть
	Cost     float64
	Image    string
}

type stepView struct {
	Text string
	HTML template.HTML // текст шага, где первое упоминание продукта — ссылка на его строку с номером
}

type tagLink struct {
	Label string
	Href  string
}

var tagLabels = []struct{ Tag, Href string }{
	{"pp", "/recipes?tag=pp"},
	{"protein", "/recipes?tag=protein"},
	{"soup", "/recipes?tag=soup"},
	{"salad", "/recipes?tag=salad"},
	{"vegetarian", "/recipes?tag=vegetarian"},
	{"sweet", "/recipes?tag=sweet"},
	{"spicy", "/recipes?tag=spicy"},
	{"hearty", "/recipes?tag=hearty"},
	{"quick", "/recipes?time=20"},
	{"kids", "/recipes?tag=kids"},
	{"kidmenu", "/recipes?tag=kidmenu"},
	{"nocook", "/recipes?eq=nocook"},
	{"premium", "/recipes?tag=premium"},
}

// stem — основа слова для грубого поиска упоминаний продукта в тексте шага:
// первые четыре буквы («кури» ловит и «куриные», и «курицу»), короткие слова целиком,
// у четырёхбуквенных с гласной на конце гласную отбрасываем («яйца» → «яйц»).
func stem(w string) string {
	r := []rune(service.Norm(w))
	if len(r) < 3 {
		return ""
	}
	n := 4
	if len(r) < n {
		n = len(r)
	}
	if len(r) == 4 && strings.ContainsRune("аоыиеяуaeiouy", r[3]) {
		n = 3
	}
	return string(r[:n])
}

// wordMatches — начинается ли слово с основы. У трёхбуквенной основы допускаем
// только окончание: «мед» ловит «медом», но не «медленно».
func wordMatches(w, sm string) bool {
	w = service.Norm(w)
	return strings.HasPrefix(w, sm) && (len([]rune(sm)) > 3 || len([]rune(w)) <= len([]rune(sm))+2)
}

// markStep — размечает текст шага: первое слово, совпавшее с основой названия продукта,
// становится ссылкой на строку списка с номером надстрочно. Всё остальное экранируется.
func markStep(text string, ings []ingredientLine) template.HTML {
	type key struct {
		n  int
		sm string
	}
	var keys []key
	for _, l := range ings {
		if l.Pantry {
			continue
		}
		first := strings.TrimRight(strings.Fields(l.Name)[0], ",()")
		if sm := stem(first); sm != "" {
			keys = append(keys, key{l.N, sm})
		}
	}
	used := map[int]bool{}
	var b strings.Builder
	rs := []rune(text)
	for i := 0; i < len(rs); {
		if !unicode.IsLetter(rs[i]) {
			b.WriteString(template.HTMLEscapeString(string(rs[i])))
			i++
			continue
		}
		j := i
		for j < len(rs) && unicode.IsLetter(rs[j]) {
			j++
		}
		w := string(rs[i:j])
		marked := false
		for _, k := range keys {
			if used[k.n] || !wordMatches(w, k.sm) {
				continue
			}
			used[k.n] = true
			fmt.Fprintf(&b, `<a class="steps__ref" href="#ing-%d">%s<sup>%d</sup></a>`, k.n, template.HTMLEscapeString(w), k.n)
			marked = true
			break
		}
		if !marked {
			b.WriteString(template.HTMLEscapeString(w))
		}
		i = j
	}
	return template.HTML(b.String())
}

// gramsOf — масса ингредиента в граммах для расчёта «на 100 г».
func gramsOf(ing planner.Ingredient, amount float64) float64 {
	if ing.Unit != "pcs" {
		return amount
	}
	switch ing.ID {
	case "eggs":
		return amount * 55
	case "avocado":
		return amount * 160
	default:
		return amount * 60
	}
}

func (s *Server) recipePage(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.localeFromPath(r)
	if !ok {
		s.notFoundPage(w, r)
		return
	}
	rememberCountry(w, r)
	rc, err := s.svc.Recipes.Find(r.Context(), r.PathValue("id"))
	if err != nil {
		s.notFoundPage(w, r)
		return
	}
	s.svc.Recipes.CountView(r.Context(), rc, viewerID(r))
	l := pl.L
	tx := rc.Text(l)
	kcal, prot, fat, carb := s.catalog.Nutrition(rc)
	cost, priced := s.catalog.PortionCost(rc, pl.Country.Code)
	pb := s.catalog.PriceBook()
	local := s.catalog.LocalPrices(pl.Country.Code)

	var ings []ingredientLine
	var ingNames []string
	var grams float64
	var dearest ingredientLine
	for i, ri := range rc.Ingredients {
		ing := s.catalog.Ingredients[ri.IngredientID]
		line := ingredientLine{N: i + 1, ID: ing.ID, Name: ing.LocalName(l), Amount: ri.Amount, Unit: ing.Unit, Pantry: ing.Pantry, Image: ing.Image}
		line.ToTaste = ing.Pantry && ri.Amount < 1
		if priced && !ing.Pantry {
			pack, official := s.catalog.IngredientPrice(ing, pl.Country.Code)
			line.Cost = ri.Amount * pack / ing.Pack
			// подпись цены показываем для официальных источников и для ручных ориентиров других стран
			if official || pl.Country.Code != "RU" {
				if !ing.Loose {
					line.PackNote = i18n.T(l, "recipe.pack", formatQty(l, ing.Pack, ing.Unit), formatMoney(pl.Country, pack))
				} else {
					line.PackNote = i18n.T(l, "recipe.perkg", formatMoney(pl.Country, pack/ing.Pack*1000))
				}
			}
			if line.Cost > dearest.Cost {
				dearest = line
			}
		}
		grams += gramsOf(ing, ri.Amount)
		ings = append(ings, line)
		ingNames = append(ingNames, fmt.Sprintf("%s — %s", line.Name, formatQty(l, ri.Amount, ing.Unit)))
	}

	// В тексте шага продукты помечены номерами строк списка.
	steps := make([]stepView, 0, len(tx.Steps))
	for _, st := range tx.Steps {
		steps = append(steps, stepView{Text: st, HTML: markStep(st, ings)})
	}

	var tags []tagLink
	for _, t := range tagLabels {
		if hasTag(rc, t.Tag) || (t.Tag == "quick" && rc.TimeMin <= 20 && !hasTag(rc, "kidmenu")) {
			tags = append(tags, tagLink{i18n.T(l, "tag."+t.Tag), pl.P + t.Href})
		}
	}

	kid := hasTag(rc, "kidmenu")
	main := ""
	if len(rc.Ingredients) > 0 {
		main = rc.Ingredients[0].IngredientID
	}
	var related []recipeCard
	seen := map[string]bool{rc.ID: true}
	for pass := 0; pass < 2 && len(related) < 4; pass++ {
		for _, x := range s.catalog.Recipes {
			if len(related) >= 4 || seen[x.ID] || x.Slot != rc.Slot || hasTag(x, "kidmenu") != kid {
				continue
			}
			sameMain := len(x.Ingredients) > 0 && x.Ingredients[0].IngredientID == main
			if (pass == 0 && sameMain) || pass == 1 {
				related = append(related, s.card(x, pl))
				seen[x.ID] = true
			}
		}
	}
	sort.SliceStable(related, func(i, j int) bool { return i < j })

	// «Коротко» — только то, чего нет в шапке и боковой панели.
	var facts []string
	eq := equipmentLabels(l, rc.Equipment)
	buy := s.svc.Partners.BuyLinks(r.Context(), pl.Country.Code, l, rc.Equipment)
	if rc.Batch {
		facts = append(facts, i18n.T(l, "recipe.fact.batch"))
	}
	if kid {
		facts = append(facts, i18n.T(l, "recipe.fact.kid", kidMinAge(rc)))
	}
	if hasTag(rc, "pp") {
		facts = append(facts, i18n.T(l, "recipe.fact.pp"))
	}

	// подпись под ценой: источник и период
	costNote := ""
	switch {
	case pl.Country.Code == "RU":
		period := ""
		if pb != nil {
			period = ", " + pb.PeriodLabel(l)
		}
		costNote = i18n.T(l, "recipe.cost.note.ru", period)
	case local != nil:
		costNote = i18n.T(l, "recipe.cost.note", i18n.T(l, local.Source)+", "+local.PeriodLabel(l))
	default:
		costNote = i18n.T(l, "recipe.cost.note", i18n.T(l, "country."+pl.Country.Code))
	}
	if dearest.Name != "" {
		costNote += i18n.T(l, "recipe.cost.dearest", strings.ToLower(dearest.Name))
	}

	base := s.baseURL(r)
	desc := tx.Description
	if desc == "" {
		desc = i18n.T(l, "recipe.meta.desc", tx.Title, rc.TimeMin, int(math.Round(kcal)))
	}
	ldSteps := make([]map[string]string, 0, len(tx.Steps))
	for _, st := range tx.Steps {
		ldSteps = append(ldSteps, map[string]string{"@type": "HowToStep", "text": st})
	}
	ld := map[string]any{
		"@context": "https://schema.org", "@type": "Recipe",
		"name": tx.Title, "description": desc, "recipeYield": i18n.T(l, "recipe.yield"),
		"inLanguage":         string(l),
		"totalTime":          fmt.Sprintf("PT%dM", rc.TimeMin),
		"recipeCategory":     planner.SlotLabel(l, rc.Slot),
		"recipeCuisine":      i18n.T(l, "recipe.cuisine"),
		"recipeIngredient":   ingNames,
		"recipeInstructions": ldSteps,
		"nutrition": map[string]any{"@type": "NutritionInformation", "calories": fmt.Sprintf("%d kcal", int(math.Round(kcal))),
			"proteinContent": fmt.Sprintf("%d g", int(math.Round(prot))), "fatContent": fmt.Sprintf("%d g", int(math.Round(fat))), "carbohydrateContent": fmt.Sprintf("%d g", int(math.Round(carb)))},
		"keywords": strings.Join(rc.Tags, ", "),
		"author":   map[string]any{"@type": "Organization", "name": i18n.T(l, "page.brand")},
	}
	if rc.Image != "" {
		ld["image"] = base + rc.Image
	}
	ldJSON, _ := json.Marshal(ld)
	slotHref := pl.P + "/recipes?slot=" + rc.Slot
	slotCrumb := planner.SlotLabel(l, rc.Slot)
	allLabel := i18n.T(l, "recipe.all", i18n.T(l, "slots."+rc.Slot))
	if kid {
		slotHref = pl.P + "/recipes?tag=kidmenu"
		slotCrumb = i18n.T(l, "recipe.crumb.kids")
		allLabel = i18n.T(l, "recipe.all.kids")
	}
	viewer := currentUser(r)
	stats := s.svc.Social.Stats(r.Context(), rc.ID, viewerID(r))
	comments, _ := s.svc.Social.Comments(r.Context(), rc.ID, viewerID(r))
	type commentView struct {
		domain.Comment
		When string
	}
	cviews := make([]commentView, 0, len(comments))
	for _, c := range comments {
		when := ""
		if t, err := time.Parse(time.RFC3339, c.CreatedAt); err == nil {
			when = i18n.DayMonthYear(l, t.Day(), int(t.Month()), t.Year())
		}
		cviews = append(cviews, commentView{c, when})
	}
	data := map[string]any{
		"Viewer": viewer, "Stats": stats, "Comments": cviews, "Author": rc.Author, "Photos": s.svc.Media.Enabled(),
		"Base": pageBase{Title: i18n.T(l, "recipe.meta.title", tx.Title), Description: desc, Canonical: base + pl.P + "/recipe/" + rc.ID, OGImage: firstNonEmpty(ogImage(base, rc.Image), brandOG(base, l)), OGType: "article", OGWide: rc.Image == "", JSONLD: template.JS(ldJSON),
			Alternates: s.alternates(r, "/recipe/"+rc.ID), NoIndex: rc.Own},
		"L": l, "P": pl.P, "Country": pl.Country,
		"NavRecipes": true,
		"R":          rc, "Title": tx.Title, "Description": tx.Description, "Kcal": kcal, "Protein": prot, "Fat": fat, "Carb": carb,
		"Pct":    map[string]int{"Kcal": int(math.Round(kcal / 20)), "Protein": int(math.Round(prot / 0.75)), "Fat": int(math.Round(fat / 0.7)), "Carb": int(math.Round(carb / 2.6))},
		"Per100": kcal / math.Max(grams, 1) * 100,
		"Cost":   cost, "Priced": priced, "CostNote": costNote,
		"Ings": ings, "Steps": steps, "Related": related, "Kid": kid, "Tags": tags,
		"Equipment": eq, "Buy": buy, "SlotHref": slotHref, "SlotCrumb": slotCrumb, "AllLabel": allLabel, "Facts": facts,
	}
	var buf bytes.Buffer
	if err := pageTpl.ExecuteTemplate(&buf, "recipe.html", data); err != nil {
		s.log.Error("recipe page", zap.Error(err))
		s.errorPage(w, r, 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// ogImage — картинка для превью ссылки: JPEG-копия фото блюда (мессенджеры не все понимают WebP);
// без фото — брендовая картинка на языке страницы.
func ogImage(base, img string) string {
	if img == "" {
		return ""
	}
	if strings.HasSuffix(img, ".webp") {
		img = strings.TrimSuffix(img, ".webp") + ".jpg"
	}
	return base + img
}

// brandOG — брендовая картинка 1200×630 на языке (frontend/public/og/<lang>.png).
func brandOG(base string, l i18n.Lang) string {
	return base + "/og/" + string(l) + ".png"
}

func kidMinAge(r planner.Recipe) int {
	for _, t := range r.Tags {
		var m int
		if n, _ := fmt.Sscanf(t, "age%d", &m); n == 1 {
			return m
		}
	}
	return 12
}
