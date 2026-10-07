package recipeimport

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const ldPage = `<!doctype html><html lang="ru-RU"><head>
<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebSite","name":"Кулинария"}</script>
<script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"BreadcrumbList"},
{"@type":["Recipe","NewsArticle"],"name":"Сырники &amp; сметана","description":"<p>Нежные сырники на завтрак.</p>",
"recipeYield":["4","4 порции"],"totalTime":"PT35M","recipeCategory":"Завтраки",
"recipeIngredient":["Творог 5% — 500 г","Яйцо — 2 шт.","Мука — 3 ст. л.","Для подачи:","Сметана 15% — 200 г","Соль по вкусу"],
"recipeInstructions":[{"@type":"HowToSection","name":"Тесто","itemListElement":[
  {"@type":"HowToStep","text":"1. Разомните творог вилкой."},
  {"@type":"HowToStep","text":"Добавьте яйца и муку.<br>Перемешайте."}]},
 {"@type":"HowToStep","text":"Обжарьте на сковороде по 3 минуты с каждой стороны."}]}]}</script>
<link rel="canonical" href="https://eda.ru/recepty/syrniki-1"></head><body><h1>Сырники</h1></body></html>`

func TestJSONLD(t *testing.T) {
	d, err := ParseDoc([]byte(ldPage))
	if err != nil {
		t.Fatal(err)
	}
	r := d.Recipe()
	if r.Title != "Сырники & сметана" || r.Description != "Нежные сырники на завтрак." {
		t.Fatalf("title/description: %q / %q", r.Title, r.Description)
	}
	if d.Canonical() != "https://eda.ru/recepty/syrniki-1" {
		t.Fatalf("canonical: %q", d.Canonical())
	}
	if r.Portions != 4 || r.TimeMin != 35 || r.Lang != "ru" {
		t.Fatalf("portions %d, time %d, lang %q", r.Portions, r.TimeMin, r.Lang)
	}
	if len(r.Ingredients) != 5 || r.Ingredients[3] != "Сметана 15% — 200 г" {
		t.Fatalf("ingredients: %q", r.Ingredients) // «Для подачи:» — заголовок раздела, выброшен
	}
	want := []string{"Разомните творог вилкой.", "Добавьте яйца и муку.", "Перемешайте.", "Обжарьте на сковороде по 3 минуты с каждой стороны."}
	if strings.Join(r.Steps, "|") != strings.Join(want, "|") {
		t.Fatalf("steps: %q", r.Steps)
	}
}

func TestLenientJSON(t *testing.T) {
	page := "<script type=\"application/ld+json\">{\"@type\":\"Recipe\",\"name\":\"Борщ\",\"recipeIngredient\":[\"Свёкла 300 г\",],\n\"recipeInstructions\":\"Сварите бульон.\nДобавьте свёклу.\"}</script>"
	d, _ := ParseDoc([]byte(page))
	r := d.Recipe()
	if r.Title != "Борщ" || len(r.Ingredients) != 1 || len(r.Steps) != 2 {
		t.Fatalf("got %+v", r)
	}
}

const microPage = `<html lang="en"><body><div itemscope itemtype="http://schema.org/Recipe">
<h1 itemprop="name">Pancakes</h1>
<div itemprop="author" itemscope itemtype="http://schema.org/Person"><span itemprop="name">Ann</span></div>
<meta itemprop="recipeYield" content="2 servings"><time itemprop="totalTime" datetime="PT1H5M">1 h 5 min</time>
<ul><li itemprop="recipeIngredient">1 ½ cups flour</li><li itemprop="recipeIngredient">2 eggs</li></ul>
<ol itemprop="recipeInstructions"><li>Mix everything.</li><li>Fry in a pan.</li></ol>
</div></body></html>`

func TestMicrodata(t *testing.T) {
	d, _ := ParseDoc([]byte(microPage))
	r := d.Recipe()
	if r.Title != "Pancakes" || r.Portions != 2 || r.TimeMin != 65 || r.Lang != "en" {
		t.Fatalf("got %+v", r)
	}
	if len(r.Ingredients) != 2 || strings.Join(r.Steps, "|") != "Mix everything.|Fry in a pan." {
		t.Fatalf("got %q / %q", r.Ingredients, r.Steps)
	}
}

