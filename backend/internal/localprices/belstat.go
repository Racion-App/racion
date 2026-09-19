package localprices

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shakinm/xlsReader/xls"
)

// Belstat — Национальный статистический комитет Беларуси: «Средние цены на товары, реализуемые
// в розничной сети» помесячно, xls, рубли за кг/л/десяток, страна и области. Берём столбец «Республика Беларусь».
type Belstat struct{}

func (Belstat) Country() string { return "BY" }
func (Belstat) Key() string     { return "price.belstat" }

const belstatBase = "https://www.belstat.gov.by/upload-belstat/upload-belstat-excel/Oficial_statistika/"

// belstatURLs — имена файлов за месяц: январь и декабрь идут в коротком формате (-2601, -2512), остальные -MM-YYYY.
func belstatURLs(t time.Time) []string {
	mm := fmt.Sprintf("%02d", int(t.Month()))
	yy := fmt.Sprintf("%02d", t.Year()%100)
	return []string{
		belstatBase + "Average_prices-" + mm + "-" + t.Format("2006") + ".xls",
		belstatBase + "Average_prices-" + yy + mm + ".xls",
	}
}

// belstatMap — начало названия строки → продукты; единица источника: кг/л (1000) или как указано в скобках.
var belstatMap = []struct {
	Prefix  string
	PerUnit float64
	Items   []itemFactor
}{
	{"Рис шлифованный", 1000, items("rice", 1, "rice_round", 1)},
	{"Мука пшеничная", 1000, items("flour", 1)},
	{"Крупа манная", 1000, items("semolina", 1)},
	{"Крупа гречневая", 1000, items("buckwheat", 1)},
	{"Хлопья овсяные", 1000, items("oats", 1, "oat_bran", 1)},
	{"Крупа перловая", 1000, items("pearl_barley", 1)},
	{"Крупа пшенная", 1000, items("millet", 1)},
	{"Хлопья кукурузные", 1000, items("corn_flakes", 1)},
	{"Горох, фасоль", 1000, items("peas_dry", 1, "lentils", 1.3, "lentils_green", 1.3)},
	{"Хлеб, изделия булочные (батон) из муки", 1000, items("bread_white", 1)},
	{"Хлеб ржаной", 1000, items("bread_rye", 1, "bread_wholegrain", 1.3)},
	{"Макароны, рожки", 1000, items("pasta", 1)},
	{"Говядина бескостная", 1000, items("beef_stew", 1)},
	{"Свинина бескостная", 1000, items("pork_neck", 1, "pork_loin", 1)},
	{"Фарш мясной", 1000, items("ground_mixed", 1, "ground_beef", 1.15)},
	{"Куры (цыплята", 1000, items("chicken_whole", 1, "chicken_thigh", 1.25, "chicken_drumstick", 1.15, "chicken_breast", 1.6, "chicken_wings", 1.1, "ground_chicken", 1.2)},
	{"Пельмени мясные", 1000, items("dumplings", 1)},
	{"Колбаса вареная высшего", 1000, items("sausage_boiled", 1)},
	{"Сосиски, сардельки", 1000, items("sausages", 1)},
	{"Рыба мороженая (филе", 1000, items("pollock_fillet", 1, "hake_fillet", 1.1, "cod_fillet", 1.5, "pink_salmon", 1.2)},
	{"Сельдь соленая", 1000, items("herring", 1)},
	{"Крабовые палочки", 1000, items("crab_sticks", 1)},
	{"Консервы рыбные натуральные", 1000, items("saury_can", 1, "tuna_can", 1.6)},
	{"Молоко цельное", 1000, items("milk", 1)},
	{"Кефир", 1000, items("kefir", 1)},
	{"Сметана", 1000, items("sour_cream", 1, "sour_cream_20", 1.1)},
	{"Творог 4% жирности и выше", 1000, items("cottage_cheese_9", 1)},
	{"Творог до 4%", 1000, items("cottage_cheese", 1)},
	{"Сыр рассольный", 1000, items("cheese_feta", 1, "cheese_suluguni", 1.1)},
	{"Сыр твердый", 1000, items("cheese_hard", 1, "cheese_cheddar", 1.2)},
	{"Сыр мягкий", 1000, items("mozzarella", 1, "cream_cheese", 1)},
	{"Йогурт (за 250 г)", 250, items("yogurt_plain", 1)},
	{"Сливки питьевые", 1000, items("cream", 1)},
	{"Ряженка", 1000, items("ryazhenka", 1)},
	{"Молоко сгущенное", 400, items("condensed_milk", 1)},
	{"Масло сливочное", 1000, items("butter", 1)},
	{"Масло растительное", 1000, items("sunflower_oil", 1)},
	{"Масло оливковое", 1000, items("olive_oil", 1)},
	{"Яйца куриные", 10, items("eggs", 1)},
	{"Яблоки", 1000, items("apple", 1)},
	{"Плоды цитрусовые", 1000, items("orange", 1, "lemon", 1.2)},
	{"Груши", 1000, items("pear", 1)},
	{"Бананы", 1000, items("banana", 1)},
	{"Сухофрукты", 1000, items("raisins", 0.8, "dried_apricots", 1.2, "dates", 1)},
	{"Ягоды", 1000, items("berries_frozen", 1)},
	{"Орехи", 1000, items("walnuts", 1, "peanuts", 0.5, "sunflower_seeds", 0.5)},
	{"Картофель", 1000, items("potato", 1)},
	{"Капуста белокочанная", 1000, items("cabbage", 1)},
	{"Лук репчатый", 1000, items("onion", 1)},
	{"Свекла", 1000, items("beet", 1)},
	{"Морковь", 1000, items("carrot", 1)},
	{"Огурцы свежие", 1000, items("cucumber", 1)},
	{"Помидоры свежие", 1000, items("tomato", 1)},
	{"Чеснок", 1000, items("garlic", 1)},
	{"Перец сладкий", 1000, items("bell_pepper", 1)},
	{"Грибы свежие", 1000, items("mushrooms", 1)},
	{"Авокадо", 1000, items("avocado", 160)}, // штучный товар: множитель — граммов в штуке
	{"Прочие зеленые культуры", 1000, items("dill", 1, "parsley", 1, "green_onion", 1, "cilantro", 1, "basil", 1.5, "mint", 1.5)},
	{"Овощи соленые", 1000, items("pickles", 1, "sauerkraut", 1, "pickles_gherkin", 1.3)},
	{"Консервы томатные", 1000, items("tomato_paste", 1, "tomatoes_can", 0.5)},
	{"Овощи замороженные", 1000, items("broccoli", 1, "cauliflower", 1, "green_beans", 1, "mixed_veg", 1, "spinach_frozen", 1.1, "peas_frozen", 1, "brussels", 1.2)},
	{"Овощи консервированные", 1000, items("corn_can", 1, "green_peas_can", 1, "beans_can", 1, "chickpeas_can", 1.2)},
	{"Сахар", 1000, items("sugar", 1)},
	{"Чай черный", 1000, items("tea", 1)},
	{"Мед натуральный", 1000, items("honey", 1)},
	{"Какао-порошок", 200, items("cocoa", 1)},
}

