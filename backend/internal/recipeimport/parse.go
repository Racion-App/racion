package recipeimport

import (
	"encoding/json"
	"html"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Recipe — то, что удалось прочитать со страницы, как есть: строки ингредиентов ещё не сопоставлены с базой.
type Recipe struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
	Portions    int      `json:"portions"` // 0 — сайт не сказал
	TimeMin     int      `json:"timeMin"`  // 0 — сайт не сказал
	Category    []string `json:"category"` // recipeCategory, кухня и ключевые слова — по ним угадывается приём пищи
	Lang        string   `json:"lang"`     // язык страницы: <html lang> или inLanguage
}

// Complete — хватает ли прочитанного на рецепт: название, продукты и хотя бы один шаг.
func (r Recipe) Complete() bool {
	return r.Title != "" && len(r.Ingredients) > 0 && len(r.Steps) > 0
}

// Doc — разобранная страница: из неё берутся и разметка рецепта, и видимый текст для нейросети.
type Doc struct {
	root *xhtml.Node
	lang string
}

func ParseDoc(b []byte) (*Doc, error) {
	root, err := xhtml.Parse(strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	d := &Doc{root: root}
	if h := find(root, func(n *xhtml.Node) bool { return n.DataAtom == atom.Html }); h != nil {
		d.lang = langCode(attr(h, "lang"))
	}
	return d, nil
}

// Lang — язык страницы из <html lang>: «ru-RU» → «ru».
func (d *Doc) Lang() string { return d.lang }

// Canonical — адрес страницы, который называет сам сайт (<link rel="canonical"> или og:url). Пусто — не назвал.
// Он чище того, куда привели переадресации: зеркала, метки рекламы, мобильные поддомены.
func (d *Doc) Canonical() string {
	var out string
	walk(d.root, func(n *xhtml.Node) bool {
		if out != "" {
			return false
		}
		switch {
		case n.DataAtom == atom.Link && strings.EqualFold(attr(n, "rel"), "canonical"):
			out = attr(n, "href")
		case n.DataAtom == atom.Meta && attr(n, "property") == "og:url":
			out = attr(n, "content")
		}
		return n.DataAtom != atom.Body // канонический адрес живёт в <head>
	})
	u, err := url.Parse(strings.TrimSpace(out))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

// Recipe ищет рецепт в разметке: сначала JSON-LD, потом microdata. Если в JSON-LD нет шагов (так бывает),
// шаги берутся из microdata той же страницы.
func (d *Doc) Recipe() Recipe {
	r := d.jsonLD()
	if !r.Complete() {
		m := d.microdata()
		switch {
		case r.Title == "" && len(r.Ingredients) == 0:
			r = m
		case len(r.Ingredients) == 0:
			r.Ingredients = m.Ingredients
		}
		if len(r.Steps) == 0 {
			r.Steps = m.Steps
		}
	}
	if r.Lang == "" {
		r.Lang = d.lang
	}
	r.normalize()
	return r
}

// ── JSON-LD ────────────────────────────────────────────────────────────────

func (d *Doc) jsonLD() Recipe {
	var out Recipe
	walk(d.root, func(n *xhtml.Node) bool {
		if out.Title != "" || len(out.Ingredients) > 0 {
			return false // первый найденный рецепт и есть рецепт страницы
		}
		if n.DataAtom != atom.Script || !strings.Contains(strings.ToLower(attr(n, "type")), "ld+json") {
			return true
		}
		var v any
		raw := text(n)
		if json.Unmarshal([]byte(raw), &v) != nil && json.Unmarshal(lenientJSON(raw), &v) != nil {
			return false
		}
		if m := findRecipe(v, 0); m != nil {
			out = fromLD(m)
		}
		return false
	})
	return out
}

// findRecipe — объект с @type Recipe где угодно внутри: в @graph, mainEntity, массивах.
func findRecipe(v any, depth int) map[string]any {
	if depth > 8 {
		return nil
	}
	switch x := v.(type) {
	case map[string]any:
		if isType(x["@type"], "Recipe") {
			return x
		}
		for _, k := range []string{"@graph", "mainEntity", "mainEntityOfPage", "itemListElement", "item", "hasPart"} {
			if m := findRecipe(x[k], depth+1); m != nil {
				return m
			}
		}
	case []any:
		for _, e := range x {
			if m := findRecipe(e, depth+1); m != nil {
				return m
			}
		}
	}
	return nil
}

func isType(v any, want string) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(strings.TrimPrefix(strings.TrimPrefix(t, "http://schema.org/"), "https://schema.org/"), want)
	case []any:
		for _, e := range t {
			if isType(e, want) {
				return true
			}
		}
	}
	return false
}