func TestText(t *testing.T) {
	page := `<html><body><nav>Меню сайта</nav><article><h1>Рецепт</h1><p>Короткая карточка</p></article>
<article><h2>Плов</h2><p>` + strings.Repeat("Рис промыть. ", 50) + `</p><script>var x=1</script></article><footer>© сайт</footer></body></html>`
	d, _ := ParseDoc([]byte(page))
	s := d.Text(1000)
	if strings.Contains(s, "Меню сайта") || strings.Contains(s, "var x") || strings.Contains(s, "© сайт") || !strings.Contains(s, "Плов") {
		t.Fatalf("text: %q", s)
	}
	if !strings.HasPrefix(s, "Рецепт\n") { // заголовок страницы идёт первым
		t.Fatalf("no title: %q", s[:40])
	}
}

func TestParseLine(t *testing.T) {
	cases := []struct {
		in   string
		qty  float64
		unit string
		name string
	}{
		{"Творог 5% — 500 г", 500, "g", "Творог 5%"},
		{"Мука — 3 ст. л.", 3, "tbsp", "Мука"},
		{"2 ст.л. сахара", 2, "tbsp", "сахара"},
		{"Яйцо — 2 шт.", 2, "pcs", "Яйцо"},
		{"3 яйца", 3, "", "яйца"},
		{"1 ½ cups flour", 1.5, "cup", "flour"},
		{"½ стакана молока", 0.5, "cup", "молока"},
		{"Чеснок 2-3 зубчика", 2.5, "clove", "Чеснок"},
		{"Молоко 2,5% — 0,5 л", 0.5, "l", "Молоко 2,5%"},
		{"1 кг картофеля", 1, "kg", "картофеля"},
		{"2 груши", 2, "", "груши"},
		{"Куриное филе (грудка) 400 гр", 400, "g", "Куриное филе"},
		{"Пшеничная мука, 6 столовых ложек", 6, "tbsp", "Пшеничная мука"},
		{"Ванилин, 2 чайных ложки", 2, "tsp", "Ванилин"},
		{"Сахар (по вкусу) — 15 г", 15, "g", "Сахар"},
	}
	for _, c := range cases {
		l := ParseLine(c.in)
		if math.Abs(l.Qty-c.qty) > 1e-9 || l.Unit != c.unit || l.Name != c.name {
			t.Errorf("%q → %v %q %q, want %v %q %q", c.in, l.Qty, l.Unit, l.Name, c.qty, c.unit, c.name)
		}
	}
	if l := ParseLine("Соль по вкусу"); !l.Taste || l.Name != "Соль" {
		t.Errorf("taste: %+v", l)
	}
}

var testProducts = []Product{
	{ID: "cottage_cheese", Names: []string{"Творог 5%", "Cottage cheese 5%"}, Unit: "g"},
	{ID: "cottage_cheese_9", Names: []string{"Творог 9%"}, Unit: "g"},
	{ID: "eggs", Names: []string{"Яйца куриные", "Eggs"}, Unit: "pcs"},
	{ID: "flour", Names: []string{"Мука пшеничная", "Wheat flour"}, Unit: "g", Pantry: true},
	{ID: "sour_cream", Names: []string{"Сметана 15%"}, Unit: "g"},
	{ID: "salt", Names: []string{"Соль", "Salt"}, Unit: "g", Pantry: true},
	{ID: "sunflower_oil", Names: []string{"Масло подсолнечное", "Sunflower oil"}, Unit: "ml", Pantry: true},
	{ID: "butter", Names: []string{"Масло сливочное", "Butter"}, Unit: "g"},
	{ID: "onion", Names: []string{"Лук репчатый", "Onion"}, Unit: "g"},
	{ID: "tomato", Names: []string{"Помидоры", "Tomatoes"}, Unit: "g"},
	{ID: "tomato_paste", Names: []string{"Томатная паста", "Tomato paste"}, Unit: "g"},
}

func TestMatchRules(t *testing.T) {
	lines := []string{"Творог 5% — 500 г", "Яйцо — 2 шт.", "Мука — 3 ст. л.", "Сметана 15% — 200 г", "Соль по вкусу",
		"Растительное масло 2 ст. л.", "2 луковицы", "Томаты черри 200 г", "Вода — 200 мл", "Сливочное масло 20 г", "Мука для обсыпки 40 г"}
	items, unmatched := MatchRules(lines, 4, testProducts)
	got := map[string]Item{}
	for _, it := range items {
		got[it.ID] = it
	}
	want := map[string]float64{"cottage_cheese": 125, "eggs": 0.5, "flour": 17.5, "sour_cream": 50, "salt": 2, "sunflower_oil": 7.5, "onion": 50, "tomato": 50, "butter": 5}
	for id, amt := range want {
		if got[id].Amount != amt {
			t.Errorf("%s: %v, want %v (%+v)", id, got[id].Amount, amt, got[id])
		}
	}
	if len(items) != len(want) {
		t.Errorf("items: %+v", items)
	}
	if len(unmatched) != 0 { // вода не покупается и в «не нашли» не попадает
		t.Errorf("unmatched: %q", unmatched)
	}
	if !strings.Contains(got["flour"].Line, "обсыпки") {
		t.Errorf("flour lines are not merged: %q", got["flour"].Line)
	}
}

