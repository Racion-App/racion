// Package seed загружает базу продуктов, рецептов и магазинов из embedded JSON.
// Идемпотентно: upsert по id, запускается при каждом старте.
package seed

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"racion/internal/planner"
)

//go:embed data/*.json
var data embed.FS

type ingredient struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Unit       string   `json:"unit"`
	Pack       float64  `json:"pack"`
	Price      float64  `json:"price"`
	Kcal       float64  `json:"kcal"`
	Protein    float64  `json:"protein"`
	Fat        float64  `json:"fat"`
	Carb       float64  `json:"carb"`
	Allergens  []string `json:"allergens"`
	Perishable bool     `json:"perishable"`
	Pantry     bool     `json:"pantry"`
	Loose      bool     `json:"loose"`
	Rosstat    *struct {
		Item   int     `json:"item"`
		Factor float64 `json:"factor"`
		Note   string  `json:"note"`
	} `json:"rosstat"`
}

type recipe struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Slot        string              `json:"slot"`
	Time        int                 `json:"time"`
	Equipment   []string            `json:"equipment"`
	Tags        []string            `json:"tags"`
	Batch       bool                `json:"batch"`
	Keep        *int                `json:"keep"`
	Freeze      bool                `json:"freeze"`
	Ingredients [][]json.RawMessage `json:"ingredients"`
	Steps       []string            `json:"steps"`
	Image       string              `json:"image"`
	Description string              `json:"description"`
}

type store struct {
	Code       string  `json:"code"`
	Country    string  `json:"country"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	PriceIndex float64 `json:"price_index"`
	Sort       int     `json:"sort"`
}

// ingredientI18n — переводы названия и ручные цены по странам (data/ingredients_i18n.json).
type ingredientI18n struct {
	En     string             `json:"en"`
	De     string             `json:"de"`
	Prices map[string]float64 `json:"prices"`
}

// recipeI18n — перевод рецепта (data/recipes_i18n_<lang>.json: id → текст).
type recipeI18n struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
}

func loadMap[T any](name string) (map[string]T, error) {
	b, err := data.ReadFile("data/" + name)
	if err != nil {
		return map[string]T{}, nil // файла может не быть — переводов пока нет
	}
	var wrap struct {
		Items map[string]T `json:"items"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return wrap.Items, nil
}

// loadRecipeI18n собирает переводы по языкам: recipes_i18n_en.json, recipes_i18n_de.json.
func loadRecipeI18n() (map[string]map[string]recipeI18n, error) {
	out := map[string]map[string]recipeI18n{} // id → lang → текст
	entries, err := data.ReadDir("data")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "recipes_i18n_") || !strings.HasSuffix(n, ".json") {
			continue
		}
		lang := strings.TrimSuffix(strings.TrimPrefix(n, "recipes_i18n_"), ".json")
		m, err := loadMap[recipeI18n](n)
		if err != nil {
			return nil, err
		}
		for id, t := range m {
			if out[id] == nil {
				out[id] = map[string]recipeI18n{}
			}
			out[id][lang] = t
		}
	}
	return out, nil
}

