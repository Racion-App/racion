package metrika

import (
	"testing"
	"time"
)

// Личные адреса недель и столов не должны расползаться по топу входных страниц: все /plan/<uuid> — одна строка.
func TestGroupPages(t *testing.T) {
	got := groupPages([]Row{
		{Name: "/", Visits: 109},
		{Name: "/plan/c6d81a0b-293b-4a20-970e-1a7b65ac97e2", Visits: 4},
		{Name: "/collection/kids-1-3", Visits: 54},
		{Name: "/plan/5e9c151e-b43f-497f-87ec-0d0ea6804dd2", Visits: 3},
		{Name: "/en/plan/0acd6328-7937-4cb5-882e-3622c26cbc84/print", Visits: 2},
		{Name: "/recipe/kid_semolina", Visits: 1},
	}, 4)
	want := []Row{{Name: "/", Visits: 109}, {Name: "/collection/kids-1-3", Visits: 54}, {Name: "/plan/…", Visits: 7}, {Name: "/en/plan/…/print", Visits: 2}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestDisabledWithoutToken(t *testing.T) {
	var nilClient *Client
	if nilClient.Enabled() || New("112818312", "").Enabled() || New("", "tok").Enabled() {
		t.Fatal("сводка без счётчика или токена должна быть выключена")
	}
	if _, err := New("1", "").Day(t.Context(), time.Now(), "ru"); err == nil {
		t.Fatal("без токена Day должен вернуть ошибку")
	}
}
