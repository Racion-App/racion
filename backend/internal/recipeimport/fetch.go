// Package recipeimport достаёт рецепт со страницы чужого сайта: скачивает её, читает разметку schema.org
// (JSON-LD или microdata) и разбирает строки ингредиентов. Сопоставление с базой продуктов — в matcher.go.
package recipeimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html/charset"
)

var (
	ErrURL     = errors.New("recipeimport: bad url")
	ErrBlocked = errors.New("recipeimport: address not allowed")
	ErrFetch   = errors.New("recipeimport: fetch failed")
	ErrNotHTML = errors.New("recipeimport: not an html page")
)

const (
	maxPage      = 4 << 20 // больше страницы рецепта не бывают; остальное — не страница рецепта
	maxRedirects = 5
	userAgent    = "Mozilla/5.0 (compatible; RacionBot/1.0; +https://racion.app/features)"
)

// Page — скачанная страница в UTF-8 и адрес, на котором она в итоге оказалась (после переадресаций).
type Page struct {
	URL  string
	HTML []byte
}

// Fetcher ходит только наружу. Адрес вводит человек, поэтому проверка стоит на уровне сокета: имя
// резолвится при каждом соединении, и локальная сеть, петля, link-local и служебные диапазоны отсекаются
// уже после резолва. Переадресация внутрь или DNS-ребиндинг через ту же проверку не пройдут.
type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher { return newFetcher(false) }

// newFetcher(true) — для тестов на httptest-сервере в 127.0.0.1.
func newFetcher(allowLocal bool) *Fetcher {
	dialer := &net.Dialer{
		Timeout: 6 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			if allowLocal {
				return nil
			}
			return checkDial(address)
		},
	}
	tr := &http.Transport{
		Proxy:                  nil, // прокси из окружения обошёл бы проверку адреса
		DialContext:            dialer.DialContext,
		TLSHandshakeTimeout:    6 * time.Second,
		ResponseHeaderTimeout:  10 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		MaxIdleConns:           8,
		IdleConnTimeout:        30 * time.Second,
	}
	return &Fetcher{client: &http.Client{
		Transport: tr,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("%w: too many redirects", ErrFetch)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrBlocked
			}
			return nil
		},
	}}
}

// ParseURL проверяет, что человек ввёл адрес страницы: http(s), есть имя хоста, порт обычный.
// Адрес без схемы («eda.ru/recepty/…») дополняется https://.
func ParseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 {
		return nil, ErrURL
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, ErrURL
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		return nil, ErrBlocked
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return nil, ErrBlocked
	}
	if a, err := netip.ParseAddr(host); err == nil && !public(a) {
		return nil, ErrBlocked
	}
	if !strings.Contains(host, ".") {
		return nil, ErrURL // «backend», «minio» и прочие имена внутри docker-сети
	}
	u.Fragment = ""
	return u, nil
}

func (f *Fetcher) Fetch(ctx context.Context, raw string) (Page, error) {
	u, err := ParseURL(raw)
	if err != nil {
		return Page{}, err
	}
	return f.fetchURL(ctx, u.String())
}

func (f *Fetcher) fetchURL(ctx context.Context, u string) (Page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Page{}, ErrURL
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
	req.Header.Set("Accept-Language", "ru,en;q=0.8")
	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			return Page{}, ErrBlocked
		}
		return Page{}, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Page{}, fmt.Errorf("%w: http %d", ErrFetch, resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if mt, _, err := mime.ParseMediaType(ct); ct != "" && (err != nil || (mt != "text/html" && mt != "application/xhtml+xml")) {
		return Page{}, ErrNotHTML
	}
	// кодировка из заголовка, <meta charset> или по содержимому: часть русских сайтов до сих пор в windows-1251
	body, err := charset.NewReader(io.LimitReader(resp.Body, maxPage+1), ct)
	if err != nil {
		return Page{}, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	b, err := io.ReadAll(io.LimitReader(body, maxPage+1))
	if err != nil {
		return Page{}, fmt.Errorf("%w: %v", ErrFetch, err)
	}
	if len(b) > maxPage {
		return Page{}, fmt.Errorf("%w: page too large", ErrFetch)
	}
	return Page{URL: resp.Request.URL.String(), HTML: b}, nil
}

// checkDial — адрес соединения уже после резолва имени: только публичные адреса и порты 80/443.
func checkDial(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return ErrBlocked
	}
	if port != "80" && port != "443" {
		return ErrBlocked
	}
	a, err := netip.ParseAddr(host)
	if err != nil || !public(a) {
		return ErrBlocked
	}
	return nil
}

var reserved = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // CGNAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64: внутри может оказаться любой IPv4
	netip.MustParsePrefix("2001:db8::/32"),
}

func public(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsMulticast() || a.IsUnspecified() {
		return false
	}
	for _, p := range reserved {
		if p.Contains(a) {
			return false
		}
	}
	return true
}
