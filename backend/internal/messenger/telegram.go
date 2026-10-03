package messenger

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TelegramClient — Bot API Telegram: sendMessage, editMessageText, answerCallbackQuery, вебхук
// с секретом в заголовке X-Telegram-Bot-Api-Secret-Token.
type TelegramClient struct {
	token, username, secret string
	base                    string // https://api.telegram.org; в тестах — подменный сервер
	http                    *http.Client
}

func NewTelegram(token, username, secret string) *TelegramClient {
	return &TelegramClient{token: token, username: strings.TrimPrefix(username, "@"), secret: secret,
		base: "https://api.telegram.org", http: &http.Client{Timeout: 20 * time.Second}}
}

// SetBase меняет адрес Bot API: свой сервер telegram-bot-api или подменный на локальном стенде.
func (c *TelegramClient) SetBase(u string) { c.base = strings.TrimRight(u, "/") }

func (c *TelegramClient) Platform() Platform { return Telegram }
func (c *TelegramClient) Username() string   { return c.username }

// StartLink — t.me/<бот>?start=<параметр>; параметр до 64 символов из A-Z, a-z, 0-9, «_» и «-».
func (c *TelegramClient) StartLink(payload string) string {
	return "https://t.me/" + c.username + "?start=" + url.QueryEscape(payload)
}

type tgButton struct {
	Text   string    `json:"text"`
	Data   string    `json:"callback_data,omitempty"`
	URL    string    `json:"url,omitempty"`
	WebApp *tgWebApp `json:"web_app,omitempty"`
}

type tgWebApp struct {
	URL string `json:"url"`
}

func tgBody(m Message) map[string]any {
	rows := make([][]tgButton, 0, len(m.Rows))
	for _, r := range m.Rows {
		row := make([]tgButton, 0, len(r))
		for _, b := range r {
			if b.App && b.URL != "" { // сайт открывается внутри Telegram; так можно только в личном чате, а бот и пишет только туда
				row = append(row, tgButton{Text: b.Text, WebApp: &tgWebApp{URL: b.URL}})
				continue
			}
			row = append(row, tgButton{Text: b.Text, Data: b.Data, URL: b.URL})
		}
		rows = append(rows, row)
	}
	body := map[string]any{"text": m.Text, "parse_mode": "HTML", "link_preview_options": map[string]bool{"is_disabled": true}}
	if len(rows) > 0 {
		body["reply_markup"] = map[string]any{"inline_keyboard": rows}
	}
	return body
}

func (c *TelegramClient) Send(ctx context.Context, chatID string, m Message) (string, error) {
	body := tgBody(m)
	body["chat_id"] = chatID
	var res struct {
		MessageID int64 `json:"message_id"`
	}
	if err := c.call(ctx, "sendMessage", body, &res); err != nil {
		return "", err
	}
	return strconv.FormatInt(res.MessageID, 10), nil
}

func (c *TelegramClient) Edit(ctx context.Context, chatID, messageID string, m Message) error {
	body := tgBody(m)
	body["chat_id"] = chatID
	body["message_id"], _ = strconv.ParseInt(messageID, 10, 64)
	err := c.call(ctx, "editMessageText", body, nil)
	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		return nil // нажали ту же кнопку дважды — текст и кнопки уже такие
	}
	return err
}

func (c *TelegramClient) Answer(ctx context.Context, callbackID, text string) error {
	return c.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": callbackID, "text": text}, nil)
}

func (c *TelegramClient) SetWebhook(ctx context.Context, hook string) error {
	return c.call(ctx, "setWebhook", map[string]any{"url": hook, "secret_token": c.secret,
		"allowed_updates": []string{"message", "callback_query"}}, nil)
}

// SetMenu — кнопка меню чата открывает сайт мини-приложением; без chatID — кнопка по умолчанию для всех чатов.
func (c *TelegramClient) SetMenu(ctx context.Context, chatID, text, url string) error {
	body := map[string]any{"menu_button": map[string]any{"type": "web_app", "text": text, "web_app": tgWebApp{URL: url}}}
	if chatID != "" {
		body["chat_id"] = chatID
	}
	return c.call(ctx, "setChatMenuButton", body, nil)
}

