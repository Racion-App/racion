package mail

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strconv"
	"strings"

	"racion/internal/i18n"
)

// Письма по шаблону: общий макет templates/layout.html и тело на каждый вид (reset.html, welcome.html).
// Макет — «табло»: главное число письма крупно и моноширинно на серой плашке, под ним одна синяя
// кнопка. Почтовые клиенты понимают только таблицы и инлайн-стили, поэтому вёрстка табличная, шрифты
// системные, картинки по абсолютным адресам сайта. Текстовая версия идёт рядом (multipart/alternative).

//go:embed templates/*.html
var templatesFS embed.FS

// Letter — письмо: вид, язык и то, что в него подставляется.
type Letter struct {
	Kind    string // "reset" — смена пароля, "welcome" — после регистрации
	Lang    i18n.Lang
	BaseURL string // https://racion.app: отсюда картинки письма
	Link    string // адрес главной кнопки
	Board   Board  // табло: главное число и подпись под ним
	Week    bool   // приветствие: человек зарегистрировался со своей недели, кнопка ведёт к ней
	Minutes int    // сброс: сколько минут работает ссылка
}

// Board — табло письма: число крупно и подпись под ним.
type Board struct{ Value, Caption string }

// view — всё, что видит шаблон: строки уже на языке письма.
type view struct {
	Lang, Subject, Preheader, Brand, Title, Lead, Button, Link, LinkText, Fallback, Note, Footer, Icon string
	Board                                                                                              Board
}

// Условные комментарии Outlook: html/template вырезает любые комментарии, поэтому они идут из функций
// как готовый HTML. Outlook для Windows рисует письмо движком Word: ему нужна таблица фиксированной
// ширины вместо max-width и кнопка из VML вместо ссылки с padding.
var funcs = template.FuncMap{
	"msoHead": func() template.HTML {
		return `<!--[if mso]><noscript><xml><o:OfficeDocumentSettings><o:PixelsPerInch>96</o:PixelsPerInch></o:OfficeDocumentSettings></xml></noscript><![endif]-->`
	},
	"msoOpen": func() template.HTML {
		return `<!--[if mso]><table role="presentation" width="560" align="center" cellpadding="0" cellspacing="0" border="0"><tr><td><![endif]-->`
	},
	"msoClose":    func() template.HTML { return `<!--[if mso]></td></tr></table><![endif]-->` },
	"notMso":      func() template.HTML { return `<!--[if !mso]><!-->` },
	"notMsoClose": func() template.HTML { return `<!--<![endif]-->` },
	"msoButton": func(href, text string) template.HTML {
		return template.HTML(`<!--[if mso]><v:roundrect xmlns:v="urn:schemas-microsoft-com:vml" xmlns:w="urn:schemas-microsoft-com:office:word" href="` +
			html.EscapeString(href) + `" style="height:47px;v-text-anchor:middle;width:512px;" arcsize="26%" stroke="f" fillcolor="` + ButtonBlue +
			`"><w:anchorlock/><center style="color:#FFFFFF;font-family:'Segoe UI',Arial,sans-serif;font-size:17px;font-weight:600;">` +
			html.EscapeString(text) + `</center></v:roundrect><![endif]-->`)
	},
}

// ButtonBlue — синий кнопки письма: #007AFF дизайн-системы с белым текстом даёт 4,0:1, ниже нормы для
// текста 17 px; этот темнее на шаг и даёт 4,7:1 в обеих темах.
const ButtonBlue = "#0071E3"

var tpls = map[string]*template.Template{}

func init() {
	for _, kind := range []string{"reset", "welcome"} {
		tpls[kind] = template.Must(template.New("").Funcs(funcs).ParseFS(templatesFS, "templates/layout.html", "templates/"+kind+".html"))
	}
}

