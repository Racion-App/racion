package service

import (
	"context"
	"slices"
	"testing"

	"racion/internal/domain"
	"racion/internal/recipeimport"
)

func TestImportGuesses(t *testing.T) {
	cases := []struct {
		r    recipeimport.Recipe
		slot string
	}{
		{recipeimport.Recipe{Title: "Сырники из творога", Category: []string{"Завтраки"}}, "breakfast"},
		{recipeimport.Recipe{Title: "Борщ с говядиной"}, "lunch"},
		{recipeimport.Recipe{Title: "Easy pancakes", Category: []string{"Breakfast", "Main course"}}, "breakfast"},
		{recipeimport.Recipe{Title: "Датское печенье"}, "snack"},
		{recipeimport.Recipe{Title: "Курица с рисом"}, "dinner"},
		{recipeimport.Recipe{Title: "Spaghetti bolognese", Category: []string{"Dinner", "Lunch", "Main course"}}, "dinner"},
	}
	for _, c := range cases {
		if got := guessSlot(c.r); got != c.slot {
			t.Errorf("%q: %s, want %s", c.r.Title, got, c.slot)
		}
	}
	eq := guessEquipment([]string{"Обжарьте лук на сковороде.", "Переложите в форму и запекайте в духовке 30 минут."})
	if !slices.Equal(eq, []string{"oven", "stove"}) {
		t.Errorf("equipment: %v", eq)
	}
	if eq := guessEquipment([]string{"Смешайте всё в миске."}); !slices.Equal(eq, []string{"stove"}) {
		t.Errorf("default equipment: %v", eq)
	}
}

func TestImportRejectsBadURLAndLimits(t *testing.T) {
	s := NewRecipeImport(nil, nil)
	for _, u := range []string{"", "not a link", "http://127.0.0.1/x", "http://backend:8080/api"} {
		_, err := s.Import(context.Background(), "u1", u, "ru")
		var ve *domain.ValidationError
		if !asValidation(err, &ve) || ve.Key != "import.err.url" {
			t.Errorf("%q: %v", u, err)
		}
	}
	for i := 0; i < importsPerHour; i++ {
		if !s.allow("u2") {
			t.Fatalf("limit hit at %d", i)
		}
	}
	if s.allow("u2") {
		t.Fatal("no hourly limit")
	}
}

func asValidation(err error, ve **domain.ValidationError) bool {
	v, ok := err.(*domain.ValidationError)
	if ok {
		*ve = v
	}
	return ok
}
