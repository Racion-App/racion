package localprices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// BLS — U.S. Bureau of Labor Statistics, Average Price Data (серии APU0000…), средние по США.
// API v2 без ключа: до 25 серий в запросе, 25 запросов в сутки — нам хватает одного.
type BLS struct{}

func (BLS) Country() string { return "US" }
func (BLS) Key() string     { return "price.bls" }

const (
	lb  = 453.6
	gal = 3785.0
)

// blsMap — серия → продукт: сколько г/мл/шт в единице серии и множитель (грудка дороже тушки и т.п.).
var blsMap = []struct {
	Series     string
	Ingredient string
	PerUnit    float64
	Factor     float64
}{
	{"APU0000701111", "flour", lb, 1},
	{"APU0000701312", "rice", lb, 1},
	{"APU0000701312", "rice_round", lb, 1.1},
	{"APU0000701322", "pasta", lb, 1},
	{"APU0000702111", "bread_white", lb, 1},
	{"APU0000702212", "bread_wholegrain", lb, 1},
	{"APU0000702212", "bread_rye", lb, 1.1},
	{"APU0000703112", "ground_beef", lb, 1},
	{"APU0000FC1101", "ground_mixed", lb, 0.9},
	{"APU0000703432", "beef_stew", lb, 1},
	{"APU0000704111", "bacon", lb, 1},
	{"APU0000704212", "pork_loin", lb, 1},
	{"APU0000FD4101", "pork_neck", lb, 1},
	{"APU0000FD2101", "ham", lb, 1},
	{"APU0000706111", "chicken_whole", lb, 1},
	{"APU0000706212", "chicken_drumstick", lb, 1},
	{"APU0000706212", "chicken_thigh", lb, 1.15},
	{"APU0000706212", "chicken_wings", lb, 1.3},
	{"APU0000FF1101", "chicken_breast", lb, 1},
	{"APU0000FF1101", "ground_chicken", lb, 0.85},
	{"APU0000FF1101", "turkey_fillet", lb, 1.1},
	{"APU0000708111", "eggs", 12, 1},
	{"APU0000FJ1101", "milk", gal, 1},
	{"APU0000FJ1101", "kefir", gal, 2.5},
	{"APU0000FS1101", "butter", lb, 1},
	{"APU0000FJ4101", "yogurt_plain", 226.8, 1},
	{"APU0000710212", "cheese_cheddar", lb, 1},
	{"APU0000710212", "cheese_hard", lb, 1},
	{"APU0000711211", "banana", lb, 1},
	{"APU0000711311", "orange", lb, 1},
	{"APU0000711412", "lemon", lb, 1},
	{"APU0000712112", "potato", lb, 1},
	{"APU0000712211", "lettuce", lb, 1},
	{"APU0000712311", "tomato", lb, 1},
	{"APU0000715211", "sugar", lb, 1},
	{"APU0000714221", "corn_can", lb, 1},
	{"APU0000714233", "peas_dry", lb, 1},
	{"APU0000714233", "lentils", lb, 1.3},
	{"APU0000714233", "lentils_green", lb, 1.3},
}

func (b BLS) Fetch(ctx context.Context, packs map[string]PackInfo) (*Result, error) {
	seen := map[string]bool{}
	var ids []string
	for _, m := range blsMap {
		if !seen[m.Series] {
			seen[m.Series] = true
			ids = append(ids, m.Series)
		}
	}
	year := time.Now().Year()
	body, _ := json.Marshal(map[string]any{"seriesid": ids, "startyear": strconv.Itoa(year - 1), "endyear": strconv.Itoa(year)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.bls.gov/publicAPI/v2/timeseries/data/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "racion/1.0 (meal planner; average prices)")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var out struct {
		Status  string `json:"status"`
		Results struct {
			Series []struct {
				ID   string `json:"seriesID"`
				Data []struct {
					Year   string `json:"year"`
					Period string `json:"period"`
					Value  string `json:"value"`
				} `json:"data"`
			} `json:"series"`
		} `json:"Results"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Status != "REQUEST_SUCCEEDED" {
		return nil, fmt.Errorf("bls: %s", out.Status)
	}
	latest := map[string]float64{}
	var period time.Time
	for _, s := range out.Results.Series {
		for _, d := range s.Data {
			v, err := strconv.ParseFloat(d.Value, 64)
			if err != nil || v <= 0 || len(d.Period) != 3 || d.Period[0] != 'M' {
				continue
			}
			latest[s.ID] = v
			y, _ := strconv.Atoi(d.Year)
			m, _ := strconv.Atoi(d.Period[1:])
			if t := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC); t.After(period) {
				period = t
			}
			break // данные идут от новых к старым
		}
	}
	r := &Result{Period: period, Pack: map[string]float64{}}
	for _, m := range blsMap {
		v, ok := latest[m.Series]
		p, ok2 := packs[m.Ingredient]
		if !ok || !ok2 {
			continue
		}
		r.Pack[m.Ingredient] = packPrice(v, m.PerUnit, p, m.Factor)
	}
	return r, nil
}
