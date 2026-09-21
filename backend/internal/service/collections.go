package service

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Коллекции: папки рецептов. Из коллекции можно собрать неделю — её рецепты получают в подборе
// бонус сильнее избранного (Params.Collection), так неделя «с дачи» выйдет из дачных блюд.

const collectionsMax = 30

type CollectionRepo interface {
	List(ctx context.Context, userID string) ([]domain.Collection, error)
	Create(ctx context.Context, userID, name string) (domain.Collection, error)
	Rename(ctx context.Context, userID, id, name string) error
	Delete(ctx context.Context, userID, id string) error
	Toggle(ctx context.Context, userID, id, recipeID string, on bool) error
	Recipes(ctx context.Context, userID, id string) ([]string, error)
	Public(ctx context.Context, curatedOnly bool) ([]domain.Collection, error)
	BySlug(ctx context.Context, slug string) (domain.Collection, error)
	Publish(ctx context.Context, userID, id string, public bool, slug string) error
	Curate(ctx context.Context, id string, name, description, cover, slug string, public bool, names, descriptions map[string]string) error
	AdminList(ctx context.Context) ([]domain.Collection, error)
	Get(ctx context.Context, id string) (domain.Collection, error)
	SetItems(ctx context.Context, id string, recipes []string) error
	AdminDelete(ctx context.Context, id string) error
}

type Collections struct {
	repo    CollectionRepo
	recipes *Recipes
}

func NewCollections(repo CollectionRepo, recipes *Recipes) *Collections {
	return &Collections{repo: repo, recipes: recipes}
}

func (c *Collections) List(ctx context.Context, userID string) ([]domain.Collection, error) {
	list, err := c.repo.List(ctx, userID)
	return c.withCovers(ctx, list), err
}

// withCovers — подставить CoverAuto: фото первого рецепта с картинкой, когда обложка не задана.
func (c *Collections) withCovers(ctx context.Context, list []domain.Collection) []domain.Collection {
	for i := range list {
		if list[i].Cover != "" {
			list[i].CoverAuto = list[i].Cover
			continue
		}
		for _, rid := range list[i].Recipes {
			if rc, err := c.recipes.Find(ctx, rid); err == nil && rc.Image != "" {
				list[i].CoverAuto = rc.Image
				break
			}
		}
	}
	return list
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 40 {
		return "", domain.Invalid("collection.name")
	}
	return name, nil
}

func (c *Collections) Create(ctx context.Context, userID, name string) (domain.Collection, error) {
	name, err := cleanName(name)
	if err != nil {
		return domain.Collection{}, err
	}
	if list, err := c.repo.List(ctx, userID); err == nil && len(list) >= collectionsMax {
		return domain.Collection{}, domain.Invalid("collection.limit")
	}
	return c.repo.Create(ctx, userID, name)
}

func (c *Collections) Rename(ctx context.Context, userID, id, name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	return c.repo.Rename(ctx, userID, id, name)
}

func (c *Collections) Delete(ctx context.Context, userID, id string) error {
	return c.repo.Delete(ctx, userID, id)
}

func (c *Collections) Toggle(ctx context.Context, userID, id, recipeID string, on bool) error {
	if _, err := c.recipes.Find(ctx, recipeID); err != nil {
		return err
	}
	return c.repo.Toggle(ctx, userID, id, recipeID, on)
}

