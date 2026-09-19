package service

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"racion/internal/domain"
)

// Рекламные предложения. Те же принципы, что у партнёрских ссылок: показываем одно предложение и только
// там, где оно отвечает на вопрос человека: у корзины (где купить продукты дешевле), в рецепте (чем готовить,
// что взять), в плане. Таргетинг по стране и региону: региональные акции не показываем всей стране.

type OfferRepo interface {
	List(ctx context.Context) ([]domain.Offer, error)
	Upsert(ctx context.Context, o domain.Offer) error
	Delete(ctx context.Context, id string) error
}

type Offers struct {
	repo  OfferRepo
	mu    sync.Mutex
	cache []domain.Offer
	at    time.Time
}

func NewOffers(repo OfferRepo) *Offers { return &Offers{repo: repo} }

// OfferQuery — контекст показа: страна обязательна; Region и его родитель (город → область) — чтобы
// региональная акция совпала и по городу, и по области; Match — техника и продукты страницы.
type OfferQuery struct {
	Country string
	Region  string
	Parent  string
	Place   string
	Match   []string
}

func (o *Offers) all(ctx context.Context) ([]domain.Offer, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cache != nil && time.Since(o.at) < time.Minute {
		return o.cache, nil
	}
	list, err := o.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	o.cache, o.at = list, time.Now()
	return list, nil
}

func (o *Offers) invalidate() {
	o.mu.Lock()
	o.cache = nil
	o.mu.Unlock()
}

// Pick — до limit предложений для контекста, по приоритету. Региональные показываем только при совпадении
// региона; общие по стране — всем. С Match предложение уместно, если на странице есть хоть один его id.
func (o *Offers) Pick(ctx context.Context, q OfferQuery, limit int) []domain.Offer {
	list, err := o.all(ctx)
	if err != nil || q.Country == "" {
		return nil
	}
	today := time.Now().UTC().Format("2006-01-02")
	out := []domain.Offer{}
	for _, x := range list {
		if !x.Active || x.Country != q.Country || x.Place != q.Place {
			continue
		}
		if x.StartsAt != nil && *x.StartsAt > today || x.EndsAt != nil && *x.EndsAt < today {
			continue
		}
		if len(x.Regions) > 0 && !contains(x.Regions, q.Region) && !contains(x.Regions, q.Parent) {
			continue
		}
		if len(x.Match) > 0 && !anyOf(x.Match, q.Match) {
			continue
		}
		out = append(out, x)
	}
	sort.SliceStable(out, func(i, j int) bool {
		// региональные точнее общих, дальше приоритет
		ri, rj := len(out[i].Regions) > 0, len(out[j].Regions) > 0
		if ri != rj {
			return ri
		}
		return out[i].Priority > out[j].Priority
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func contains(list []string, v string) bool {
	if v == "" {
		return false
	}
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func anyOf(want, have []string) bool {
	for _, w := range want {
		if contains(have, w) {
			return true
		}
	}
	return false
}

func (o *Offers) AdminList(ctx context.Context) ([]domain.Offer, error) { return o.repo.List(ctx) }

var offerIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)

func (o *Offers) Save(ctx context.Context, x domain.Offer) (domain.Offer, error) {
	x.ID = strings.ToLower(strings.TrimSpace(x.ID))
	x.Country = strings.ToUpper(strings.TrimSpace(x.Country))
	x.Title, x.Body, x.CTA, x.Promo, x.Erid = strings.TrimSpace(x.Title), strings.TrimSpace(x.Body), strings.TrimSpace(x.CTA), strings.TrimSpace(x.Promo), strings.TrimSpace(x.Erid)
	x.URL, x.Image = strings.TrimSpace(x.URL), strings.TrimSpace(x.Image)
	x.Regions, x.Match = cleanList(x.Regions), cleanList(x.Match)
	switch {
	case !offerIDRe.MatchString(x.ID):
		return x, &domain.ValidationError{Key: "admin.offers.err.id"}
	case len(x.Country) != 2:
		return x, &domain.ValidationError{Key: "admin.offers.err.country"}
	case x.Place != "cart" && x.Place != "recipe" && x.Place != "plan":
		return x, &domain.ValidationError{Key: "admin.offers.err.place"}
	case x.Title == "" || len([]rune(x.Title)) > 80:
		return x, &domain.ValidationError{Key: "admin.offers.err.title"}
	case len([]rune(x.Body)) > 300:
		return x, &domain.ValidationError{Key: "admin.offers.err.body"}
	}
	if u, err := url.Parse(x.URL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return x, &domain.ValidationError{Key: "admin.offers.err.url"}
	}
	if x.Image != "" {
		if u, err := url.Parse(x.Image); err != nil || (u.Scheme != "https" && !strings.HasPrefix(x.Image, "/")) {
			return x, &domain.ValidationError{Key: "admin.offers.err.image"}
		}
	}
	for _, d := range []*string{x.StartsAt, x.EndsAt} {
		if d != nil && *d != "" {
			if _, err := time.Parse("2006-01-02", *d); err != nil {
				return x, &domain.ValidationError{Key: "admin.offers.err.date"}
			}
		}
	}
	if x.Affiliate && x.Country == "RU" && x.Erid == "" {
		return x, &domain.ValidationError{Key: "admin.offers.err.erid"}
	}
	if err := o.repo.Upsert(ctx, x); err != nil {
		return x, err
	}
	o.invalidate()
	return x, nil
}

func (o *Offers) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("offer: empty id")
	}
	if err := o.repo.Delete(ctx, id); err != nil {
		return err
	}
	o.invalidate()
	return nil
}

func cleanList(in []string) []string {
	out := []string{}
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
