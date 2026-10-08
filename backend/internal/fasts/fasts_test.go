package fasts

import (
	"testing"
	"time"
)

func day(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }

func TestEaster(t *testing.T) {
	// православная Пасха по новому стилю
	for _, c := range []struct{ y, m, d int }{{2024, 5, 5}, {2025, 4, 20}, {2026, 4, 12}, {2027, 5, 2}, {2028, 4, 16}, {2030, 4, 28}} {
		if got := Easter(c.y, time.UTC); !got.Equal(day(c.y, c.m, c.d)) {
			t.Errorf("%d: %s", c.y, got.Format("2006-01-02"))
		}
	}
}

func TestDates(t *testing.T) {
	byID := map[string]Fast{}
	for _, f := range Of(2027, time.UTC) {
		byID[f.ID] = f
	}
	if f := byID["great"]; !f.Start.Equal(day(2027, 3, 15)) || !f.End.Equal(day(2027, 5, 1)) {
		t.Errorf("Великий пост 2027: %v — %v", f.Start, f.End)
	}
	if f := byID["apostles"]; !f.Start.Equal(day(2027, 6, 28)) || !f.End.Equal(day(2027, 7, 11)) {
		t.Errorf("Петров пост 2027: %v — %v", f.Start, f.End)
	}
	ap26 := Of(2026, time.UTC)[1]
	if ap26.ID != "apostles" || !ap26.Start.Equal(day(2026, 6, 8)) {
		t.Errorf("Петров пост 2026: %+v", ap26)
	}
}

func TestUpcoming(t *testing.T) {
	cases := []struct {
		now time.Time
		id  string
		on  bool
	}{
		{day(2026, 10, 8), "nativity", false},
		{day(2026, 12, 15), "nativity", true},
		{day(2027, 1, 6), "nativity", true},
		{day(2027, 1, 7), "great", false},
		{day(2027, 3, 20), "great", true},
		{day(2027, 5, 10), "apostles", false},
		{day(2027, 7, 20), "dormition", false},
		{day(2027, 8, 20), "dormition", true},
		{day(2027, 9, 1), "nativity", false},
	}
	for _, c := range cases {
		f, on := Upcoming(c.now)
		if f.ID != c.id || on != c.on {
			t.Errorf("%s: %s %v, want %s %v", c.now.Format("2006-01-02"), f.ID, on, c.id, c.on)
		}
	}
}

func TestKind(t *testing.T) {
	nat := Fast{ID: "nativity", Start: day(2026, 11, 28), End: day(2027, 1, 6)}
	great := Fast{ID: "great", Start: day(2027, 3, 15), End: day(2027, 5, 1)}
	ap := Fast{ID: "apostles", Start: day(2026, 6, 8), End: day(2026, 7, 11)}
	dorm := Fast{ID: "dormition", Start: day(2026, 8, 14), End: day(2026, 8, 27)}
	cases := []struct {
		f    Fast
		d    time.Time
		want string
	}{
		{nat, day(2026, 11, 27), "out"},
		{nat, day(2026, 11, 28), "fish"}, // суббота
		{nat, day(2026, 11, 30), "lean"}, // понедельник
		{nat, day(2026, 12, 1), "fish"},  // вторник
		{nat, day(2026, 12, 4), "fish"},  // пятница, но Введение
		{nat, day(2026, 12, 22), "oil"},  // вторник после 19 декабря
		{nat, day(2026, 12, 27), "fish"}, // воскресенье
		{nat, day(2027, 1, 3), "oil"},    // воскресенье, рыбы уже нет
		{nat, day(2027, 1, 5), "lean"},
		{nat, day(2027, 1, 6), "eve"},
		{great, day(2027, 3, 15), "none"}, // Чистый понедельник
		{great, day(2027, 3, 16), "lean"},
		{great, day(2027, 3, 20), "oil"},  // суббота
		{great, day(2027, 4, 7), "fish"},  // Благовещение, среда
		{great, day(2027, 4, 24), "oil"},  // Лазарева суббота
		{great, day(2027, 4, 25), "fish"}, // Вербное воскресенье
		{great, day(2027, 4, 30), "none"}, // Страстная пятница
		{great, day(2027, 5, 1), "lean"},  // Великая суббота
		{ap, day(2026, 6, 8), "lean"},     // понедельник
		{ap, day(2026, 6, 9), "fish"},     // вторник
		{ap, day(2026, 6, 10), "lean"},    // среда
		{ap, day(2026, 7, 7), "fish"},     // Рождество Иоанна Предтечи, вторник
		{dorm, day(2026, 8, 14), "lean"},  // пятница
		{dorm, day(2026, 8, 15), "oil"},   // суббота
		{dorm, day(2026, 8, 19), "fish"},  // Преображение
	}
	for _, c := range cases {
		if got := Kind(c.f, c.d); got != c.want {
			t.Errorf("%s %s: %s, want %s", c.f.ID, c.d.Format("2006-01-02"), got, c.want)
		}
	}
	// Благовещение на Страстной — без рыбы: 2028, Пасха 16 апреля, 7 апреля — пятница Страстной... это день «none»;
	// в 2029 Пасха 8 апреля, 7 апреля — Великая суббота
	g29 := Of(2029, time.UTC)[0]
	if got := Kind(g29, day(2029, 4, 7)); got != "oil" {
		t.Errorf("Благовещение на Страстной 2029: %s", got)
	}
}
