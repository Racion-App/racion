package vkusvill

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func server(t *testing.T, reply string, check func(body map[string]any)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "Racion/") {
			t.Errorf("User-Agent %q: без него их защита отвечает 403", ua)
		}
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if check != nil {
			check(body)
		}
		_, _ = io.WriteString(w, reply)
	}))
}

const okText = `{\"ok\":true,\"data\":{\"link\":\"https://vkusvill.ru/?share_basket=360196399\"}}`

func TestCartLinkJSON(t *testing.T) {
	s := server(t, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"`+okText+`"}]}}`, func(b map[string]any) {
		params := b["params"].(map[string]any)
		if params["name"] != "vkusvill_cart_link_create" {
			t.Errorf("инструмент %v", params["name"])
		}
		products := params["arguments"].(map[string]any)["products"].([]any)
		if len(products) != CartMax {
			t.Errorf("ушло %d товаров, ждали обрезку до %d", len(products), CartMax)
		}
	})
	defer s.Close()
	items := make([]CartItem, 25)
	for i := range items {
		items[i] = CartItem{XMLID: 1000 + i, Q: 1}
	}
	link, err := New(s.URL).CartLink(context.Background(), items)
	if err != nil || link != "https://vkusvill.ru/?share_basket=360196399" {
		t.Fatalf("link=%q err=%v", link, err)
	}
}

func TestCartLinkSSE(t *testing.T) {
	s := server(t, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\""+okText+"\"}]}}\n\n", nil)
	defer s.Close()
	link, err := New(s.URL).CartLink(context.Background(), []CartItem{{XMLID: 88523, Q: 2}})
	if err != nil || !strings.Contains(link, "share_basket=") {
		t.Fatalf("link=%q err=%v", link, err)
	}
}

func TestCartLinkErrors(t *testing.T) {
	s := server(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"Number must be less than or equal to 40"}}`, nil)
	defer s.Close()
	if _, err := New(s.URL).CartLink(context.Background(), []CartItem{{XMLID: 1, Q: 99}}); err == nil || !strings.Contains(err.Error(), "40") {
		t.Fatalf("ждали ошибку сервера, получили %v", err)
	}
	if _, err := New(s.URL).CartLink(context.Background(), nil); err == nil {
		t.Fatal("пустая корзина должна быть ошибкой")
	}
	bad := server(t, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"ok\":true,\"data\":{\"link\":\"https://evil.example/\"}}"}]}}`, nil)
	defer bad.Close()
	if _, err := New(bad.URL).CartLink(context.Background(), []CartItem{{XMLID: 1, Q: 1}}); err == nil {
		t.Fatal("ссылка не на vkusvill.ru не должна уходить человеку")
	}
}

// reply — ответ MCP с JSON-текстом инструмента, как его отдаёт их сервер.
func reply(inner any) string {
	text, _ := json.Marshal(inner)
	out, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": string(text)}}, "isError": false}})
	return string(out)
}

func TestProduct(t *testing.T) {
	// при акции обычная цена лежит в old: неделю считаем по ней, а не со скидкой по карте
	s := server(t, reply(map[string]any{"ok": true, "data": map[string]any{"id": 64315, "xml_id": 64315, "name": "Хлопья овсяные, 400&nbsp;г",
		"url": "https://vkusvill.ru/goods/khlopya-ovsyanye-64315/", "unit": "шт", "price": map[string]any{"current": 52, "old": 65}, "weight": map[string]any{"value": 0.4, "unit": "кг"}}}),
		func(b map[string]any) {
			p := b["params"].(map[string]any)
			if p["name"] != "vkusvill_product_details" || p["arguments"].(map[string]any)["id"] != float64(64315) {
				t.Errorf("запрос %v", p)
			}
		})
	defer s.Close()
	p, err := New(s.URL).Product(context.Background(), 64315)
	if err != nil || p.Price != 65 || p.Weight != 0.4 || p.Unit != "шт" || p.Name != "Хлопья овсяные, 400 г" || p.XMLID != 64315 {
		t.Fatalf("%+v %v", p, err)
	}
	gone := server(t, reply(map[string]any{"ok": false, "error": map[string]any{"code": "invalid_input", "message": "Товар не найден", "http_status": 404}}), nil)
	defer gone.Close()
	if _, err := New(gone.URL).Product(context.Background(), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("снятый товар — ErrNotFound, а не %v", err)
	}
	busy := server(t, reply(map[string]any{"ok": false, "error": map[string]any{"code": "rate_limit", "message": "Слишком много запросов", "http_status": 429}}), nil)
	defer busy.Close()
	if _, err := New(busy.URL).Product(context.Background(), 1); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("перегрузка их сервера — не «товар снят»: %v", err)
	}
}