func load[T any](name string) ([]T, error) {
	b, err := data.ReadFile("data/" + name)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Items []T `json:"items"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return wrap.Items, nil
}

// loadRecipes читает все data/recipes*.json — база разбита на несколько файлов по темам.
func loadRecipes() ([]recipe, error) {
	entries, err := data.ReadDir("data")
	if err != nil {
		return nil, err
	}
	var all []recipe
	seen := map[string]string{}
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "recipes") || !strings.HasSuffix(n, ".json") || strings.HasPrefix(n, "recipes_i18n") || n == "recipes_detail.json" || n == "recipes_notes.json" {
			continue
		}
		part, err := load[recipe](n)
		if err != nil {
			return nil, err
		}
		for _, r := range part {
			if prev, dup := seen[r.ID]; dup {
				return nil, fmt.Errorf("recipe id %q in both %s and %s", r.ID, prev, n)
			}
			seen[r.ID] = n
		}
		all = append(all, part...)
	}
	return all, nil
}

// Validate проверяет ссылки рецептов на ингредиенты. Битый рецепт — ошибка старта, а не тихий пропуск.
func Validate() error {
	ings, err := load[ingredient]("ingredients.json")
	if err != nil {
		return err
	}
	recs, err := loadRecipes()
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, i := range ings {
		known[i.ID] = true
	}
	for _, r := range recs {
		for _, pair := range r.Ingredients {
			var id string
			if len(pair) != 2 || json.Unmarshal(pair[0], &id) != nil {
				return fmt.Errorf("recipe %s: bad ingredient ref", r.ID)
			}
			if !known[id] {
				return fmt.Errorf("recipe %s references unknown ingredient %q", r.ID, id)
			}
		}
	}
	return nil
}

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	if err := Validate(); err != nil {
		return err
	}
	ings, _ := load[ingredient]("ingredients.json")
	recs, _ := loadRecipes()
	stores, err := load[store]("stores.json")
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	ingI18n, err := loadMap[ingredientI18n]("ingredients_i18n.json")
	if err != nil {
		return err
	}
	recI18n, err := loadRecipeI18n()
	if err != nil {
		return err
	}
	// подробные шаги от нейросети (cmd/racionai detail), если файл есть
	detail := map[string]recipeI18n{}
	if raw, rerr := data.ReadFile("data/recipes_detail.json"); rerr == nil {
		var f struct {
			Items map[string]recipeI18n `json:"items"`
		}
		if json.Unmarshal(raw, &f) == nil {
			detail = f.Items
		}
	}
	// фото: список id из images.json (пишет scripts/gen_images.py); файлы лежат в frontend/public/images/recipes
	withImage := map[string]bool{}
	if b, err := data.ReadFile("data/images.json"); err == nil {
		var m struct {
			Items []string `json:"items"`
		}
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("images.json: %w", err)
		}
		for _, id := range m.Items {
			withImage[id] = true
		}
	}
	for _, s := range stores {
		if s.Country == "" {
			s.Country = "RU"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO stores (code, country, name, kind, price_index, note, sort) VALUES ($1,$2,$3,$4,$5,'',$6)
			ON CONFLICT (code) DO UPDATE SET country=EXCLUDED.country, name=EXCLUDED.name, kind=EXCLUDED.kind, price_index=EXCLUDED.price_index, sort=EXCLUDED.sort`,
			s.Code, s.Country, s.Name, s.Kind, s.PriceIndex, s.Sort); err != nil {
			return err
		}
	}
	for _, i := range ings {
		if i.Allergens == nil {
			i.Allergens = []string{}
		}
		var rItem *int
		rFactor, rNote := 1.0, ""
		if i.Rosstat != nil && i.Rosstat.Item > 0 {
			v := i.Rosstat.Item
			rItem = &v
			if i.Rosstat.Factor > 0 {
				rFactor = i.Rosstat.Factor
			}
			rNote = i.Rosstat.Note
		}
		names := map[string]string{}
		prices := map[string]float64{}
		if t, ok := ingI18n[i.ID]; ok {
			if t.En != "" {
				names["en"] = t.En
			}
			if t.De != "" {
				names["de"] = t.De
			}
			for k, v := range t.Prices {
				prices[k] = v
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO ingredients (id, name, category, unit, pack, price, kcal, protein, fat, carb, allergens, perishable, pantry, loose, rosstat_item, rosstat_factor, rosstat_note, names, prices)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
			ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, category=EXCLUDED.category, unit=EXCLUDED.unit, pack=EXCLUDED.pack, price=EXCLUDED.price,
			kcal=EXCLUDED.kcal, protein=EXCLUDED.protein, fat=EXCLUDED.fat, carb=EXCLUDED.carb, allergens=EXCLUDED.allergens,
			perishable=EXCLUDED.perishable, pantry=EXCLUDED.pantry, loose=EXCLUDED.loose,
			rosstat_item=EXCLUDED.rosstat_item, rosstat_factor=EXCLUDED.rosstat_factor, rosstat_note=EXCLUDED.rosstat_note, names=EXCLUDED.names, prices=EXCLUDED.prices`,
			i.ID, i.Name, i.Category, i.Unit, i.Pack, i.Price, i.Kcal, i.Protein, i.Fat, i.Carb, i.Allergens, i.Perishable, i.Pantry, i.Loose, rItem, rFactor, rNote, names, prices); err != nil {
			return fmt.Errorf("ingredient %s: %w", i.ID, err)
		}
	}
	for _, r := range recs {
		if r.Equipment == nil {
			r.Equipment = []string{}
		}
		if r.Tags == nil {
			r.Tags = []string{}
		}
		tr := recI18n[r.ID]
		if tr == nil {
			tr = map[string]recipeI18n{}
		}
		if r.Image == "" && withImage[r.ID] {
			r.Image = "/images/recipes/" + r.ID + ".webp"
		}
		if d, ok := detail[r.ID]; ok && len(d.Steps) > 0 { // подробные шаги от нейросети поверх коротких
			r.Steps = d.Steps
			if d.Description != "" {
				r.Description = d.Description
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO recipes (id, title, slot, time_min, equipment, tags, batch, steps, image, description, i18n, keep_days, can_freeze) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title, slot=EXCLUDED.slot, time_min=EXCLUDED.time_min, equipment=EXCLUDED.equipment,
			tags=EXCLUDED.tags, batch=EXCLUDED.batch, steps=EXCLUDED.steps, image=EXCLUDED.image, description=EXCLUDED.description, i18n=recipes.i18n || EXCLUDED.i18n, keep_days=EXCLUDED.keep_days, can_freeze=EXCLUDED.can_freeze
			WHERE recipes.edited_at IS NULL`,
			r.ID, r.Title, r.Slot, r.Time, r.Equipment, r.Tags, r.Batch, r.Steps, r.Image, r.Description, tr, r.Keep, r.Freeze); err != nil {
			return fmt.Errorf("recipe %s: %w", r.ID, err)
		}
		// продукты рецепта, правленного из админки, тоже его
		var edited bool
		if err := tx.QueryRow(ctx, `SELECT edited_at IS NOT NULL FROM recipes WHERE id = $1`, r.ID).Scan(&edited); err != nil {
			return err
		}
		if edited {
			continue
		}
		if _, err := tx.Exec(ctx, `DELETE FROM recipe_ingredients WHERE recipe_id = $1`, r.ID); err != nil {
			return err
		}
		for n, pair := range r.Ingredients {
			var id string
			var amount float64
			_ = json.Unmarshal(pair[0], &id)
			if err := json.Unmarshal(pair[1], &amount); err != nil {
				return fmt.Errorf("recipe %s: bad amount for %s", r.ID, id)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO recipe_ingredients (recipe_id, ingredient_id, amount, sort) VALUES ($1,$2,$3,$4)`, r.ID, id, amount, n); err != nil {
				return err
			}
		}
	}
	// картинки продуктов: манифест ingredient_images.json (scripts/gen_ingredient_images.py)
	if raw, err := data.ReadFile("data/ingredient_images.json"); err == nil {
		var f struct {
			Items []string `json:"items"`
		}
		if json.Unmarshal(raw, &f) == nil {
			for _, id := range f.Items {
				if _, err := tx.Exec(ctx, `UPDATE ingredients SET image = $2 WHERE id = $1`, id, "/images/ingredients/"+id+".webp"); err != nil {
					return err
				}
			}
		}
	}
	// заметки к рецептам по языкам (recipes_notes.json, cmd/racionai notes) — наши, обновляем всегда
	if raw, err := data.ReadFile("data/recipes_notes.json"); err == nil {
		var f struct {
			Items map[string]map[string]planner.Notes `json:"items"`
		}
		if json.Unmarshal(raw, &f) == nil {
			for id, notes := range f.Items {
				if _, err := tx.Exec(ctx, `UPDATE recipes SET notes = $2 WHERE id = $1`, id, notes); err != nil {
					return fmt.Errorf("notes %s: %w", id, err)
				}
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return seedCollections(ctx, pool)
}

// seedCollections — редакционные подборки из collections.json: upsert по slug, рецепты целиком. Файл — источник
// правды (правки из админки переносятся в него экспортом, см. ops/export_collections.py); подборки, которых
// в файле нет, не трогаются. Рецепты, которых нет в базе, пропускаются.
func seedCollections(ctx context.Context, pool *pgxpool.Pool) error {
	raw, err := data.ReadFile("data/collections.json")
	if err != nil {
		return nil
	}
	var list []struct {
		Slug, Name, Description, Cover string
		Public                         bool
		Names, Descriptions            map[string]string
		Recipes                        []string
		SEO                            map[string]json.RawMessage `json:"seo"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("collections.json: %w", err)
	}
	for _, c := range list {
		if c.Names == nil {
			c.Names = map[string]string{}
		}
		if c.Descriptions == nil {
			c.Descriptions = map[string]string{}
		}
		var id string
		if c.SEO == nil {
			c.SEO = map[string]json.RawMessage{}
		}
		if err := pool.QueryRow(ctx, `INSERT INTO collections (user_id, name, description, cover, slug, public, curated, name_i18n, description_i18n, seo)
			VALUES (NULL, $1, $2, $3, $4, $5, true, $6, $7, $8)
			ON CONFLICT (slug) WHERE slug IS NOT NULL DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, cover = EXCLUDED.cover,
			public = EXCLUDED.public, curated = true, name_i18n = EXCLUDED.name_i18n, description_i18n = EXCLUDED.description_i18n, seo = EXCLUDED.seo
			RETURNING id`, c.Name, c.Description, c.Cover, c.Slug, c.Public, c.Names, c.Descriptions, c.SEO).Scan(&id); err != nil {
			return fmt.Errorf("collection %s: %w", c.Slug, err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM collection_items WHERE collection_id = $1`, id); err != nil {
			return err
		}
		for n, rid := range c.Recipes {
			if _, err := pool.Exec(ctx, `INSERT INTO collection_items (collection_id, recipe_id, added_at)
				SELECT $1, $2, now() + make_interval(secs => $3) WHERE EXISTS (SELECT 1 FROM recipes WHERE id = $2) ON CONFLICT DO NOTHING`, id, rid, n); err != nil {
				return err
			}
		}
	}
	return nil
}

// Substitutes — таблица замен продуктов из data/substitutes.json: id → варианты {id, ratio, note}.
func Substitutes() map[string][]SubEntry {
	out := map[string][]SubEntry{}
	raw, err := data.ReadFile("data/substitutes.json")
	if err != nil {
		return out
	}
	var f struct {
		Items map[string][]SubEntry `json:"items"`
	}
	if json.Unmarshal(raw, &f) == nil {
		out = f.Items
	}
	return out
}

type SubEntry struct {
	ID     string   `json:"id"`
	Ratio  float64  `json:"ratio"`
	Note   string   `json:"note"`
	NoteEn string   `json:"note_en"` // заметка по-английски: для всех языков, кроме русского
	Not    []string `json:"not"`     // не предлагать рецептам с этими тегами (бульон вместо вина — не в напиток)
}

// Occasions — события из data/occasions.json.
func Occasions() []planner.Occasion {
	raw, err := data.ReadFile("data/occasions.json")
	if err != nil {
		return nil
	}
	var f struct {
		Items []planner.Occasion `json:"items"`
	}
	_ = json.Unmarshal(raw, &f)
	return f.Items
}
