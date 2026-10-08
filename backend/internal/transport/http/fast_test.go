package http

import (
	"testing"
	"time"
)

func TestFastKindOf(t *testing.T) {
	start := time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 1, 6, 0, 0, 0, 0, time.UTC)
	day := func(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }
	cases := []struct {
		d    time.Time
		want string
	}{
		{day(2026, 11, 27), "out"},
		{day(2026, 11, 28), "fish"}, // суббота
		{day(2026, 11, 30), "lean"}, // понедельник
		{day(2026, 12, 1), "fish"},  // вторник
		{day(2026, 12, 2), "lean"},  // среда
		{day(2026, 12, 4), "fish"},  // пятница, но Введение
		{day(2026, 12, 19), "fish"}, // суббота, святитель Николай
		{day(2026, 12, 22), "oil"},  // вторник после 19 декабря
		{day(2026, 12, 23), "lean"}, // среда
		{day(2026, 12, 27), "fish"}, // воскресенье
		{day(2027, 1, 1), "lean"},   // пятница
		{day(2027, 1, 3), "oil"},    // воскресенье, рыбы уже нет
		{day(2027, 1, 5), "lean"},   // вторник
		{day(2027, 1, 6), "eve"},
		{day(2027, 1, 7), "out"},
	}
	for _, c := range cases {
		if got := fastKindOf(c.d, start, end); got != c.want {
			t.Errorf("%s: %s, want %s", c.d.Format("2006-01-02"), got, c.want)
		}
	}
}