// Render — тема, текстовая версия и HTML письма.
func Render(l Letter) (subject, text, html string, err error) {
	t, ok := tpls[l.Kind]
	if !ok {
		return "", "", "", fmt.Errorf("mail: unknown letter %q", l.Kind)
	}
	lang := l.Lang
	brand := i18n.T(lang, "page.brand")
	v := view{
		Lang:     string(lang),
		Brand:    brand,
		Link:     l.Link,
		LinkText: linkText(l.Link),
		Fallback: i18n.T(lang, "mail.fallback"),
		Footer:   brand + " · info@racion.app",
		Icon:     strings.TrimRight(l.BaseURL, "/") + "/mail/receipt.png",
		Board:    l.Board,
	}
	switch l.Kind {
	case "reset":
		v.Subject = i18n.T(lang, "mail.reset.subject")
		v.Preheader = i18n.T(lang, "mail.reset.preheader", l.Minutes)
		v.Title = i18n.T(lang, "mail.reset.title")
		v.Lead = i18n.T(lang, "mail.reset.lead")
		v.Button = i18n.T(lang, "mail.reset.button")
		v.Note = i18n.T(lang, "mail.reset.note")
		// текст собран из тех же фраз, что и HTML: срок ссылки в обеих частях один
		text = strings.Join([]string{i18n.T(lang, "mail.hello"), v.Lead + " " + v.Preheader, v.Button + ": " + l.Link, v.Note, brand + ", info@racion.app"}, "\n\n")
	case "welcome":
		v.Subject = i18n.T(lang, "mail.welcome.subject")
		v.Preheader = i18n.T(lang, "mail.welcome.preheader")
		v.Title = i18n.T(lang, "mail.welcome.title")
		v.Lead = i18n.T(lang, "mail.welcome.lead")
		v.Button = i18n.T(lang, "mail.welcome.button")
		if l.Week {
			v.Button = i18n.T(lang, "mail.welcome.button.week")
		}
		v.Note = i18n.T(lang, "mail.welcome.note")
		lines := []string{i18n.T(lang, "mail.hello"), v.Title + ". " + v.Lead}
		if l.Board.Value != "" {
			lines = append(lines, l.Board.Value+" "+l.Board.Caption+".")
		}
		text = strings.Join(append(lines, v.Button+": "+l.Link, v.Note, brand+", info@racion.app"), "\n\n")
	}
	for _, s := range []*string{&v.Title, &v.Lead, &v.Note, &v.Fallback, &v.Board.Caption} {
		*s = nbsp(lang, *s)
	}
	var b bytes.Buffer
	if err := t.ExecuteTemplate(&b, "layout", v); err != nil {
		return "", "", "", fmt.Errorf("mail: render %s: %w", l.Kind, err)
	}
	return v.Subject, text, b.String(), nil
}

// ResetBoard — табло письма о смене пароля: срок ссылки как на табло, «60:00».
func ResetBoard(lang i18n.Lang, minutes int) Board {
	return Board{Value: fmt.Sprintf("%02d:00", minutes), Caption: i18n.T(lang, "mail.reset.board")}
}

// WelcomeBoard — табло приветствия: сколько блюд в неделе, с которой человек зарегистрировался. Без
// недели табло нет: число ради числа письму не нужно, а сумма в рублях после регистрации похожа на счёт.
func WelcomeBoard(lang i18n.Lang, dishes int) Board {
	if dishes <= 0 {
		return Board{}
	}
	return Board{Value: strconv.Itoa(dishes), Caption: i18n.Plural(lang, dishes, "dishes") + " " + i18n.T(lang, "mail.welcome.board")}
}

// linkText — запасная ссылка для глаз: полная, только без https:// (адресная строка допишет его сама).
// Обрезать нельзя: её копируют, когда кнопка не нажимается, и обрезанная никуда не ведёт.
func linkText(u string) string {
	return strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
}

// oneLetter — однобуквенные предлоги и союзы русского и украинского: в конце строки они висят.
var oneLetter = regexp.MustCompile(`(^|[ («\x{00a0}])([ВвСсКкОоУуАаИиЯяЗзІіЙйЖж]) `)

// nbsp — в русском и украинском однобуквенное слово держится за следующее неразрывным пробелом.
// Два прохода: в «и с телефона» второй предлог стоит сразу за первым.
func nbsp(lang i18n.Lang, s string) string {
	if lang != "ru" && lang != "uk" {
		return s
	}
	for i := 0; i < 2; i++ {
		s = oneLetter.ReplaceAllString(s, "$1$2\u00a0")
	}
	return s
}
