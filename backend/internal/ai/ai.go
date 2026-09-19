// Package ai — вызовы OpenAI (Chat Completions, JSON-ответ). Используется для переводов локалей и рецептов
// и для улучшения текста своих рецептов. Ключ — OPENAI_API_KEY; без ключа клиент отвечает ErrDisabled.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrDisabled = errors.New("ai: no api key")

// HTTPError — ответ провайдера не 200: по коду пул решает, ждать (429), отключить провайдера (402, кончились
// кредиты) или считать запрос неудачным.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("ai: http %d: %s", e.Status, e.Body) }

// Quota — кончились кредиты или дневной лимит: провайдера надо отложить надолго.
func (e *HTTPError) Quota() bool {
	b := strings.ToLower(e.Body)
	return e.Status == 402 || strings.Contains(b, "insufficient_quota") || strings.Contains(b, "no credits") || strings.Contains(b, "billing")
}

type Client struct {
	key   string
	model string
	http  *http.Client
	base  string
	codex *codexBackend // если задан — запросы идут через Codex CLI, а не API
	fast  bool          // без внутренних повторов при 429: пул сам переключает провайдера
}

// New — клиент; model по умолчанию gpt-5-nano (дёшево и достаточно для переводов).
func New(key, model string) *Client {
	if model == "" {
		model = "gpt-5-nano"
	}
	return NewWithBase("https://api.openai.com/v1", key, model)
}

// NewWithBase — тот же клиент к любому OpenAI-совместимому эндпоинту (например, локальному прокси
// подписки ChatGPT на http://127.0.0.1:10531/v1, где ключ не нужен — тогда key любой непустой).
func NewWithBase(base, key, model string) *Client {
	if model == "" {
		model = "gpt-5-nano"
	}
	return &Client{key: key, model: model, http: &http.Client{Timeout: 180 * time.Second}, base: strings.TrimRight(base, "/")}
}

func (c *Client) Enabled() bool { return c != nil && c.key != "" }

