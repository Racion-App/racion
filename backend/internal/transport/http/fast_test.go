package http

import (
	"strings"
	"testing"
	"time"

	"racion/internal/i18n"
)

// Календарь сам переходит от поста к посту: на любую дату — нужный пост, его заголовок, статус и вопросы
// без пропущенных ключей локали.
func TestFastCalendarSwitches(t *testing.T) {
	day := func(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 12, 0, 0, 0, time.UTC) }
	cases := []struct {
		now          time.Time
		id, title    string
		status, kind string
	}{
		{day(2026, 10, 8), "nativity", "Рождественский пост 2026–2027", "До Рождественского поста 51 день", "fish"},
		{day(2026, 12, 15), "nativity", "Рождественский пост 2026–2027", "Идёт 18-й день поста из 40", "fish"},
		{day(2027, 1, 10), "great", "Великий пост 2027", "До Великого поста 64 дня", "none"},
		{day(2027, 3, 20), "great", "Великий пост 2027", "Идёт 6-й день поста из 48", "oil"},
		{day(2027, 6, 1), "apostles", "Петров пост 2027", "До Петрова поста 27 дней", "lean"},
		{day(2027, 8, 19), "dormition", "Успенский пост 2027", "Идёт 6-й день поста из 14", "fish"},
	}
	for _, c := range cases {
		fc := newFastCalendar(i18n.RU, c.now)
		if fc.ID != c.id || !strings.HasPrefix(fc.Title, c.title) || fc.Status != c.status || fc.SelectedKind != c.kind {
			t.Errorf("%s: %s %q %q %s", c.now.Format("2006-01-02"), fc.ID, fc.Title, fc.Status, fc.SelectedKind)
		}
		for _, qa := range fc.FAQ {
			if strings.HasPrefix(qa.Q, "fast.") || strings.HasPrefix(qa.A, "fast.") || strings.Contains(qa.A, "{") {
				t.Errorf("%s: незаполненный вопрос %q / %q", c.id, qa.Q, qa.A)
			}
		}
		for _, k := range fc.Legend {
			if strings.HasPrefix(k.Label, "fast.") {
				t.Errorf("%s: нет подписи %s", c.id, k.Kind)
			}
		}
	}
	// у Великого поста есть дни без еды, у Петрова — нет ни их, ни Сочельника
	if fc := newFastCalendar(i18n.RU, day(2027, 3, 20)); !fc.Kinds["none"] || fc.Kinds["eve"] {
		t.Errorf("Великий пост: виды дней %v", fc.Kinds)
	}
	if fc := newFastCalendar(i18n.RU, day(2027, 6, 1)); fc.Kinds["none"] || fc.Kinds["eve"] || fc.Kinds["oil"] {
		t.Errorf("Петров пост: виды дней %v", fc.Kinds)
	}
}
