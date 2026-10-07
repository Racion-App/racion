//go:build live

package recipeimport

// Проверка на живых сайтах: go test -tags live -run Live -v ./internal/recipeimport/ -args <url>...
// По умолчанию не собирается: сеть и чужие сайты в обычных тестах не нужны.

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"testing"
	"time"
)

func TestLive(t *testing.T) {
	var cat struct {
		Items []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Unit   string `json:"unit"`
			Pantry bool   `json:"pantry"`
		} `json:"items"`
	}
	b, err := os.ReadFile("../seed/data/ingredients.json")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(b, &cat)
	var i18n struct {
		Items map[string]map[string]any `json:"items"`
	}
	b, _ = os.ReadFile("../seed/data/ingredients_i18n.json")
	_ = json.Unmarshal(b, &i18n)
	var products []Product
	for _, it := range cat.Items {
		names := []string{it.Name}
		for _, v := range i18n.Items[it.ID] {
			if s, ok := v.(string); ok {
				names = append(names, s)
			}
		}
		products = append(products, Product{ID: it.ID, Name: it.Name, Names: names, Unit: it.Unit, Pantry: it.Pantry})
	}
	f := NewFetcher()
	for _, u := range flag.Args() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		p, err := f.Fetch(ctx, u)
		cancel()
		if err != nil {
			t.Logf("%s: %v", u, err)
			continue
		}
		d, _ := ParseDoc(p.HTML)
		r := d.Recipe()
		t.Logf("%s\n  title %q lang %q portions %d time %d complete %v cat %q\n  steps %d, first %q", u, r.Title, r.Lang, r.Portions, r.TimeMin, r.Complete(), r.Category, len(r.Steps), first(r.Steps))
		if !r.Complete() {
			txt := d.Text(14000)
			t.Logf("  text %d chars: %q", len([]rune(txt)), first([]string{txt}))
			continue
		}
		items, un := MatchRules(r.Ingredients, r.Portions, products)
		for _, it := range items {
			t.Logf("  + %-18s %7.2f  ← %s", it.ID, it.Amount, it.Line)
		}
		for _, l := range un {
			t.Logf("  ? %s", l)
		}
	}
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	r := []rune(s[0])
	if len(r) > 120 {
		r = r[:120]
	}
	return string(r)
}
