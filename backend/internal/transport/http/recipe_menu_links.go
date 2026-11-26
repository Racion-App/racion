package http

import "slices"

type recipeMenuLink struct{ Label, Href string }

var dairyFreeBreakfasts = []string{
	"kid_df_rice_pear", "kid_df_buckwheat_apple", "kid_df_millet_pumpkin",
	"kid_df_oat_banana", "kid_df_steam_omelet_zucchini", "kid_df_banana_oat_pancakes",
}

func recipeMenuLinks(id string) []recipeMenuLink {
	if id == "wean_rice_porridge" || slices.Contains(dairyFreeBreakfasts, id) {
		return []recipeMenuLink{
			{"Завтраки без молока для детей 1–3 лет", "/collection/kids-1-3#breakfasts"},
			{"Меню на неделю для ребёнка в 2 года", "/menu/child-2#week"},
		}
	}
	switch id {
	case "cheese_board", "ny_canape_cheese_grapes", "ny_tartlets_caviar", "ny_eggs_stuffed_caviar", "ny_salad_olivier_tongue", "ny_salad_mandarin_chicken", "olivier", "herring_fur_coat":
		return []recipeMenuLink{{"Собрать новогодний стол", "/collection/new-year-table"}, {"Выбрать коктейли на Новый год", "/collection/cocktails/new-year"}}
	}
	var links []recipeMenuLink
	for _, guide := range cocktailGuides {
		if guide.Slug != "new-year" && guide.Slug != "christmas" {
			continue
		}
		for _, section := range guide.Sections {
			if slices.Contains(section.Recipes, id) {
				links = append(links, recipeMenuLink{guide.Name, guide.Path()})
				break
			}
		}
	}
	return links
}
