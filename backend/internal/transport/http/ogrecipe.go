package http

import (
	"container/list"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/webp"

	"racion/internal/i18n"
	"racion/internal/ogimg"
	"racion/internal/planner"
)

// /og/recipe/{id}.jpg?l=ru&c=RU — карточка 1200×630 для превью ссылки на рецепт: фото, название, факты, призыв.
// Рисуется по запросу и кэшируется в памяти (последние 400 карточек ≈ 40 МБ); ключ включает название,
// поэтому правка рецепта в админке даёт новую картинку. Фото — из каталога images (том фронтенда).

var imagesDir string

type ogCache struct {
	mu   sync.Mutex
	m    map[string]*list.Element
	l    *list.List
	size int
}

type ogEntry struct {
	key  string
	data []byte
}

var ogc = &ogCache{m: map[string]*list.Element{}, l: list.New()}

func (c *ogCache) get(k string) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[k]; ok {
		c.l.MoveToFront(el)
		return el.Value.(*ogEntry).data
	}
	return nil
}

func (c *ogCache) put(k string, b []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[k]; ok {
		c.l.MoveToFront(el)
		el.Value.(*ogEntry).data = b
		return
	}
	c.m[k] = c.l.PushFront(&ogEntry{k, b})
	c.size++
	for c.size > 400 {
		last := c.l.Back()
		c.l.Remove(last)
		delete(c.m, last.Value.(*ogEntry).key)
		c.size--
	}
}

func loadPhoto(rel string) image.Image {
	if rel == "" || imagesDir == "" {
		return nil
	}
	// rel вида /images/recipes/id.webp — ищем jpg-копию (быстрее), потом webp
	p := filepath.Join(imagesDir, strings.TrimPrefix(rel, "/images/"))
	for _, cand := range []string{strings.TrimSuffix(p, ".webp") + ".jpg", p} {
		f, err := os.Open(cand)
		if err != nil {
			continue
		}
		var img image.Image
		if strings.HasSuffix(cand, ".webp") {
			img, err = webp.Decode(f)
		} else {
			img, _, err = image.Decode(f)
		}
		f.Close()
		if err == nil {
			return img
		}
	}
	return nil
}

func (s *Server) ogRecipe(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("id"), ".jpg")
	l := i18n.RU
	if lv, ok := i18n.Valid(r.URL.Query().Get("l")); ok {
		l = lv
	}
	country, _ := countryValid(strings.ToUpper(r.URL.Query().Get("c")))
	if country.Code == "" {
		country = planner.CountryOf(langCountry(l))
	}
	rc, err := s.svc.Recipes.Find(r.Context(), id)
	if err != nil || rc.Own && !rc.Public {
		http.NotFound(w, r)
		return
	}
	tx := rc.Text(l)
	c := s.catalog
	kcal, _, _, _ := c.Nutrition(rc)
	facts := []string{fmt.Sprintf("%d %s", rc.TimeMin, i18n.T(l, "min")), fmt.Sprintf("%d %s", int(math.Round(kcal)), i18n.T(l, "kcal"))}
	if cost, ok := c.PortionCost(rc, country.Code); ok && cost > 0 {
		facts = append(facts, formatMoney(country, cost))
	}
	card := ogimg.Card{Domain: strings.TrimPrefix(strings.TrimPrefix(s.baseURL(r), "https://"), "http://"), Title: tx.Title, Facts: strings.Join(facts, " · "), CTA: i18n.T(l, "og.cta")}
	key := id + "|" + string(l) + "|" + country.Code + "|" + tx.Title + "|" + card.Facts + "|" + rc.Image
	data := ogc.get(key)
	if data == nil {
		card.Photo = loadPhoto(rc.Image)
		data, err = ogimg.Render(card)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		ogc.put(key, data)
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(data)
}

// /og/collection/{slug}.jpg?l=ru — карточка подборки: обложка (фото первого рецепта), название, число рецептов.
func (s *Server) ogCollection(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSuffix(r.PathValue("slug"), ".jpg")
	l := i18n.RU
	if lv, ok := i18n.Valid(r.URL.Query().Get("l")); ok {
		l = lv
	}
	col, recipes, err := s.svc.Collections.BySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	col = col.Localized(string(l))
	facts := fmt.Sprintf("%d %s", len(recipes), i18n.Plural(l, len(recipes), "catalog.recipe"))
	card := ogimg.Card{Domain: strings.TrimPrefix(strings.TrimPrefix(s.baseURL(r), "https://"), "http://"), Title: col.Name, Facts: facts, CTA: i18n.T(l, "og.cta.collection")}
	key := "c|" + slug + "|" + string(l) + "|" + col.Name + "|" + facts + "|" + col.CoverAuto
	data := ogc.get(key)
	if data == nil {
		card.Photo = loadPhoto(col.CoverAuto)
		data, err = ogimg.Render(card)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		ogc.put(key, data)
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(data)
}