// Model — имя модели (для статусов переводов и админки).
func (c *Client) Model() string { return c.model }

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// JSON шлёт system+user и разбирает ответ-JSON в out. Повторяет при 429/5xx до трёх раз.
func (c *Client) JSON(ctx context.Context, system, user string, out any) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	if c.codex != nil {
		return c.codexJSON(ctx, system, user, out)
	}
	payload := map[string]any{
		"model":           c.model,
		"messages":        []message{{"system", system}, {"user", user}},
		"response_format": map[string]string{"type": "json_object"},
	}
	if strings.Contains(c.base, "api.openai.com") {
		payload["reasoning_effort"] = "low" // переводу размышления не нужны, а ждать их долго
	}
	body, _ := json.Marshal(payload)
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, "POST", c.base+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if res.StatusCode == 429 || res.StatusCode >= 500 {
			he := &HTTPError{Status: res.StatusCode, Body: string(truncate(raw))}
			lastErr = he
			if he.Quota() || c.fast {
				return lastErr // кредиты кончились — ждать бессмысленно; в пуле ждёт сам пул
			}
			// бесплатные тарифы режут по запросам в минуту: ждём дольше с каждой попыткой (5, 10, 20, 40, 60 с)
			wait := time.Duration(5<<attempt) * time.Second
			if wait > 60*time.Second {
				wait = 60 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if res.StatusCode != 200 {
			return &HTTPError{Status: res.StatusCode, Body: string(truncate(raw))}
		}
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 {
			return fmt.Errorf("openai: bad response: %s", truncate(raw))
		}
		content := strings.TrimSpace(parsed.Choices[0].Message.Content)
		if err := json.Unmarshal([]byte(content), out); err != nil {
			return fmt.Errorf("openai: bad json in answer: %w: %s", err, truncate([]byte(content)))
		}
		return nil
	}
	return lastErr
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

// ── Переводы ───────────────────────────────────────────────────────────────

const translateSystem = `You are a professional UI and cookbook translator. Translate UI strings of a consumer app about weekly meal planning and groceries into %s.
Input is a JSON object: key → {"ru": Russian original, "en": English translation}. Translate the meaning of the Russian original; use the English as a hint for terms.
Rules: keep keys unchanged; keep placeholders like {n}, {name}, {kcal}, {0} exactly as they are; keep units, numbers and punctuation such as "≈", "·", "—";
be natural and concise, as a native product writer would; keep "Racion" as the product name (transliterate only if the script differs);
keys ending in .one/.few/.many are plural forms of one word for 1 / 2–4 / 5+ items: give the correct form for %s, and if the language does not inflect, repeat the same form.
Answer with JSON: {"translations": {key: translated string}} — every value is a plain string in the target language (never an object), exactly the same keys.`

// TranslateStrings переводит словарь порциями по 25 ключей, по 5 порций параллельно.
// ref — английский перевод как подсказка (может быть пустым). Ошибка порции не роняет остальные.
func (c *Client) TranslateStrings(ctx context.Context, to string, src, ref map[string]string, progress func(done, total int)) (map[string]string, error) {
	keys := make([]string, 0, len(src))
	for k := range src {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	const batch = 25
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		out    = map[string]string{}
		failed error
		done   int
		sem    = make(chan struct{}, 5)
	)
	for i := 0; i < len(keys); i += batch {
		part := map[string]map[string]string{}
		for _, k := range keys[i:min(i+batch, len(keys))] {
			part[k] = map[string]string{"ru": src[k], "en": ref[k]}
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, part map[string]map[string]string) {
			defer wg.Done()
			defer func() { <-sem }()
			user, _ := json.Marshal(part)
			var got struct {
				Translations map[string]string `json:"translations"`
			}
			var err error
			for attempt := 0; attempt < 2; attempt++ { // модель изредка возвращает не тот формат — переспрашиваем
				if err = c.JSON(ctx, fmt.Sprintf(translateSystem, langName(to), langName(to)), string(user), &got); err == nil {
					break
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed = errors.Join(failed, fmt.Errorf("keys %d–%d: %w", i, i+len(part), err))
				return
			}
			for k, v := range got.Translations {
				if _, ok := part[k]; ok && strings.TrimSpace(v) != "" {
					out[k] = v
				}
			}
			done += len(part)
			if progress != nil {
				progress(done, len(keys))
			}
		}(i, part)
	}
	wg.Wait()
	return out, failed
}

// RecipeText — то, что переводится у рецепта.
type RecipeText struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
}

const recipeSystem = `You translate home-cooking recipes from %s into %s for a meal-planning app. Input: {"src": the original, "en": an English translation or null}.
Translate the original (use English as a hint when present). Keep quantities, temperatures and times exactly; use the wording a native cook would use, with idiomatic dish names where they exist;
keep the same number of steps, each a short imperative sentence.
Answer with JSON: {"title": "...", "description": "...", "steps": ["...", ...]}.`

// TranslateRecipe переводит рецепт; ref — английская версия как подсказка (nil — нет).
func (c *Client) TranslateRecipe(ctx context.Context, to string, in RecipeText, ref *RecipeText) (RecipeText, error) {
	return c.TranslateRecipeFrom(ctx, "ru", to, in, ref)
}

// TranslateRecipeFrom — то же с явным языком оригинала (свои рецепты пишут на любом языке).
func (c *Client) TranslateRecipeFrom(ctx context.Context, from, to string, in RecipeText, ref *RecipeText) (RecipeText, error) {
	user, _ := json.Marshal(map[string]any{"src": in, "en": ref})
	var out RecipeText
	if err := c.JSON(ctx, fmt.Sprintf(recipeSystem, langName(from), langName(to)), string(user), &out); err != nil {
		return out, err
	}
	if out.Title == "" || len(out.Steps) != len(in.Steps) {
		return out, fmt.Errorf("openai: incomplete recipe translation for %q", in.Title)
	}
	return out, nil
}

// DetailStandard — что считается подробным рецептом; общий для правки, проверки и массовой доработки базы.
const DetailStandard = `A detailed recipe lets a person who has never cooked follow it without questions. Standard:
- first step is preparation: what to wash, peel, how to cut (size in cm), what to preheat and to what temperature;
- every cooking step names the heat level (low / medium / high), the time in minutes and the doneness cue (golden, soft when pierced, liquid reduced by half, internal temperature for meat);
- pan or pot size, whether to cover with a lid, when to salt, when to stir;
- resting, serving and what to serve with; a note on what can be swapped or prepared ahead;
- 6 to 12 steps for cooked dishes, each one or two short imperative sentences; no vague words like "cook until done";
- write like a cookbook, in flowing sentences, never as "time: … / heat: … / goal: …" fields and never numbered inside the text;
- voice: the neutral cookbook form of the language — Russian infinitives («нарезать», «обжарить», never «нарежь»/«нарежьте»), English imperatives; a short label before the colon («Подготовка:», «Подача:») is fine;
- a no-cook item (a fruit, a cheese plate, a sandwich, a bowl assembled from ready things) gets 2 to 4 natural steps: wash, cut, assemble, serve; do not invent heat, timers or padding for it.`

const improveSystem = `You are an editor for a home-cooking app. Rewrite a user's recipe written in %s so that it meets this standard:
` + DetailStandard + `
Keep the dish, the ingredient list and the quantities exactly as given; do not invent ingredients that are not mentioned (salt, pepper, oil and water are allowed).
Fix grammar, make the title short and appetizing, write a one-or-two-sentence description.
Answer with JSON: {"title": "...", "description": "...", "steps": ["...", ...]}.`

// ImproveRecipe правит текст своего рецепта на его же языке.
func (c *Client) ImproveRecipe(ctx context.Context, lang string, in RecipeText) (RecipeText, error) {
	user, _ := json.Marshal(in)
	var out RecipeText
	if err := c.JSON(ctx, fmt.Sprintf(improveSystem, langName(lang)), string(user), &out); err != nil {
		return out, err
	}
	if out.Title == "" || len(out.Steps) == 0 {
		return out, errors.New("openai: empty improvement")
	}
	out.Steps = stripNumbers(out.Steps)
	return out, nil
}

var reStepNo = regexp.MustCompile(`^\s*(?:шаг\s*)?\d{1,2}\s*[.)]\s*`)

// stripNumbers убирает «1) », «2. » в начале шагов: нумерует список интерфейс.
func stripNumbers(steps []string) []string {
	out := make([]string, 0, len(steps))
	for _, st := range steps {
		if st = strings.TrimSpace(reStepNo.ReplaceAllString(st, "")); st != "" {
			out = append(out, st)
		}
	}
	return out
}

// ── Модерация ──────────────────────────────────────────────────────────────

type RecipeCheck struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	Ingredients []string `json:"ingredients"`
}

