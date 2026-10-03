package planner

import "testing"

// По ссылке план видят чужие люди: имён взрослых, детей и аккаунтов в нём быть не должно,
// а цели, порции и возраст остаются — по ним идут в магазин.
func TestAnonymizeDropsNamesKeepsTheRest(t *testing.T) {
	p := Plan{
		Params: Params{
			Members: []Member{{Name: "Маша", Goal: "lose"}, {Name: "Петя", Goal: "gain"}},
			Kids:    []Child{{Name: "Соня", AgeMonths: 30}},
		},
		Members: []MemberView{{Name: "Маша", Goal: "lose", Kcal: 1600}, {Name: "Петя", Goal: "gain", Kcal: 2700}},
		Family:  []string{"vadim.petrov"},
	}
	p.Anonymize()
	for i, m := range p.Params.Members {
		if m.Name != "" {
			t.Errorf("params.members[%d].name = %q, want empty", i, m.Name)
		}
	}
	for i, m := range p.Members {
		if m.Name != "" {
			t.Errorf("members[%d].name = %q, want empty", i, m.Name)
		}
	}
	if p.Params.Kids[0].Name != "" {
		t.Errorf("kids[0].name = %q, want empty", p.Params.Kids[0].Name)
	}
	if p.Family != nil {
		t.Errorf("family = %v, want nil", p.Family)
	}
	if p.Members[0].Goal != "lose" || p.Members[0].Kcal != 1600 || p.Params.Kids[0].AgeMonths != 30 {
		t.Error("goals, kcal and ages must survive anonymization")
	}
}
