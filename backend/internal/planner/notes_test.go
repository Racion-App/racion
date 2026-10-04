package planner

import (
	"testing"

	"racion/internal/i18n"
)

// Советы: язык страницы, запас — английский; русские только на русской странице.
func TestNotesFor(t *testing.T) {
	ru := Notes{Why: "Остуди смесь"}
	en := Notes{Why: "Cool the mix"}
	r := Recipe{Notes: map[string]Notes{"ru": ru, "en": en, "de": {}}}
	cases := map[string]Notes{"ru": ru, "en": en, "de": en, "ja": en}
	for l, want := range cases {
		if got := r.NotesFor(i18n.Lang(l)); got != want {
			t.Errorf("%s: %+v, want %+v", l, got, want)
		}
	}
	onlyRu := Recipe{Notes: map[string]Notes{"ru": ru}}
	if got := onlyRu.NotesFor("en"); !got.Empty() {
		t.Errorf("en without English notes must be empty, got %+v", got)
	}
	if got := onlyRu.NotesFor("ru"); got != ru {
		t.Errorf("ru: %+v", got)
	}
}
