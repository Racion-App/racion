package service

import (
	"context"
	"errors"
	"testing"

	"racion/internal/domain"
	"racion/internal/planner"
)

// Сервисы работают через порты, поэтому тестируются без БД и без HTTP: хранилище в памяти.

type memRecipes struct {
	items map[string]planner.Recipe
	owner map[string]string
}

func (m *memRecipes) ByUser(_ context.Context, userID string) ([]planner.Recipe, error) {
	var out []planner.Recipe
	for id, r := range m.items {
		if m.owner[id] == userID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memRecipes) AddView(_ context.Context, _ string) error { return nil }

func (m *memRecipes) Get(_ context.Context, id string) (planner.Recipe, error) {
	r, ok := m.items[id]
	if !ok {
		return r, domain.ErrNotFound
	}
	return r, nil
}
func (m *memRecipes) Count(_ context.Context, userID string) (int, error) {
	n := 0
	for _, o := range m.owner {
		if o == userID {
			n++
		}
	}
	return n, nil
}
func (m *memRecipes) Insert(_ context.Context, userID string, rc planner.Recipe) error {
	m.items[rc.ID], m.owner[rc.ID] = rc, userID
	return nil
}
func (m *memRecipes) Update(_ context.Context, userID string, rc planner.Recipe) error {
	if m.owner[rc.ID] != userID {
		return domain.ErrNotFound
	}
	m.items[rc.ID] = rc
	return nil
}
func (m *memRecipes) SetPublic(_ context.Context, userID, id string, public bool) error {
	if m.owner[id] != userID {
		return domain.ErrNotFound
	}
	r := m.items[id]
	r.Public = public
	m.items[id] = r
	return nil
}
func (m *memRecipes) Delete(_ context.Context, userID, id string) error {
	if m.owner[id] != userID {
		return domain.ErrNotFound
	}
	delete(m.items, id)
	delete(m.owner, id)
	return nil
}

func testCatalog() *planner.Catalog {
	c := planner.NewCatalog()
	c.Ingredients["chicken_breast"] = planner.Ingredient{ID: "chicken_breast", Name: "Куриное филе", Unit: "g", Pack: 800, Price: 390, Kcal: 113, Protein: 24}
	c.Ingredients["potato"] = planner.Ingredient{ID: "potato", Name: "Картофель", Unit: "g", Pack: 2000, Price: 110, Kcal: 77, Carb: 17, Loose: true}
	return c
}

func TestOwnRecipeLifecycle(t *testing.T) {
	repo := &memRecipes{items: map[string]planner.Recipe{}, owner: map[string]string{}}
	svc := &Recipes{repo: repo, catalog: planner.NewCatalogRef(testCatalog())}
	ctx := context.Background()

	_, err := svc.CreateOwn(ctx, "u1", domain.OwnRecipeInput{Title: "x", Slot: "dinner", TimeMin: 10, Steps: []string{"a"}, Ingredients: []planner.RecipeIngredient{{IngredientID: "potato", Amount: 100}}})
	if ve, ok := IsValidation(err); !ok || ve.Key != "own.err.title" {
		t.Fatalf("short title must fail with own.err.title, got %v", err)
	}
	_, err = svc.CreateOwn(ctx, "u1", domain.OwnRecipeInput{Title: "Запеканка", Slot: "dinner", TimeMin: 10, Steps: []string{"a"}, Ingredients: []planner.RecipeIngredient{{IngredientID: "nope", Amount: 100}}})
	if ve, ok := IsValidation(err); !ok || ve.Key != "own.err.ingredient" {
		t.Fatalf("unknown ingredient must fail, got %v", err)
	}
	rc, err := svc.CreateOwn(ctx, "u1", domain.OwnRecipeInput{Title: " Запеканка ", Slot: "dinner", TimeMin: 40, Equipment: []string{"oven", "bogus"}, Tags: []string{"hearty", "premium"},
		Steps: []string{"Нарезать", "", "Запечь"}, Ingredients: []planner.RecipeIngredient{{IngredientID: "chicken_breast", Amount: 150}, {IngredientID: "potato", Amount: 200}, {IngredientID: "potato", Amount: 50}}})
	if err != nil {
		t.Fatal(err)
	}
	if rc.Title != "Запеканка" || len(rc.Equipment) != 1 || len(rc.Tags) != 1 || len(rc.Steps) != 2 || len(rc.Ingredients) != 2 || !rc.Own || rc.ID[:2] != OwnPrefix {
		t.Fatalf("input must be cleaned: %+v", rc)
	}
	if got, _ := svc.Find(ctx, rc.ID); got.ID != rc.ID {
		t.Fatalf("own recipe must be findable by id")
	}
	// чужой пользователь не может править и удалять
	if _, err := svc.UpdateOwn(ctx, "u2", rc.ID, domain.OwnRecipeInput{Title: "Чужая", Slot: "lunch", TimeMin: 5, Steps: []string{"a"}, Ingredients: []planner.RecipeIngredient{{IngredientID: "potato", Amount: 10}}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign update must be not found, got %v", err)
	}
	if err := svc.DeleteOwn(ctx, "u2", rc.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign delete must be not found, got %v", err)
	}
	// каталог владельца видит рецепт, общий — нет
	uid := "u1"
	if _, ok := svc.CatalogFor(ctx, &uid).RecipeByID[rc.ID]; !ok {
		t.Fatalf("owner catalog must include own recipe")
	}
	if _, ok := svc.CatalogFor(ctx, nil).RecipeByID[rc.ID]; ok {
		t.Fatalf("guest catalog must not include own recipes")
	}
	if err := svc.DeleteOwn(ctx, "u1", rc.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialsClean(t *testing.T) {
	c := Credentials{Email: "  Test@Example.COM ", Password: "12345678", Name: "  Вадим "}
	if err := c.clean(); err != nil || c.Email != "test@example.com" || c.Name != "Вадим" {
		t.Fatalf("clean: %v %+v", err, c)
	}
	bad := Credentials{Email: "nope", Password: "12345678"}
	if ve, ok := IsValidation(bad.clean()); !ok || ve.Key != "auth.bad_email" {
		t.Fatalf("bad email must be validation error")
	}
	short := Credentials{Email: "a@b.cc", Password: "123"}
	if ve, ok := IsValidation(short.clean()); !ok || ve.Key != "auth.short_password" {
		t.Fatalf("short password must be validation error")
	}
}
