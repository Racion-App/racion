package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"racion/internal/domain"
	"racion/internal/planner"
)

// Свои рецепты живут только в каталоге владельца (Catalog.WithRecipes): в общий список, поиск и sitemap
// они не попадают, зато участвуют в подборе его недель наравне с базой.

const (
	OwnPrefix   = "u_"
	ownMaxCount = 200
)

// ownTags — теги, которые можно поставить своему блюду. Детские и «премиум» влияют на подбор, их не даём.
var ownTags = []string{"pp", "protein", "soup", "salad", "vegetarian", "sweet", "spicy", "hearty"}

type Recipes struct {
	repo         UserRecipeRepo
	users        UserRepo
	catalog      *planner.CatalogRef
	mediaOwns    func(url string) bool // ссылка на фото ведёт в наше хранилище
	translations *Translations
}

// SetMedia подключает проверку ссылок на фото.
func (s *Recipes) SetMedia(owns func(url string) bool) { s.mediaOwns = owns }

// Catalog — общий каталог (для страниц, поиска, справочников).
func (s *Recipes) Catalog() *planner.Catalog { return s.catalog.Load() }

// CatalogFor — каталог с рецептами пользователя; для гостя — общий.
func (s *Recipes) CatalogFor(ctx context.Context, userID *string) *planner.Catalog {
	if userID == nil || *userID == "" {
		return s.catalog.Load()
	}
	own, err := s.repo.ByUser(ctx, *userID)
	if err != nil {
		return s.catalog.Load()
	}
	return s.catalog.Load().WithRecipes(own)
}

// Find — базовый рецепт или свой (по префиксу id). Свой отдаётся без проверки владельца: ссылка на план
// общая, значит блюдо из него должно открываться и у того, кому план переслали; id случайный.
func (s *Recipes) Find(ctx context.Context, id string) (planner.Recipe, error) {
	if rc, ok := s.catalog.Load().RecipeByID[id]; ok {
		return rc, nil
	}
	if strings.HasPrefix(id, OwnPrefix) {
		rc, err := s.repo.Get(ctx, id)
		if err != nil {
			return rc, err
		}
		if rc.Status == StatusApproved && s.users != nil {
			rc.Author, _ = s.users.NickOf(ctx, rc.OwnerID)
		}
		return rc, nil
	}
	return planner.Recipe{}, domain.ErrNotFound
}

// SetPublic открывает свой рецепт для всех по ссылке (или закрывает).
func (s *Recipes) SetPublic(ctx context.Context, userID, id string, public bool) error {
	return s.repo.SetPublic(ctx, userID, id, public)
}

func (s *Recipes) Own(ctx context.Context, userID string) ([]planner.Recipe, error) {
	out, err := s.repo.ByUser(ctx, userID)
	if out == nil {
		out = []planner.Recipe{}
	}
	return out, err
}

func (s *Recipes) CreateOwn(ctx context.Context, userID string, in domain.OwnRecipeInput) (planner.Recipe, error) {
	if err := s.validate(&in); err != nil {
		return planner.Recipe{}, err
	}
	if n, err := s.repo.Count(ctx, userID); err == nil && n >= ownMaxCount {
		return planner.Recipe{}, domain.Invalid("own.err.limit")
	}
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	rc := toRecipe(OwnPrefix+hex.EncodeToString(b), in)
	if err := s.repo.Insert(ctx, userID, rc); err != nil {
		return planner.Recipe{}, err
	}
	s.scheduleTranslation(ctx, rc, nil)
	return rc, nil
}

// SetTranslations подключает очередь переводов: новый или изменённый текст уходит на все языки.
func (s *Recipes) SetTranslations(t *Translations) { s.translations = t }

func (s *Recipes) scheduleTranslation(ctx context.Context, rc planner.Recipe, prev *planner.Recipe) {
	if s.translations == nil || !s.translations.Enabled() {
		return
	}
	if prev != nil && prev.Title == rc.Title && prev.Description == rc.Description && slices.Equal(prev.Steps, rc.Steps) && (rc.Lang == "" || prev.Lang == rc.Lang) {
		return // текст не менялся — переводы актуальны
	}
	if err := s.translations.Schedule(ctx, rc.ID, rc.Lang); err != nil {
		_ = err // очередь — не повод ронять сохранение
	}
}

