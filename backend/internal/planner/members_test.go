package planner

import "testing"

// Семья: разные цели дают разные порции, «не ест обед дома» уменьшает закупку обедов,
// «хочется на неделе» приводит продукт в меню.
func TestMembersAndWants(t *testing.T) {
	c := testCatalog(t)
	p := baseParams()
	p.Kids = nil
	p.Members = []Member{
		{Name: "Вадим", Goal: "gain", Appetite: "big"},
		{Name: "Аня", Goal: "lose", Appetite: "normal", Slots: []string{"breakfast", "dinner"}},
	}
	p.Wants = []string{"salmon_fillet", "nope"}
	plan := c.Build(p)

	if len(plan.Members) != 2 || plan.Members[0].Factor <= 1 || plan.Members[1].Factor >= 1 {
		t.Fatalf("factors must reflect goals: %+v", plan.Members)
	}
	if plan.Params.Goal != "healthy" || plan.Params.Adults != 2 {
		t.Fatalf("family goal mixes to healthy, adults = members: %s %d", plan.Params.Goal, plan.Params.Adults)
	}
	if plan.SlotPortions["lunch"] >= plan.SlotPortions["dinner"] {
		t.Fatalf("lunch is eaten by one member only: %+v", plan.SlotPortions)
	}
	if len(plan.Params.Wants) != 1 {
		t.Fatalf("unknown wants must be dropped: %v", plan.Params.Wants)
	}
	found := false
	for _, d := range plan.Days {
		if len(d.PerMember) != 2 || d.PerMember[0] <= d.PerMember[1] {
			t.Fatalf("per-member kcal must follow factors and slots: %+v", d.PerMember)
		}
		for _, dish := range d.Dishes {
			if dish.WhyCode.Wanted == "salmon_fillet" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("wanted product must appear in the week with a why note")
	}
	// замена сохраняет порции по приёмам
	sw, err := c.Swap(plan, 1, "dinner")
	if err != nil || sw.SlotPortions["lunch"] != plan.SlotPortions["lunch"] {
		t.Fatalf("swap must keep slot portions: %v %+v", err, sw.SlotPortions)
	}
	// старый формат без семьи работает как раньше
	old := baseParams()
	if pl := c.Build(old); len(pl.Members) != old.Adults || pl.Members[0].Factor != 1 {
		t.Fatalf("legacy adults become equal members: %+v", pl.Members)
	}
}
