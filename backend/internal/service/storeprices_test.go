package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/planner"
	"racion/internal/vkusvill"
)

func vvBase() *planner.StorePrices {
	return &planner.StorePrices{Store: "vkusvill", Date: "2026-10-03", Items: map[string]planner.StoreItem{
		"milk":   {XMLID: 1, Name: "Молоко 2,5%", Unit: "шт", Pack: 930, Price: 99},
		"eggs":   {XMLID: 2, Name: "Яйца С1, 10 шт", Unit: "шт", Pack: 10, Price: 149},
		"potato": {XMLID: 3, Name: "Картофель", Unit: "кг", Price: 79},
		"cheese": {XMLID: 4, Name: "Сыр", Unit: "шт", Pack: 200, Price: 300},
		"dill":   {XMLID: 5, Name: "Укроп", Unit: "шт", Pack: 50, Price: 70},
		"tofu":   {XMLID: 6, Name: "Тофу", Unit: "шт", Pack: 300, Price: 180},
	}}
}

var vvUnits = map[string]string{"milk": "ml", "eggs": "pcs", "potato": "g", "cheese": "g", "dill": "g", "tofu": "g"}

func TestMergeStorePrices(t *testing.T) {
	day := time.Date(2026, 10, 5, 4, 0, 0, 0, time.UTC)
	recs := map[int]domain.StorePrice{
		1: {XMLID: 1, Unit: "шт", Weight: 1, Price: 109, Found: true, CheckedAt: day},        // стало литр: упаковка 1000 мл
		2: {XMLID: 2, Unit: "шт", Weight: 0.6, Price: 159, Found: true, CheckedAt: day},      // яйца по весу не пересчитываем
		3: {XMLID: 3, Unit: "кг", Price: 85, Found: true, CheckedAt: day},                    // весовое: цена за кг
		4: {XMLID: 4, Unit: "шт", Weight: 0.2, Price: 1290, Found: true, CheckedAt: day},     // вчетверо дороже — другой товар
		5: {XMLID: 5, Found: false, CheckedAt: day},                                          // сняли с продажи
		6: {XMLID: 6, Unit: "шт", Price: 190, Found: true, CheckedAt: day.AddDate(0, 0, -5)}, // сверка старше снимка
	}
	sp, skipped := mergeStorePrices(vvBase(), recs, vvUnits)
	if it := sp.Items["milk"]; it.Price != 109 || it.Pack != 1000 {
		t.Errorf("молоко %+v", it)
	}
	if it := sp.Items["eggs"]; it.Price != 159 || it.Pack != 10 {
		t.Errorf("яйца %+v", it)
	}
	if it := sp.Items["potato"]; it.Price != 85 || it.PerUnit() != 0.085 {
		t.Errorf("картофель %+v", it)
	}
	if it := sp.Items["cheese"]; it.Price != 300 {
		t.Errorf("скачок цены принят: %+v", it)
	}
	if _, ok := sp.Items["dill"]; ok {
		t.Error("снятый с продажи товар остался в расчёте")
	}
	if it := sp.Items["tofu"]; it.Price != 180 {
		t.Errorf("сверка старше снимка подменила цену: %+v", it)
	}
	if sp.Date != "2026-10-05" || len(skipped) != 2 {
		t.Errorf("дата %s, не взяли %v", sp.Date, skipped)
	}
}

type memStorePrices struct {
	recs    map[int]domain.StorePrice
	claimed bool
}

func (m *memStorePrices) Claim(context.Context, string, time.Duration) (bool, error) {
	if m.claimed {
		return false, nil
	}
	m.claimed = true
	return true, nil
}
func (m *memStorePrices) Save(_ context.Context, _ string, p domain.StorePrice) error {
	m.recs[p.XMLID] = p
	return nil
}
func (m *memStorePrices) Load(context.Context, string) (map[int]domain.StorePrice, error) {
	return m.recs, nil
}

// fakeVV — каталог сети: товар 5 снят, на товаре 6 их сервер падает.
type fakeVV struct{ calls int }

func (f *fakeVV) Product(_ context.Context, id int) (vkusvill.Product, error) {
	f.calls++
	switch id {
	case 5:
		return vkusvill.Product{}, vkusvill.ErrNotFound
	case 6:
		return vkusvill.Product{}, errors.New("vkusvill: http 502")
	}
	unit := "шт"
	if id == 3 { // картофель на развес
		unit = "кг"
	}
	return vkusvill.Product{XMLID: id, Unit: unit, Weight: 0.93, Price: 120}, nil
}

func TestStorePricesRefresh(t *testing.T) {
	repo, vv := &memStorePrices{recs: map[int]domain.StorePrice{}}, &fakeVV{}
	var applied *planner.StorePrices
	s := NewStorePrices(repo, vv, vvBase(), vvUnits, func(sp *planner.StorePrices) { applied = sp }, zap.NewNop())
	s.pause = 0
	n, err := s.Refresh(context.Background())
	if err != nil || n != 5 || vv.calls != 6 {
		t.Fatalf("сверено %d из %d, err=%v", n, vv.calls, err)
	}
	if repo.recs[6].XMLID != 0 {
		t.Error("сбой их сервера записан как сверка")
	}
	if _, ok := applied.Items["dill"]; ok || applied.Items["milk"].Price != 120 || applied.Items["tofu"].Price != 180 {
		t.Errorf("применено %+v", applied.Items)
	}
	if n, _ := s.Refresh(context.Background()); n != 0 {
		t.Error("вторая сверка за сутки")
	}
}
