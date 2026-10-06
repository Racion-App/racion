package planner

import (
	"slices"
	"testing"
)

// Сроки по таблице 5.1: мясо с 6 мес, желток с 7, творог, кефир и рыба с 8; фрукты не первыми.
func TestWeaningNext(t *testing.T) {
	if f, _ := WeaningNext(6, nil); f.Group != "veg" {
		t.Fatalf("первый прикорм — овощи, а не %s", f.ID)
	}
	if f, ok := WeaningNext(4, []string{"zucchini", "cauliflower", "broccoli", "buckwheat", "rice_round", "cornmeal"}); ok && f.From > 4 {
		t.Fatalf("в 4 мес нельзя %s", f.ID)
	}
	intro := []string{"zucchini", "cauliflower", "broccoli", "buckwheat", "rice_round", "cornmeal", "pumpkin", "carrot", "potato"}
	if f, _ := WeaningNext(6, intro); f.Group != "meat" {
		t.Fatalf("после овощей и каш в 6 мес — мясо, а не %s", f.ID)
	}
	all := []string{}
	for _, f := range WeaningFoods {
		if f.From <= 7 {
			all = append(all, f.ID)
		}
	}
	if f, ok := WeaningNext(7, all); ok {
		t.Fatalf("в 7 мес после всего до 7 мес ничего нового, а предлагается %s", f.ID)
	}
	if f, _ := WeaningNext(8, all); f.From != 8 {
		t.Fatalf("в 8 мес — творог, кефир или рыба, а не %s", f.ID)
	}
}

func TestWeaningGrams(t *testing.T) {
	cases := []struct {
		group string
		month int
		want  float64
	}{{"veg", 6, 150}, {"cereal", 8, 180}, {"cereal", 10, 200}, {"meat", 6, 15}, {"meat", 11, 50}, {"curd", 7, 0}, {"curd", 8, 40}, {"kefir", 9, 200}, {"yolk", 7, 0.25}}
	for _, c := range cases {
		if got := WeaningGrams(c.group, c.month); got != c.want {
			t.Errorf("%s в %d мес: %v, по таблице %v", c.group, c.month, got, c.want)
		}
	}
	r := WeaningRamp(150)
	if r[0] > 10 || r[6] != 150 || !slices.IsSorted(r) {
		t.Fatalf("наращивание за неделю: %v", r)
	}
}

// Меню только для ребёнка на прикорме: взрослых нет, семейных блюд нет, в списке покупок — продукты прикорма.
func TestKidsOnlyWeaning(t *testing.T) {
	c := testCatalog(t)
	p := Params{Country: "RU", Adults: 0, Slots: []string{"breakfast", "lunch", "dinner"}, Goal: "none",
		Kids: []Child{{AgeMonths: 7, Feeding: FeedWeaning, Introduced: []string{"zucchini", "broccoli", "buckwheat", "rice_round", "turkey_fillet"}}}}
	plan := c.Build(p)
	if plan.Params.Adults != 0 {
		t.Fatalf("взрослых стало %d", plan.Params.Adults)
	}
	for _, d := range plan.Days {
		if len(d.Dishes) > 0 {
			t.Fatalf("семейные блюда в меню только для ребёнка: %d в %s", len(d.Dishes), d.Label)
		}
	}
	if len(plan.KidsMenus) != 1 || plan.KidsMenus[0].Weaning == nil {
		t.Fatal("нет недели прикорма")
	}
	w := plan.KidsMenus[0].Weaning
	if w.New == nil || len(w.Ramp) != 7 {
		t.Fatalf("нет продукта недели: %+v", w.New)
	}
	found := false
	for _, g := range plan.Shopping {
		for _, it := range g.Items {
			if it.IngredientID == "zucchini" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("кабачка нет в списке покупок")
	}
}
