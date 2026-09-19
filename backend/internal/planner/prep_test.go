package planner

import "testing"

func TestKeepRules(t *testing.T) {
	cases := []struct {
		title  string
		tags   []string
		days   int
		freeze bool
	}{
		{"Борщ", []string{"soup"}, 3, true}, {"Куриные котлеты с пюре", nil, 3, true}, {"Греческий салат", []string{"salad"}, 0, false},
		{"Овсянка с ягодами", nil, 0, false}, {"Паста с креветками и чесноком", nil, 2, false}, {"Плов с курицей", nil, 3, true},
		{"Лосось в аэрогриле с брокколи", []string{"fish"}, 2, false}, {"Сырники со сметаной", nil, 2, true}, {"Свекольник холодный", nil, 2, false},
		{"Тефтели в томатном соусе", nil, 3, true}, {"Курица, запечённая с картофелем", nil, 3, false},
	}
	for _, c := range cases {
		d, f := Keep(Recipe{Title: c.title, Tags: c.tags})
		if d != c.days || f != c.freeze {
			t.Errorf("%s: got %d/%v want %d/%v", c.title, d, f, c.days, c.freeze)
		}
	}
	if d, f := Keep(Recipe{Title: "Что угодно", KeepDays: ptr(5), Freeze: true}); d != 5 || !f {
		t.Errorf("explicit fields ignored")
	}
}

func TestPrepAllowed(t *testing.T) {
	soup := Recipe{Title: "Борщ", Tags: []string{"soup"}, TimeMin: 70}
	salad := Recipe{Title: "Салат с тунцом", Tags: []string{"salad"}, TimeMin: 15}
	pasta := Recipe{Title: "Паста с креветками", TimeMin: 20}
	if pi, ok := prepAllowed(PrepOne, soup, 1); !ok || pi.Mode != "fridge" || pi.Session != -1 {
		t.Errorf("soup mon: %+v %v", pi, ok)
	}
	if pi, ok := prepAllowed(PrepOne, soup, 6); !ok || pi.Mode != "freezer" {
		t.Errorf("soup sun: %+v %v", pi, ok)
	}
	if pi, ok := prepAllowed(PrepTwo, soup, 5); !ok || pi.Mode != "fridge" || pi.Session != 2 {
		t.Errorf("soup sun two sessions: %+v %v", pi, ok)
	}
	if pi, ok := prepAllowed(PrepOne, salad, 4); !ok || pi.Mode != "fresh" {
		t.Errorf("salad: %+v %v", pi, ok)
	}
	if _, ok := prepAllowed(PrepOne, pasta, 4); ok {
		t.Errorf("pasta on friday from sunday must be rejected")
	}
	if _, ok := prepAllowed(PrepTwo, pasta, 4); !ok {
		t.Errorf("pasta on friday from wednesday must be fine")
	}
}

func ptr(n int) *int { return &n }
