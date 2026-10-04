package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/planner"
	"racion/internal/vkusvill"
)

// StorePrices — цены сетей с открытым каталогом. Снимок из данных проекта говорит, каким товаром сети
// закрыт наш продукт; раз в сутки сервер сверяет по каждому товару цену и вес и подставляет свежие.
// Выбор товара сверка не меняет: это решение руками, ошибка в нём кладёт человеку в корзину чужое.
type StorePrices struct {
	repo    StorePriceRepo
	catalog StoreCatalog
	base    *planner.StorePrices
	units   map[string]string // наш продукт → единица (g, ml, pcs): вес упаковки сети переводим в неё
	apply   func(*planner.StorePrices)
	log     *zap.Logger
	pause   time.Duration // между запросами к их серверу: не нагружать чужой каталог
}

type StorePriceRepo interface {
	Claim(ctx context.Context, store string, every time.Duration) (bool, error)
	Save(ctx context.Context, store string, p domain.StorePrice) error
	Load(ctx context.Context, store string) (map[int]domain.StorePrice, error)
}

// StoreCatalog — каталог сети: товар по номеру.
type StoreCatalog interface {
	Product(ctx context.Context, id int) (vkusvill.Product, error)
}

// StorePriceEvery — как часто сверять цены. Чуть меньше суток: проверка раз в 6 часов ловит сутки
// без того, чтобы сверка каждый день уезжала на шесть часов позже.
const StorePriceEvery = 20 * time.Hour

func NewStorePrices(repo StorePriceRepo, catalog StoreCatalog, base *planner.StorePrices, units map[string]string, apply func(*planner.StorePrices), log *zap.Logger) *StorePrices {
	return &StorePrices{repo: repo, catalog: catalog, base: base, units: units, apply: apply, log: log, pause: 300 * time.Millisecond}
}

// Load — при запуске: цены последней сверки из базы поверх снимка.
func (s *StorePrices) Load(ctx context.Context) error {
	recs, err := s.repo.Load(ctx, s.base.Store)
	if err != nil {
		return err
	}
	s.use(recs)
	return nil
}

// Refresh — сверка всех товаров снимка, если прошлая была раньше StorePriceEvery. Сбой их сервера по
// одному товару не останавливает остальные и не считается снятием с продажи: снят — только по явному
// ответу «товар не найден».
func (s *StorePrices) Refresh(ctx context.Context) (int, error) {
	ok, err := s.repo.Claim(ctx, s.base.Store, StorePriceEvery)
	if err != nil || !ok {
		return 0, err
	}
	seen := map[int]bool{}
	checked, failed := 0, 0
	for _, it := range s.base.Items {
		if seen[it.XMLID] {
			continue
		}
		seen[it.XMLID] = true
		rec := domain.StorePrice{XMLID: it.XMLID, Found: true}
		p, err := s.catalog.Product(ctx, it.XMLID)
		switch {
		case errors.Is(err, vkusvill.ErrNotFound):
			rec.Found = false
		case err != nil:
			if ctx.Err() != nil {
				return checked, ctx.Err()
			}
			failed++
			continue
		default:
			rec.Name, rec.URL, rec.Unit, rec.Weight, rec.Price = p.Name, p.URL, p.Unit, p.Weight, p.Price
		}
		rec.CheckedAt = time.Now()
		if err := s.repo.Save(ctx, s.base.Store, rec); err != nil {
			return checked, err
		}
		checked++
		if s.pause > 0 && !sleep(ctx, s.pause) {
			return checked, ctx.Err()
		}
	}
	recs, err := s.repo.Load(ctx, s.base.Store)
	if err != nil {
		return checked, err
	}
	s.use(recs)
	if failed > 0 {
		s.log.Warn("store prices: some products not checked", zap.String("store", s.base.Store), zap.Int("failed", failed), zap.Int("checked", checked))
	}
	return checked, nil
}

// use — подставить в каталог снимок с ценами сверки и сказать в лог, что не взяли.
func (s *StorePrices) use(recs map[int]domain.StorePrice) {
	sp, skipped := mergeStorePrices(s.base, recs, s.units)
	s.apply(sp)
	s.log.Info("store prices", zap.String("store", sp.Store), zap.String("date", sp.Date), zap.Int("items", len(sp.Items)), zap.Strings("skipped", skipped))
}

// mergeStorePrices — снимок с ценами последней сверки. Снятый с продажи товар из снимка уходит: такой
// продукт считается по индексу сети, а не по цене, которой больше нет. Цена, прыгнувшая больше чем втрое,
// и смена единицы продажи — признак, что под номером теперь другой товар: такое не берём, а называем в
// skipped, чтобы человек проверил сопоставление.
func mergeStorePrices(base *planner.StorePrices, recs map[int]domain.StorePrice, units map[string]string) (*planner.StorePrices, []string) {
	out := &planner.StorePrices{Store: base.Store, Date: base.Date, Items: make(map[string]planner.StoreItem, len(base.Items))}
	baseDate, _ := time.Parse("2006-01-02", base.Date)
	var latest time.Time
	var skipped []string
	for id, it := range base.Items {
		r, ok := recs[it.XMLID]
		if !ok || r.CheckedAt.Before(baseDate) { // сверка старше снимка: снимок свежее
			out.Items[id] = it
			continue
		}
		if !r.Found {
			skipped = append(skipped, id+": снят с продажи")
			continue
		}
		if r.Unit != it.Unit || r.Price <= 0 || r.Price > it.Price*3 || r.Price*3 < it.Price {
			skipped = append(skipped, fmt.Sprintf("%s: %s %.0f → %s %.0f", id, it.Unit, it.Price, r.Unit, r.Price))
			out.Items[id] = it
			continue
		}
		it.Price = r.Price
		if r.Name != "" {
			it.Name = r.Name
		}
		if r.URL != "" {
			it.URL = r.URL
		}
		// упаковку в граммах и миллилитрах пересчитываем из веса; штучное (яйца по 10) задано руками
		if u := units[id]; it.Unit == "шт" && (u == "g" || u == "ml") && r.Weight > 0 {
			it.Pack = math.Round(r.Weight * 1000)
		}
		out.Items[id] = it
		if r.CheckedAt.After(latest) {
			latest = r.CheckedAt
		}
	}
	if latest.After(baseDate) {
		out.Date = latest.Format("2006-01-02")
	}
	return out, skipped
}
