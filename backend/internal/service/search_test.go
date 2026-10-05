package service

import "testing"

func TestHaveSet(t *testing.T) {
	s := haveSet([]string{"chicken_whole", "sour_cream_20"})
	for _, id := range []string{"chicken_breast", "chicken_thigh", "sour_cream", "sour_cream_20"} {
		if !s[id] {
			t.Errorf("%s должен считаться имеющимся", id)
		}
	}
	if s["ground_chicken"] || s["ground_beef"] {
		t.Error("фарш — другой продукт")
	}
}
