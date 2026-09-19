// Package locales — файлы переводов. Один JSON на язык: backend/locales/<code>.json.
//
// Формат файла: плоский словарь «ключ → строка» плюс служебный объект "_meta":
//
//	"_meta": {
//	  "name": "Español",          // название на самом языке (в списке языков)
//	  "english": "Spanish",
//	  "flag": "es",               // код флага (ISO 3166-1 alpha-2, как в flag-icons)
//	  "country": "ES",            // страна по умолчанию для этого языка
//	  "plural": "one-other",      // правило множественного числа: one-other | east-slavic | polish | czech | none
//	  "decimal": ",",             // десятичный разделитель
//	  "months": ["enero", …],     // названия месяцев (12)
//	  "monthsGen": [...],         // необязательно: месяцы в родительном падеже для дат
//	  "date": "{d} de {month} de {y}"  // формат «14 сентября 2026»
//	}
//
// Плейсхолдеры: в строках интерфейса — именованные {name}; в строках сервера — по порядку {0}, {1}.
// Ключи с суффиксами .one / .few / .many — формы слова по числу. Чего нет в файле — берётся из ru.json.
// Чтобы добавить язык, достаточно положить рядом новый файл: сервер, приложение и страницы подхватят его.
package locales

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.json
var files embed.FS

type Meta struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	English   string   `json:"english"`
	Flag      string   `json:"flag"`
	Country   string   `json:"country"`
	Plural    string   `json:"plural"`
	Decimal   string   `json:"decimal"`
	Months    []string `json:"months"`
	MonthsGen []string `json:"monthsGen,omitempty"`
	Date      string   `json:"date"`
	MonthYear string   `json:"monthYear,omitempty"` // «{month} {y}» по умолчанию; zh/ja — «{y}年{month}»
	Keys      int      `json:"keys"`                // сколько строк переведено (для списка языков)
	Hash      string   `json:"hash"`                // версия файла (хеш содержимого): приложение кэширует словарь по ней
}

type Locale struct {
	Meta    Meta
	Strings map[string]string
	Raw     []byte // исходный JSON для отдачи приложению
}

// All — все языки по коду; Order — коды в порядке показа (ru, en, затем по алфавиту английского названия).
var (
	All   = map[string]*Locale{}
	Order []string
)

func init() {
	entries, _ := fs.ReadDir(files, ".")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		code := strings.TrimSuffix(e.Name(), ".json")
		raw, _ := files.ReadFile(e.Name())
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			panic("locales: " + e.Name() + ": " + err.Error())
		}
		loc := &Locale{Strings: map[string]string{}, Raw: raw}
		for k, v := range m {
			if k == "_meta" {
				_ = json.Unmarshal(v, &loc.Meta)
				continue
			}
			var s string
			if json.Unmarshal(v, &s) == nil {
				loc.Strings[k] = s
			}
		}
		loc.Meta.Code = code
		loc.Meta.Keys = len(loc.Strings)
		sum := sha256.Sum256(raw)
		loc.Meta.Hash = hex.EncodeToString(sum[:4])
		if loc.Meta.Plural == "" {
			loc.Meta.Plural = "one-other"
		}
		All[code] = loc
	}
	for code := range All {
		Order = append(Order, code)
	}
	sort.Slice(Order, func(i, j int) bool {
		rank := func(c string) int {
			switch c {
			case "ru":
				return 0
			case "en":
				return 1
			}
			return 2
		}
		if rank(Order[i]) != rank(Order[j]) {
			return rank(Order[i]) < rank(Order[j])
		}
		return All[Order[i]].Meta.English < All[Order[j]].Meta.English
	})
}
