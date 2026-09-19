package ai

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Pool — несколько провайдеров по очереди: бесплатные уровни (Mistral, Gemini, Groq, OpenRouter) режут по
// запросам в минуту и в день, платные могут остаться без кредитов. Пул держит у каждого провайдера паузу между
// запросами, дневной счётчик и «отдых» после 429; запрос уходит первому, кто сейчас свободен, при отказе —
// следующему. Какая модель ответила, пул возвращает наружу: это показывается в статусе перевода.

type Provider struct {
	Name      string        // mistral | gemini | groq | openrouter | openai | local
	Client    *Client
	MinGap    time.Duration // пауза между запросами (1 запрос/с у Mistral → 1.1s)
	PerMinute int           // 0 — без лимита
	PerDay    int           // 0 — без лимита

	mu        sync.Mutex
	last      time.Time
	minute    []time.Time // моменты запросов за последнюю минуту
	dayStart  time.Time
	dayCount  int
	restUntil time.Time // после 429 или кончившихся кредитов
	okCount   int
	failCount int
	lastErr   string
}

// ProviderStatus — для админки и статусов переводов.
type ProviderStatus struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	Today     int    `json:"today"`     // запросов за сегодня
	PerDay    int    `json:"perDay"`    // дневной лимит (0 — нет)
	OK        int    `json:"ok"`        // удачных с запуска
	Failed    int    `json:"failed"`    // неудачных с запуска
	Resting   bool   `json:"resting"`   // отдыхает после 429/402
	RestUntil string `json:"restUntil,omitempty"`
	LastError string `json:"lastError,omitempty"`
}

type Pool struct {
	providers []*Provider
}

func NewPool(providers ...*Provider) *Pool {
	var list []*Provider
	for _, p := range providers {
		if p != nil && p.Client != nil && p.Client.Enabled() {
			p.Client.fast = true
			list = append(list, p)
		}
	}
	return &Pool{providers: list}
}

func (p *Pool) Enabled() bool { return p != nil && len(p.providers) > 0 }

// Model — модель первого провайдера (для логов «помощник включён»).
func (p *Pool) Model() string {
	if !p.Enabled() {
		return ""
	}
	return p.providers[0].Name + "/" + p.providers[0].Client.Model()
}

func (p *Pool) Status() []ProviderStatus {
	out := []ProviderStatus{}
	if p == nil {
		return out
	}
	now := time.Now()
	for _, pr := range p.providers {
		pr.mu.Lock()
		pr.rollDay(now)
		st := ProviderStatus{Name: pr.Name, Model: pr.Client.Model(), Today: pr.dayCount, PerDay: pr.PerDay, OK: pr.okCount, Failed: pr.failCount, Resting: now.Before(pr.restUntil), LastError: pr.lastErr}
		if st.Resting {
			st.RestUntil = pr.restUntil.UTC().Format(time.RFC3339)
		}
		pr.mu.Unlock()
		out = append(out, st)
	}
	return out
}

func (pr *Provider) rollDay(now time.Time) {
	if now.Sub(pr.dayStart) >= 24*time.Hour {
		pr.dayStart = now
		pr.dayCount = 0
	}
}

// reserve — можно ли слать сейчас; если да, ждёт паузу и занимает слот. false — провайдер занят/отдыхает.
func (pr *Provider) reserve(ctx context.Context) bool {
	pr.mu.Lock()
	now := time.Now()
	pr.rollDay(now)
	if now.Before(pr.restUntil) || (pr.PerDay > 0 && pr.dayCount >= pr.PerDay) {
		pr.mu.Unlock()
		return false
	}
	if pr.PerMinute > 0 {
		keep := pr.minute[:0]
		for _, t := range pr.minute {
			if now.Sub(t) < time.Minute {
				keep = append(keep, t)
			}
		}
		pr.minute = keep
		if len(pr.minute) >= pr.PerMinute {
			pr.mu.Unlock()
			return false
		}
	}
	wait := pr.MinGap - now.Sub(pr.last)
	pr.last = now.Add(max(wait, 0))
	pr.dayCount++
	pr.minute = append(pr.minute, pr.last)
	pr.mu.Unlock()
	if wait > 0 {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(wait):
		}
	}
	return true
}

func (pr *Provider) report(err error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if err == nil {
		pr.okCount++
		pr.lastErr = ""
		return
	}
	pr.failCount++
	pr.lastErr = err.Error()
	var he *HTTPError
	if errors.As(err, &he) {
		switch {
		case he.Quota():
			pr.restUntil = time.Now().Add(6 * time.Hour) // кредиты или дневной лимит: проверим через несколько часов
		case he.Status == 429:
			pr.restUntil = time.Now().Add(90 * time.Second)
		case he.Status >= 500:
			pr.restUntil = time.Now().Add(30 * time.Second)
		}
	}
}

var ErrBusy = errors.New("ai: all providers are busy or resting")