type RecipeVerdict struct {
	OK       bool        `json:"ok"`
	Reason   string      `json:"reason"`
	Detailed bool        `json:"detailed"`           // рецепт расписан достаточно подробно
	Improved *RecipeText `json:"improved,omitempty"` // если нет — подробная версия для автора
}

const checkSystem = `You moderate user-submitted recipes for a family meal-planning app. Decide whether the recipe can be published as is.
Publish (ok=true) when it is a genuine, edible home-cooking recipe with a sensible title, ingredients that match the steps and nothing harmful.
Send to manual review (ok=false) when anything looks off: not a recipe or nonsense; advertising, links, phone numbers, contacts or promo codes;
profanity, insults, politics or adult content; dangerous instructions (undercooked poultry or pork, raw eggs for children, non-food items, alcohol as a main component);
title that does not match the content; steps that cannot be followed. When unsure, ok=false.
If ok=true, also judge whether the recipe is detailed enough by this standard:
` + DetailStandard + `
When it is not, set detailed=false and write "improved": a rewritten version that meets the standard, in the language of the recipe, keeping the ingredients and quantities exactly.
Answer with JSON {"ok": true|false, "reason": "one sentence in the language of the recipe", "detailed": true|false, "improved": {"title": "...", "description": "...", "steps": ["..."]} | null}.`

// CheckRecipe — вердикт нейросети: можно публиковать или на ручную проверку с причиной.
func (c *Client) CheckRecipe(ctx context.Context, in RecipeCheck) (RecipeVerdict, error) {
	user, _ := json.Marshal(in)
	var out RecipeVerdict
	err := c.JSON(ctx, checkSystem, string(user), &out)
	return out, err
}

