package mail

import (
	"strings"
	"testing"
	"time"

	"racion/internal/i18n"
	"racion/locales"
)

func TestRenderEveryLanguage(t *testing.T) {
	link := "https://racion.app/login?reset=" + strings.Repeat("ab", 32)
	for code := range locales.All {
		lang := i18n.Lang(code)
		for _, l := range []Letter{
			{Kind: "reset", Lang: lang, BaseURL: "https://racion.app", Link: link, Minutes: 60, Board: ResetBoard(lang, 60)},
			{Kind: "welcome", Lang: lang, BaseURL: "https://racion.app", Link: "https://racion.app/plan/AbC123", Week: true, Board: WelcomeBoard(lang, 21)},
			{Kind: "welcome", Lang: lang, BaseURL: "https://racion.app", Link: "https://racion.app/"},
		} {
			subject, text, html, err := Render(l)
			if err != nil {
				t.Fatalf("%s %s: %v", code, l.Kind, err)
			}
			// ключ без перевода i18n.T возвращает как есть: в письме его быть не должно
			for _, part := range []string{subject, text, html} {
				if strings.Contains(part, "mail.reset.") || strings.Contains(part, "mail.welcome.") || strings.Contains(part, "mail.fallback") || strings.Contains(part, "mail.hello") || strings.Contains(part, "page.brand") {
					t.Errorf("%s %s: raw locale key in letter: %.80q", code, l.Kind, part)
				}
			}
			if !strings.Contains(html, `href="`+l.Link+`"`) || !strings.Contains(text, l.Link) {
				t.Errorf("%s %s: link missing", code, l.Kind)
			}
			if !strings.Contains(html, `lang="`+code+`"`) || !strings.Contains(html, "https://racion.app/mail/receipt.png") {
				t.Errorf("%s %s: lang or brand image missing", code, l.Kind)
			}
			if l.Board.Value != "" && !strings.Contains(html, l.Board.Value) {
				t.Errorf("%s %s: board %q missing", code, l.Kind, l.Board.Value)
			}
			if l.Board.Value == "" && strings.Contains(html, `class="r-board"`) {
				t.Errorf("%s %s: empty board drawn", code, l.Kind)
			}
		}
	}
}

func TestResetBoardAndShortLink(t *testing.T) {
	if b := ResetBoard("ru", 60); b.Value != "60:00" || b.Caption == "" {
		t.Fatalf("board %+v", b)
	}
	if got := linkText("https://racion.app/login?reset=7916abcdef"); got != "racion.app/login?reset=7916abcdef" {
		t.Fatalf("link text %q", got)
	}
	if got := nbsp("ru", "открыть с телефона и с компьютера, а отметки"); got != "открыть с телефона и с компьютера, а отметки" {
		t.Fatalf("nbsp %q", got)
	}
	if got := nbsp("en", "a b c"); got != "a b c" {
		t.Fatalf("nbsp touched english: %q", got)
	}
	if b := WelcomeBoard("ru", 0); b.Value != "" {
		t.Fatalf("welcome board without a week: %+v", b)
	}
	if b := WelcomeBoard("ru", 21); b.Value != "21" || !strings.HasPrefix(b.Caption, "блюдо ") {
		t.Fatalf("welcome board %+v", b)
	}
}

func TestOutlookAndTextPart(t *testing.T) {
	link := "https://racion.app/login?reset=" + strings.Repeat("ab", 32)
	_, text, html, err := Render(Letter{Kind: "reset", Lang: "ru", BaseURL: "https://racion.app", Link: link, Minutes: 60, Board: ResetBoard("ru", 60)})
	if err != nil {
		t.Fatal(err)
	}
	// html/template вырезает комментарии: условные блоки Outlook должны дойти до письма
	for _, want := range []string{"<!--[if mso]><table", "<!--[if mso]></td></tr></table><![endif]-->", "<v:roundrect", "<!--[if !mso]><!-->", "<!--<![endif]-->", "o:OfficeDocumentSettings"} {
		if !strings.Contains(html, want) {
			t.Errorf("html lacks %q", want)
		}
	}
	if !strings.Contains(html, "racion.app/login?reset="+strings.Repeat("ab", 32)+"</a>") {
		t.Error("fallback link is not shown in full")
	}
	if !strings.Contains(html, ButtonBlue) || strings.Contains(html, "#007AFF;border-radius:12px") {
		t.Error("button is not the accessible blue")
	}
	if !strings.Contains(text, "60 минут") || !strings.Contains(text, link) {
		t.Errorf("text part: %q", text)
	}
}

func TestBuildMessageMultipart(t *testing.T) {
	html := "<p>" + strings.Repeat("длинная строка без переносов ", 80) + "</p>"
	msg := string(buildMessage("Рацион <info@racion.app>", "a@b.c", "Новый пароль", "Привет\nссылка", html, time.Unix(0, 0)))
	if !strings.Contains(msg, "Content-Type: multipart/alternative; boundary=") {
		t.Fatal("not multipart")
	}
	if strings.Index(msg, "Content-Type: text/plain") > strings.Index(msg, "Content-Type: text/html") {
		t.Fatal("text part must come before html")
	}
	for _, line := range strings.Split(msg, "\r\n") {
		if len(line) > 998 {
			t.Fatalf("line longer than SMTP allows: %d", len(line))
		}
	}
	if strings.Contains(strings.ReplaceAll(msg, "\r\n", ""), "\n") {
		t.Fatal("bare LF in message")
	}
	plain := string(buildMessage("Рацион <info@racion.app>", "a@b.c", "Тема", "текст", "", time.Unix(0, 0)))
	if strings.Contains(plain, "multipart") || !strings.Contains(plain, "Content-Type: text/plain; charset=utf-8") {
		t.Fatal("plain letter must stay single-part")
	}
}