func fromLD(m map[string]any) Recipe {
	r := Recipe{
		Title:       clean(str(m["name"])),
		Description: clean(str(m["description"])),
		Portions:    portions(m["recipeYield"]),
		Lang:        langCode(str(m["inLanguage"])),
	}
	ings := m["recipeIngredient"]
	if ings == nil {
		ings = m["ingredients"]
	}
	for _, s := range strs(ings) {
		r.Ingredients = append(r.Ingredients, clean(s))
	}
	r.Steps = instructions(m["recipeInstructions"], 0)
	r.TimeMin = minutes(str(m["totalTime"]))
	if r.TimeMin == 0 {
		r.TimeMin = minutes(str(m["prepTime"])) + minutes(str(m["cookTime"]))
	}
	for _, k := range []string{"recipeCategory", "recipeCuisine", "keywords"} {
		for _, s := range strs(m[k]) {
			for _, part := range strings.Split(s, ",") {
				if part = strings.TrimSpace(clean(part)); part != "" {
					r.Category = append(r.Category, part)
				}
			}
		}
	}
	return r
}

// instructions разворачивает recipeInstructions: строку, список строк, HowToStep, HowToSection, ItemList.
func instructions(v any, depth int) []string {
	if depth > 4 {
		return nil
	}
	switch x := v.(type) {
	case string:
		return splitSteps(x)
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, instructions(e, depth+1)...)
		}
		return out
	case map[string]any:
		if el, ok := x["itemListElement"]; ok {
			return instructions(el, depth+1) // раздел: его название — не шаг
		}
		if t := str(x["text"]); t != "" {
			return splitSteps(t)
		}
		if t := str(x["name"]); t != "" {
			return splitSteps(t)
		}
	}
	return nil
}

var reBlockTag = regexp.MustCompile(`(?i)<\s*(br|/p|/li|/div|/h\d)\s*/?>`)

