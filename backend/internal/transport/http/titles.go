package http

import (
	"sort"
	"strings"
	"sync"

	"racion/internal/i18n"
	"racion/internal/planner"
)

// Одинаковые заголовки страниц рецептов. Вебмастер Яндекса (октябрь 2026) нашёл 203 страницы с повторяющимся
// <title>: детская и взрослая «Манная каша», одно блюдо в двух разделах, разные блюда, которые перевод свёл
// к одному названию. H1 на странице остаётся названием блюда, а заголовок для поиска уточняется: детскому —
// «для детей», остальным, кроме первого, — приём пищи и время.

type twinIndex struct {
	cat *planner.Catalog
	m   map[string][]string // название в нижнем регистре → id рецептов, первым — тот, кто оставляет название как есть
}

var twinCache sync.Map // i18n.Lang → *twinIndex

// titleTwins — рецепты каталога с тем же названием на этом языке (только те, у кого на языке есть свой текст).
func (s *Server) titleTwins(l i18n.Lang, title string) []string {
	cat := s.svc.Catalog.Base()
	if v, ok := twinCache.Load(l); ok && v.(*twinIndex).cat == cat {
		return v.(*twinIndex).m[strings.ToLower(title)]
	}
	m := map[string][]string{}
	for _, rc := range cat.Recipes {
		if rc.Hidden || !hasLang(rc, l) {
			continue
		}
		k := strings.ToLower(rc.Text(l).Title)
		m[k] = append(m[k], rc.ID)
	}
	for k, ids := range m {
		if len(ids) < 2 {
			delete(m, k)
			continue
		}
		// обычное блюдо раньше детского, с фото раньше без фото, дальше по id — порядок стабилен между деплоями
		sort.SliceStable(ids, func(i, j int) bool {
			a, b := cat.RecipeByID[ids[i]], cat.RecipeByID[ids[j]]
			if ka, kb := hasTag(a, "kidmenu"), hasTag(b, "kidmenu"); ka != kb {
				return !ka
			}
			if (a.Image != "") != (b.Image != "") {
				return a.Image != ""
			}
			return ids[i] < ids[j]
		})
	}
	twinCache.Store(l, &twinIndex{cat: cat, m: m})
	return m[strings.ToLower(title)]
}

// recipePageTitle — заголовок страницы рецепта для поиска без повторов на языке.
func (s *Server) recipePageTitle(l i18n.Lang, rc planner.Recipe, title string) string {
	twins := s.titleTwins(l, title)
	if len(twins) < 2 || twins[0] == rc.ID {
		return title
	}
	cat := s.svc.Catalog.Base()
	if hasTag(rc, "kidmenu") {
		// первый из детских — просто «для детей», следующие ещё и с приёмом пищи
		for _, id := range twins {
			if hasTag(cat.RecipeByID[id], "kidmenu") {
				if id == rc.ID {
					return i18n.T(l, "recipe.title.kid", title)
				}
				break
			}
		}
		title = i18n.T(l, "recipe.title.kid", title)
	}
	return i18n.T(l, "recipe.title.alt", title, strings.ToLower(planner.SlotLabel(l, rc.Slot)), i18n.Minutes(l, rc.TimeMin))
}
