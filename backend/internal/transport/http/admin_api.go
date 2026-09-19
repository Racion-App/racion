package http

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
	"racion/internal/service"
)

// API для скриптов и помощников (Claude, Codex): ключ из админки в Authorization: Bearer rk_…,
// схема с допустимыми значениями, база продуктов с id и пакетное добавление рецептов с построчным отчётом.

// adminRecipeSchema — что нужно знать, чтобы собрать рецепт: приёмы пищи, техника, теги, правила.
func (s *Server) adminRecipeSchema(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermRecipes) == nil {
		return
	}
	l := i18n.FromRequest(r)
	slots := []map[string]string{}
	for _, id := range []string{"breakfast", "lunch", "dinner", "snack"} {
		slots = append(slots, map[string]string{"id": id, "label": i18n.T(l, "slot."+id)})
	}
	equipment := []map[string]string{}
	for _, id := range planner.EquipmentOrder {
		equipment = append(equipment, map[string]string{"id": id, "label": planner.EquipmentLabel(l, id)})
	}
	tags := s.svc.CatalogAdmin.Tags()
	sort.Strings(tags)
	writeJSON(w, 200, map[string]any{
		"slots":     slots,
		"equipment": equipment,
		"tags":      tags,
		"units":     []string{"g", "ml", "pcs"},
		"rules": map[string]any{
			"id":          "латиница, цифры и _, уникальный; пример: chicken_rice_bowl",
			"title":       "2–80 знаков, на русском",
			"description": "до 300 знаков: одна-две фразы, что это за блюдо и чем хорошо",
			"timeMin":     "1–600 минут от начала до тарелки",
			"steps":       "1–20 шагов, каждый до 500 знаков, повелительное наклонение, с температурами и временем",
			"ingredients": "1–30 продуктов, ingredientId из GET /api/admin/ingredients, amount — на ОДНУ порцию взрослого в единице продукта (g/ml/pcs)",
			"equipment":   "что нужно из списка; пусто — только плита",
			"tags":        "из списка tags; новые теги не придумывать",
			"batch":       "true, если блюдо удобно готовить на два дня",
			"keep":        "режим заготовок: сколько дней готовое блюдо стоит в холодильнике (0 — есть свежим); без поля — по правилам",
			"freeze":      "true, если готовое блюдо можно заморозить",
			"image":       "оставить пустым — фото делаем сами",
			"kcal":        "считается сервером по продуктам; ужин должен давать 500–800 ккал на порцию, обед 500–800, завтрак 350–550, перекус 150–300",
		},
		"batchEndpoint": "POST /api/admin/recipes/batch — массив рецептов, ответ построчно {id, ok, error}",
	})
}

// adminIngredients — база продуктов с id, единицей и КБЖУ на 100 г/мл; ?q= фильтр по названию.
func (s *Server) adminIngredients(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermRecipes) == nil {
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	l := i18n.FromRequest(r)
	type row struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		NameEn   string  `json:"nameEn"`
		Unit     string  `json:"unit"`
		Category string  `json:"category"`
		Kcal     float64 `json:"kcal"`
		Protein  float64 `json:"protein"`
		Fat      float64 `json:"fat"`
		Carb     float64 `json:"carb"`
		Pantry   bool    `json:"pantry"`
	}
	out := []row{}
	for _, ing := range s.svc.Catalog.Base().Ingredients {
		name, en := ing.LocalName(l), ing.LocalName("en")
		if q != "" && !strings.Contains(strings.ToLower(name), q) && !strings.Contains(strings.ToLower(en), q) && !strings.Contains(ing.ID, q) {
			continue
		}
		out = append(out, row{ing.ID, name, en, ing.Unit, ing.Category, ing.Kcal, ing.Protein, ing.Fat, ing.Carb, ing.Pantry})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	writeJSON(w, 200, map[string]any{"items": out, "total": len(out)})
}

// adminSaveRecipesBatch — до 50 рецептов за раз; каждый проверяется отдельно, ошибки не останавливают остальные.
func (s *Server) adminSaveRecipesBatch(w http.ResponseWriter, r *http.Request) {
	if s.requirePerm(w, r, service.PermRecipes) == nil {
		return
	}
	var in []domain.CatalogRecipeInput
	if !decode(w, r, 4<<20, &in) {
		return
	}
	if len(in) == 0 || len(in) > 50 {
		writeErr(w, 400, "1–50 recipes per batch")
		return
	}
	l := i18n.FromRequest(r)
	type res struct {
		ID    string  `json:"id"`
		OK    bool    `json:"ok"`
		Kcal  float64 `json:"kcal,omitempty"`
		Error string  `json:"error,omitempty"`
	}
	out := make([]res, 0, len(in))
	okN := 0
	for _, rc := range in {
		saved, err := s.svc.CatalogAdmin.Save(r.Context(), rc)
		if err != nil {
			msg := err.Error()
			var ve *domain.ValidationError
			if errors.As(err, &ve) {
				msg = i18n.T(l, ve.Key)
			}
			out = append(out, res{ID: rc.ID, Error: msg})
			continue
		}
		okN++
		kcal, _, _, _ := s.svc.Catalog.Base().Nutrition(saved)
		out = append(out, res{ID: saved.ID, OK: true, Kcal: kcal})
	}
	writeJSON(w, 200, map[string]any{"ok": okN, "failed": len(in) - okN, "items": out})
}

// --- ключи API (только для тех, кто может править каталог)

func (s *Server) apiKeys(w http.ResponseWriter, r *http.Request) {
	u := s.requirePerm(w, r, service.PermRecipes)
	if u == nil {
		return
	}
	list, err := s.svc.APIKeys.List(r.Context(), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) apiKeyCreate(w http.ResponseWriter, r *http.Request) {
	u := s.requirePerm(w, r, service.PermRecipes)
	if u == nil {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, 4<<10, &in) {
		return
	}
	k, err := s.svc.APIKeys.Create(r.Context(), u.ID, in.Name)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, k)
}

func (s *Server) apiKeyDelete(w http.ResponseWriter, r *http.Request) {
	u := s.requirePerm(w, r, service.PermRecipes)
	if u == nil {
		return
	}
	if err := s.svc.APIKeys.Delete(r.Context(), u.ID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}
