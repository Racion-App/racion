package localprices

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// KazStat — Бюро национальной статистики Казахстана: еженедельный xlsx «Индекс цен и средние цены
// на социально-значимые продовольственные товары», лист 5 — средние цены за кг/л/десяток, столбец «Республика Казахстан».
// Ссылка на файл меняется каждую неделю, поэтому сначала читаем листинг электронных таблиц.
type KazStat struct{}

func (KazStat) Country() string { return "KZ" }
func (KazStat) Key() string     { return "price.kazstat" }

const kazList = "https://stat.gov.kz/ru/industries/economy/prices/spreadsheets/"

var kazHref = regexp.MustCompile(`/api/iblock/element/(\d+)/file/ru/`)
var kazDate = regexp.MustCompile(`на (\d{1,2}) (\S+) (\d{4})`)

var kazMonths = map[string]time.Month{"января": 1, "февраля": 2, "марта": 3, "апреля": 4, "мая": 5, "июня": 6, "июля": 7, "августа": 8, "сентября": 9, "октября": 10, "ноября": 11, "декабря": 12}

var kazMap = []struct {
	Prefix  string
	PerUnit float64
	Items   []itemFactor
}{
	{"Рис шлифованный", 1000, items("rice", 1, "rice_round", 1)},
	{"Крупа гречневая", 1000, items("buckwheat", 1)},
	{"Мука пшеничная", 1000, items("flour", 1)},
	{"Хлеб пшеничный", 1000, items("bread_white", 1, "bread_wholegrain", 1.4)},
	{"Рожки", 1000, items("pasta", 1)},
	{"Говядина бескостная", 1000, items("beef_stew", 1)},
	{"Баранина", 1000, items("lamb", 1)},
	{"Куры", 1000, items("chicken_whole", 1, "chicken_breast", 1.5, "chicken_wings", 1.1)},
	{"Мясо кур (бедро", 1000, items("chicken_thigh", 1, "chicken_drumstick", 0.95)},
	{"Мясной фарш", 1000, items("ground_mixed", 1, "ground_beef", 1.1, "ground_chicken", 0.6)},
	{"Рыба свежая", 1000, items("pollock_fillet", 1.2, "hake_fillet", 1.3)},
	{"Молоко", 1000, items("milk", 1)},
	{"Сыр твердый", 1000, items("cheese_hard", 1, "cheese_cheddar", 1.2)},
	{"Творог", 1000, items("cottage_cheese", 1, "cottage_cheese_9", 1.1)},
	{"Сметана", 1000, items("sour_cream", 1, "sour_cream_20", 1.1)},
	{"Кефир", 1000, items("kefir", 1)},
	{"Яйца", 10, items("eggs", 1)},
	{"Масло подсолнечное", 1000, items("sunflower_oil", 1)},
	{"Масло сливочное", 1000, items("butter", 1)},
	{"Яблоки", 1000, items("apple", 1)},
	{"Капуста белокочанная", 1000, items("cabbage", 1)},
	{"Огурцы", 1000, items("cucumber", 1)},
	{"Помидоры", 1000, items("tomato", 1)},
	{"Лук репчатый", 1000, items("onion", 1)},
	{"Морковь", 1000, items("carrot", 1)},
	{"Картофель", 1000, items("potato", 1)},
	{"Сахар", 1000, items("sugar", 1)},
	{"Соль", 1000, items("salt", 1)},
	{"Чай черный", 1000, items("tea", 1)},
}

func (k KazStat) Fetch(ctx context.Context, packs map[string]PackInfo) (*Result, error) {
	html, err := get(ctx, kazList, "text/html")
	if err != nil {
		return nil, err
	}
	id, period := "", time.Time{}
	// Строка таблицы: <div class="divTableRow" id="…"><a href="/api/iblock/element/ID/file/ru/">…название…</a>.
	// Ищем название и ближайшую ссылку перед ним (в пределах строки); заголовки фильтра ссылок не имеют.
	page := string(html)
	const phrase = "социально-значимые продовольственные"
	for from := 0; ; {
		i := strings.Index(page[from:], phrase)
		if i < 0 {
			break
		}
		i += from
		from = i + len(phrase)
		lo := i - 2500
		if lo < 0 {
			lo = 0
		}
		hrefs := kazHref.FindAllStringSubmatchIndex(page[lo:i], -1)
		if len(hrefs) == 0 {
			continue
		}
		last := hrefs[len(hrefs)-1]
		id = page[lo+last[2] : lo+last[3]]
		tail := page[i:min(i+200, len(page))]
		if d := kazDate.FindStringSubmatch(tail); d != nil {
			var day, year int
			fmt.Sscanf(d[1], "%d", &day)
			fmt.Sscanf(d[3], "%d", &year)
			if mo, ok := kazMonths[strings.ToLower(d[2])]; ok {
				period = time.Date(year, mo, day, 0, 0, 0, 0, time.UTC)
			}
		}
		break // листинг идёт от новых к старым
	}
	if id == "" {
		return nil, fmt.Errorf("kazstat: file link not found in listing")
	}
	data, err := get(ctx, "https://stat.gov.kz/api/iblock/element/"+id+"/file/ru/", "")
	if err != nil {
		return nil, err
	}
	r, err := parseKaz(data, packs)
	if err != nil {
		return nil, err
	}
	if period.IsZero() {
		period = time.Now().UTC()
	}
	r.Period = period
	return r, nil
}

func parseKaz(data []byte, packs map[string]PackInfo) (*Result, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("kazstat xlsx: %w", err)
	}
	defer f.Close()
	// лист со средними ценами: заголовок начинается с «5. Средние цены»
	var rows [][]string
	for _, name := range f.GetSheetList() {
		rs, err := f.GetRows(name)
		if err != nil || len(rs) == 0 {
			continue
		}
		if len(rs[0]) > 0 && strings.HasPrefix(strings.TrimSpace(rs[0][0]), "5.") && strings.Contains(rs[0][0], "Средние цены") {
			rows = rs
			break
		}
	}
	if rows == nil {
		return nil, fmt.Errorf("kazstat: sheet with average prices not found")
	}
	r := &Result{Pack: map[string]float64{}}
	used := map[string]bool{}
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		name := strings.TrimSpace(row[0])
		for _, m := range kazMap {
			if used[m.Prefix] || !strings.HasPrefix(name, m.Prefix) {
				continue
			}
			v, ok := parseNum(row[1])
			if !ok || v <= 0 {
				break
			}
			used[m.Prefix] = true
			for _, it := range m.Items {
				if p, ok := packs[it.ID]; ok {
					r.Pack[it.ID] = packPrice(v, m.PerUnit, p, it.Factor)
				}
			}
			break
		}
	}
	if len(r.Pack) == 0 {
		return nil, fmt.Errorf("kazstat: no known rows")
	}
	return r, nil
}
