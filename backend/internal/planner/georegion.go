package planner

import (
	"strings"
	"unicode"
)

// RegionByPlace подбирает регион или город Росстата по английским названиям из базы DB-IP
// («Sverdlovsk», «Yekaterinburg»). Сначала ищем город: его цены точнее. Английское имя сравниваем
// с транслитерацией русского названия; для известных экзонимов есть таблица. Пусто — не нашли.
func RegionByPlace(regions []Region, subdivision, city string) string {
	if len(regions) == 0 {
		return ""
	}
	regionCode := ""
	if sub := stem(norm(exonym(subdivision))); sub != "" {
		for _, rg := range regions {
			if rg.Kind != "region" {
				continue
			}
			own := stem(norm(translit(rg.Name)))
			if own == sub || (len(sub) >= 5 && strings.HasPrefix(own, sub)) {
				regionCode = rg.Code
				break
			}
		}
	}
	if c := norm(exonym(city)); c != "" {
		for _, rg := range regions {
			if rg.Kind != "city" && rg.Kind != "region" {
				continue
			}
			// Москва и Санкт-Петербург в списке Росстата — регионы, а не города
			if norm(translit(rg.Name)) == c && (regionCode == "" || rg.Parent == regionCode || rg.Kind == "region") {
				return rg.Code
			}
		}
	}
	return regionCode
}

// exonym — английские названия, которые не получаются транслитерацией.
var exonyms = map[string]string{
	"moscow": "moskva", "moscow oblast": "moskovskaya", "saint petersburg": "sanktpeterburg", "st petersburg": "sanktpeterburg",
	"sevastopol": "sevastopol", "crimea": "krym", "republic of crimea": "krym", "yekaterinburg": "ekaterinburg",
	"nizhniy novgorod": "nizhniynovgorod", "nizhny novgorod": "nizhniynovgorod", "rostov-on-don": "rostovnadonu", "rostov": "rostovskaya",
	"naberezhnyye chelny": "naberezhnyechelny", "north ossetia": "severnayaosetiya", "north ossetia-alania": "severnayaosetiya",
	"kabardino-balkaria": "kabardinobalkarskaya", "karachay-cherkessia": "karachaevocherkesskaya", "chechnya": "chechenskaya",
	"chuvashia": "chuvashskaya", "udmurtia": "udmurtskaya", "bashkortostan": "bashkortostan", "tatarstan": "tatarstan",
	"mari el": "mariyel", "mordovia": "mordoviya", "sakha": "sakha", "yakutia": "sakha", "tyva": "tyva", "tuva": "tyva",
	"khakassia": "khakasiya", "buryatia": "buryatiya", "kalmykia": "kalmykiya", "adygea": "adygeya", "karelia": "kareliya",
	"komi": "komi", "dagestan": "dagestan", "ingushetia": "ingushetiya", "altai": "altay", "altai republic": "altay",
	"altai krai": "altayskiy", "krasnoyarsk krai": "krasnoyarskiy", "primorsky krai": "primorskiy", "primorye": "primorskiy",
	"khabarovsk krai": "khabarovskiy", "kamchatka": "kamchatskiy", "kamchatka krai": "kamchatskiy", "perm krai": "permskiy",
	"stavropol krai": "stavropolskiy", "krasnodar krai": "krasnodarskiy", "zabaykalsky krai": "zabaykalskiy", "transbaikal": "zabaykalskiy",
	"jewish autonomous oblast": "evreyskaya", "chukotka": "chukotskiy", "khanty-mansia": "khantymansiyskiy", "yamalo-nenets": "yamalonenetskiy",
	"nenets": "nenetskiy", "kemerovo": "kemerovskaya", "kuzbass": "kemerovskaya", "oryol": "orlovskaya", "orel": "orlovskaya",
	"tver": "tverskaya", "yaroslavl": "yaroslavskaya", "vladimir": "vladimirskaya", "ivanovo": "ivanovskaya", "kostroma": "kostromskaya",
	"tula": "tulskaya", "kaluga": "kaluzhskaya", "smolensk": "smolenskaya", "bryansk": "bryanskaya", "kursk": "kurskaya",
	"belgorod": "belgorodskaya", "lipetsk": "lipetskaya", "tambov": "tambovskaya", "ryazan": "ryazanskaya", "voronezh": "voronezhskaya",
	"arkhangelsk": "arkhangelskaya", "vologda": "vologodskaya", "kaliningrad": "kaliningradskaya", "leningrad": "leningradskaya",
	"murmansk": "murmanskaya", "novgorod": "novgorodskaya", "pskov": "pskovskaya", "astrakhan": "astrakhanskaya", "volgograd": "volgogradskaya",
	"kirov": "kirovskaya", "orenburg": "orenburgskaya", "penza": "penzenskaya", "samara": "samarskaya", "saratov": "saratovskaya",
	"ulyanovsk": "ulyanovskaya", "kurgan": "kurganskaya", "sverdlovsk": "sverdlovskaya", "tyumen": "tyumenskaya", "chelyabinsk": "chelyabinskaya",
	"irkutsk": "irkutskaya", "novosibirsk": "novosibirskaya", "omsk": "omskaya", "tomsk": "tomskaya", "amur": "amurskaya",
	"magadan": "magadanskaya", "sakhalin": "sakhalinskaya", "nizhny novgorod oblast": "nizhegorodskaya", "nizhniy novgorod oblast": "nizhegorodskaya",
}

