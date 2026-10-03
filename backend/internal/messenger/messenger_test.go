package messenger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

type seen struct {
	method, path, query, auth string
	body                      map[string]any
}

func fake(t *testing.T, reply string, status int, got *seen) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.query, got.auth = r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got.body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
}

var msg = Message{Text: "<b>Молочное</b>", Rows: [][]Button{{{Text: "Молоко", Data: "t:x:0:milk"}}, {{Text: "Открыть", URL: "https://racion.app/plan/1"}}}}

func TestTelegramSend(t *testing.T) {
	var got seen
	s := fake(t, `{"ok":true,"result":{"message_id":42}}`, 200, &got)
	defer s.Close()
	c := NewTelegram("TOKEN", "@racion_bot", "sec")
	c.base = s.URL
	id, err := c.Send(context.Background(), "100", msg)
	if err != nil || id != "42" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if got.path != "/botTOKEN/sendMessage" || got.body["chat_id"] != "100" || got.body["parse_mode"] != "HTML" {
		t.Errorf("запрос %s %+v", got.path, got.body)
	}
	kb := got.body["reply_markup"].(map[string]any)["inline_keyboard"].([]any)
	first := kb[0].([]any)[0].(map[string]any)
	link := kb[1].([]any)[0].(map[string]any)
	if first["callback_data"] != "t:x:0:milk" || link["url"] != "https://racion.app/plan/1" || link["callback_data"] != nil {
		t.Errorf("клавиатура %+v", kb)
	}
	if c.StartLink("pabc") != "https://t.me/racion_bot?start=pabc" {
		t.Errorf("ссылка %s", c.StartLink("pabc"))
	}
}