type itemFactor struct {
	ID     string
	Factor float64
}

func items(kv ...any) []itemFactor {
	out := make([]itemFactor, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, itemFactor{kv[i].(string), toF(kv[i+1])})
	}
	return out
}

func toF(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case float64:
		return x
	}
	return 1
}

func (b Belstat) Fetch(ctx context.Context, packs map[string]PackInfo) (*Result, error) {
	now := time.Now()
	var lastErr error
	for back := 0; back < 4; back++ {
		t := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -back, 0)
		for _, u := range belstatURLs(t) {
			data, err := get(ctx, u, "")
			if err != nil {
				lastErr = err
				continue
			}
			r, err := parseBelstat(data, packs)
			if err != nil {
				lastErr = err
				continue
			}
			r.Period = t
			return r, nil
		}
	}
	return nil, fmt.Errorf("belstat: no monthly file found: %w", lastErr)
}

func parseBelstat(data []byte, packs map[string]PackInfo) (*Result, error) {
	wb, err := xls.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("belstat xls: %w", err)
	}
	sheet, err := wb.GetSheet(0)
	if err != nil {
		return nil, err
	}
	r := &Result{Pack: map[string]float64{}}
	used := map[string]bool{}
	for i := 0; i <= sheet.GetNumberRows(); i++ {
		row, err := sheet.GetRow(i)
		if err != nil {
			continue
		}
		nameCell, err := row.GetCol(0)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(nameCell.GetString())
		if name == "" {
			continue
		}
		for _, m := range belstatMap {
			if used[m.Prefix] || !strings.HasPrefix(name, m.Prefix) {
				continue
			}
			priceCell, err := row.GetCol(1)
			if err != nil {
				break
			}
			v, ok := parseNum(priceCell.GetString())
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
		return nil, fmt.Errorf("belstat: no known rows")
	}
	return r, nil
}

func parseNum(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, ",", "."), " ", ""))
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}
