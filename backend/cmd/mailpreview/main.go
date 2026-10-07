// Command mailpreview — письмо на диск без отправки: посмотреть вёрстку в браузере и снять снимок.
//
//	go run ./cmd/mailpreview -kind reset -lang ru -out reset.html
//	go run ./cmd/mailpreview -kind welcome -week -lang en -out welcome.html -text
package main

import (
	"flag"
	"fmt"
	"os"

	"racion/internal/i18n"
	"racion/internal/mail"
)

func main() {
	kind := flag.String("kind", "reset", "reset | welcome")
	lang := flag.String("lang", "ru", "язык письма")
	base := flag.String("base", "https://racion.app", "адрес сайта: ссылки письма")
	img := flag.String("img", "", "откуда картинки письма, если не с -base (локальный стенд)")
	week := flag.Bool("week", false, "приветствие: регистрация со своей недели")
	out := flag.String("out", "", "куда записать HTML (пусто — в stdout)")
	text := flag.Bool("text", false, "напечатать тему и текстовую версию")
	flag.Parse()

	if *img == "" {
		*img = *base
	}
	l := mail.Letter{Kind: *kind, Lang: i18n.Lang(*lang), BaseURL: *img}
	switch *kind {
	case "reset":
		l.Link = *base + "/login?reset=79169fbe9f6b8702e6106e4d7e163c0b0b301532b12f81dc3728350aa6a38ea8" // выдуманный токен: начало как на эскизе
		l.Minutes = 60
		l.Board = mail.ResetBoard(l.Lang, l.Minutes)
	case "welcome":
		l.Link, l.Week = *base+"/", *week
		if *week {
			l.Link = *base + "/plan/AbC123"
		}
		if *week {
			l.Board = mail.WelcomeBoard(l.Lang, 21)
		}
	}
	subject, txt, html, err := mail.Render(l)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *text {
		fmt.Fprintf(os.Stderr, "Subject: %s\n\n%s\n", subject, txt)
	}
	if *out == "" {
		fmt.Print(html)
		return
	}
	if err := os.WriteFile(*out, []byte(html), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