func TestTelegramErrors(t *testing.T) {
	var got seen
	blocked := fake(t, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`, 403, &got)
	defer blocked.Close()
	c := NewTelegram("T", "b", "s")
	c.base = blocked.URL
	if _, err := c.Send(context.Background(), "1", msg); !errors.Is(err, ErrBlocked) {
		t.Errorf("ждали ErrBlocked, получили %v", err)
	}
	same := fake(t, `{"ok":false,"error_code":400,"description":"Bad Request: message is not modified"}`, 400, &got)
	defer same.Close()
	c.base = same.URL
	if err := c.Edit(context.Background(), "1", "2", msg); err != nil {
		t.Errorf("повторное нажатие той же кнопки — не ошибка: %v", err)
	}
}

func TestTelegramParse(t *testing.T) {
	c := NewTelegram("T", "b", "sec")
	req := func(secret, body string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
		return r
	}
	if _, _, err := c.Parse(req("wrong", `{}`)); !errors.Is(err, ErrForged) {
		t.Errorf("чужой вебхук принят: %v", err)
	}
	u, ok, err := c.Parse(req("sec", `{"message":{"message_id":5,"chat":{"id":77,"type":"private"},"from":{"language_code":"de-DE"},"text":"/start p0123"}}`))
	if err != nil || !ok || !u.Start || u.Payload != "p0123" || u.ChatID != "77" || u.Lang != "de" {
		t.Errorf("старт %+v ok=%v err=%v", u, ok, err)
	}
	if _, ok, _ := c.Parse(req("sec", `{"message":{"message_id":6,"chat":{"id":-100,"type":"group"},"text":"привет"}}`)); ok {
		t.Error("бот отвечает на разговор в группе")
	}
	u, ok, _ = c.Parse(req("sec", `{"callback_query":{"id":"cb1","data":"t:x:0:milk","from":{"language_code":"ru"},"message":{"message_id":9,"chat":{"id":77}}}}`))
	if !ok || u.Callback != "cb1" || u.Data != "t:x:0:milk" || u.MessageID != "9" || u.ChatID != "77" {
		t.Errorf("нажатие %+v", u)
	}
}

func TestMaxSend(t *testing.T) {
	var got seen
	s := fake(t, `{"message":{"body":{"mid":"mid.abc","text":"x"}}}`, 200, &got)
	defer s.Close()
	c := NewMax("MTOKEN", "racion", "sec")
	c.base = s.URL
	id, err := c.Send(context.Background(), "555", msg)
	if err != nil || id != "mid.abc" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if got.method != http.MethodPost || got.path != "/messages" || got.query != "chat_id=555" || got.auth != "MTOKEN" || got.body["format"] != "html" {
		t.Errorf("запрос %s %s?%s auth=%q %+v", got.method, got.path, got.query, got.auth, got.body)
	}
	att := got.body["attachments"].([]any)[0].(map[string]any)
	rows := att["payload"].(map[string]any)["buttons"].([]any)
	cb := rows[0].([]any)[0].(map[string]any)
	ln := rows[1].([]any)[0].(map[string]any)
	if att["type"] != "inline_keyboard" || cb["type"] != "callback" || cb["payload"] != "t:x:0:milk" || ln["type"] != "link" || ln["url"] == nil {
		t.Errorf("клавиатура %+v", att)
	}
	if c.StartLink("p1") != "https://max.ru/racion/start/p1" {
		t.Errorf("ссылка %s", c.StartLink("p1"))
	}
	if err := c.Edit(context.Background(), "555", "mid.abc", msg); err != nil || got.method != http.MethodPut || got.query != "message_id=mid.abc" {
		t.Errorf("правка %s ?%s err=%v", got.method, got.query, err)
	}
}

func TestSetProfile(t *testing.T) {
	var calls []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		calls = append(calls, r.Method+" "+r.URL.Path[strings.LastIndex(r.URL.Path, "/"):]+" "+fmt.Sprint(body["language_code"]))
		if r.URL.Path == "/me" && (body["description"] != "Список по отделам" || len(body["commands"].([]any)) != 1) {
			t.Errorf("профиль MAX %+v", body)
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	}))
	defer s.Close()
	profiles := map[string]Profile{
		"":   {About: "List", Description: "Shopping list", Commands: []Command{{Name: "list", Description: "Latest week"}}},
		"ru": {About: "Список", Description: "Список по отделам", Commands: []Command{{Name: "list", Description: "Последняя неделя"}}},
	}
	tg := NewTelegram("T", "b", "s")
	tg.base = s.URL
	if err := tg.SetProfile(context.Background(), profiles); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 6 || !strings.Contains(strings.Join(calls, ";"), "POST /setMyCommands ru") || !strings.Contains(strings.Join(calls, ";"), "POST /setMyDescription <nil>") {
		t.Errorf("Telegram: %v", calls)
	}
	calls = nil
	mx := NewMax("T", "b", "s")
	mx.base = s.URL
	if err := mx.SetProfile(context.Background(), profiles); err != nil || len(calls) != 1 || calls[0] != "PATCH /me <nil>" {
		t.Errorf("MAX: %v %v", calls, err)
	}
}

// sign подписывает initData по алгоритму из документации Telegram: поля без hash по алфавиту через
// перевод строки, ключ — HMAC-SHA256 токена бота ключом «WebAppData».
func sign(token string, vals url.Values) string {
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		lines = append(lines, k+"="+vals.Get(k))
	}
	key := hmac.New(sha256.New, []byte("WebAppData"))
	key.Write([]byte(token))
	m := hmac.New(sha256.New, key.Sum(nil))
	m.Write([]byte(strings.Join(lines, "\n")))
	vals.Set("hash", hex.EncodeToString(m.Sum(nil)))
	return vals.Encode()
}

func TestVerifyInitData(t *testing.T) {
	c := NewTelegram("123:ABC", "b", "s")
	now := time.Unix(1_790_000_000, 0)
	raw := sign("123:ABC", url.Values{
		"auth_date": {fmt.Sprint(now.Unix() - 60)}, "query_id": {"AAHdF6IQ"}, "start_param": {"pabc"}, "signature": {"c2ln"},
		"user": {`{"id":42,"first_name":"Анна","last_name":"К","username":"anna","language_code":"ru-RU","photo_url":"https://t.me/i/userpic/320/a.jpg"}`},
	})
	u, err := c.VerifyInitData(raw, now)
	if err != nil || u.ID != "42" || u.Name != "Анна К" || u.Username != "anna" || u.Lang != "ru" || u.StartParam != "pabc" || u.Photo == "" {
		t.Fatalf("%+v %v", u, err)
	}
	if _, err := NewTelegram("999:X", "b", "s").VerifyInitData(raw, now); !errors.Is(err, ErrForged) {
		t.Errorf("подпись другого бота принята: %v", err)
	}
	if _, err := c.VerifyInitData(strings.Replace(raw, "anna", "eve", 1), now); !errors.Is(err, ErrForged) {
		t.Errorf("подменённое поле принято: %v", err)
	}
	if _, err := c.VerifyInitData(raw, now.Add(25*time.Hour)); !errors.Is(err, ErrExpired) {
		t.Errorf("вчерашние данные приняты: %v", err)
	}
	if _, err := c.VerifyInitData("", now); !errors.Is(err, ErrForged) {
		t.Errorf("пустые данные: %v", err)
	}
}

func TestTelegramAppButtonAndUser(t *testing.T) {
	body := tgBody(Message{Text: "x", Rows: [][]Button{{{Text: "Открыть", URL: "https://racion.app/plan/1", App: true}}}})
	b := body["reply_markup"].(map[string]any)["inline_keyboard"].([][]tgButton)[0][0]
	if b.WebApp == nil || b.WebApp.URL != "https://racion.app/plan/1" || b.URL != "" {
		t.Errorf("кнопка мини-приложения %+v", b)
	}
	c := NewTelegram("T", "b", "sec")
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"message":{"message_id":5,"chat":{"id":77,"type":"private"},"from":{"id":77,"language_code":"ru"},"text":"/start u123"}}`))
	r.Header.Set("X-Telegram-Bot-Api-Secret-Token", "sec")
	if u, ok, _ := c.Parse(r); !ok || u.UserID != "77" || u.Payload != "u123" {
		t.Errorf("человек в обновлении %+v", u)
	}
}