// VerifyInitData проверяет подпись данных мини-приложения: HMAC-SHA256 отсортированных полей ключом
// HMAC-SHA256("WebAppData", токен бота). Данные старше суток не принимаем: по ним вошёл бы тот,
// кто перехватил старую ссылку запуска.
func (c *TelegramClient) VerifyInitData(raw string, now time.Time) (WebAppUser, error) {
	vals, err := url.ParseQuery(raw)
	hash := vals.Get("hash")
	if err != nil || hash == "" {
		return WebAppUser{}, ErrForged
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
	secret := hmacSHA256([]byte("WebAppData"), []byte(c.token))
	want := hex.EncodeToString(hmacSHA256(secret, []byte(strings.Join(lines, "\n"))))
	if !hmac.Equal([]byte(want), []byte(hash)) {
		return WebAppUser{}, ErrForged
	}
	ts, _ := strconv.ParseInt(vals.Get("auth_date"), 10, 64)
	signed := time.Unix(ts, 0)
	if ts == 0 || now.Sub(signed) > 24*time.Hour || signed.Sub(now) > 5*time.Minute {
		return WebAppUser{}, ErrExpired
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
		return WebAppUser{}, ErrForged
	}
	return WebAppUser{ID: strconv.FormatInt(u.ID, 10), Name: strings.TrimSpace(u.FirstName + " " + u.LastName), Username: u.Username,
		Lang: lang2(u.Lang), Photo: u.Photo, StartParam: vals.Get("start_param")}, nil
}

func tgID(id int64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}

// SetProfile — описание, строка профиля и команды на каждом языке: Telegram показывает тот вариант,
// что совпал с языком приложения у человека.
func (c *TelegramClient) SetProfile(ctx context.Context, profiles map[string]Profile) error {
	for lang, p := range profiles {
		cmds := make([]map[string]string, 0, len(p.Commands))
		for _, cmd := range p.Commands {
			cmds = append(cmds, map[string]string{"command": cmd.Name, "description": cmd.Description})
		}
		calls := []struct {
			method string
			body   map[string]any
		}{
			{"setMyDescription", map[string]any{"description": p.Description}},
			{"setMyShortDescription", map[string]any{"short_description": p.About}},
			{"setMyCommands", map[string]any{"commands": cmds}},
		}
		for _, call := range calls {
			if lang != "" {
				call.body["language_code"] = lang
			}
			if err := c.call(ctx, call.method, call.body, nil); err != nil {
				return fmt.Errorf("%s [%s]: %w", call.method, lang, err)
			}
		}
	}
	return nil
}

func (c *TelegramClient) Parse(r *http.Request) (Update, bool, error) {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(c.secret)) != 1 {
		return Update{}, false, ErrForged
	}
	var in struct {
		Message *struct {
			MessageID int64 `json:"message_id"`
			Chat      struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			} `json:"chat"`
			From struct {
				ID   int64  `json:"id"`
				Lang string `json:"language_code"`
			} `json:"from"`
			Text string `json:"text"`
		} `json:"message"`
		Callback *struct {
			ID   string `json:"id"`
			Data string `json:"data"`
			From struct {
				ID   int64  `json:"id"`
				Lang string `json:"language_code"`
			} `json:"from"`
			Message struct {
				MessageID int64 `json:"message_id"`
				Chat      struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"callback_query"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return Update{}, false, err
	}
	switch {
	case in.Callback != nil:
		return Update{Platform: Telegram, ChatID: strconv.FormatInt(in.Callback.Message.Chat.ID, 10), UserID: tgID(in.Callback.From.ID), Lang: lang2(in.Callback.From.Lang),
			Callback: in.Callback.ID, Data: in.Callback.Data, MessageID: strconv.FormatInt(in.Callback.Message.MessageID, 10)}, true, nil
	case in.Message != nil && in.Message.Chat.Type == "private": // в группах бот не отвечает на разговоры
		u := Update{Platform: Telegram, ChatID: strconv.FormatInt(in.Message.Chat.ID, 10), UserID: tgID(in.Message.From.ID), Lang: lang2(in.Message.From.Lang), Text: in.Message.Text}
		u.Payload, u.Start = splitStart(in.Message.Text)
		return u, true, nil
	}
	return Update{}, false, nil
}

// call — POST к Bot API. 403 «bot was blocked by the user» превращается в ErrBlocked.
func (c *TelegramClient) call(ctx context.Context, method string, body any, dst any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+c.token+"/"+method, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// в ошибке сети лежит полный адрес запроса, а в адресе у Telegram — токен бота: в лог его не пускаем
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	defer resp.Body.Close()
	var out struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
		ErrorCode   int             `json:"error_code"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return fmt.Errorf("telegram %s: http %d", method, resp.StatusCode)
	}
	if !out.OK {
		if out.ErrorCode == http.StatusForbidden {
			return ErrBlocked
		}
		return fmt.Errorf("telegram %s: %s", method, out.Description)
	}
	if dst != nil {
		return json.Unmarshal(out.Result, dst)
	}
	return nil
}
