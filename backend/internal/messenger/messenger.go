// Package messenger — боты «Рациона» в Telegram и MAX за одним интерфейсом. Логика бота (что
// ответить, какой список показать) живёт в сервисе и не знает, в каком мессенджере работает:
// оба устроены одинаково — сообщение с кнопками, правка его на месте, нажатие приходит вебхуком.
package messenger

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type Platform string

const (
	Telegram Platform = "telegram"
	Max      Platform = "max"
)

// Button — кнопка под сообщением. Data — нажатие обрабатывает бот; URL — кнопка открывает ссылку.
type Button struct {
	Text string
	Data string
	URL  string
}

// Message — текст в HTML (только <b> и <i>: оба мессенджера понимают их одинаково) и ряды кнопок.
type Message struct {
	Text string
	Rows [][]Button
}

// Update — то, что прислал мессенджер, приведённое к одному виду.
type Update struct {
	Platform  Platform
	ChatID    string
	Lang      string // язык интерфейса человека, если мессенджер его сообщил
	Start     bool   // человек нажал «Начать» или пришёл по ссылке с параметром
	Payload   string // параметр из ссылки запуска
	Text      string // обычное сообщение
	Callback  string // id нажатия: на него надо ответить, иначе у человека крутится индикатор
	Data      string // данные нажатой кнопки
	MessageID string // сообщение, под которым нажали кнопку
}

// Client — один мессенджер.
type Client interface {
	Platform() Platform
	Username() string
	StartLink(payload string) string
	Send(ctx context.Context, chatID string, m Message) (messageID string, err error)
	Edit(ctx context.Context, chatID, messageID string, m Message) error
	Answer(ctx context.Context, callbackID, text string) error
	SetWebhook(ctx context.Context, url string) error
	// Parse проверяет подпись вебхука и разбирает обновление. ok=false — обновление не про нас
	// (участник вышел из группы и т. п.): отвечаем 200 и ничего не делаем.
	Parse(r *http.Request) (u Update, ok bool, err error)
}

// ErrBlocked — человек заблокировал бота: больше ему не пишем, пока сам не вернётся.
var ErrBlocked = errors.New("messenger: bot blocked by user")

// ErrForged — вебхук без нашей подписи.
var ErrForged = errors.New("messenger: webhook secret mismatch")

// splitStart — «/start p123» → payload «p123»; true, если это команда запуска.
func splitStart(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text != "/start" && !strings.HasPrefix(text, "/start ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(text, "/start")), true
}

// lang2 — «ru-RU», «en_US» → «ru», «en».
func lang2(s string) string {
	if len(s) >= 2 {
		return strings.ToLower(s[:2])
	}
	return ""
}