// splitSteps — шаг из JSON-LD бывает HTML с абзацами: каждый абзац — свой шаг.
func splitSteps(s string) []string {
	s = reBlockTag.ReplaceAllString(s, "\n")
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = clean(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// ── Microdata ──────────────────────────────────────────────────────────────

func (d *Doc) microdata() Recipe {
	scope := find(d.root, func(n *xhtml.Node) bool {
		return strings.Contains(strings.ToLower(attr(n, "itemtype")), "schema.org/recipe")
	})
	if scope == nil {
		return Recipe{}
	}
	var r Recipe
	var steps []*xhtml.Node
	walk(scope, func(n *xhtml.Node) bool {
		props := strings.Fields(attr(n, "itemprop"))
		if n != scope && hasAttr(n, "itemscope") && !slices.Contains(props, "recipeInstructions") && !slices.Contains(props, "recipeIngredient") {
			return false // вложенный объект — автор, отзыв, пищевая ценность — не наш
		}
		for _, p := range props {
			switch p {
			case "name":
				if r.Title == "" && n.Parent != nil && within(n, scope) {
					r.Title = clean(propValue(n))
				}
			case "description":
				if r.Description == "" {
					r.Description = clean(propValue(n))
				}
			case "recipeIngredient", "ingredients":
				if s := clean(propValue(n)); s != "" {
					r.Ingredients = append(r.Ingredients, s)
				}
			case "recipeInstructions":
				steps = append(steps, n)
				return false // шаги внутри (HowToStep) уже в тексте блока
			case "recipeYield":
				if r.Portions == 0 {
					r.Portions = portions(propValue(n))
				}
			case "totalTime":
				r.TimeMin = minutes(propValue(n))
			case "cookTime", "prepTime":
				if r.TimeMin == 0 {
					r.TimeMin += minutes(propValue(n))
				}
			case "recipeCategory", "recipeCuisine":
				if s := clean(propValue(n)); s != "" {
					r.Category = append(r.Category, s)
				}
			}
		}
		return true
	})
	for _, n := range steps {
		r.Steps = append(r.Steps, blockLines(n)...)
	}
	return r
}

// within — name самого рецепта, а не ингредиента или шага: ближайший предок с itemscope — сам рецепт.
func within(n, scope *xhtml.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p == scope {
			return true
		}
		if hasAttr(p, "itemscope") {
			return false
		}
	}
	return false
}

func propValue(n *xhtml.Node) string {
	switch n.DataAtom {
	case atom.Meta:
		return attr(n, "content")
	case atom.Time:
		if v := attr(n, "datetime"); v != "" {
			return v
		}
	case atom.Img:
		return attr(n, "alt")
	}
	if v := attr(n, "content"); v != "" {
		return v
	}
	return text(n)
}

// blockLines — текст блока по абзацам и пунктам списка.
func blockLines(n *xhtml.Node) []string {
	var b strings.Builder
	var rec func(*xhtml.Node)
	rec = func(c *xhtml.Node) {
		if c.Type == xhtml.TextNode {
			b.WriteString(c.Data)
			return
		}
		if c.Type == xhtml.ElementNode && skipTag(c) {
			return
		}
		for k := c.FirstChild; k != nil; k = k.NextSibling {
			rec(k)
		}
		if c.Type == xhtml.ElementNode && block(c) {
			b.WriteString("\n")
		}
	}
	rec(n)
	var out []string
	for _, line := range strings.Split(b.String(), "\n") {
		if line = clean(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// ── Видимый текст для нейросети ────────────────────────────────────────────

// Text — видимый текст страницы без меню, подвала и скриптов, не длиннее limit символов. Если на странице
// есть <article> или <main>, берётся он: там рецепт, а не соседние ссылки.
func (d *Doc) Text(limit int) string {
	// статей на странице бывает несколько (карточки «похожие рецепты»): берём самую длинную
	var body *xhtml.Node
	best := 0
	walk(d.root, func(n *xhtml.Node) bool {
		if n.DataAtom != atom.Article {
			return true
		}
		if l := len(text(n)); l > best {
			body, best = n, l
		}
		return false
	})
	if best < 400 {
		body = nil
	}
	if body == nil {
		body = find(d.root, func(n *xhtml.Node) bool { return n.DataAtom == atom.Main })
	}
	if body == nil {
		body = find(d.root, func(n *xhtml.Node) bool { return n.DataAtom == atom.Body })
	}
	if body == nil {
		return ""
	}
	title := ""
	if t := find(d.root, func(n *xhtml.Node) bool { return n.DataAtom == atom.H1 }); t != nil {
		title = clean(text(t))
	}
	lines := blockLines(body)
	if title != "" && (len(lines) == 0 || lines[0] != title) {
		lines = append([]string{title}, lines...)
	}
	s := strings.Join(lines, "\n")
	if utf8.RuneCountInString(s) > limit {
		s = string([]rune(s)[:limit])
	}
	return s
}

func skipTag(n *xhtml.Node) bool {
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Noscript, atom.Svg, atom.Nav, atom.Header, atom.Footer, atom.Aside,
		atom.Iframe, atom.Template, atom.Button, atom.Select, atom.Head:
		return true
	}
	return false
}

func block(n *xhtml.Node) bool {
	switch n.DataAtom {
	case atom.P, atom.Div, atom.Li, atom.Br, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Tr,
		atom.Section, atom.Article, atom.Ul, atom.Ol, atom.Table, atom.Dd, atom.Dt, atom.Blockquote, atom.Figure:
		return true
	}
	return false
}

// ── Приведение ─────────────────────────────────────────────────────────────

const (
	maxSteps    = 20
	maxStepLen  = 500
	maxTitle    = 80
	maxDesc     = 300
	maxIngLines = 60
)

// normalize подгоняет прочитанное под форму своего рецепта: длины, число шагов, без заголовков разделов.
func (r *Recipe) normalize() {
	r.Title = cutWords(r.Title, maxTitle)
	r.Description = cutSentence(r.Description, maxDesc)
	ings := r.Ingredients[:0:0]
	seen := map[string]bool{}
	for _, s := range r.Ingredients {
		s = strings.TrimSpace(s)
		// «Для соуса:» — заголовок раздела, а не продукт
		if s == "" || strings.HasSuffix(s, ":") || seen[strings.ToLower(s)] || utf8.RuneCountInString(s) > 200 {
			continue
		}
		seen[strings.ToLower(s)] = true
		ings = append(ings, s)
	}
	if len(ings) > maxIngLines {
		ings = ings[:maxIngLines]
	}
	r.Ingredients = ings
	var steps []string
	for _, s := range r.Steps {
		s = reStepNo.ReplaceAllString(strings.TrimSpace(s), "")
		if s == "" {
			continue
		}
		steps = append(steps, splitLong(s, maxStepLen)...)
	}
	r.Steps = fitSteps(steps, maxSteps, maxStepLen)
	if r.Portions < 0 || r.Portions > 50 {
		r.Portions = 0
	}
	if r.TimeMin < 0 || r.TimeMin > 600 {
		r.TimeMin = 0
	}
}

var reStepNo = regexp.MustCompile(`^(?i)(?:шаг|step|schritt|krok|paso|étape|passo|adım)?\s*\d{1,2}\s*[.):](?:\s+|$)`)

// splitLong режет длинный шаг по предложениям, чтобы каждая часть влезла в limit.
func splitLong(s string, limit int) []string {
	if utf8.RuneCountInString(s) <= limit {
		return []string{s}
	}
	var out []string
	cur := ""
	for _, sent := range sentences(s) {
		if cur != "" && utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(sent) > limit {
			out = append(out, cur)
			cur = ""
		}
		if cur == "" {
			cur = sent
		} else {
			cur += " " + sent
		}
		for utf8.RuneCountInString(cur) > limit {
			out = append(out, cutWords(cur, limit))
			cur = strings.TrimSpace(string([]rune(cur)[utf8.RuneCountInString(cutWords(cur, limit)):]))
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func sentences(s string) []string {
	var out []string
	start := 0
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if (rs[i] == '.' || rs[i] == '!' || rs[i] == '?') && i+1 < len(rs) && rs[i+1] == ' ' {
			out = append(out, strings.TrimSpace(string(rs[start:i+1])))
			start = i + 1
		}
	}
	if rest := strings.TrimSpace(string(rs[start:])); rest != "" {
		out = append(out, rest)
	}
	return out
}

// fitSteps — больше max шагов форма не примет: склеиваем самые короткие соседние, пока влезают в limit.
func fitSteps(steps []string, max, limit int) []string {
	for len(steps) > max {
		best, bestLen := -1, math.MaxInt
		for i := 0; i+1 < len(steps); i++ {
			l := utf8.RuneCountInString(steps[i]) + 1 + utf8.RuneCountInString(steps[i+1])
			if l <= limit && l < bestLen {
				best, bestLen = i, l
			}
		}
		if best < 0 {
			return steps[:max]
		}
		steps = append(steps[:best], append([]string{steps[best] + " " + steps[best+1]}, steps[best+2:]...)...)
	}
	return steps
}

func cutWords(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	rs := []rune(s)[:n]
	if i := strings.LastIndexAny(string(rs), " ,;"); i > len(string(rs))/2 {
		return strings.TrimSpace(string(rs)[:i])
	}
	return strings.TrimSpace(string(rs))
}

// cutSentence — подводка до limit: целыми предложениями, если получается.
func cutSentence(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	out := ""
	for _, sent := range sentences(s) {
		if utf8.RuneCountInString(out)+1+utf8.RuneCountInString(sent) > n {
			break
		}
		out = strings.TrimSpace(out + " " + sent)
	}
	if out == "" {
		return cutWords(s, n)
	}
	return out
}

var (
	reTags  = regexp.MustCompile(`<[^>]*>`)
	reSpace = regexp.MustCompile(`\s+`)
)

// clean — строка из разметки: без тегов, сущностей и лишних пробелов. Сущности бывают закодированы дважды.
func clean(s string) string {
	s = html.UnescapeString(html.UnescapeString(s))
	s = reTags.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		if len(x) > 0 {
			return str(x[0])
		}
	case map[string]any:
		if s := str(x["@value"]); s != "" {
			return s
		}
		return str(x["name"])
	}
	return ""
}

func strs(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, e := range x {
			if s := str(e); s != "" {
				out = append(out, s)
			}
		}
		return out
	case map[string]any:
		if s := str(x); s != "" {
			return []string{s}
		}
	}
	return nil
}

var reInt = regexp.MustCompile(`\d+`)

// portions — recipeYield: 4, "4", "4 порции", ["4", "4 servings"]. Первое целое.
func portions(v any) int {
	for _, s := range strs(v) {
		if m := reInt.FindString(s); m != "" {
			n, _ := strconv.Atoi(m)
			if n > 0 && n <= 50 {
				return n
			}
		}
	}
	if f, ok := v.(float64); ok && f > 0 && f <= 50 {
		return int(f)
	}
	return 0
}

var (
	reISO     = regexp.MustCompile(`(?i)^P(?:(\d+)D)?(?:T(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)
	reHours   = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:ч|час|h|hr|hour|std|godz|saat|heure|ora|hora|uur)`)
	reMinutes = regexp.MustCompile(`(?i)(\d+)\s*(?:мин|m|min|минут|perc|dk|分)`)
)

// minutes — время из ISO 8601 (PT1H30M) или из текста («1 ч 30 мин»).
func minutes(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if m := reISO.FindStringSubmatch(s); m != nil {
		f := func(i int) float64 { v, _ := strconv.ParseFloat(m[i], 64); return v }
		total := f(1)*24*60 + f(2)*60 + f(3) + f(4)/60
		return int(math.Round(total))
	}
	total := 0.0
	if m := reHours.FindStringSubmatch(s); m != nil {
		v, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		total += v * 60
	}
	if m := reMinutes.FindStringSubmatch(s); m != nil {
		v, _ := strconv.Atoi(m[1])
		total += float64(v)
	}
	return int(math.Round(total))
}

func langCode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "-_"); i > 0 {
		s = s[:i]
	}
	if len(s) != 2 {
		return ""
	}
	return s
}

// lenientJSON чинит то, что встречается в JSON-LD сайтов: сырые переводы строк внутри строк и висячие запятые.
func lenientJSON(s string) []byte {
	var b strings.Builder
	in, esc := false, false
	for _, c := range s {
		switch {
		case esc:
			esc = false
		case in && c == '\\':
			esc = true
		case c == '"':
			in = !in
		case in && c == '\n':
			b.WriteString(`\n`)
			continue
		case in && c == '\r':
			continue
		case in && c == '\t':
			b.WriteString(`\t`)
			continue
		}
		b.WriteRune(c)
	}
	return reTrailingComma.ReplaceAll([]byte(b.String()), []byte("$1"))
}

var reTrailingComma = regexp.MustCompile(`,\s*([}\]])`)

// ── Дерево ─────────────────────────────────────────────────────────────────

// walk обходит дерево; f возвращает false — детей узла не смотреть.
func walk(n *xhtml.Node, f func(*xhtml.Node) bool) {
	if n.Type == xhtml.ElementNode && !f(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}

func find(n *xhtml.Node, ok func(*xhtml.Node) bool) *xhtml.Node {
	var out *xhtml.Node
	walk(n, func(c *xhtml.Node) bool {
		if out != nil {
			return false
		}
		if ok(c) {
			out = c
			return false
		}
		return true
	})
	return out
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *xhtml.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func text(n *xhtml.Node) string {
	var b strings.Builder
	var rec func(*xhtml.Node)
	rec = func(c *xhtml.Node) {
		if c.Type == xhtml.TextNode {
			b.WriteString(c.Data)
		}
		for k := c.FirstChild; k != nil; k = k.NextSibling {
			rec(k)
		}
	}
	rec(n)
	return b.String()
}