// Опрос: сначала снимаем вебхук, потом getUpdates; каждый следующий запрос подтверждает полученное
// через offset, обновления из групп отсеиваются так же, как у вебхука.
func TestTelegramPoll(t *testing.T) {
	var methods []string
	var offsets []any
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		methods = append(methods, method)
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if method != "getUpdates" {
			_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
			return
		}
		offsets = append(offsets, body["offset"])
		calls++
		if calls == 1 {
			_, _ = io.WriteString(w, `{"ok":true,"result":[
				{"update_id":10,"message":{"message_id":1,"chat":{"id":7,"type":"private"},"from":{"id":7,"language_code":"ru"},"text":"/list"}},
				{"update_id":11,"message":{"message_id":2,"chat":{"id":-5,"type":"group"},"text":"шум"}},
				{"update_id":12,"callback_query":{"id":"c1","data":"l:k:1","from":{"id":7},"message":{"message_id":3,"chat":{"id":7}}}}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":[]}`)
	}))
	defer s.Close()
	c := NewTelegram("T", "b", "s")
	c.base = s.URL
	ctx, cancel := context.WithCancel(context.Background())
	var got []Update
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()
	err := c.Poll(ctx, func(u Update) { got = append(got, u) })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("опрос закончился не по отмене: %v", err)
	}
	if methods[0] != "deleteWebhook" || len(got) != 2 || got[0].Text != "/list" || got[1].Callback != "c1" {
		t.Fatalf("методы %v, обновления %+v", methods[:min(len(methods), 4)], got)
	}
	if len(offsets) < 2 || offsets[0] != float64(0) || offsets[1] != float64(13) {
		t.Errorf("offset: %v", offsets[:min(len(offsets), 3)])
	}
}

func TestMaxParse(t *testing.T) {
	c := NewMax("T", "b", "sec")
	req := func(secret, body string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set("X-Max-Bot-Api-Secret", secret)
		return r
	}
	if _, _, err := c.Parse(req("", `{}`)); !errors.Is(err, ErrForged) {
		t.Errorf("вебхук без подписи принят: %v", err)
	}
	u, ok, _ := c.Parse(req("sec", `{"update_type":"bot_started","chat_id":321,"user":{"user_id":9001},"payload":"p0123","user_locale":"ru"}`))
	if !ok || !u.Start || u.Payload != "p0123" || u.ChatID != "321" || u.Lang != "ru" || u.UserID != "9001" {
		t.Errorf("старт %+v", u)
	}
	u, ok, _ = c.Parse(req("sec", `{"update_type":"message_callback","callback":{"callback_id":"c9","payload":"l:x:1"},"message":{"recipient":{"chat_id":321},"body":{"mid":"mid.7"}}}`))
	if !ok || u.Callback != "c9" || u.Data != "l:x:1" || u.MessageID != "mid.7" || u.ChatID != "321" {
		t.Errorf("нажатие %+v", u)
	}
	if _, ok, _ := c.Parse(req("sec", `{"update_type":"user_added"}`)); ok {
		t.Error("чужой тип обновления должен пропускаться")
	}
	u, ok, _ = c.Parse(req("sec", `{"update_type":"message_created","message":{"recipient":{"chat_id":321,"chat_type":"dialog"},"sender":{"is_bot":false},"body":{"mid":"m1","text":"/start p9"}}}`))
	if !ok || !u.Start || u.Payload != "p9" || u.ChatID != "321" {
		t.Errorf("сообщение в диалоге %+v", u)
	}
	for name, body := range map[string]string{
		"общий чат":  `{"update_type":"message_created","message":{"recipient":{"chat_id":-7,"chat_type":"chat"},"sender":{"is_bot":false},"body":{"text":"привет"}}}`,
		"свой ответ": `{"update_type":"message_created","message":{"recipient":{"chat_id":321,"chat_type":"dialog"},"sender":{"is_bot":true},"body":{"text":"Неделя подключена"}}}`,
	} {
		if _, ok, _ := c.Parse(req("sec", body)); ok {
			t.Errorf("%s: бот ответил бы", name)
		}
	}
}
