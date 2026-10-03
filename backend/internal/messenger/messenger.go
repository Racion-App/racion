// Package messenger — боты «Рациона» в Telegram и MAX за одним интерфейсом. Логика бота (что
// ответить, какой список показать) живёт в сервисе и не знает, в каком мессенджере работает:
// оба устроены одинаково — сообщение с кнопками, правка его на месте, нажатие приходит вебхуком.
package messenger

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

type Platform string

const (
	Telegram Platform = "telegram"
	Max      Platform = "max"
)

// Button — кнопка под сообщением. Data — нажатие обрабатывает бот; URL — кнопка открывает ссылку.
// App — открыть ссылку как мини-приложение внутри мессенджера, где он это умеет (Telegram);
// в остальных это обычная ссылка.
type Button struct {
	Text string
	Data string
	URL  string
	App  bool
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
	UserID    string // человек в мессенджере: по нему аккаунт сайта узнаёт привязку
	Lang      string // язык интерфейса человека, если мессенджер его сообщил
	Start     bool   // человек нажал «Начать» или пришёл по ссылке с параметром
	Payload   string // параметр из ссылки запуска
	Text      string // обычное сообщение
	Callback  string // id нажатия: на него надо ответить, иначе у человека крутится индикатор
	Data      string // данные нажатой кнопки
	MessageID string // сообщение, под которым нажали кнопку
}

// Profile — как бот выглядит в мессенджере до первого сообщения.
type Profile struct {
	About       string // строка в профиле и в превью ссылки, до 120 символов
	Description string // экран «что умеет бот» до нажатия «Начать», до 512 символов
	Commands    []Command
}

type Command struct {
	Name        string // без «/», латиницей в нижнем регистре
	Description string
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
	// SetProfile — описание и команды по языкам; ключ "" — для всех остальных языков.
	SetProfile(ctx context.Context, profiles map[string]Profile) error
	// SetMenu — кнопка слева от поля ввода открывает сайт мини-приложением; где такой кнопки нет — ничего.
	SetMenu(ctx context.Context, chatID, text, url string) error
	// Parse проверяет подпись вебхука и разбирает обновление. ok=false — обновление не про нас
	// (участник вышел из группы и т. п.): отвечаем 200 и ничего не делаем.
	Parse(r *http.Request) (u Update, ok bool, err error)
}

// Poller — мессенджер умеет отдавать обновления опросом: сервер сам забирает их, входящие соединения
// не нужны (Telegram, getUpdates). Poll работает, пока не отменят ctx.
type Poller interface {
	Poll(ctx context.Context, handle func(Update)) error
}

// MiniApp — мессенджер открывает сайт внутри себя и подписывает данные человека (Telegram).
type MiniApp interface {
	VerifyInitData(raw string, now time.Time) (WebAppUser, error)
}

// WebAppUser — человек, открывший сайт мини-приложением; подпись мессенджера проверена.
type WebAppUser struct {
	ID         string
	Name       string // имя и фамилия, как в мессенджере
	Username   string
	Lang       string
	Photo      string
	StartParam string // параметр из ссылки запуска мини-приложения
}

// ErrBlocked — человек заблокировал бота или ещё не начинал с ним чат: писать ему нельзя.
var ErrBlocked = errors.New("messenger: bot blocked by user")

// ErrForged — вебхук или данные мини-приложения без нашей подписи.
var ErrForged = errors.New("messenger: signature mismatch")

// ErrExpired — данные мини-приложения подписаны давно: войти по ним нельзя.
var ErrExpired = errors.New("messenger: init data expired")

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
