package http

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"racion/internal/i18n"
	"racion/internal/planner"
)

// Страницы возраста — «Прикорм по месяцам» и «Меню ребёнка в 1, 1,5, 2, 3 года» — открываются одной
// шкалой возраста: «до года · 1 год · 1,5 года · 2 года · 3 года». Каждый сегмент — своя страница со
// своим адресом для поиска. Под шкалой — нормы плитками и «тарелка дня»: приёмы одного дня по кругу,
// с фото блюд из базы.

type ageStep struct {
	Label, Href string
	Current     bool
}

// ageScale — сегменты шкалы; current — "weaning" или slug меню ребёнка.
func ageScale(pl pageLocale, current string) []ageStep {
	out := []ageStep{{Label: i18n.T(pl.L, "agescale.0"), Href: pl.P + "/weaning", Current: current == "weaning"}}
	for _, m := range kidMenuPresets {
		out = append(out, ageStep{Label: i18n.T(pl.L, "agescale."+strconv.Itoa(m.Age)), Href: pl.P + "/menu/" + m.Slug, Current: current == m.Slug})
	}
	return out
}

// normTile — плитка нормы: число, единица отдельно (на телефоне она переносится под число) и подпись.
type normTile struct{ Value, Unit, Label string }

// plateItem — приём на «тарелке дня». Milk — грудь или смесь: фото бутылочки. SlotNum — подпись приёма
// это время («10:00»), её набираем моноширинным, как все числа. Product — фото продукта на белом фоне
// (кефир): ему нужна подложка, иначе круг сливается с карточкой.
type plateItem struct {
	Slot, Title, Detail, Href, Image string
	Milk, SlotNum, Product           bool
}

// milkFeedImage — одна фотография для всех молочных кормлений прикорма («грудь или смесь»).
const milkFeedImage = "/images/kids/milk-feed.webp"

// slotOrder — приёмы по ходу дня: в меню ребёнка ужин записан раньше перекуса, а на тарелке
// полдник идёт после обеда.
var slotOrder = map[string]int{"breakfast": 0, "lunch": 1, "snack": 2, "dinner": 3, "bedtime": 4}

// kidPlate — первый день недели ребёнка: блюда с фото рецептов, кефир перед сном — с фото продукта.
func (s *Server) kidPlate(day planner.KidDay, pl pageLocale) []plateItem {
	dishes := slices.Clone(day.Dishes)
	rank := func(slot string) int {
		if n, ok := slotOrder[slot]; ok {
			return n
		}
		return len(slotOrder) // неизвестный приём — в конец
	}
	sort.SliceStable(dishes, func(i, j int) bool { return rank(dishes[i].Slot) < rank(dishes[j].Slot) })
	out := make([]plateItem, 0, len(dishes))
	for _, x := range dishes {
		it := plateItem{Slot: planner.SlotLabel(pl.L, x.Slot), Title: x.Title}
		switch {
		case x.RecipeID != "":
			it.Href = pl.P + "/recipe/" + x.RecipeID
			it.Image = s.catalog.RecipeByID[x.RecipeID].Image
		case x.Kind == planner.KidBedtime:
			it.Image, it.Product = s.catalog.Ingredients["kefir"].Image, true
		}
		out = append(out, it)
	}
	return out
}

// plateDay — день для «тарелки»: первый, где у всех блюд есть фото, иначе понедельник. Пустой круг
// в первом экране выглядит поломкой, а фото новых блюд появляются не сразу.
func (s *Server) plateDay(days []planner.KidDay) planner.KidDay {
	for _, d := range days {
		ok := true
		for _, x := range d.Dishes {
			if x.RecipeID != "" && s.catalog.RecipeByID[x.RecipeID].Image == "" {
				ok = false
				break
			}
		}
		if ok {
			return d
		}
	}
	return days[0]
}

// weaningPlate — примерный день прикорма: кормления по времени. У кормления с прикормом — фото
// главного рецепта и граммы, у молочного — знак и «грудь или смесь».
func (s *Server) weaningPlate(feeds []planner.WeaningFeed, pl pageLocale) []plateItem {
	var out []plateItem
	for _, f := range feeds {
		it := plateItem{Slot: f.Time, SlotNum: true}
		if len(f.Items) == 0 {
			it.Milk, it.Title, it.Image = true, i18n.T(pl.L, "weaning.milk"), milkFeedImage
			out = append(out, it)
			continue
		}
		var names, amounts []string
		for _, x := range f.Items {
			// масло к каше и пюре — в точном списке под тарелкой, в подписи оно только удлиняет строку
			if x.Group == "oil" || x.Group == "butter" {
				continue
			}
			// «Каша: гречка» → «гречка»: что это каша и пюре, видно по фото, а подпись под кругом короткая
			name := x.Name
			if i := strings.Index(name, ": "); i >= 0 {
				name = name[i+2:]
			}
			names = append(names, name)
			amounts = append(amounts, weaningAmount(pl.L, x))
			if it.Image == "" && x.Recipe != "" {
				id := strings.TrimPrefix(x.Recipe, pl.P+"/recipe/")
				if rc, ok := s.catalog.RecipeByID[id]; ok && rc.Image != "" {
					it.Image, it.Href = rc.Image, pl.P+"/recipe/"+id
				}
			}
		}
		it.Title, it.Detail = upperFirst(strings.Join(names, " · ")), strings.Join(amounts, " + ")
		out = append(out, it)
	}
	return out
}

// upperFirst — первая буква строки заглавная: «гречка · яблоки» → «Гречка · яблоки».
func upperFirst(s string) string {
	for i, r := range s {
		return strings.ToUpper(string(r)) + s[i+len(string(r)):]
	}
	return s
}

// weaningAmount — «150 г», «200 мл», «¼ желтка» для подписи на тарелке.
func weaningAmount(l i18n.Lang, x planner.WeaningItem) string {
	if x.Unit == "pcs" {
		if x.Grams == 0.25 {
			return "¼"
		}
		return "½"
	}
	return strconv.FormatFloat(x.Grams, 'f', -1, 64) + " " + i18n.T(l, "unit."+x.Unit)
}
