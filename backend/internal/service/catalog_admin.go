package service

import (
	"context"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"racion/internal/domain"
	"racion/internal/planner"
)

// CatalogAdmin — рецепты базы из админки: поиск, правка, новые, удаление. Правки пишутся в БД
// и помечаются edited_at (сидинг их не затирает), после чего каталог перечитывается и подменяется.

type CatalogRecipeRepo interface {
	Save(ctx context.Context, rc planner.Recipe, isNew bool) error
	SoftDelete(ctx context.Context, id string) error
}

type CatalogAdmin struct {
	repo    CatalogRecipeRepo
	catalog *planner.CatalogRef
	reload  func(ctx context.Context) error
}

func NewCatalogAdmin(repo CatalogRecipeRepo, catalog *planner.CatalogRef, reload func(ctx context.Context) error) *CatalogAdmin {
	return &CatalogAdmin{repo: repo, catalog: catalog, reload: reload}
}

// Search — рецепты базы по подстроке названия или id, до 40 штук.
func (a *CatalogAdmin) Search(q string, limit int) []planner.Recipe {
	q = strings.ToLower(strings.TrimSpace(q))
	c := a.catalog.Load()
	var out []planner.Recipe
	for _, r := range c.Recipes {
		if q == "" || strings.Contains(strings.ToLower(r.Title), q) || strings.Contains(r.ID, q) || slices.Contains(r.Tags, q) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	if len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []planner.Recipe{}
	}
	return out
}

func (a *CatalogAdmin) Get(id string) (planner.Recipe, error) {
	if rc, ok := a.catalog.Load().RecipeByID[id]; ok {
		return rc, nil
	}
	return planner.Recipe{}, domain.ErrNotFound
}

// Save — новый рецепт (id из названия) или правка существующего; затем перечитываем каталог.
func (a *CatalogAdmin) Save(ctx context.Context, in domain.CatalogRecipeInput) (planner.Recipe, error) {
	c := a.catalog.Load()
	rc, err := a.validate(c, in)
	if err != nil {
		return rc, err
	}
	isNew := false
	if rc.ID == "" {
		rc.ID = uniqueSlug(c, rc.Title)
		isNew = true
	} else if _, ok := c.RecipeByID[rc.ID]; !ok {
		return rc, domain.ErrNotFound
	} else {
		rc.I18n = c.RecipeByID[rc.ID].I18n // переводы не трогаем
	}
	if err := a.repo.Save(ctx, rc, isNew); err != nil {
		return rc, err
	}
	if err := a.reload(ctx); err != nil {
		return rc, err
	}
	return rc, nil
}

func (a *CatalogAdmin) Delete(ctx context.Context, id string) error {
	if _, ok := a.catalog.Load().RecipeByID[id]; !ok {
		return domain.ErrNotFound
	}
	if err := a.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	return a.reload(ctx)
}

// Tags — какие теги встречаются в базе (для подсказок в форме).
func (a *CatalogAdmin) Tags() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range a.catalog.Load().Recipes {
		for _, t := range r.Tags {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (a *CatalogAdmin) validate(c *planner.Catalog, in domain.CatalogRecipeInput) (planner.Recipe, error) {
	rc := planner.Recipe{ID: strings.TrimSpace(in.ID), Title: strings.TrimSpace(in.Title), Description: strings.TrimSpace(in.Description),
		Slot: in.Slot, TimeMin: in.TimeMin, Batch: in.Batch, Image: strings.TrimSpace(in.Image)}
	if n := utf8.RuneCountInString(rc.Title); n < 2 || n > 80 {
		return rc, domain.Invalid("own.err.title")
	}
	if utf8.RuneCountInString(rc.Description) > 300 {
		return rc, domain.Invalid("own.err.description")
	}
	if !slices.Contains(planner.SlotOrder, rc.Slot) {
		return rc, domain.Invalid("own.err.slot")
	}
	if rc.TimeMin < 1 || rc.TimeMin > 600 {
		return rc, domain.Invalid("own.err.time")
	}
	rc.Equipment = []string{}
	for _, e := range in.Equipment {
		if slices.Contains(planner.EquipmentOrder, e) && !slices.Contains(rc.Equipment, e) {
			rc.Equipment = append(rc.Equipment, e)
		}
	}
	rc.Tags = []string{}
	for _, t := range in.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && regexp.MustCompile(`^[a-z0-9_-]{2,24}$`).MatchString(t) && !slices.Contains(rc.Tags, t) {
			rc.Tags = append(rc.Tags, t)
		}
	}
	rc.Steps = []string{}
	for _, st := range in.Steps {
		if st = strings.TrimSpace(st); st != "" {
			if utf8.RuneCountInString(st) > 500 {
				return rc, domain.Invalid("own.err.step")
			}
			rc.Steps = append(rc.Steps, st)
		}
	}
	if len(rc.Steps) == 0 || len(rc.Steps) > 20 {
		return rc, domain.Invalid("own.err.steps")
	}
	rc.Ingredients = []planner.RecipeIngredient{}
	for _, ri := range in.Ingredients {
		ing, ok := c.Ingredients[ri.IngredientID]
		if !ok {
			return rc, domain.Invalid("own.err.ingredient")
		}
		if ri.Amount <= 0 || ri.Amount > 5000 {
			return rc, domain.Invalid("own.err.amount")
		}
		if slices.ContainsFunc(rc.Ingredients, func(x planner.RecipeIngredient) bool { return x.IngredientID == ing.ID }) {
			continue
		}
		rc.Ingredients = append(rc.Ingredients, planner.RecipeIngredient{IngredientID: ing.ID, Amount: ri.Amount})
	}
	if len(rc.Ingredients) == 0 || len(rc.Ingredients) > 30 {
		return rc, domain.Invalid("own.err.ingredients")
	}
	return rc, nil
}

var translit = map[rune]string{'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya"}

// uniqueSlug — id из названия латиницей: «Сырники с изюмом» → syrniki_s_izyumom (и _2, если занято).
func uniqueSlug(c *planner.Catalog, title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('_')
		default:
			if t, ok := translit[r]; ok {
				b.WriteString(t)
			}
		}
	}
	base := strings.Trim(regexp.MustCompile(`_+`).ReplaceAllString(b.String(), "_"), "_")
	if len(base) > 40 {
		base = base[:40]
	}
	if base == "" {
		base = "recipe"
	}
	id := base
	for n := 2; ; n++ {
		if _, taken := c.RecipeByID[id]; !taken && !strings.HasPrefix(id, OwnPrefix) {
			return id
		}
		id = base + "_" + strings.TrimLeft(strings.Repeat("0", 0)+itoa(n), "0")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
