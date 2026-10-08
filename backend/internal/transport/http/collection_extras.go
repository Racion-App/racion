package http

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Подборки к Новому году и к посту — главные страницы своих тем: всё, что раньше жило на /menu/new-year и
// /menu/post, теперь здесь, чтобы у темы был один адрес. Дополнения есть на ru, en, de (тексты только там).

// topicCovers — фото шапки темы (ima2, frontend/public/images/topics): общий стол вместо обложки-блюда.
var topicCovers = map[string]string{"new-year-table": "/images/topics/new-year.webp", "lent-menu": "/images/topics/post.webp"}

// lentParams — те же ограничения, что у события «Постная неделя» (occasions.json, id lent).
func lentParams(country string) planner.Params {
	p := planner.Params{Country: country, ExcludeTags: []string{"meat", "poultry", "fish", "seafood", "offal"}, Allergens: []string{"dairy", "eggs"},
		Equipment: []string{"stove", "oven", "microwave", "multicooker", "airfryer", "blender", "mixer", "grill", "steamer", "meatgrinder"}}
	if o, ok := planner.OccasionByID("lent"); ok && o.Preset != nil {
		p.ExcludeTags, p.Allergens = o.Preset.ExcludeTags, o.Preset.Allergens
	}
	return p
}

type extraGroup struct {
	Key, Title string
	Cards      []recipeCard
}

// collectionExtras дописывает в данные страницы подборки блоки её темы; extraFAQ идёт перед вопросами подборки.
func (s *Server) collectionExtras(slug string, pl pageLocale, recipes []planner.Recipe, data map[string]any) {
	if !topicLang(pl.L) {
		return
	}
	l := pl.L
	now := time.Now().In(moscow)
	if c, ok := topicCovers[slug]; ok && imagesDir != "" {
		if _, err := os.Stat(filepath.Join(imagesDir, strings.TrimPrefix(c, "/images/"))); err == nil {
			data["Cover"] = c
		}
	}
	switch slug {
	case "new-year-table":
		views, def := s.feastViews(pl)
		if len(views) == 0 {
			return
		}
		data["Countdown"] = newYearCountdown(l, now)
		// главная кнопка стола — сам праздничный стол на шестерых, а не неделя из новогодних блюд
		data["PlanHref"], data["PlanLabel"] = pl.P+"/event/newyear?guests=6", i18n.T(l, "feast.cta.btn")
		data["Feast"] = views
		data["Stores"] = s.feastStores(def, pl)
		data["Prep"] = feastPrep(l)
		var six, cheap feastView
		salads := map[string]int{}
		for _, v := range views {
			salads[v.Key] = v.Salads
			switch v.Key {
			case "6":
				six = v
			case "cheap":
				cheap = v
			}
		}
		country := i18n.T(l, "country."+pl.Country.Code)
		data["ExtraFAQ"] = []domain.QA{
			{Q: i18n.T(l, "feast.faq.cost.q", six.Guests), A: i18n.T(l, "feast.faq.cost.a", six.Guests, six.Total, six.PerGuest, country, cheap.Total)},
			{Q: i18n.T(l, "feast.faq.salads.q"), A: i18n.T(l, "feast.faq.salads.a", salads["4"], salads["6"], salads["8"])},
			{Q: i18n.T(l, "feast.faq.ahead.q"), A: i18n.T(l, "feast.faq.ahead.a")},
		}
	case "lent-menu":
		fc := newFastCalendar(l, now)
		mine := map[string]bool{}
		for _, rc := range recipes {
			mine[rc.ID] = true
		}
		fish := s.fishDayCards(pl)
		more, total, extra := s.moreLenten(pl, mine)
		// панели дней берут блюда из подборки и из остального постного каталога: без масла в подборке почти ничего
		fc.Panes = s.fastPanes(pl, append(slices.Clone(recipes), extra...), fish)
		data["Countdown"] = fc.Status
		data["Fast"] = fc
		data["FishCards"] = fish
		data["MoreGroups"], data["MoreTotal"] = more, total
		data["ExtraFAQ"] = fc.FAQ
	}
}

// withPhotoFirst — карточки с фото вперёд, порядок внутри сохраняется.
func withPhotoFirst(list []planner.Recipe) {
	sort.SliceStable(list, func(i, j int) bool { return list[i].Image != "" && list[j].Image == "" })
}