func (s *Recipes) UpdateOwn(ctx context.Context, userID, id string, in domain.OwnRecipeInput) (planner.Recipe, error) {
	if err := s.validate(&in); err != nil {
		return planner.Recipe{}, err
	}
	prev, _ := s.repo.Get(ctx, id)
	rc := toRecipe(id, in)
	if err := s.repo.Update(ctx, userID, rc); err != nil {
		return planner.Recipe{}, err
	}
	if rc.Lang == "" {
		rc.Lang = prev.Lang
	}
	s.scheduleTranslation(ctx, rc, &prev)
	return rc, nil
}

func (s *Recipes) DeleteOwn(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}

func toRecipe(id string, in domain.OwnRecipeInput) planner.Recipe {
	return planner.Recipe{ID: id, Title: in.Title, Description: in.Description, Slot: in.Slot, TimeMin: in.TimeMin,
		Equipment: in.Equipment, Tags: in.Tags, Steps: in.Steps, Ingredients: in.Ingredients, Own: true, Image: in.Image, Lang: in.Lang}
}

// validate чистит поля и возвращает ошибку с ключом i18n.
func (s *Recipes) validate(in *domain.OwnRecipeInput) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Image = strings.TrimSpace(in.Image)
	if in.Image != "" && (s.mediaOwns == nil || !s.mediaOwns(in.Image)) {
		return domain.Invalid("photo.bad")
	}
	if n := utf8.RuneCountInString(in.Title); n < 2 || n > 80 {
		return domain.Invalid("own.err.title")
	}
	if utf8.RuneCountInString(in.Description) > 300 {
		return domain.Invalid("own.err.description")
	}
	if !slices.Contains(planner.SlotOrder, in.Slot) {
		return domain.Invalid("own.err.slot")
	}
	if in.TimeMin < 1 || in.TimeMin > 600 {
		return domain.Invalid("own.err.time")
	}
	eq := []string{}
	for _, e := range in.Equipment {
		if slices.Contains(planner.EquipmentOrder, e) && !slices.Contains(eq, e) {
			eq = append(eq, e)
		}
	}
	in.Equipment = eq
	tags := []string{}
	for _, t := range in.Tags {
		if slices.Contains(ownTags, t) && !slices.Contains(tags, t) {
			tags = append(tags, t)
		}
	}
	in.Tags = tags
	steps := []string{}
	for _, st := range in.Steps {
		st = strings.TrimSpace(st)
		if st == "" {
			continue
		}
		if utf8.RuneCountInString(st) > 500 {
			return domain.Invalid("own.err.step")
		}
		steps = append(steps, st)
	}
	if len(steps) == 0 || len(steps) > 20 {
		return domain.Invalid("own.err.steps")
	}
	in.Steps = steps
	ings := []planner.RecipeIngredient{}
	for _, ri := range in.Ingredients {
		ing, ok := s.catalog.Load().Ingredients[ri.IngredientID]
		if !ok {
			return domain.Invalid("own.err.ingredient")
		}
		if ri.Amount <= 0 || ri.Amount > 5000 {
			return domain.Invalid("own.err.amount")
		}
		if slices.ContainsFunc(ings, func(x planner.RecipeIngredient) bool { return x.IngredientID == ri.IngredientID }) {
			continue
		}
		ings = append(ings, planner.RecipeIngredient{IngredientID: ing.ID, Amount: ri.Amount})
	}
	if len(ings) == 0 || len(ings) > 30 {
		return domain.Invalid("own.err.ingredients")
	}
	in.Ingredients = ings
	return nil
}

// IsValidation — удобная проверка для транспортов.
func IsValidation(err error) (*domain.ValidationError, bool) {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}

// CountView — просмотр страницы своего рецепта не автором: счётчик для кабинета.
func (s *Recipes) CountView(ctx context.Context, rc planner.Recipe, viewerID string) {
	if !rc.Own || rc.OwnerID == viewerID {
		return
	}
	_ = s.repo.AddView(ctx, rc.ID)
}