type fakeAI struct {
	reply string
	err   error
	user  string
}

func (f *fakeAI) JSON(_ context.Context, _, user string, out any) error {
	f.user = user
	if f.err != nil {
		return f.err
	}
	return json.Unmarshal([]byte(f.reply), out)
}

func TestMatchAI(t *testing.T) {
	ai := &fakeAI{reply: `{"items":[{"i":0,"id":"cottage_cheese","total":500},{"i":1,"id":"eggs","total":3},{"i":2,"id":"invented","total":10},{"i":3,"id":null,"total":null},{"i":4,"id":"salt","total":90000},{"i":5,"id":"salt","total":null}]}`}
	lines := []string{"творог 500 г", "3 яйца", "что-то", "шафран", "соль 90 кг", "соль по вкусу", "Сливочное масло — 40 г", "Сыр пармезан 50 г"}
	items, unmatched, err := MatchAI(context.Background(), ai, lines, 4, testProducts)
	if err != nil {
		t.Fatal(err)
	}
	// на порцию делит сервер: 500 г / 4, 3 яйца / 4 → 0,75; «по вкусу» — 2 г соли на порцию
	// масло нейросеть пропустила — правила нашли его уверенно; «сыр пармезан» — неуверенно, остаётся человеку
	if len(items) != 4 || items[0].Amount != 125 || items[1].Amount != 0.75 || items[2].ID != "salt" || items[2].Amount != 2 || items[3].ID != "butter" || items[3].Amount != 10 {
		t.Fatalf("items: %+v", items)
	}
	if strings.Join(unmatched, "|") != "что-то|шафран|соль 90 кг|Сыр пармезан 50 г" {
		t.Fatalf("unmatched: %q", unmatched)
	}
	if !strings.Contains(ai.user, `cottage_cheese|`) || strings.Contains(ai.user, `portions`) {
		t.Fatalf("prompt: %s", ai.user)
	}
}

func TestParseURL(t *testing.T) {
	ok := []string{"https://eda.ru/recepty/1", "eda.ru/recepty/1", "http://www.example.com:443/x"}
	for _, s := range ok {
		if _, err := ParseURL(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	bad := []string{"", "ftp://eda.ru/x", "javascript:alert(1)", "http://localhost/", "http://127.0.0.1/", "http://10.0.0.5/x",
		"http://[::1]/", "http://169.254.169.254/latest/meta-data", "http://backend:8080/api", "http://minio/x", "https://eda.ru:8443/x",
		"http://user:pass@eda.ru/", "http://192.168.1.1/", "http://100.64.1.1/", "http://foo.internal/"}
	for _, s := range bad {
		if _, err := ParseURL(s); err == nil {
			t.Errorf("%s: accepted", s)
		}
	}
	for _, a := range []string{"127.0.0.1:80", "10.1.2.3:443", "[::1]:443", "[::ffff:192.168.0.1]:80", "93.184.216.34:8080", "[fd00::1]:443"} {
		if checkDial(a) == nil {
			t.Errorf("dial %s allowed", a)
		}
	}
	if checkDial("93.184.216.34:443") != nil {
		t.Error("public address blocked")
	}
}

func TestFetchCharsetAndType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cp1251":
			w.Header().Set("Content-Type", "text/html; charset=windows-1251")
			_, _ = w.Write([]byte{'<', 'p', '>', 0xd1, 0xee, 0xeb, 0xfc, '<', '/', 'p', '>'}) // «Соль»
		case "/img":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		case "/move":
			http.Redirect(w, r, "/cp1251", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	f := newFetcher(true)
	// httptest слушает 127.0.0.1 — ParseURL такой адрес не пустит, поэтому идём мимо него, как после проверки
	get := func(path string) (Page, error) { return f.fetchURL(context.Background(), srv.URL+path) }
	p, err := get("/move")
	if err != nil || !strings.Contains(string(p.HTML), "Соль") || !strings.HasSuffix(p.URL, "/cp1251") {
		t.Fatalf("cp1251: %v %q %s", err, p.HTML, p.URL)
	}
	if _, err := get("/img"); !errors.Is(err, ErrNotHTML) {
		t.Fatalf("img: %v", err)
	}
	if _, err := get("/404"); !errors.Is(err, ErrFetch) {
		t.Fatalf("404: %v", err)
	}
	if _, err := NewFetcher().Fetch(context.Background(), srv.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("local fetch: %v", err)
	}
}