// fishDayCards — рыбные блюда для рыбных дней поста: без мяса, молочного и яиц.
func (s *Server) fishDayCards(pl pageLocale) []recipeCard {
	p := lentParams(pl.Country.Code)
	p.ExcludeTags = slices.DeleteFunc(slices.Clone(p.ExcludeTags), func(t string) bool { return t == "fish" })
	var list []planner.Recipe
	for _, rc := range s.catalog.Recipes {
		if rc.IsJar() || hasTag(rc, "kidmenu") || hasTag(rc, "premium") || !hasTag(rc, "fish") || rc.Slot == "breakfast" || !s.catalog.Allowed(rc, p) {
			continue
		}
		list = append(list, rc)
	}
	// разнообразие: сначала с фото и без особой техники (аэрогриль, мультиварка — не у всех), и по одной
	// рыбе на главный продукт — иначе в списке четыре лосося подряд
	special := func(rc planner.Recipe) bool {
		return slices.Contains(rc.Equipment, "airfryer") || slices.Contains(rc.Equipment, "multicooker")
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if (a.Image != "") != (b.Image != "") {
			return a.Image != ""
		}
		return !special(a) && special(b)
	})
	var out []recipeCard
	seen := map[string]bool{}
	for pass := 0; pass < 2 && len(out) < 12; pass++ {
		for _, rc := range list {
			if len(out) == 12 {
				break
			}
			main := ""
			if len(rc.Ingredients) > 0 {
				main = rc.Ingredients[0].IngredientID
			}
			if seen[rc.ID] || (pass == 0 && seen["main:"+main]) {
				continue
			}
			seen[rc.ID], seen["main:"+main] = true, true
			out = append(out, s.card(rc, pl))
		}
	}
	return out
}

// hasOil — есть ли в рецепте растительное или любое другое масло: для дней «без масла».
func hasOil(rc planner.Recipe) bool {
	for _, ri := range rc.Ingredients {
		if strings.Contains(ri.IngredientID, "oil") || ri.IngredientID == "butter" {
			return true
		}
	}
	return false
}

// fastPanes — по нажатию на день календаря: что можно и четыре блюда под такой день.
func (s *Server) fastPanes(pl pageLocale, lent []planner.Recipe, fish []recipeCard) []fastPane {
	l := pl.L
	pick := func(match func(planner.Recipe) bool) []recipeCard {
		var list []planner.Recipe
		for _, rc := range lent {
			if match(rc) {
				list = append(list, rc)
			}
		}
		withPhotoFirst(list)
		var out []recipeCard
		for i, rc := range list {
			if i == 4 {
				break
			}
			out = append(out, s.card(rc, pl))
		}
		return out
	}
	var eve []recipeCard
	for _, id := range []string{"xm_kutia", "xm_uzvar", "xm_vareniki_potato_mushroom", "xm_borscht_mushrooms_lean"} {
		if rc, ok := s.catalog.RecipeByID[id]; ok && !rc.Hidden {
			eve = append(eve, s.card(rc, pl))
		}
	}
	panes := []fastPane{
		{Kind: "fish", Cards: fish[:min(4, len(fish))]},
		{Kind: "oil", Cards: pick(func(rc planner.Recipe) bool { return hasOil(rc) && rc.Slot != "breakfast" && !hasTag(rc, "sweet") })},
		{Kind: "lean", Cards: pick(func(rc planner.Recipe) bool {
			return !hasOil(rc) && rc.Slot != "snack" && !hasTag(rc, "sweet") && !hasTag(rc, "drink") && !hasTag(rc, "sauce")
		})},
		{Kind: "eve", Cards: eve},
	}
	for i := range panes {
		panes[i].Label = i18n.T(l, "fast.kind."+panes[i].Kind)
		panes[i].Text = i18n.T(l, "fast.pane."+panes[i].Kind)
	}
	return panes
}

// moreLenten — постные блюда каталога, которых нет в подборке: по видам блюд, с фото вперёд.
func (s *Server) moreLenten(pl pageLocale, mine map[string]bool) ([]extraGroup, int, []planner.Recipe) {
	l := pl.L
	p := lentParams(pl.Country.Code)
	kind := func(rc planner.Recipe) string {
		switch {
		case hasTag(rc, "soup"):
			return "soup"
		case hasTag(rc, "sweet") || hasTag(rc, "dessert") || hasTag(rc, "drink"):
			return "sweet"
		case hasTag(rc, "salad") || hasTag(rc, "starter"):
			return "salads"
		case rc.Slot == "breakfast":
			return "breakfast"
		}
		return "mains"
	}
	by := map[string][]planner.Recipe{}
	total := 0
	var all []planner.Recipe
	for _, rc := range s.catalog.Recipes {
		if mine[rc.ID] || rc.IsJar() || hasTag(rc, "kidmenu") || hasTag(rc, "side") || hasTag(rc, "sauce") || !s.catalog.Allowed(rc, p) {
			continue
		}
		k := kind(rc)
		by[k] = append(by[k], rc)
		all = append(all, rc)
		total++
	}
	var out []extraGroup
	for _, k := range []string{"mains", "soup", "breakfast", "salads", "sweet"} {
		list := by[k]
		if len(list) == 0 {
			continue
		}
		withPhotoFirst(list)
		g := extraGroup{Key: k, Title: i18n.T(l, "lent.more."+k)}
		for _, rc := range list {
			g.Cards = append(g.Cards, s.card(rc, pl))
		}
		out = append(out, g)
	}
	return out, total, all
}