func langName(code string) string {
	names := map[string]string{"ru": "Russian", "en": "English", "de": "German", "es": "Spanish", "fr": "French", "it": "Italian", "pt": "Portuguese",
		"pl": "Polish", "uk": "Ukrainian", "tr": "Turkish", "kk": "Kazakh", "nl": "Dutch", "cs": "Czech", "sv": "Swedish", "zh": "Chinese", "ja": "Japanese", "ko": "Korean"}
	if n, ok := names[code]; ok {
		return n
	}
	return code
}

// Notes — заметки к рецепту для страницы: пять коротких тем. Пишутся по правилам plain-prose
// (github.com/dripips/plain-prose): без вводных, без штампов, без троек ради ритма, конкретика в граммах,
// градусах и минутах, только продукты из рецепта. Каждая тема — одно-два предложения, 15–40 слов.
type Notes struct {
	Why      string `json:"why"`
	Swaps    string `json:"swaps"`
	Mistakes string `json:"mistakes"`
	Keep     string `json:"keep"`
	Serve    string `json:"serve"`
}

// NotesInput — что даём модели: рецепт целиком плюс список продуктов с граммовками.
type NotesInput struct {
	RecipeText
	Ingredients []string `json:"ingredients"`
	Slot        string   `json:"slot"`
	Tags        []string `json:"tags"`
}

const notesSystem = `You write short practical notes for a home-cooking recipe page, in %[1]s, for a meal-planning app. Input: the recipe (title, description, steps, ingredients with amounts per portion, meal slot, tags).
Return JSON with five fields. HARD LIMIT: each field is at most two sentences and at most 35 words; one sentence is often enough. Plain text, no markdown.
- "why": one concrete reason a key step is done this way (temperature, order, cut, timing). Not praise.
- "swaps": one or two substitutions for ingredients from the list, with the amount or the effect. Only common products people have at home; never invent products. If nothing sensible, empty string.
- "mistakes": the single most common error with this dish and how to notice it by sight, smell or timing.
- "keep": how many days it keeps in the fridge and how to reheat; say plainly if it should be eaten fresh. Mention the freezer only if it genuinely freezes well.
- "serve": one concrete pairing or occasion from everyday food.
House style:
- Address the reader informally and consistently in the singular (Russian: «ты» — «замени», «подавай», «храни»; never «вы», never impersonal «солят», «подают»). English: imperative.
- Facts only: grams, degrees, minutes, what you see. No openers ("it is worth noting"), no closers ("enjoy"), no exclamation marks, no emoji, no praise ("delicious", "perfect", "ideal"), no "not X but Y" templates, no rhetorical questions, no jargon or abbreviations ("PP", "KBJU").
- No bureaucratic phrasing (Russian: «осуществить», «является», «данный», «в рамках», «с целью»; English: "utilize", "leverage", "ensure").
- Do not repeat the steps or the description; add what they do not say. Use only ingredients that are in the list or are truly common substitutes.`

// RecipeNotes — заметки на языке lang.
func (c *Client) RecipeNotes(ctx context.Context, lang string, in NotesInput) (Notes, error) {
	user, _ := json.Marshal(in)
	var out Notes
	if err := c.JSON(ctx, fmt.Sprintf(notesSystem, langName(lang)), string(user), &out); err != nil {
		return out, err
	}
	if out.Why == "" && out.Swaps == "" && out.Mistakes == "" && out.Keep == "" && out.Serve == "" {
		return out, errors.New("ai: empty notes")
	}
	// модель любит растекаться: поле длиннее 45 слов — попросить короче один раз
	if notesTooLong(out) {
		user2 := string(user) + `

Your previous answer was too long. Rewrite: at most two sentences and 35 words per field; cut adjectives and repeated advice.`
		var again Notes
		if err := c.JSON(ctx, fmt.Sprintf(notesSystem, langName(lang)), user2, &again); err == nil && !notesTooLong(again) {
			return again, nil
		}
	}
	return out, nil
}

func notesTooLong(n Notes) bool {
	for _, f := range []string{n.Why, n.Swaps, n.Mistakes, n.Keep, n.Serve} {
		if len(strings.Fields(f)) > 45 {
			return true
		}
	}
	return false
}
