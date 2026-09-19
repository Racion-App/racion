package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"

	"racion/internal/ai"
	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Переводы своих рецептов: при добавлении или правке рецепт ставится в очередь на все языки, кроме языка
// оригинала; воркер переводит по одному через пул нейросетей (бесплатные уровни: 1 запрос в секунду) и
// пишет результат в user_recipes.i18n. Автор видит статус по языкам и модель, которая перевела.

const (
	TrQueued  = "queued"
	TrRunning = "running"
	TrDone    = "done"
	TrError   = "error"
)

type TranslationRepo interface {
	Enqueue(ctx context.Context, recipeID string, langs []string) error
	Next(ctx context.Context) (recipeID, lang string, ok bool, err error)
	SetStatus(ctx context.Context, recipeID, lang, status, model, errText string) error
	ByRecipe(ctx context.Context, recipeID string) ([]domain.Translation, error)
	Summary(ctx context.Context, recipeIDs []string) (map[string]domain.TranslationSummary, error)
	ResetStale(ctx context.Context, olderThan time.Duration) error
}

type TranslationText interface {
	SetI18n(ctx context.Context, id, lang string, text planner.RecipeText) error
	ClearI18n(ctx context.Context, id string) error
}

type Translations struct {
	repo  TranslationRepo
	texts TranslationText
	own   UserRecipeRepo
	pool  *ai.Pool
	log   *zap.Logger
	wake  chan struct{}
}

func NewTranslations(repo TranslationRepo, texts TranslationText, own UserRecipeRepo, pool *ai.Pool, log *zap.Logger) *Translations {
	return &Translations{repo: repo, texts: texts, own: own, pool: pool, log: log, wake: make(chan struct{}, 1)}
}

func (t *Translations) Enabled() bool { return t != nil && t.pool != nil && t.pool.Enabled() }

// Targets — на какие языки переводить рецепт с языка from: все поддерживаемые, кроме исходного.
func Targets(from string) []string {
	out := make([]string, 0, len(i18n.Langs))
	for _, l := range i18n.Langs {
		if string(l) != from {
			out = append(out, string(l))
		}
	}
	return out
}

// Schedule — поставить рецепт в очередь заново (после создания или правки текста): старые переводы стираются.
func (t *Translations) Schedule(ctx context.Context, recipeID, from string) error {
	if !t.Enabled() {
		return nil
	}
	if err := t.texts.ClearI18n(ctx, recipeID); err != nil {
		return err
	}
	if err := t.repo.Enqueue(ctx, recipeID, Targets(from)); err != nil {
		return err
	}
	t.kick()
	return nil
}

// Retry — повторить языки с ошибкой и недостающие (кнопка у автора).
func (t *Translations) Retry(ctx context.Context, userID, recipeID string) error {
	rc, err := t.own.Get(ctx, recipeID)
	if err != nil {
		return err
	}
	if rc.OwnerID != userID {
		return domain.ErrForbidden
	}
	if !t.Enabled() {
		return domain.Invalid("api.ai.off")
	}
	if err := t.repo.Enqueue(ctx, recipeID, Targets(rc.Lang)); err != nil {
		return err
	}
	t.kick()
	return nil
}

// Status — состояние переводов рецепта для автора + какие провайдеры сейчас работают.
func (t *Translations) Status(ctx context.Context, userID, recipeID string) (domain.TranslationStatus, error) {
	rc, err := t.own.Get(ctx, recipeID)
	if err != nil {
		return domain.TranslationStatus{}, err
	}
	if rc.OwnerID != userID {
		return domain.TranslationStatus{}, domain.ErrForbidden
	}
	items, err := t.repo.ByRecipe(ctx, recipeID)
	if err != nil {
		return domain.TranslationStatus{}, err
	}
	st := domain.TranslationStatus{Source: rc.Lang, Enabled: t.Enabled(), Items: items}
	for _, s := range t.Providers() {
		if !s.Resting {
			st.Providers = append(st.Providers, s.Name+"/"+s.Model)
		}
	}
	return st, nil
}

// Summary — «готово N из M, сейчас: es (модель)» для списка своих рецептов.
func (t *Translations) Summary(ctx context.Context, ids []string) map[string]domain.TranslationSummary {
	if t == nil || len(ids) == 0 {
		return map[string]domain.TranslationSummary{}
	}
	m, err := t.repo.Summary(ctx, ids)
	if err != nil {
		t.log.Warn("translations summary", zap.Error(err))
		return map[string]domain.TranslationSummary{}
	}
	return m
}

func (t *Translations) Providers() []ai.ProviderStatus {
	if t == nil || t.pool == nil {
		return []ai.ProviderStatus{}
	}
	return t.pool.Status()
}

func (t *Translations) kick() {
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

// Run — воркер: берёт следующую задачу, переводит, пишет статус. Без свободного провайдера ждёт минуту.
func (t *Translations) Run(ctx context.Context) {
	if !t.Enabled() {
		return
	}
	// зависшие «running» после перезапуска — обратно в очередь
	_ = t.repo.ResetStale(ctx, 10*time.Minute)
	for {
		did, err := t.step(ctx)
		if ctx.Err() != nil {
			return
		}
		wait := 30 * time.Second
		switch {
		case err != nil && errors.Is(err, ai.ErrBusy):
			wait = 60 * time.Second
		case err != nil:
			t.log.Warn("translation", zap.Error(err))
			wait = 5 * time.Second
		case did:
			wait = 0
		}
		if wait == 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-t.wake:
		case <-time.After(wait):
		}
	}
}

func (t *Translations) step(ctx context.Context) (bool, error) {
	recipeID, lang, ok, err := t.repo.Next(ctx)
	if err != nil || !ok {
		return false, err
	}
	rc, err := t.own.Get(ctx, recipeID)
	if err != nil {
		_ = t.repo.SetStatus(ctx, recipeID, lang, TrError, "", "recipe missing")
		return true, nil
	}
	if err := t.repo.SetStatus(ctx, recipeID, lang, TrRunning, "", ""); err != nil {
		return false, err
	}
	in := ai.RecipeText{Title: rc.Title, Description: rc.Description, Steps: rc.Steps}
	var ref *ai.RecipeText
	if en, ok := rc.I18n["en"]; ok && rc.Lang != "en" && en.Title != "" {
		ref = &ai.RecipeText{Title: en.Title, Description: en.Description, Steps: en.Steps}
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	out, model, err := t.pool.TranslateRecipeFrom(cctx, rc.Lang, lang, in, ref)
	cancel()
	if err != nil {
		var he *ai.HTTPError
		if errors.Is(err, ai.ErrBusy) || (errors.As(err, &he) && (he.Status == 429 || he.Quota() || he.Status >= 500)) {
			// лимит провайдера — вернуть в очередь и подождать
			_ = t.repo.SetStatus(ctx, recipeID, lang, TrQueued, "", "")
			return false, ai.ErrBusy
		}
		_ = t.repo.SetStatus(ctx, recipeID, lang, TrError, model, truncateErr(err))
		return true, nil
	}
	if err := t.texts.SetI18n(ctx, recipeID, lang, planner.RecipeText{Title: out.Title, Description: out.Description, Steps: out.Steps}); err != nil {
		return false, err
	}
	return true, t.repo.SetStatus(ctx, recipeID, lang, TrDone, model, "")
}

func truncateErr(err error) string {
	s := err.Error()
	if i := strings.Index(s, "{"); i > 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return strings.TrimSpace(s)
}
