// Package messenger — боты «Рациона» в Telegram и MAX за одним интерфейсом. Логика бота (что
// ответить, какой список показать) живёт в сервисе и не знает, в каком мессенджере работает:
// оба устроены одинаково — сообщение с кнопками, правка его на месте, нажатие приходит вебхуком.
package messenger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Platform string

const (
	Telegram Platform = "telegram"
	Max      Platform = "max"
)

// Button — кнопка под сообщением. Data — нажатие обрабатывает бот; URL — кнопка открывает ссылку.
// App — открыть сайт мини-приложением внутри мессенджера. Telegram открывает сам URL, а MAX — только
// мини-приложение бота с адресом из кабинета бизнеса, поэтому ему нужен Start: параметр запуска
// («p<ключ недели>», «me»), по которому сайт откроет нужную страницу.
type Button struct {
	Text  string
	Data  string
	URL   string
	App   bool
	Start string
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

// MiniApp — мессенджер открывает сайт внутри себя и подписывает данные человека (Telegram, MAX).
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
	// ChatID — личный чат человека с ботом: туда бот пишет. В Telegram это id человека, в MAX — свой id
	// диалога, он приходит, только если мини-приложение открыли из этого диалога; иначе пусто.
	ChatID string
}

// verifyWebAppData — подпись данных мини-приложения, одна у Telegram и MAX: HMAC-SHA256 полей без hash,
// отсортированных по имени и соединённых переводом строки, ключом HMAC-SHA256("WebAppData", токен
// бота), в hex. Данные старше суток не принимаем: по ним вошёл бы тот, кто перехватил старую ссылку.
func verifyWebAppData(token, raw string, now time.Time) (WebAppUser, url.Values, error) {
	vals, err := url.ParseQuery(raw)
	hash := vals.Get("hash")
	if err != nil || hash == "" {
		return WebAppUser{}, nil, ErrForged
	}
	vals.Del("hash")
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + vals.Get(k)
	}
	secret := hmacSHA256([]byte("WebAppData"), []byte(token))
	want := hex.EncodeToString(hmacSHA256(secret, []byte(strings.Join(lines, "\n"))))
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(hash))) {
		return WebAppUser{}, nil, ErrForged
	}
	ts, _ := strconv.ParseInt(vals.Get("auth_date"), 10, 64)
	if ts > 1e11 { // миллисекунды вместо секунд
		ts /= 1000
	}
	signed := time.Unix(ts, 0)
	if ts == 0 || now.Sub(signed) > 24*time.Hour || signed.Sub(now) > 5*time.Minute {
		return WebAppUser{}, nil, ErrExpired
	}
	var u struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
		Lang      string `json:"language_code"`
		Photo     string `json:"photo_url"`
	}
	if json.Unmarshal([]byte(vals.Get("user")), &u) != nil || u.ID == 0 {
		return WebAppUser{}, nil, ErrForged
	}
	return WebAppUser{ID: strconv.FormatInt(u.ID, 10), Name: strings.TrimSpace(u.FirstName + " " + u.LastName), Username: u.Username,
		Lang: lang2(u.Lang), Photo: u.Photo, StartParam: vals.Get("start_param")}, vals, nil
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
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
