package localprices

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestLive гоняет адаптеры по настоящим источникам. Запуск: LOCALPRICES_LIVE=1 go test ./internal/localprices -run TestLive -v
func TestLive(t *testing.T) {
	if os.Getenv("LOCALPRICES_LIVE") == "" {
		t.Skip("set LOCALPRICES_LIVE=1")
	}
	b, err := os.ReadFile("../seed/data/ingredients.json")
	if err != nil {
		t.Fatal(err)
	}
	var ings struct {
		Items []struct {
			ID       string  `json:"id"`
			Unit     string  `json:"unit"`
			Pack     float64 `json:"pack"`
			Category string  `json:"category"`
		} `json:"items"`
	}
	_ = json.Unmarshal(b, &ings)
	b, _ = os.ReadFile("../seed/data/ingredients_i18n.json")
	var i18n struct {
		Items map[string]struct {
			Prices map[string]float64 `json:"prices"`
		} `json:"items"`
	}
	_ = json.Unmarshal(b, &i18n)
	packs := map[string]PackInfo{}
	for _, i := range ings.Items {
		packs[i.ID] = PackInfo{Pack: i.Pack, Unit: i.Unit, Category: i.Category, Local: i18n.Items[i.ID].Prices}
	}
	only := os.Getenv("LOCALPRICES_ONLY")
	for _, src := range Sources() {
		if only != "" && !strings.Contains(","+only+",", ","+src.Country()+",") {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		r, err := src.Fetch(ctx, packs)
		cancel()
		if err != nil {
			t.Errorf("%s: %v", src.Country(), err)
			continue
		}
		ids := make([]string, 0, len(r.Pack))
		for id := range r.Pack {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		t.Logf("%s %s period=%s items=%d", src.Country(), src.Key(), r.Period.Format("2006-01-02"), len(r.Pack))
		for _, id := range ids {
			t.Logf("  %-20s %8.2f  (manual %.2f)", id, r.Pack[id], packs[id].Local[src.Country()])
		}
	}
}
