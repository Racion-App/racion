package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"racion/internal/ai"
	"racion/internal/domain"
	"racion/internal/planner"
)

// Модерация своих рецептов. Статусы (user_recipes.status):
//   private  — только автору и по его ссылке;
//   checking — отправлен на публикацию, проверяет нейросеть;
//   review   — нейросеть усомнилась (или недоступна): ручная проверка в течение суток;
//   approved — на сайте: с автором, в разделе «от пользователей», в sitemap;
//   rejected — отклонён модератором, с причиной.

const (
	StatusPrivate  = "private"
	StatusChecking = "checking"
	StatusReview   = "review"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusImprove  = "improve" // нейросеть предлагает подробнее — ждём решения автора
	publishPerHour = 10
)

type RecipeChecker interface {
	Enabled() bool
	CheckRecipe(ctx context.Context, in ai.RecipeCheck) (ai.RecipeVerdict, error)
}

type ModerationRepo interface {
	SetStatus(ctx context.Context, id, status, note string, public bool) error
	SetSuggestion(ctx context.Context, id, note string, suggestion any) error
	ApplyText(ctx context.Context, id, title, description string, steps []string) error
	Queue(ctx context.Context) ([]planner.Recipe, error)
	Approved(ctx context.Context, limit int) ([]planner.Recipe, error)
}

type Moderation struct {
	repo    ModerationRepo
	recipes UserRecipeRepo
	users   UserRepo
	ai      RecipeChecker
	log     *zap.Logger
	mu      sync.Mutex
	usage   map[string][]time.Time
}

func NewModeration(repo ModerationRepo, recipes UserRecipeRepo, users UserRepo, checker RecipeChecker, log *zap.Logger) *Moderation {
	if log == nil {
		log = zap.NewNop()
	}
	return &Moderation{repo: repo, recipes: recipes, users: users, ai: checker, log: log, usage: map[string][]time.Time{}}
}

// Publish — автор просит опубликовать: статус checking, проверка нейросетью в фоне.
// Если нейросети нет — сразу в ручную очередь.
func (m *Moderation) Publish(ctx context.Context, userID, id string) (string, error) {
	rc, err := m.recipes.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if rc.OwnerID != userID {
		return "", domain.ErrForbidden
	}
	switch rc.Status {
	case StatusApproved, StatusChecking, StatusReview, StatusImprove:
		return rc.Status, nil
	}
	if !m.allow(userID) {
		return rc.Status, domain.Invalid("moderation.limit")
	}
	if m.ai == nil || !m.ai.Enabled() {
		if err := m.repo.SetStatus(ctx, id, StatusReview, "auto-check unavailable", false); err != nil {
			return "", err
		}
		return StatusReview, nil
	}
	if err := m.repo.SetStatus(ctx, id, StatusChecking, "", false); err != nil {
		return "", err
	}
	go m.check(rc)
	return StatusChecking, nil
}

// check — нейросеть смотрит на рецепт: настоящее ли это блюдо, нет ли рекламы, ссылок, опасных советов.
func (m *Moderation) check(rc planner.Recipe) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	names := make([]string, 0, len(rc.Ingredients))
	for _, ri := range rc.Ingredients {
		names = append(names, ri.IngredientID)
	}
	v, err := m.ai.CheckRecipe(ctx, ai.RecipeCheck{Title: rc.Title, Description: rc.Description, Steps: rc.Steps, Ingredients: names})
	switch {
	case err != nil:
		m.log.Warn("moderation check failed", zap.String("id", rc.ID), zap.Error(err))
		_ = m.repo.SetStatus(ctx, rc.ID, StatusReview, "auto-check unavailable", false)
	case v.OK && !v.Detailed && v.Improved != nil && len(v.Improved.Steps) > 0:
		// блюдо настоящее, но расписано скупо: предлагаем автору подробную версию, публикация — после его решения
		_ = m.repo.SetSuggestion(ctx, rc.ID, strings.TrimSpace(v.Reason), v.Improved)
	case v.OK:
		_ = m.repo.SetStatus(ctx, rc.ID, StatusApproved, "", true)
	default:
		_ = m.repo.SetStatus(ctx, rc.ID, StatusReview, strings.TrimSpace(v.Reason), false)
	}
}

// Suggestion — решение автора по подробной версии: принять (текст заменяется, рецепт публикуется)
// или оставить как есть (уходит на ручную проверку).
func (m *Moderation) Suggestion(ctx context.Context, userID, id string, accept bool) (string, error) {
	rc, err := m.recipes.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if rc.OwnerID != userID {
		return "", domain.ErrForbidden
	}
	if rc.Status != StatusImprove || rc.Suggestion == nil {
		return rc.Status, nil
	}
	if !accept {
		if err := m.repo.SetStatus(ctx, id, StatusReview, "author kept the short version", false); err != nil {
			return "", err
		}
		return StatusReview, nil
	}
	sg := rc.Suggestion
	if len(sg.Steps) > 20 || len([]rune(sg.Title)) > 80 {
		sg.Steps = sg.Steps[:min(20, len(sg.Steps))]
		sg.Title = string([]rune(sg.Title)[:min(80, len([]rune(sg.Title)))])
	}
	if len([]rune(sg.Description)) > 300 {
		sg.Description = string([]rune(sg.Description)[:300])
	}
	if err := m.repo.ApplyText(ctx, id, sg.Title, sg.Description, sg.Steps); err != nil {
		return "", err
	}
	if err := m.repo.SetStatus(ctx, id, StatusApproved, "", true); err != nil {
		return "", err
	}
	return StatusApproved, nil
}

// Queue — что ждёт модератора: сначала самые старые. У каждого — ник автора.
func (m *Moderation) Queue(ctx context.Context) ([]planner.Recipe, error) {
	list, err := m.repo.Queue(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Author, _ = m.users.NickOf(ctx, list[i].OwnerID)
	}
	if list == nil {
		list = []planner.Recipe{}
	}
	return list, nil
}

// Decide — решение модератора: approve публикует, reject закрывает с причиной для автора.
func (m *Moderation) Decide(ctx context.Context, id string, approve bool, note string) error {
	note = strings.TrimSpace(note)
	if len(note) > 500 {
		note = note[:500]
	}
	if approve {
		return m.repo.SetStatus(ctx, id, StatusApproved, note, true)
	}
	if note == "" {
		return domain.Invalid("moderation.note")
	}
	return m.repo.SetStatus(ctx, id, StatusRejected, note, false)
}

// Approved — опубликованные рецепты пользователей для сайта.
func (m *Moderation) Approved(ctx context.Context, limit int) ([]planner.Recipe, error) {
	list, err := m.repo.Approved(ctx, limit)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Author, _ = m.users.NickOf(ctx, list[i].OwnerID)
	}
	return list, nil
}

func (m *Moderation) allow(userID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	keep := m.usage[userID][:0]
	for _, t := range m.usage[userID] {
		if now.Sub(t) < time.Hour {
			keep = append(keep, t)
		}
	}
	if len(keep) >= publishPerHour {
		m.usage[userID] = keep
		return false
	}
	m.usage[userID] = append(keep, now)
	return true
}