// call — по очереди провайдеров; f получает клиента и возвращает ошибку. Возвращает имя модели, что ответила.
func (p *Pool) call(ctx context.Context, f func(c *Client) error) (string, error) {
	if !p.Enabled() {
		return "", ErrDisabled
	}
	var last error = ErrBusy
	for _, pr := range p.providers {
		if !pr.reserve(ctx) {
			continue
		}
		err := f(pr.Client)
		pr.report(err)
		if err == nil {
			return pr.Name + "/" + pr.Client.Model(), nil
		}
		last = err
		var he *HTTPError
		if ctx.Err() != nil || !errors.As(err, &he) {
			return "", err // сеть или плохой JSON — не вина лимитов, дальше не пробуем
		}
	}
	return "", last
}

// JSONModel — как JSON, но возвращает и модель, которая ответила.
func (p *Pool) JSONModel(ctx context.Context, system, user string, out any) (string, error) {
	return p.call(ctx, func(c *Client) error { return c.JSON(ctx, system, user, out) })
}

func (p *Pool) JSON(ctx context.Context, system, user string, out any) error {
	_, err := p.call(ctx, func(c *Client) error { return c.JSON(ctx, system, user, out) })
	return err
}

func (p *Pool) TranslateRecipe(ctx context.Context, to string, in RecipeText, ref *RecipeText) (RecipeText, error) {
	out, _, err := p.TranslateRecipeFrom(ctx, "ru", to, in, ref)
	return out, err
}

// TranslateRecipeFrom — перевод с указанием модели, которая его сделала.
func (p *Pool) TranslateRecipeFrom(ctx context.Context, from, to string, in RecipeText, ref *RecipeText) (RecipeText, string, error) {
	var out RecipeText
	model, err := p.call(ctx, func(c *Client) error {
		var e error
		out, e = c.TranslateRecipeFrom(ctx, from, to, in, ref)
		return e
	})
	return out, model, err
}

func (p *Pool) ImproveRecipe(ctx context.Context, lang string, in RecipeText) (RecipeText, error) {
	var out RecipeText
	_, err := p.call(ctx, func(c *Client) error {
		var e error
		out, e = c.ImproveRecipe(ctx, lang, in)
		return e
	})
	return out, err
}

func (p *Pool) CheckRecipe(ctx context.Context, in RecipeCheck) (RecipeVerdict, error) {
	var out RecipeVerdict
	_, err := p.call(ctx, func(c *Client) error {
		var e error
		out, e = c.CheckRecipe(ctx, in)
		return e
	})
	return out, err
}

// Providers — сборка пула из ключей окружения в заданном порядке. Лимиты — бесплатных уровней (сентябрь 2026).
type ProviderKeys struct {
	Order      []string
	Mistral    string
	Gemini     string
	Groq       string
	OpenRouter string
	OpenAI     string
	LocalURL   string // OpenAI-совместимый прокси без ключа (ima2)
	Models     map[string]string
}

func NewPoolFromKeys(k ProviderKeys) *Pool {
	model := func(name, def string) string {
		if m := k.Models[name]; m != "" {
			return m
		}
		return def
	}
	build := map[string]func() *Provider{
		"mistral": func() *Provider {
			return &Provider{Name: "mistral", Client: NewWithBase("https://api.mistral.ai/v1", k.Mistral, model("mistral", "ministral-14b-latest")), MinGap: 1100 * time.Millisecond}
		},
		"gemini": func() *Provider {
			return &Provider{Name: "gemini", Client: NewWithBase("https://generativelanguage.googleapis.com/v1beta/openai", k.Gemini, model("gemini", "gemini-2.5-flash-lite")), MinGap: 4 * time.Second, PerMinute: 15, PerDay: 1000}
		},
		"groq": func() *Provider {
			return &Provider{Name: "groq", Client: NewWithBase("https://api.groq.com/openai/v1", k.Groq, model("groq", "llama-3.3-70b-versatile")), MinGap: 2 * time.Second, PerMinute: 30, PerDay: 900}
		},
		"openrouter": func() *Provider {
			return &Provider{Name: "openrouter", Client: NewWithBase("https://openrouter.ai/api/v1", k.OpenRouter, model("openrouter", "google/gemma-4-31b-it:free")), MinGap: 3 * time.Second, PerMinute: 20, PerDay: 50}
		},
		"openai": func() *Provider {
			return &Provider{Name: "openai", Client: New(k.OpenAI, model("openai", "gpt-5-nano"))}
		},
		"local": func() *Provider {
			if k.LocalURL == "" {
				return nil
			}
			return &Provider{Name: "local", Client: NewWithBase(k.LocalURL, "local", model("local", "gpt-5.5")), MinGap: 500 * time.Millisecond}
		},
	}
	keys := map[string]string{"mistral": k.Mistral, "gemini": k.Gemini, "groq": k.Groq, "openrouter": k.OpenRouter, "openai": k.OpenAI, "local": k.LocalURL}
	order := k.Order
	if len(order) == 0 {
		order = []string{"mistral", "gemini", "groq", "openrouter", "openai", "local"}
	}
	var list []*Provider
	for _, name := range order {
		if keys[name] == "" {
			continue
		}
		if mk, ok := build[name]; ok {
			list = append(list, mk())
		}
	}
	return NewPool(list...)
}
