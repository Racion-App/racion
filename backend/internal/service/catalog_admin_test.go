package service

import (
	"testing"

	"racion/internal/planner"
)

func TestRecipeI18n(t *testing.T) {
	steps := []string{"раз", "два"}
	in := map[string]planner.RecipeText{
		"en": {Title: " Navy-style pasta ", Steps: []string{"one", "two"}},
		"de": {Title: "Nudeln", Steps: []string{"eins"}}, // шагов меньше — перевод другого текста
		"ru": {Title: "Макароны", Steps: steps},          // русский — это сам рецепт
		"xx": {Title: "?", Steps: []string{"a", "b"}},    // такого языка нет
		"fr": {Title: "", Steps: []string{"un", "deux"}}, // без названия
		"ja": {Title: "ナポリ風パスタ", Steps: []string{"一", "二"}, Description: " 説明 "},
	}
	got := recipeI18n(in, len(steps))
	if len(got) != 2 || got["en"].Title != "Navy-style pasta" || got["ja"].Description != "説明" {
		t.Fatalf("recipeI18n = %+v", got)
	}
	if recipeI18n(nil, 2) == nil || len(recipeI18n(nil, 2)) != 0 {
		t.Fatal("nil — без переводов")
	}
}
