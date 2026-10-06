package messenger

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"racion/internal/rucert"
)

// MaxClient — Bot API мессенджера MAX (platform-api2.max.ru, схема — github.com/max-messenger/
// max-bot-api-client-go/schema.yaml). Токен в заголовке Authorization, клавиатура — вложение
// inline_keyboard, вебхук с секретом в заголовке X-Max-Bot-Api-Secret. Сертификат API выдан
// удостоверяющим центром Минцифры, поэтому клиент доверяет и ему (rucert).
type MaxClient struct {
	token, username, secret string
	base                    string
	http                    *http.Client
}

func NewMax(token, username, secret string) *MaxClient {
	return &MaxClient{token: token, username: strings.TrimPrefix(username, "@"), secret: secret,
		base: "https://platform-api2.max.ru", http: rucert.Client(20 * time.Second)}
}

// SetBase меняет адрес API: для локального стенда с подменным сервером.
func (c *MaxClient) SetBase(u string) { c.base = strings.TrimRight(u, "/") }

func (c *MaxClient) Platform() Platform { return Max }
func (c *MaxClient) Username() string   { return c.username }

// StartLink — max.ru/<бот>?start=<параметр>; параметр до 128 символов, длиннее MAX молча отбрасывает.
func (c *MaxClient) StartLink(payload string) string {
	return "https://max.ru/" + c.username + "?start=" + url.QueryEscape(payload)
}

// AppLink — мини-приложение бота с параметром запуска: max.ru/<бот>?startapp=<параметр> (латиница,
// цифры, «_» и «-», до 512 символов). Внутри MAX ссылка открывает мини-приложение, а не браузер.
// Без параметра — «home»: сайт откроет главную.
func (c *MaxClient) AppLink(start string) string {
	if start == "" {
		start = "home"
	}
	return "https://max.ru/" + c.username + "?startapp=" + url.QueryEscape(start)
}

type maxButton struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Payload string `json:"payload,omitempty"`
	URL     string `json:"url,omitempty"`
}

// body — сообщение в виде MAX. Кнопка «Открыть сайт» (App) ведёт в мини-приложение бота ссылкой
// с параметром запуска: открыть произвольный адрес мини-приложением MAX не умеет.
func (c *MaxClient) body(m Message) map[string]any {
	body := map[string]any{"text": m.Text, "format": "html", "attachments": []any{}}
	if len(m.Rows) > 0 {
		rows := make([][]maxButton, 0, len(m.Rows))
		for _, r := range m.Rows {
			row := make([]maxButton, 0, len(r))
			for _, b := range r {
				switch {
				case b.App:
					row = append(row, maxButton{Type: "link", Text: b.Text, URL: c.AppLink(b.Start)})
				case b.URL != "":
					row = append(row, maxButton{Type: "link", Text: b.Text, URL: b.URL})
				default:
					row = append(row, maxButton{Type: "callback", Text: b.Text, Payload: b.Data})
				}
			}
			rows = append(rows, row)
		}
		body["attachments"] = []any{map[string]any{"type": "inline_keyboard", "payload": map[string]any{"buttons": rows}}}
	}
	return body
}

func (c *MaxClient) Send(ctx context.Context, chatID string, m Message) (string, error) {
	var res struct {
		Message struct {
			Body struct {
				Mid string `json:"mid"`
			} `json:"body"`
		} `json:"message"`
	}
	if err := c.call(ctx, http.MethodPost, "/messages?chat_id="+url.QueryEscape(chatID), c.body(m), &res); err != nil {
		return "", err
	}
	return res.Message.Body.Mid, nil
}

func (c *MaxClient) Edit(ctx context.Context, _ string, messageID string, m Message) error {
	return c.call(ctx, http.MethodPut, "/messages?message_id="+url.QueryEscape(messageID), c.body(m), nil)
}

// VerifyInitData проверяет подпись данных мини-приложения (verifyWebAppData — тот же алгоритм, что
// в Telegram). Диалог с ботом в MAX — отдельный id: он приходит в поле chat, если мини-приложение
// открыли из этого диалога. Иначе ChatID пустой, и бот не знает, куда писать.
func (c *MaxClient) VerifyInitData(raw string, now time.Time) (WebAppUser, error) {
	u, vals, err := verifyWebAppData(c.token, raw, now)
	if err != nil {
		return u, err
	}
	var chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	}
	if json.Unmarshal([]byte(vals.Get("chat")), &chat) == nil && chat.ID != 0 && strings.EqualFold(chat.Type, "dialog") {
		u.ChatID = strconv.FormatInt(chat.ID, 10)
	}
	return u, nil
}