func exonym(s string) string {
	k := strings.ToLower(strings.TrimSpace(s))
	k = strings.NewReplacer("’", "", "'", "", "ʹ", "").Replace(k)
	if v, ok := exonyms[k]; ok {
		return v
	}
	for _, suf := range []string{" oblast", " region", " krai", " republic", " autonomous okrug", " autonomous oblast", " city"} {
		if strings.HasSuffix(k, suf) {
			if v, ok := exonyms[strings.TrimSuffix(k, suf)]; ok {
				return v
			}
			return strings.TrimSuffix(k, suf)
		}
	}
	k = strings.TrimPrefix(k, "republic of ")
	if v, ok := exonyms[k]; ok {
		return v
	}
	return k
}

// stem — основа для сравнения регионов: «sverdlovskaya» → «sverdlov», чтобы совпасть с любой формой прилагательного.
func stem(s string) string {
	for _, suf := range []string{"skaya", "skiy", "sky", "skii", "ckaya", "aya", "iya", "iy"} {
		if strings.HasSuffix(s, suf) && len(s) > len(suf)+3 {
			return strings.TrimSuffix(s, suf)
		}
	}
	return s
}

// norm — только латинские буквы в нижнем регистре: без пробелов, дефисов, апострофов и диакритики.
func norm(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case unicode.Is(unicode.Latin, r):
			// é → e и т. п.: оставляем базовую букву по таблице ниже
			if v, ok := latinBase[r]; ok {
				b.WriteRune(v)
			}
		}
	}
	return b.String()
}

var latinBase = map[rune]rune{'é': 'e', 'ё': 'e', 'á': 'a', 'í': 'i', 'ó': 'o', 'ú': 'u', 'ý': 'y', 'ä': 'a', 'ö': 'o', 'ü': 'u'}

// translit — русское название латиницей в том виде, в каком его чаще пишет DB-IP (BGN/PCGN без апострофов):
// «Свердловская область» → «sverdlovskaya» (служебные слова убираем).
func translit(s string) string {
	s = strings.ToLower(s)
	// служебные слова убираем целиком, а не как подстроки: в «Новгород» тоже есть «город»
	var words []string
	for _, w := range strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '-' || r == '(' || r == ')' }) {
		switch w {
		case "республика", "область", "край", "автономный", "автономная", "округ", "город", "федерального", "значения", "кузбасс", "чувашия", "адыгея", "алания":
			continue
		}
		words = append(words, w)
	}
	s = strings.Join(words, " ")
	var b strings.Builder
	for _, r := range s {
		if v, ok := cyr[r]; ok {
			b.WriteString(v)
		} else if r == ' ' || r == '-' {
			b.WriteRune(' ')
		}
	}
	return strings.TrimSpace(b.String())
}

var cyr = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z", 'и': "i", 'й': "y",
	'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f",
	'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}