// Items — рецепты коллекции целиком (для кабинета).
func (c *Collections) Items(ctx context.Context, userID, id string) ([]planner.Recipe, error) {
	ids, err := c.repo.Recipes(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	out := make([]planner.Recipe, 0, len(ids))
	for _, rid := range ids {
		if rc, err := c.recipes.Find(ctx, rid); err == nil {
			out = append(out, rc)
		}
	}
	return out, nil
}

// Publish — открыть свою коллекцию по ссылке: slug из названия (+хвост id), закрыть — обратно.
func (c *Collections) Publish(ctx context.Context, userID, id string, public bool) (domain.Collection, error) {
	col, err := c.repo.Get(ctx, id)
	if err != nil {
		return col, err
	}
	slug := col.Slug
	if slug == "" {
		slug = uniqueSlug(c.recipes.catalog.Load(), col.Name) + "-" + id[:6]
	}
	if err := c.repo.Publish(ctx, userID, id, public, slug); err != nil {
		return col, err
	}
	return c.repo.Get(ctx, id)
}

// Curated — редакционные подборки для каталога и sitemap.
func (c *Collections) Curated(ctx context.Context) []domain.Collection {
	list, _ := c.repo.Public(ctx, true)
	return c.withCovers(ctx, list)
}

// BySlug — публичная подборка с рецептами.
func (c *Collections) BySlug(ctx context.Context, slug string) (domain.Collection, []planner.Recipe, error) {
	col, err := c.repo.BySlug(ctx, slug)
	if err != nil {
		return col, nil, err
	}
	col = c.withCovers(ctx, []domain.Collection{col})[0]
	out := make([]planner.Recipe, 0, len(col.Recipes))
	for _, rid := range col.Recipes {
		if rc, err := c.recipes.Find(ctx, rid); err == nil && !rc.Hidden && (!rc.Own || rc.Status == StatusApproved) {
			out = append(out, rc)
		}
	}
	return col, out, nil
}

// CuratedInput — редакционная подборка из админки.
type CuratedInput struct {
	ID           string            `json:"id"` // пусто — новая
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	Description  string            `json:"description"`
	Cover        string            `json:"cover"`
	Public       bool              `json:"public"`
	Recipes      []string          `json:"recipes"`
	Names        map[string]string `json:"names"` // переводы названия: код языка → текст
	Descriptions map[string]string `json:"descriptions"`
}

// SaveCurated — создать или обновить редакционную подборку (владелец — админ, который её завёл).
func (c *Collections) SaveCurated(ctx context.Context, actor domain.User, in CuratedInput) (domain.Collection, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return domain.Collection{}, err
	}
	if utf8.RuneCountInString(in.Description) > 600 {
		return domain.Collection{}, domain.Invalid("collection.name")
	}
	id := in.ID
	if id == "" {
		col, err := c.repo.Create(ctx, actor.ID, name)
		if err != nil {
			return col, err
		}
		id = col.ID
	}
	slug := strings.Trim(strings.ToLower(strings.TrimSpace(in.Slug)), "-")
	if slug == "" {
		slug = uniqueSlug(c.recipes.catalog.Load(), name)
	}
	if !regexp.MustCompile(`^[a-z0-9_-]{2,60}$`).MatchString(slug) {
		return domain.Collection{}, domain.Invalid("collection.slug")
	}
	names, descs := map[string]string{}, map[string]string{}
	for l, v := range in.Names {
		if _, ok := i18n.Valid(l); !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if n := utf8.RuneCountInString(v); n > 40 {
			return domain.Collection{}, domain.Invalid("collection.name")
		} else if n > 0 {
			names[l] = v
		}
	}
	for l, v := range in.Descriptions {
		if _, ok := i18n.Valid(l); !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if n := utf8.RuneCountInString(v); n > 600 {
			return domain.Collection{}, domain.Invalid("collection.name")
		} else if n > 0 {
			descs[l] = v
		}
	}
	if err := c.repo.Curate(ctx, id, name, strings.TrimSpace(in.Description), strings.TrimSpace(in.Cover), slug, in.Public, names, descs); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return domain.Collection{}, domain.Invalid("collection.slug.taken")
		}
		return domain.Collection{}, err
	}
	var ids []string
	for _, rid := range in.Recipes {
		if _, err := c.recipes.Find(ctx, rid); err == nil && !slices.Contains(ids, rid) {
			ids = append(ids, rid)
		}
	}
	if err := c.repo.SetItems(ctx, id, ids); err != nil {
		return domain.Collection{}, err
	}
	return c.repo.Get(ctx, id)
}

func (c *Collections) AdminList(ctx context.Context) ([]domain.Collection, error) {
	list, err := c.repo.AdminList(ctx)
	return c.withCovers(ctx, list), err
}
func (c *Collections) AdminDelete(ctx context.Context, id string) error {
	return c.repo.AdminDelete(ctx, id)
}

// IDs — id рецептов коллекции для планировщика; при сбое пусто.
func (c *Collections) IDs(ctx context.Context, userID, id string) []string {
	ids, _ := c.repo.Recipes(ctx, userID, id)
	return ids
}
