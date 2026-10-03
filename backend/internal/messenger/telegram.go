package messenger

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	Text string `json:"text"`
	Data string `json:"callback_data,omitempty"`
	URL  string `json:"url,omitempty"`
}

func tgBody(m Message) map[string]any {
	rows := make([][]tgButton, 0, len(m.Rows))
	for _, r := range m.Rows {
		row := make([]tgButton, 0, len(r))
		for _, b := range r {
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
				Lang string `json:"language_code"`
			} `json:"from"`
			Text string `json:"text"`
		} `json:"message"`
		Callback *struct {
			ID   string `json:"id"`
			Data string `json:"data"`
			From struct {
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
		return Update{Platform: Telegram, ChatID: strconv.FormatInt(in.Callback.Message.Chat.ID, 10), Lang: lang2(in.Callback.From.Lang),
			Callback: in.Callback.ID, Data: in.Callback.Data, MessageID: strconv.FormatInt(in.Callback.Message.MessageID, 10)}, true, nil
	case in.Message != nil && in.Message.Chat.Type == "private": // в группах бот не отвечает на разговоры
		u := Update{Platform: Telegram, ChatID: strconv.FormatInt(in.Message.Chat.ID, 10), Lang: lang2(in.Message.From.Lang), Text: in.Message.Text}
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
