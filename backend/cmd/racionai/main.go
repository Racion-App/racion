// racionai — переводы и доработка рецептов нейросетью: по умолчанию через Codex CLI (подписка ChatGPT),
// либо через OpenAI API (-backend openai, OPENAI_API_KEY).
//
//	go run ./cmd/racionai locales -to es,fr,pl      # locales/ru.json → locales/es.json …
//	go run ./cmd/racionai recipes -to es,fr,pl      # рецепты базы → internal/seed/data/recipes_i18n_es.json …
//
// Ключ — OPENAI_API_KEY из окружения или из ../.env. Перевод рецептов возобновляемый: уже переведённые
// id пропускаются, файл дописывается по ходу. Готовые файлы попадают в репозиторий и подхватываются
// при сборке (локали) и при старте сервера (рецепты, через seed).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"racion/internal/ai"
	"racion/internal/i18n"
	"racion/locales"
)

// langMeta — что не переводится нейросетью: флаг, страна, правило числа, месяцы, форматы дат.
var langMeta = map[string]map[string]any{
	"de": {"name": "Deutsch", "english": "German", "flag": "de", "country": "DE", "plural": "one-other", "decimal": ",",
		"months": []string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
		"date":   "{d}. {month} {y}", "monthYear": "{month} {y}"},
	"es": {"name": "Español", "english": "Spanish", "flag": "es", "country": "ES", "plural": "one-other", "decimal": ",",
		"months": []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"},
		"date":   "{d} de {month} de {y}", "monthYear": "{month} de {y}"},
	"fr": {"name": "Français", "english": "French", "flag": "fr", "country": "FR", "plural": "one-other", "decimal": ",",
		"months": []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
		"date":   "{d} {month} {y}"},
	"it": {"name": "Italiano", "english": "Italian", "flag": "it", "country": "IT", "plural": "one-other", "decimal": ",",
		"months": []string{"gennaio", "febbraio", "marzo", "aprile", "maggio", "giugno", "luglio", "agosto", "settembre", "ottobre", "novembre", "dicembre"},
		"date":   "{d} {month} {y}"},
	"pt": {"name": "Português", "english": "Portuguese", "flag": "br", "country": "BR", "plural": "one-other", "decimal": ",",
		"months": []string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"},
		"date":   "{d} de {month} de {y}", "monthYear": "{month} de {y}"},
	"pl": {"name": "Polski", "english": "Polish", "flag": "pl", "country": "PL", "plural": "polish", "decimal": ",",
		"months":    []string{"styczeń", "luty", "marzec", "kwiecień", "maj", "czerwiec", "lipiec", "sierpień", "wrzesień", "październik", "listopad", "grudzień"},
		"monthsGen": []string{"stycznia", "lutego", "marca", "kwietnia", "maja", "czerwca", "lipca", "sierpnia", "września", "października", "listopada", "grudnia"},
		"date":      "{d} {month} {y}"},
	"uk": {"name": "Українська", "english": "Ukrainian", "flag": "ua", "country": "UA", "plural": "east-slavic", "decimal": ",",
		"months":    []string{"січень", "лютий", "березень", "квітень", "травень", "червень", "липень", "серпень", "вересень", "жовтень", "листопад", "грудень"},
		"monthsGen": []string{"січня", "лютого", "березня", "квітня", "травня", "червня", "липня", "серпня", "вересня", "жовтня", "листопада", "грудня"},
		"date":      "{d} {month} {y}"},
	"tr": {"name": "Türkçe", "english": "Turkish", "flag": "tr", "country": "TR", "plural": "none", "decimal": ",",
		"months": []string{"Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran", "Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık"},
		"date":   "{d} {month} {y}"},
	"kk": {"name": "Қазақша", "english": "Kazakh", "flag": "kz", "country": "KZ", "plural": "none", "decimal": ",",
		"months": []string{"қаңтар", "ақпан", "наурыз", "сәуір", "мамыр", "маусым", "шілде", "тамыз", "қыркүйек", "қазан", "қараша", "желтоқсан"},
		"date":   "{y} ж. {d} {month}", "monthYear": "{y} ж. {month}"},
	"nl": {"name": "Nederlands", "english": "Dutch", "flag": "nl", "country": "NL", "plural": "one-other", "decimal": ",",
		"months": []string{"januari", "februari", "maart", "april", "mei", "juni", "juli", "augustus", "september", "oktober", "november", "december"},
		"date":   "{d} {month} {y}"},
	"cs": {"name": "Čeština", "english": "Czech", "flag": "cz", "country": "CZ", "plural": "czech", "decimal": ",",
		"months":    []string{"leden", "únor", "březen", "duben", "květen", "červen", "červenec", "srpen", "září", "říjen", "listopad", "prosinec"},
		"monthsGen": []string{"ledna", "února", "března", "dubna", "května", "června", "července", "srpna", "září", "října", "listopadu", "prosince"},
		"date":      "{d}. {month} {y}"},
	"zh": {"name": "中文", "english": "Chinese", "flag": "cn", "country": "CN", "plural": "none", "decimal": ".",
		"months": []string{"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
		"date":   "{y}年{month}{d}日", "monthYear": "{y}年{month}"},
	"ja": {"name": "日本語", "english": "Japanese", "flag": "jp", "country": "JP", "plural": "none", "decimal": ".",
		"months": []string{"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
		"date":   "{y}年{month}{d}日", "monthYear": "{y}年{month}"},
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	to := fs.String("to", "", "языки через запятую: es,fr,pl")
	workers := fs.Int("workers", 6, "параллельных запросов (для recipes)")
	force := fs.Bool("force", false, "recipes: переводить заново и уже переведённые (после detail)")
	backend := fs.String("backend", "local", "local (прокси подписки ChatGPT на 127.0.0.1:10531) | codex (Codex CLI) | openai (API-ключ) | mistral | gemini | groq | openrouter | custom (AI_BASE_URL, AI_API_KEY)")
	model := fs.String("model", "", "codex: модель (пусто — из ~/.codex/config.toml)")
	effort := fs.String("effort", "low", "codex: model_reasoning_effort: minimal | low | medium")
	only := fs.String("only", "", "notes: только эти id через запятую")
	limit := fs.Int("limit", 0, "notes: не больше N рецептов за прогон (0 — все)")
	_ = fs.Parse(os.Args[2:])
	if *to == "" {
		usage()
	}
	loadDotEnv()
	// -backend codex — через Codex CLI по подписке ChatGPT (CODEX_BIN — путь к codex.exe, иначе из PATH);
	// -backend openai — по OPENAI_API_KEY
	var client *ai.Client
	switch *backend {
	case "codex":
		client = ai.NewCodex(os.Getenv("CODEX_BIN"), *model, *effort)
	case "local": // OpenAI-совместимый прокси подписки (ima2): без ключа, без системного промпта агента — дешевле по квоте, чем codex exec
		base := os.Getenv("LOCAL_AI_URL")
		if base == "" {
			base = "http://127.0.0.1:10531/v1"
		}
		m := *model
		if m == "" {
			m = "gpt-5.5"
		}
		client = ai.NewWithBase(base, "local", m)
	case "openai":
		client = ai.New(os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL"))
		if !client.Enabled() {
			fmt.Fprintln(os.Stderr, "нет OPENAI_API_KEY (в окружении или в ../.env)")
			os.Exit(1)
		}
	default:
		// Бесплатные уровни OpenAI-совместимых провайдеров: ключ и модель по умолчанию — из таблицы,
		// custom — из AI_BASE_URL / AI_API_KEY. Все они лимитируют запросы в секунду: запускайте с -workers 1.
		pre, ok := providers[*backend]
		if !ok {
			fmt.Fprintf(os.Stderr, "неизвестный backend %q\n", *backend)
			os.Exit(2)
		}
		key := os.Getenv(pre.keyEnv)
		if key == "" {
			fmt.Fprintf(os.Stderr, "нет %s (в окружении или в ../.env)\n", pre.keyEnv)
			os.Exit(1)
		}
		base := pre.base
		if base == "" {
			base = os.Getenv("AI_BASE_URL")
		}
		m := *model
		if m == "" {
			m = os.Getenv("AI_MODEL")
		}
		if m == "" {
			m = pre.model
		}
		if base == "" || m == "" {
			fmt.Fprintln(os.Stderr, "для custom нужны AI_BASE_URL и -model (или AI_MODEL)")
			os.Exit(1)
		}
		client = ai.NewWithBase(base, key, m)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	langs := strings.Split(*to, ",")
	var err error
	switch os.Args[1] {
	case "locales":
		err = genLocales(ctx, client, langs)
	case "recipes":
		err = genRecipes(ctx, client, langs, *workers, *force)
	case "detail":
		err = genDetail(ctx, client, *workers)
	case "ingredients":
		err = genIngredients(ctx, client, langs)
	case "notes":
		err = genNotes(ctx, client, langs, *workers, *only, *limit, *force)
	case "collections":
		err = genCollectionText(ctx, client, langs, *only, *force)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

// providers — бесплатные/дешёвые OpenAI-совместимые эндпоинты (сентябрь 2026):
//
//	mistral    — La Plateforme, тариф Experiment: ~1 млрд токенов в месяц бесплатно, 1 запрос/с; все модели.
//	gemini     — Google AI Studio: flash-lite бесплатно до 1000 запросов в день, 15 в минуту.
//	groq       — 30 запросов/мин, но 100 тыс. токенов в день на 70b — только на мелкие прогоны.
//	openrouter — модели с суффиксом :free: 50 запросов в день без покупки кредитов, 1000 после $10.
var providers = map[string]struct{ base, keyEnv, model string }{
	"mistral":    {"https://api.mistral.ai/v1", "MISTRAL_API_KEY", "ministral-14b-latest"}, // medium/small требуют включённого тарифа Experiment
	"gemini":     {"https://generativelanguage.googleapis.com/v1beta/openai", "GEMINI_API_KEY", "gemini-2.5-flash-lite"},
	"groq":       {"https://api.groq.com/openai/v1", "GROQ_API_KEY", "llama-3.3-70b-versatile"},
	"openrouter": {"https://openrouter.ai/api/v1", "OPENROUTER_API_KEY", "google/gemma-4-31b-it:free"},
	"custom":     {"", "AI_API_KEY", ""},
}

func usage() {
	fmt.Fprintln(os.Stderr, "использование: racionai locales|recipes|ingredients -to es,fr,… | racionai detail -to ru")
	os.Exit(2)
}

// genLocales — locales/<code>.json из ru.json (en.json как подсказка). Уже переведённые ключи не трогает.
func genLocales(ctx context.Context, client *ai.Client, langs []string) error {
	ru, en := locales.All["ru"].Strings, locales.All["en"].Strings
	for _, l := range langs {
		meta, ok := langMeta[l]
		if !ok {
			return fmt.Errorf("нет описания языка %q в langMeta — добавьте флаг, страну, месяцы", l)
		}
		path := filepath.Join("locales", l+".json")
		have := map[string]string{}
		if loc, ok := locales.All[l]; ok {
			for k, v := range loc.Strings {
				have[k] = v
			}
		}
		todo := map[string]string{}
		for k, v := range ru {
			if _, done := have[k]; !done {
				todo[k] = v
			}
		}
		fmt.Printf("%s: %d ключей, переводим %d\n", l, len(ru), len(todo))
		got, err := client.TranslateStrings(ctx, l, todo, en, func(done, total int) { fmt.Printf("  %s %d/%d\n", l, done, total) })
		for k, v := range got {
			have[k] = v
		}
		if werr := writeLocale(path, meta, have, ru); werr != nil {
			return werr
		}
		if err != nil {
			return fmt.Errorf("%s: %w (файл сохранён частично, запустите ещё раз)", l, err)
		}
	}
	return nil
}

// writeLocale пишет файл: _meta первым, дальше ключи по алфавиту (только те, что есть в ru.json).
func writeLocale(path string, meta map[string]any, strs, ru map[string]string) error {
	keys := make([]string, 0, len(strs))
	for k := range strs {
		if _, ok := ru[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{\n  \"_meta\": ")
	m, _ := json.MarshalIndent(meta, "  ", "  ")
	b.Write(m)
	for _, k := range keys {
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(strs[k])
		b.WriteString(",\n  " + string(kb) + ": " + string(vb))
	}
	b.WriteString("\n}\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

type recipeFile struct {
	Note  string                   `json:"_note"`
	Items map[string]ai.RecipeText `json:"items"`
}

// genRecipes переводит все рецепты базы (internal/seed/data/recipes*.json) на языки; результат —
// recipes_i18n_<lang>.json в том же формате, что и ручные переводы en/de.
func genRecipes(ctx context.Context, client *ai.Client, langs []string, workers int, force bool) error {
	dir := filepath.Join("internal", "seed", "data")
	src := map[string]ai.RecipeText{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "recipes") || !strings.HasSuffix(n, ".json") || strings.HasPrefix(n, "recipes_i18n") || n == "recipes_detail.json" || n == "recipes_notes.json" {
			continue
		}
		var f struct {
			Items []struct {
				ID          string   `json:"id"`
				Title       string   `json:"title"`
				Description string   `json:"description"`
				Steps       []string `json:"steps"`
			} `json:"items"`
		}
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		for _, r := range f.Items {
			src[r.ID] = ai.RecipeText{Title: r.Title, Description: r.Description, Steps: r.Steps}
		}
	}
	// подробные шаги (racionai detail) — источник перевода, как и в seed
	for id, d := range readRecipeFile(filepath.Join(dir, "recipes_detail.json")).Items {
		if r, ok := src[id]; ok && len(d.Steps) > 0 {
			r.Steps = d.Steps
			if d.Description != "" {
				r.Description = d.Description
			}
			src[id] = r
		}
	}
	en := readRecipeFile(filepath.Join(dir, "recipes_i18n_en.json"))
	ids := make([]string, 0, len(src))
	for id := range src {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, l := range langs {
		path := filepath.Join(dir, "recipes_i18n_"+l+".json")
		out := readRecipeFile(path)
		if out.Items == nil || force {
			out.Items = map[string]ai.RecipeText{}
		}
		out.Note = "Переводы рецептов (" + l + "), сделаны нейросетью через cmd/racionai; правки приветствуются. Ключ — id рецепта."
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, workers)
		done, failed, total := 0, 0, 0
		// готовым считается перевод с шагами: заголовок без шагов (ручная заглушка) переводится заново
		ready := func(id string) bool {
			t, ok := out.Items[id]
			return ok && len(t.Steps) > 0
		}
		for _, id := range ids {
			if !ready(id) {
				total++
			}
		}
		save := func() error {
			raw, _ := json.MarshalIndent(out, "", "  ")
			return os.WriteFile(path, raw, 0o644)
		}
		fmt.Printf("%s: рецептов %d, переводим %d\n", l, len(ids), total)
		for _, id := range ids {
			if ready(id) || ctx.Err() != nil {
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(id string) {
				defer wg.Done()
				defer func() { <-sem }()
				var ref *ai.RecipeText
				if r, ok := en.Items[id]; ok {
					ref = &r
				}
				tx, err := client.TranslateRecipe(ctx, l, src[id], ref)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					failed++
					fmt.Printf("  %s %s: %v\n", l, id, err)
					return
				}
				out.Items[id] = tx
				done++
				if done%20 == 0 {
					fmt.Printf("  %s %d/%d\n", l, done, total)
					_ = save()
				}
			}(id)
		}
		wg.Wait()
		if err := save(); err != nil {
			return err
		}
		fmt.Printf("%s: готово %d, ошибок %d → %s\n", l, done, failed, path)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// genDetail — подробные версии всех рецептов базы по стандарту ai.DetailStandard → internal/seed/data/recipes_detail.json
// (id → title, description, steps). Seed накладывает их поверх коротких, если рецепт не правили из админки.
// Возобновляемый: готовые id пропускаются. После — заново перевести шаги: racionai recipes -to … -force.
func genDetail(ctx context.Context, client *ai.Client, workers int) error {
	dir := filepath.Join("internal", "seed", "data")
	src := map[string]ai.RecipeText{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "recipes") || !strings.HasSuffix(n, ".json") || strings.HasPrefix(n, "recipes_i18n") || n == "recipes_detail.json" || n == "recipes_notes.json" {
			continue
		}
		var f struct {
			Items []struct {
				ID          string   `json:"id"`
				Title       string   `json:"title"`
				Description string   `json:"description"`
				Steps       []string `json:"steps"`
			} `json:"items"`
		}
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		for _, r := range f.Items {
			src[r.ID] = ai.RecipeText{Title: r.Title, Description: r.Description, Steps: r.Steps}
		}
	}
	path := filepath.Join(dir, "recipes_detail.json")
	out := readRecipeFile(path)
	if out.Items == nil {
		out.Items = map[string]ai.RecipeText{}
	}
	out.Note = "Подробные версии шагов по стандарту ai.DetailStandard, сделаны нейросетью через cmd/racionai detail; правки приветствуются."
	ids := make([]string, 0, len(src))
	for id := range src {
		if _, done := out.Items[id]; !done {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	fmt.Printf("detail: рецептов %d, дорабатываем %d\n", len(src), len(ids))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	done, failed := 0, 0
	save := func() error {
		raw, _ := json.MarshalIndent(out, "", "  ")
		return os.WriteFile(path, raw, 0o644)
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			tx, err := client.ImproveRecipe(ctx, "ru", src[id])
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				fmt.Printf("  %s: %v\n", id, err)
				return
			}
			tx.Title = src[id].Title // название базы не меняем: на него завязаны переводы и фото
			out.Items[id] = tx
			done++
			if done%20 == 0 {
				fmt.Printf("  %d/%d\n", done, len(ids))
				_ = save()
			}
		}(id)
	}
	wg.Wait()
	if err := save(); err != nil {
		return err
	}
	fmt.Printf("detail: готово %d, ошибок %d → %s\n", done, failed, path)
	return ctx.Err()
}

func readRecipeFile(path string) recipeFile {
	var f recipeFile
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &f)
	}
	return f
}

// loadDotEnv подхватывает .env / ../.env, если ключа нет в окружении.
func loadDotEnv() {
	if os.Getenv("OPENAI_API_KEY") != "" {
		return
	}
	for _, p := range []string{".env", "../.env"} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if ok && os.Getenv(k) == "" {
				os.Setenv(k, strings.TrimSpace(strings.SplitN(v, " #", 2)[0]))
			}
		}
		f.Close()
		return
	}
}

// genIngredients переводит названия продуктов (internal/seed/data/ingredients_i18n.json: id → {lang: name});
// русский берётся из ingredients.json, английский — как опора. Возобновляемый: готовые языки у продукта пропускаются.
func genIngredients(ctx context.Context, client *ai.Client, langs []string) error {
	dir := filepath.Join("internal", "seed", "data")
	raw, err := os.ReadFile(filepath.Join(dir, "ingredients.json"))
	if err != nil {
		return err
	}
	var base struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		return err
	}
	path := filepath.Join(dir, "ingredients_i18n.json")
	raw, err = os.ReadFile(path)
	if err != nil {
		return err
	}
	var file struct {
		Note  string                    `json:"_note"`
		Items map[string]map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	if file.Items == nil {
		file.Items = map[string]map[string]any{}
	}
	save := func() error {
		out, _ := json.MarshalIndent(file, "", "  ")
		return os.WriteFile(path, out, 0o644)
	}
	for _, l := range langs {
		src, ref := map[string]string{}, map[string]string{}
		for _, it := range base.Items {
			cur := file.Items[it.ID]
			if v, ok := cur[l].(string); ok && v != "" {
				continue
			}
			src[it.ID] = it.Name
			if en, ok := cur["en"].(string); ok {
				ref[it.ID] = en
			}
		}
		fmt.Printf("%s: продуктов %d, переводим %d\n", l, len(base.Items), len(src))
		got, terr := client.TranslateStrings(ctx, l, src, ref, func(done, total int) { fmt.Printf("  %s %d/%d\n", l, done, total) })
		for id, v := range got {
			if file.Items[id] == nil {
				file.Items[id] = map[string]any{}
			}
			file.Items[id][l] = v
		}
		if err := save(); err != nil {
			return err
		}
		if terr != nil {
			return fmt.Errorf("%s: %w (файл сохранён частично, запустите ещё раз)", l, terr)
		}
	}
	return nil
}

// genNotes — заметки к рецептам («Советы» на странице) на языках -to → internal/seed/data/recipes_notes.json
// {items: {id: {lang: Notes}}}. Пишутся на каждом языке заново, не переводом. Возобновляемый: готовые
// пары id+язык пропускаются (-force — переписать). Стиль — plain-prose, правила зашиты в промпт.
func genNotes(ctx context.Context, client *ai.Client, langs []string, workers int, only string, limit int, force bool) error {
	dir := filepath.Join("internal", "seed", "data")
	type rec struct {
		ID          string              `json:"id"`
		Title       string              `json:"title"`
		Description string              `json:"description"`
		Slot        string              `json:"slot"`
		Tags        []string            `json:"tags"`
		Steps       []string            `json:"steps"`
		Ingredients [][]json.RawMessage `json:"ingredients"`
	}
	var ingNames map[string]string
	{
		var f struct {
			Items []struct {
				ID, Name string
			} `json:"items"`
		}
		raw, err := os.ReadFile(filepath.Join(dir, "ingredients.json"))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			return err
		}
		ingNames = map[string]string{}
		for _, i := range f.Items {
			ingNames[i.ID] = i.Name
		}
	}
	src := map[string]rec{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "recipes") || !strings.HasSuffix(n, ".json") || strings.HasPrefix(n, "recipes_i18n") || n == "recipes_detail.json" || n == "recipes_notes.json" {
			continue
		}
		var f struct {
			Items []rec `json:"items"`
		}
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		for _, r := range f.Items {
			src[r.ID] = r
		}
	}
	// подробные шаги важнее коротких
	for id, d := range readRecipeFile(filepath.Join(dir, "recipes_detail.json")).Items {
		if r, ok := src[id]; ok && len(d.Steps) > 0 {
			r.Steps = d.Steps
			if d.Description != "" {
				r.Description = d.Description
			}
			src[id] = r
		}
	}
	// английские тексты — для английских заметок вход даём на английском, если есть
	en := readRecipeFile(filepath.Join(dir, "recipes_i18n_en.json")).Items
	path := filepath.Join(dir, "recipes_notes.json")
	var out struct {
		Note  string                         `json:"_note"`
		Items map[string]map[string]ai.Notes `json:"items"`
	}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	if out.Items == nil {
		out.Items = map[string]map[string]ai.Notes{}
	}
	out.Note = "Заметки к рецептам по языкам (раздел «Советы»): why / swaps / mistakes / keep / serve. Сделаны нейросетью через cmd/racionai notes по правилам plain-prose; правки приветствуются."
	onlySet := map[string]bool{}
	for _, id := range strings.Split(only, ",") {
		if id = strings.TrimSpace(id); id != "" {
			onlySet[id] = true
		}
	}
	type job struct{ id, lang string }
	var jobs []job
	ids := make([]string, 0, len(src))
	for id := range src {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if len(onlySet) > 0 && !onlySet[id] {
			continue
		}
		for _, l := range langs {
			if _, done := out.Items[id][l]; done && !force {
				continue
			}
			jobs = append(jobs, job{id, l})
		}
	}
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}
	fmt.Printf("notes: рецептов %d, задач %d\n", len(src), len(jobs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	done, failed := 0, 0
	save := func() error {
		raw, _ := json.MarshalIndent(out, "", "  ")
		return os.WriteFile(path, raw, 0o644)
	}
	for _, j := range jobs {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			r := src[j.id]
			in := ai.NotesInput{RecipeText: ai.RecipeText{Title: r.Title, Description: r.Description, Steps: r.Steps}, Slot: r.Slot, Tags: r.Tags}
			if j.lang == "en" {
				if t, ok := en[j.id]; ok && t.Title != "" {
					in.RecipeText = t
				}
			}
			for _, pair := range r.Ingredients {
				var id string
				var amt float64
				_ = json.Unmarshal(pair[0], &id)
				_ = json.Unmarshal(pair[1], &amt)
				name := ingNames[id]
				if name == "" {
					name = id
				}
				in.Ingredients = append(in.Ingredients, fmt.Sprintf("%s — %g", name, amt))
			}
			n, err := client.RecipeNotes(ctx, j.lang, in)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				fmt.Printf("  %s/%s: %v\n", j.id, j.lang, err)
				return
			}
			if out.Items[j.id] == nil {
				out.Items[j.id] = map[string]ai.Notes{}
			}
			out.Items[j.id][j.lang] = n
			done++
			if done%10 == 0 {
				_ = save()
				fmt.Printf("  %d/%d\n", done, len(jobs))
			}
		}(j)
	}
	wg.Wait()
	if err := save(); err != nil {
		return err
	}
	fmt.Printf("notes: готово %d, ошибок %d → %s\n", done, failed, path)
	return nil
}

// genCollectionText — редакционный текст страниц подборок (вступление, как пользоваться, вопросы-ответы)
// на языках -to → поле seo в internal/seed/data/collections.json. Ккал и цена порции считаются грубо
// по ingredients.json (RU), чтобы у модели были цифры; на странице они всё равно берутся из каталога.
func genCollectionText(ctx context.Context, client *ai.Client, langs []string, only string, force bool) error {
	dir := filepath.Join("internal", "seed", "data")
	type ing struct {
		ID    string  `json:"id"`
		Pack  float64 `json:"pack"`
		Price float64 `json:"price"`
		Kcal  float64 `json:"kcal"`
	}
	var ingf struct {
		Items []ing `json:"items"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "ingredients.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &ingf); err != nil {
		return err
	}
	ings := map[string]ing{}
	for _, i := range ingf.Items {
		ings[i.ID] = i
	}
	type rec struct {
		ID          string              `json:"id"`
		Title       string              `json:"title"`
		Slot        string              `json:"slot"`
		Time        int                 `json:"time"`
		Ingredients [][]json.RawMessage `json:"ingredients"`
	}
	recs := map[string]rec{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "recipes") || !strings.HasSuffix(n, ".json") || strings.HasPrefix(n, "recipes_i18n") || n == "recipes_detail.json" || n == "recipes_notes.json" {
			continue
		}
		var f struct {
			Items []rec `json:"items"`
		}
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		for _, r := range f.Items {
			recs[r.ID] = r
		}
	}
	en := readRecipeFile(filepath.Join(dir, "recipes_i18n_en.json")).Items
	slotNames := map[string]map[string]string{
		"ru": {"breakfast": "завтрак", "lunch": "обед", "dinner": "ужин", "snack": "перекус"},
		"en": {"breakfast": "breakfast", "lunch": "lunch", "dinner": "dinner", "snack": "snack"},
	}
	describe := func(r rec, lang string) string {
		var kcal, cost float64
		for _, pair := range r.Ingredients {
			var id string
			var amt float64
			_ = json.Unmarshal(pair[0], &id)
			_ = json.Unmarshal(pair[1], &amt)
			if i, ok := ings[id]; ok {
				kcal += i.Kcal * amt / 100
				if i.Pack > 0 {
					cost += i.Price * amt / i.Pack
				}
			}
		}
		title := r.Title
		if lang == "en" {
			if t, ok := en[r.ID]; ok && t.Title != "" {
				title = t.Title
			}
		}
		sn := slotNames[lang]
		if sn == nil {
			sn = slotNames["en"]
		}
		return fmt.Sprintf("%s — %s, %d min, %.0f kcal, %.0f RUB", title, sn[r.Slot], r.Time, kcal, cost)
	}
	path := filepath.Join(dir, "collections.json")
	raw, err = os.ReadFile(path)
	if err != nil {
		return err
	}
	var cols []map[string]any
	if err := json.Unmarshal(raw, &cols); err != nil {
		return err
	}
	onlySet := map[string]bool{}
	for _, id := range strings.Split(only, ",") {
		if id = strings.TrimSpace(id); id != "" {
			onlySet[id] = true
		}
	}
	done, failed := 0, 0
	for _, c := range cols {
		slug, _ := c["slug"].(string)
		if len(onlySet) > 0 && !onlySet[slug] {
			continue
		}
		seo, _ := c["seo"].(map[string]any)
		if seo == nil {
			seo = map[string]any{}
		}
		names, _ := c["names"].(map[string]any)
		descs, _ := c["descriptions"].(map[string]any)
		ids, _ := c["recipes"].([]any)
		for _, lang := range langs {
			if _, ok := seo[lang]; ok && !force {
				continue
			}
			in := ai.CollectionInput{Name: fmt.Sprint(c["name"]), Description: fmt.Sprint(c["description"]), Country: "RU", Button: i18n.T(i18n.Lang(lang), "coll.week")}
			if lang != "ru" {
				if n, ok := names[lang].(string); ok && n != "" {
					in.Name = n
				}
				if d, ok := descs[lang].(string); ok && d != "" {
					in.Description = d
				}
			}
			for _, id := range ids {
				if r, ok := recs[fmt.Sprint(id)]; ok {
					in.Recipes = append(in.Recipes, describe(r, lang))
				}
			}
			t, err := client.CollectionText(ctx, lang, in)
			if err != nil {
				failed++
				fmt.Printf("  %s/%s: %v\n", slug, lang, err)
				continue
			}
			seo[lang] = t
			done++
			fmt.Printf("  %s/%s\n", slug, lang)
		}
		c["seo"] = seo
	}
	out, _ := json.MarshalIndent(cols, "", " ")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}
	fmt.Printf("collections: готово %d, ошибок %d → %s\n", done, failed, path)
	return nil
}
