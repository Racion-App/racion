package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"racion/internal/ai"
	"racion/internal/domain"
)

// Assistant — нейросеть для своих рецептов: поправить текст или перевести на язык интерфейса.
// Продукты и количества не трогает — они считаются по базе. Квота: assistantPerHour запросов на пользователя.

const assistantPerHour = 20

// RecipeAI — то, что нужно сервису от клиента OpenAI (internal/ai).
type RecipeAI interface {
	Enabled() bool
	ImproveRecipe(ctx context.Context, lang string, in ai.RecipeText) (ai.RecipeText, error)
	TranslateRecipe(ctx context.Context, to string, in ai.RecipeText, ref *ai.RecipeText) (ai.RecipeText, error)
}

type Assistant struct {
	ai    RecipeAI
	mu    sync.Mutex
	usage map[string][]time.Time // пользователь → моменты запросов за последний час
}

func NewAssistant(client RecipeAI) *Assistant {
	return &Assistant{ai: client, usage: map[string][]time.Time{}}
}

func (s *Assistant) Enabled() bool { return s != nil && s.ai != nil && s.ai.Enabled() }

// Run — action: improve | translate; lang — язык текста (improve) или язык, на который перевести (translate).
func (s *Assistant) Run(ctx context.Context, userID, action, lang string, in ai.RecipeText) (ai.RecipeText, error) {
	if !s.Enabled() {
		return in, domain.Invalid("api.ai.off")
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	steps := in.Steps[:0:0]
	for _, st := range in.Steps {
		if st = strings.TrimSpace(st); st != "" {
			steps = append(steps, st)
		}
	}
	in.Steps = steps
	if in.Title == "" || len(in.Steps) == 0 {
		return in, domain.Invalid("api.ai.empty")
	}
	if utf8.RuneCountInString(in.Title) > 120 || utf8.RuneCountInString(in.Description) > 600 || len(in.Steps) > 30 {
		return in, domain.Invalid("own.err.title")
	}
	for _, st := range in.Steps {
		if utf8.RuneCountInString(st) > 600 {
			return in, domain.Invalid("own.err.steps")
		}
	}
	if !s.allow(userID) {
		return in, domain.Invalid("api.ai.limit")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	var out ai.RecipeText
	var err error
	switch action {
	case "translate":
		out, err = s.ai.TranslateRecipe(ctx, lang, in, nil)
	default:
		out, err = s.ai.ImproveRecipe(ctx, lang, in)
	}
	if err != nil {
		if errors.Is(err, ai.ErrDisabled) {
			return in, domain.Invalid("api.ai.off")
		}
		return in, domain.Invalid("api.ai.busy")
	}
	// ответ подрезаем под лимиты формы: название 80, подводка 300
	out.Title = cut(strings.TrimSpace(out.Title), 80)
	out.Description = cut(strings.TrimSpace(out.Description), 300)
	return out, nil
}

func (s *Assistant) allow(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	keep := s.usage[userID][:0]
	for _, t := range s.usage[userID] {
		if now.Sub(t) < time.Hour {
			keep = append(keep, t)
		}
	}
	if len(keep) >= assistantPerHour {
		s.usage[userID] = keep
		return false
	}
	s.usage[userID] = append(keep, now)
	return true
}

func cut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n]))
}