func (c *MaxClient) Answer(ctx context.Context, callbackID, text string) error {
	body := map[string]any{}
	if text != "" {
		body["notification"] = text
	}
	return c.call(ctx, http.MethodPost, "/answers?callback_id="+url.QueryEscape(callbackID), body, nil)
}

func (c *MaxClient) SetWebhook(ctx context.Context, hook string) error {
	return c.call(ctx, http.MethodPost, "/subscriptions", map[string]any{"url": hook, "secret": c.secret,
		"update_types": []string{"bot_started", "message_created", "message_callback"}}, nil)
}

type maxUser struct {
	UserID int64 `json:"user_id"`
	IsBot  bool  `json:"is_bot"`
}

func (u maxUser) id() string {
	if u.UserID == 0 {
		return ""
	}
	return strconv.FormatInt(u.UserID, 10)
}

// SetMenu — у ботов MAX нет кнопки меню с сайтом: мини-приложения там подключаются в кабинете бизнеса.
func (c *MaxClient) SetMenu(context.Context, string, string, string) error { return nil }

// SetProfile — в MAX описание и команды бота меняются только в кабинете бизнеса: PATCH /me
// с октября 2026 отвечает 404 «Path /me is not recognized». Нечего и пытаться при каждом запуске.
func (c *MaxClient) SetProfile(context.Context, map[string]Profile) error { return nil }

func (c *MaxClient) Parse(r *http.Request) (Update, bool, error) {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Max-Bot-Api-Secret")), []byte(c.secret)) != 1 {
		return Update{}, false, ErrForged
	}
	var in struct {
		Type    string  `json:"update_type"`
		ChatID  int64   `json:"chat_id"`
		Payload string  `json:"payload"`
		Locale  string  `json:"user_locale"`
		User    maxUser `json:"user"`
		Message *struct {
			Recipient struct {
				ChatID   int64  `json:"chat_id"`
				ChatType string `json:"chat_type"`
			} `json:"recipient"`
			Sender *maxUser `json:"sender"`
			Body   struct {
				Mid  string `json:"mid"`
				Text string `json:"text"`
			} `json:"body"`
		} `json:"message"`
		Callback *struct {
			ID      string  `json:"callback_id"`
			Payload string  `json:"payload"`
			User    maxUser `json:"user"`
		} `json:"callback"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return Update{}, false, err
	}
	u := Update{Platform: Max, Lang: lang2(in.Locale)}
	switch in.Type {
	case "bot_started":
		u.ChatID, u.UserID, u.Start, u.Payload = strconv.FormatInt(in.ChatID, 10), in.User.id(), true, in.Payload
	case "message_created":
		// отвечаем только человеку в личном диалоге: в общем чате бот не встревает в разговор,
		// а своё же сообщение не должно запускать ответ на него
		if in.Message == nil || in.Message.Recipient.ChatType != "dialog" || (in.Message.Sender != nil && in.Message.Sender.IsBot) {
			return Update{}, false, nil
		}
		u.ChatID, u.Text = strconv.FormatInt(in.Message.Recipient.ChatID, 10), in.Message.Body.Text
		if in.Message.Sender != nil {
			u.UserID = in.Message.Sender.id()
		}
		u.Payload, u.Start = splitStart(u.Text)
	case "message_callback":
		if in.Callback == nil || in.Message == nil {
			return Update{}, false, nil
		}
		u.ChatID, u.UserID, u.Callback, u.Data = strconv.FormatInt(in.Message.Recipient.ChatID, 10), in.Callback.User.id(), in.Callback.ID, in.Callback.Payload
		u.MessageID = in.Message.Body.Mid
	default:
		return Update{}, false, nil
	}
	return u, true, nil
}

// call — запрос к Bot API MAX; 403 — бот заблокирован или удалён из чата.
func (c *MaxClient) call(ctx context.Context, method, path string, body any, dst any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("max %s: %w", path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusForbidden:
		return ErrBlocked
	case resp.StatusCode != http.StatusOK:
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &e)
		return fmt.Errorf("max %s %s: http %d %s", method, strings.SplitN(path, "?", 2)[0], resp.StatusCode, e.Message)
	}
	// методы без результата отвечают {"success": false, "message": "..."} при отказе
	var ok struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &ok) == nil && ok.Success != nil && !*ok.Success {
		return fmt.Errorf("max %s: %s", strings.SplitN(path, "?", 2)[0], ok.Message)
	}
	if dst != nil {
		return json.Unmarshal(data, dst)
	}
	return nil
}
