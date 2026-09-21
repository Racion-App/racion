package planner

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LoadCatalog читает всю базу в память.
func LoadCatalog(ctx context.Context, pool *pgxpool.Pool) (*Catalog, error) {
	return loadCatalog(ctx, pool, nil)
}

// Reload — свежий каталог из БД с теми же ценниками (они обновляются в фоне и общие для всех копий).
// Нужен после правки рецептов из админки: новый каталог кладут в CatalogRef, старые копии доживают запрос.
func (c *Catalog) Reload(ctx context.Context, pool *pgxpool.Pool) (*Catalog, error) {
	return loadCatalog(ctx, pool, c.prices)
}

func loadCatalog(ctx context.Context, pool *pgxpool.Pool, prices *priceStore) (*Catalog, error) {
	c := NewCatalog()
	if prices != nil {
		c.prices = prices
	}

	rows, err := pool.Query(ctx, `SELECT id, name, category, unit, pack, price, kcal, protein, fat, carb, allergens, perishable, pantry, loose, COALESCE(rosstat_item, 0), rosstat_factor, rosstat_note, names, prices, image, tier FROM ingredients`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var i Ingredient
		if err := rows.Scan(&i.ID, &i.Name, &i.Category, &i.Unit, &i.Pack, &i.Price, &i.Kcal, &i.Protein, &i.Fat, &i.Carb, &i.Allergens, &i.Perishable, &i.Pantry, &i.Loose, &i.RosstatItem, &i.RosstatFactor, &i.RosstatNote, &i.Names, &i.Prices, &i.Image, &i.Tier); err != nil {
			rows.Close()
			return nil, err
		}
		if i.Allergens == nil {
			i.Allergens = []string{}
		}
		c.Ingredients[i.ID] = i
	}
	rows.Close()

	rows, err = pool.Query(ctx, `SELECT id, title, slot, time_min, equipment, tags, batch, steps, image, description, i18n, notes, keep_days, can_freeze, hidden FROM recipes WHERE NOT deleted ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Recipe
		if err := rows.Scan(&r.ID, &r.Title, &r.Slot, &r.TimeMin, &r.Equipment, &r.Tags, &r.Batch, &r.Steps, &r.Image, &r.Description, &r.I18n, &r.Notes, &r.KeepDays, &r.Freeze, &r.Hidden); err != nil {
			rows.Close()
			return nil, err
		}
		if r.Equipment == nil {
			r.Equipment = []string{}
		}
		if r.Tags == nil {
			r.Tags = []string{}
		}
		c.Recipes = append(c.Recipes, r)
	}
	rows.Close()

	rows, err = pool.Query(ctx, `SELECT recipe_id, ingredient_id, amount FROM recipe_ingredients ORDER BY recipe_id, sort`)
	if err != nil {
		return nil, err
	}
	byRecipe := map[string][]RecipeIngredient{}
	for rows.Next() {
		var rid string
		var ri RecipeIngredient
		if err := rows.Scan(&rid, &ri.IngredientID, &ri.Amount); err != nil {
			rows.Close()
			return nil, err
		}
		byRecipe[rid] = append(byRecipe[rid], ri)
	}
	rows.Close()
	for i := range c.Recipes {
		c.Recipes[i].Ingredients = byRecipe[c.Recipes[i].ID]
		c.RecipeByID[c.Recipes[i].ID] = c.Recipes[i]
	}

	rows, err = pool.Query(ctx, `SELECT code, country, name, kind, price_index, sort FROM stores ORDER BY sort`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s Store
		if err := rows.Scan(&s.Code, &s.Country, &s.Name, &s.Kind, &s.PriceIndex, &s.Sort); err != nil {
			rows.Close()
			return nil, err
		}
		c.Stores[s.Code] = s
		c.StoreList = append(c.StoreList, s)
	}
	rows.Close()
	sort.Slice(c.StoreList, func(i, j int) bool { return c.StoreList[i].Sort < c.StoreList[j].Sort })

	// Живые цены других стран, если синхронизация уже была.
	rows, err = pool.Query(ctx, `SELECT country, ingredient_id, pack_price, period, source FROM local_prices`)
	if err != nil {
		return nil, err
	}
	locals := map[string]*LocalPrices{}
	for rows.Next() {
		var country, id, source string
		var price float64
		var period time.Time
		if err := rows.Scan(&country, &id, &price, &period, &source); err != nil {
			rows.Close()
			return nil, err
		}
		lp := locals[country]
		if lp == nil {
			lp = &LocalPrices{Country: country, Source: source, Period: period, Pack: map[string]float64{}}
			locals[country] = lp
		}
		lp.Pack[id] = price
	}
	rows.Close()
	for _, lp := range locals {
		c.SetLocalPrices(lp)
	}
	return c, nil
}
