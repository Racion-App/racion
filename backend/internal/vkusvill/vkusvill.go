// Package vkusvill — открытые инструменты MCP-сервера ВкусВилла (mcp.vkusvill.ru): сейчас нужна только
// ссылка на корзину. Поиск товаров нужен один раз, при сопоставлении продуктов, и живёт в скрипте.
package vkusvill

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultURL = "https://mcp.vkusvill.ru/mcp"

// CartMax — больше товаров их инструмент не принимает. Ссылка заменяет корзину человека, а не
// добавляет к ней, поэтому разбить длинный список на несколько ссылок нельзя.
const CartMax = 20

type Client struct {
	url  string
	http *http.Client
}

func New(url string) *Client {
	if url == "" {
		url = DefaultURL
	}
	return &Client{url: url, http: &http.Client{Timeout: 25 * time.Second}}
}

// CartItem — товар и количество: штуки для упаковок, килограммы для весового.
type CartItem struct {
	XMLID int     `json:"xml_id"`
	Q     float64 `json:"q"`
}

// CartLink — ссылка, открыв которую человек получает готовую корзину во ВкусВилле.
func (c *Client) CartLink(ctx context.Context, items []CartItem) (string, error) {
	if len(items) == 0 {
		return "", errors.New("vkusvill: пустая корзина")
	}
	if len(items) > CartMax {
		items = items[:CartMax]
	}
	var out struct {
		OK   bool `json:"ok"`
		Data struct {
			Link string `json:"link"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := c.call(ctx, "vkusvill_cart_link_create", map[string]any{"products": items}, &out); err != nil {
		return "", err
	}
	if !out.OK || !strings.HasPrefix(out.Data.Link, "https://vkusvill.ru/") {
		msg := "нет ссылки в ответе"
		if out.Error != nil {
			msg = out.Error.Message
		}
		return "", fmt.Errorf("vkusvill: %s", msg)
	}
	return out.Data.Link, nil
}

// call — tools/call по JSON-RPC. Их защита отбивает запросы без внятного User-Agent, поэтому
// представляемся явно; ответ бывает и обычным JSON, и потоком событий с одной строкой data.
func (c *Client) call(ctx context.Context, tool string, args any, dst any) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", "Racion/1.0 (+https://racion.app/developers)")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("vkusvill: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vkusvill: http %d", resp.StatusCode)
	}
	raw = sseData(raw)
	var rpc struct {
		Result *struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &rpc); err != nil {
		return fmt.Errorf("vkusvill: ответ не JSON: %w", err)
	}
	if rpc.Error != nil {
		return fmt.Errorf("vkusvill: %s", rpc.Error.Message)
	}
	if rpc.Result == nil || len(rpc.Result.Content) == 0 {
		return errors.New("vkusvill: пустой ответ")
	}
	if rpc.Result.IsError {
		return fmt.Errorf("vkusvill: %s", rpc.Result.Content[0].Text)
	}
	return json.Unmarshal([]byte(rpc.Result.Content[0].Text), dst)
}

// sseData вынимает JSON из ответа-потока («data: {...}»); обычный JSON возвращает как есть.
func sseData(raw []byte) []byte {
	s := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(s, "event:") && !strings.HasPrefix(s, "data:") {
		return raw
	}
	var parts []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "data:") {
			parts = append(parts, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return []byte(strings.Join(parts, ""))
}
