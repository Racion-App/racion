package http

import (
	"net/http"

	"racion/internal/ai"
	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
	"racion/internal/service"
)

type recipeView struct {
	planner.Recipe
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Steps       []string           `json:"steps"`
	SlotLabel   string             `json:"slotLabel"`
	Ingredients []recipeIngredient `json:"ingredients"`
	Kcal        float64            `json:"kcal"`
	Protein     float64            `json:"protein"`
	Fat         float64            `json:"fat"`
	Carb        float64            `json:"carb"`
	Stats       domain.RecipeStats `json:"stats"`
}

type recipeIngredient struct {
	IngredientID string  `json:"ingredientId"`
	Name         string  `json:"name"`
	Amount       float64 `json:"amount"`
	Unit         string  `json:"unit"`
	Pantry       bool    `json:"pantry"`
	Image        string  `json:"image,omitempty"`
}

// recipeSubs — замены продуктов рецепта с пересчётом ккал и цены порции: ?country=RU.
func (s *Server) recipeSubs(w http.ResponseWriter, r *http.Request) {
	country := planner.CountryOf(r.URL.Query().Get("country")).Code
	rows, err := s.svc.Subs.For(r.Context(), r.PathValue("id"), country, i18n.FromRequest(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, rows)
}

// recipe — рецепт для листа на чеке: базовый или свой.
func (s *Server) recipe(w http.ResponseWriter, r *http.Request) {
	rc, err := s.svc.Recipes.Find(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	lang := i18n.FromRequest(r)
	tx := rc.Text(lang)
	rc.I18n = nil
	view := recipeView{Recipe: rc, Title: tx.Title, Description: tx.Description, Steps: tx.Steps, SlotLabel: planner.SlotLabel(lang, rc.Slot)}
	for _, ri := range rc.Ingredients {
		ing := s.catalog.Ingredients[ri.IngredientID]
		view.Ingredients = append(view.Ingredients, recipeIngredient{ri.IngredientID, ing.LocalName(lang), ri.Amount, ing.Unit, ing.Pantry, ing.Image})
	}
	view.Kcal, view.Protein, view.Fat, view.Carb = s.catalog.Nutrition(rc)
	view.Stats = s.svc.Social.Stats(r.Context(), rc.ID, viewerID(r))
	writeJSON(w, 200, view)
}

// ── Свои рецепты ───────────────────────────────────────────────────────────

type ownRecipeView struct {
	planner.Recipe
	SlotLabel    string                     `json:"slotLabel"`
	Kcal         float64                    `json:"kcal"`
	Cost         float64                    `json:"cost"`
	Likes        int                        `json:"likes"`
	Comments     int                        `json:"comments"`
	Favorites    int                        `json:"favorites"`
	Translations *domain.TranslationSummary `json:"translations,omitempty"`
}

func (s *Server) ownView(r *http.Request, rc planner.Recipe) ownRecipeView {
	lang := i18n.FromRequest(r)
	country := planner.CountryOf(r.URL.Query().Get("country"))
	kcal, _, _, _ := s.catalog.Nutrition(rc)
	cost, _ := s.catalog.PortionCost(rc, country.Code)
	return ownRecipeView{Recipe: rc, SlotLabel: planner.SlotLabel(lang, rc.Slot), Kcal: kcal, Cost: cost}
}

func (s *Server) listOwnRecipes(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	own, err := s.svc.Recipes.Own(r.Context(), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	ids := make([]string, 0, len(own))
	for _, rc := range own {
		ids = append(ids, rc.ID)
	}
	sums := s.svc.Translations.Summary(r.Context(), ids)
	out := make([]ownRecipeView, 0, len(own))
	for _, rc := range own {
		v := s.ownView(r, rc)
		if sm, ok := sums[rc.ID]; ok {
			v.Translations = &sm
		}
		if rc.Status == service.StatusApproved {
			st := s.svc.Social.Stats(r.Context(), rc.ID, "")
			v.Likes, v.Comments, v.Favorites = st.Likes, st.Comments, st.Favorites
		}
		out = append(out, v)
	}
	writeJSON(w, 200, out)
}

func (s *Server) createOwnRecipe(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var in domain.OwnRecipeInput
	if !decode(w, r, 64<<10, &in) {
		return
	}
	in.Lang = string(i18n.FromRequest(r))
	rc, err := s.svc.Recipes.CreateOwn(r.Context(), u.ID, in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, s.ownView(r, rc))
}

func (s *Server) updateOwnRecipe(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var in domain.OwnRecipeInput
	if !decode(w, r, 64<<10, &in) {
		return
	}
	in.Lang = string(i18n.FromRequest(r))
	rc, err := s.svc.Recipes.UpdateOwn(r.Context(), u.ID, r.PathValue("id"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, s.ownView(r, rc))
}

func (s *Server) deleteOwnRecipe(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if err := s.svc.Recipes.DeleteOwn(r.Context(), u.ID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// assistRecipe — помощник: {action: improve|translate, lang, title, description, steps} → исправленный текст.
// Ничего не сохраняет: результат попадает в форму, сохраняет человек.
func (s *Server) assistRecipe(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var in struct {
		Action string `json:"action"`
		Lang   string `json:"lang"`
		ai.RecipeText
	}
	if !decode(w, r, 64<<10, &in) {
		return
	}
	lang, ok := i18n.Valid(in.Lang)
	if !ok {
		lang = i18n.FromRequest(r)
	}
	out, err := s.svc.AI.Run(r.Context(), u.ID, in.Action, string(lang), in.RecipeText)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

// ── Переводы своих рецептов ─────────────────────────────────────────────────

func (s *Server) ownTranslations(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	st, err := s.svc.Translations.Status(r.Context(), u.ID, r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, st)
}

func (s *Server) ownTranslate(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if err := s.svc.Translations.Retry(r.Context(), u.ID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	st, err := s.svc.Translations.Status(r.Context(), u.ID, r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, st)
}

// adminAI — состояние провайдеров нейросетей: запросы за день, лимиты, кто отдыхает после 429.
func (s *Server) adminAI(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermStats) == nil {
		return
	}
	writeJSON(w, 200, map[string]any{"providers": s.svc.Translations.Providers()})
}
