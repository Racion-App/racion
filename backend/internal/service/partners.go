package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"racion/internal/domain"
	"racion/internal/i18n"
)

// Партнёрские ссылки. Принцип: ссылка появляется только там, где она отвечает на вопрос человека —
// «чем готовить» (техника в рецепте) и «где взять продукты» (корзина). Никаких баннеров и вставок в чек.
// Пока ссылка не помечена партнёрской, это обычная навигация «где купить»; с флагом Affiliate интерфейс
// подписывает «Реклама» и показывает erid — так требует закон о рекламе РФ, в других странах — disclosure.

//go:embed partners_default.json
var partnersDefault []byte

// Buyable — техника, за которой действительно идут в магазин. Плиту, духовку и микроволновку не предлагаем.
var Buyable = []string{"airfryer", "multicooker", "steamer", "grill", "blender", "mixer", "meatgrinder"}

type PartnerRepo interface {
	List(ctx context.Context) ([]domain.Partner, error)
	Upsert(ctx context.Context, p domain.Partner) error
	SeedDefaults(ctx context.Context, list []domain.Partner) error
	Delete(ctx context.Context, code string) error
}

type Partners struct {
	repo  PartnerRepo
	mu    sync.Mutex
	cache []domain.Partner
	at    time.Time
}

func NewPartners(repo PartnerRepo) *Partners { return &Partners{repo: repo} }

// Seed — дефолты для стран без записей; правки админа не трогает.
func (p *Partners) Seed(ctx context.Context) error {
	var list []domain.Partner
	if err := json.Unmarshal(partnersDefault, &list); err != nil {
		return fmt.Errorf("partners defaults: %w", err)
	}
	for i := range list {
		list[i].Active = true
	}
	return p.repo.SeedDefaults(ctx, list)
}

func (p *Partners) all(ctx context.Context) ([]domain.Partner, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cache != nil && time.Since(p.at) < time.Minute {
		return p.cache, nil
	}
	list, err := p.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	p.cache, p.at = list, time.Now()
	return list, nil
}

func (p *Partners) invalidate() {
	p.mu.Lock()
	p.cache = nil
	p.mu.Unlock()
}

// CountryView — что отдаём приложению: активные магазины страны по видам и слова для поиска техники
// на языке страны (в немецкий Amazon идём с «Heißluftfritteuse», а не с «Аэрогриль»).
type CountryView struct {
	Goods   []domain.Partner  `json:"goods"`
	Grocery []domain.Partner  `json:"grocery"`
	Queries map[string]string `json:"queries"`
}

func (p *Partners) ForCountry(ctx context.Context, country string) (CountryView, error) {
	list, err := p.all(ctx)
	if err != nil {
		return CountryView{}, err
	}
	v := CountryView{Goods: []domain.Partner{}, Grocery: []domain.Partner{}, Queries: map[string]string{}}
	for _, x := range list {
		if x.Country != country || !x.Active {
			continue
		}
		if x.Kind == "grocery" {
			v.Grocery = append(v.Grocery, x)
		} else {
			v.Goods = append(v.Goods, x)
		}
	}
	l := queryLang(country)
	for _, k := range Buyable {
		// слово для поиска: partner.q.<ключ>, если у языка есть уточнение («мангал» вместо «гриль»), иначе название техники
		if q := i18n.T(l, "partner.q."+k); q != "partner.q."+k {
			v.Queries[k] = q
		} else {
			v.Queries[k] = i18n.T(l, "equipment."+k)
		}
	}
	return v, nil
}

// BuyLink — ссылка «где купить» для одного вида техники: первый активный магазин страны.
type BuyLink struct {
	Key, Label, Name, URL, Erid string
	Affiliate                   bool
}

// BuyLinks — для SSR-страницы рецепта: по ссылке на каждую покупаемую технику из списка.
func (p *Partners) BuyLinks(ctx context.Context, country string, l i18n.Lang, equipment []string) []BuyLink {
	v, err := p.ForCountry(ctx, country)
	if err != nil || len(v.Goods) == 0 {
		return nil
	}
	shop := v.Goods[0]
	var out []BuyLink
	for _, e := range equipment {
		if q, ok := v.Queries[e]; ok {
			out = append(out, BuyLink{Key: e, Label: i18n.T(l, "equipment."+e), Name: shop.Name, URL: Expand(shop.URL, q), Erid: shop.Erid, Affiliate: shop.Affiliate})
		}
	}
	return out
}

// Expand — подставляет запрос в шаблон {q}.
func Expand(tpl, q string) string { return strings.ReplaceAll(tpl, "{q}", url.QueryEscape(q)) }

func queryLang(country string) i18n.Lang {
	if l := i18n.LangByCountry(country); l != "" {
		return i18n.Lang(l)
	}
	return "en"
}

// --- админка

func (p *Partners) AdminList(ctx context.Context) ([]domain.Partner, error) {
	list, err := p.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Country != list[j].Country {
			return list[i].Country < list[j].Country
		}
		if list[i].Kind != list[j].Kind {
			return list[i].Kind < list[j].Kind
		}
		return list[i].Priority < list[j].Priority
	})
	return list, nil
}

var codeRe = regexp.MustCompile(`^[a-z0-9_]{2,40}$`)

func (p *Partners) Save(ctx context.Context, x domain.Partner) (domain.Partner, error) {
	x.Code = strings.ToLower(strings.TrimSpace(x.Code))
	x.Country = strings.ToUpper(strings.TrimSpace(x.Country))
	x.Name = strings.TrimSpace(x.Name)
	x.URL = strings.TrimSpace(x.URL)
	x.Erid = strings.TrimSpace(x.Erid)
	switch {
	case !codeRe.MatchString(x.Code):
		return x, &domain.ValidationError{Key: "admin.partners.err.code"}
	case len(x.Country) != 2:
		return x, &domain.ValidationError{Key: "admin.partners.err.country"}
	case x.Kind != "goods" && x.Kind != "grocery":
		return x, &domain.ValidationError{Key: "admin.partners.err.kind"}
	case x.Name == "":
		return x, &domain.ValidationError{Key: "admin.partners.err.name"}
	case !strings.HasPrefix(x.URL, "https://") || !strings.Contains(x.URL, "{q}"):
		return x, &domain.ValidationError{Key: "admin.partners.err.url"}
	}
	if err := p.repo.Upsert(ctx, x); err != nil {
		return x, err
	}
	p.invalidate()
	return x, nil
}

func (p *Partners) Delete(ctx context.Context, code string) error {
	if code == "" {
		return errors.New("empty code")
	}
	if err := p.repo.Delete(ctx, code); err != nil {
		return err
	}
	p.invalidate()
	return nil
}
