// Package oauth — вход через внешние сервисы: VK ID, Яндекс, Google, GitHub, Apple.
// Провайдер включается, когда заданы его ключи; список для страницы входа зависит от страны по IP
// (для России только VK и Яндекс, остальным — все).
package oauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Profile — что мы узнаём о человеке у провайдера.
type Profile struct {
	ID     string
	Email  string
	Name   string
	Avatar string
}

// Provider — настройки одного сервиса.
type Provider struct {
	ID, Name         string
	ClientID, Secret string
	// Apple: секрет — подписанный JWT из ключа .p8
	AppleTeamID, AppleKeyID, AppleKey string
	PKCE                              bool // VK ID требует PKCE
	FormPost                          bool // Apple присылает код POST-формой
}

// Config — ключи из окружения; пустые пары отключают провайдера.
type Config struct {
	VKID, VKSecret         string
	YandexID, YandexSecret string
	GoogleID, GoogleSecret string
	GitHubID, GitHubSecret string
	AppleClientID          string // Services ID
	AppleTeamID            string
	AppleKeyID             string
	AppleKey               string // содержимое .p8 (PEM), переводы строк можно как \n
}

// Order — порядок кнопок на странице входа.
var Order = []string{"vk", "yandex", "google", "github", "apple"}

// Registry — включённые провайдеры.
type Registry struct {
	providers map[string]Provider
	client    *http.Client
}

func New(c Config) *Registry {
	r := &Registry{providers: map[string]Provider{}, client: &http.Client{Timeout: 15 * time.Second}}
	if c.VKID != "" && c.VKSecret != "" {
		r.providers["vk"] = Provider{ID: "vk", Name: "VK", ClientID: c.VKID, Secret: c.VKSecret, PKCE: true}
	}
	if c.YandexID != "" && c.YandexSecret != "" {
		r.providers["yandex"] = Provider{ID: "yandex", Name: "Яндекс", ClientID: c.YandexID, Secret: c.YandexSecret}
	}
	if c.GoogleID != "" && c.GoogleSecret != "" {
		r.providers["google"] = Provider{ID: "google", Name: "Google", ClientID: c.GoogleID, Secret: c.GoogleSecret}
	}
	if c.GitHubID != "" && c.GitHubSecret != "" {
		r.providers["github"] = Provider{ID: "github", Name: "GitHub", ClientID: c.GitHubID, Secret: c.GitHubSecret}
	}
	if c.AppleClientID != "" && c.AppleTeamID != "" && c.AppleKeyID != "" && c.AppleKey != "" {
		r.providers["apple"] = Provider{ID: "apple", Name: "Apple", ClientID: c.AppleClientID, AppleTeamID: c.AppleTeamID, AppleKeyID: c.AppleKeyID,
			AppleKey: strings.ReplaceAll(c.AppleKey, `\n`, "\n"), FormPost: true}
	}
	return r
}

// Available — провайдеры для страны: RU — только VK и Яндекс, остальным — все включённые.
func (r *Registry) Available(country string) []Provider {
	var out []Provider
	for _, id := range Order {
		p, ok := r.providers[id]
		if !ok {
			continue
		}
		if country == "RU" && id != "vk" && id != "yandex" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (r *Registry) Get(id string) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

func (r *Registry) Enabled() []string {
	var out []string
	for _, id := range Order {
		if _, ok := r.providers[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

// State — что мы запоминаем в cookie на время похода к провайдеру.
type State struct {
	Nonce    string `json:"n"`
	Verifier string `json:"v,omitempty"` // PKCE
	Plan     string `json:"p,omitempty"`
	Next     string `json:"x,omitempty"`
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func NewState(plan, next string, pkce bool) State {
	st := State{Nonce: randomString(24), Plan: plan, Next: next}
	if pkce {
		st.Verifier = randomString(48)
	}
	return st
}

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// AuthURL — куда отправить человека.
func (r *Registry) AuthURL(p Provider, redirect string, st State) string {
	q := url.Values{}
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", redirect)
	q.Set("state", st.Nonce)
	q.Set("response_type", "code")
	switch p.ID {
	case "vk":
		q.Set("scope", "email")
		q.Set("code_challenge", challenge(st.Verifier))
		q.Set("code_challenge_method", "S256")
		return "https://id.vk.com/authorize?" + q.Encode()
	case "yandex":
		return "https://oauth.yandex.ru/authorize?" + q.Encode()
	case "google":
		q.Set("scope", "openid email profile")
		q.Set("prompt", "select_account")
		return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
	case "github":
		q.Set("scope", "read:user user:email")
		return "https://github.com/login/oauth/authorize?" + q.Encode()
	case "apple":
		q.Set("scope", "name email")
		q.Set("response_mode", "form_post")
		return "https://appleid.apple.com/auth/authorize?" + q.Encode()
	}
	return ""
}

// Callback — что пришло от провайдера (query или form_post).
type Callback struct {
	Code     string
	State    string
	DeviceID string // VK ID
	User     string // Apple: JSON с именем только при первом входе
}

// Exchange — код на профиль.
func (r *Registry) Exchange(ctx context.Context, p Provider, redirect string, st State, cb Callback) (Profile, error) {
	if cb.State == "" || cb.State != st.Nonce {
		return Profile{}, errors.New("state mismatch")
	}
	if cb.Code == "" {
		return Profile{}, errors.New("no code")
	}
	switch p.ID {
	case "vk":
		return r.vk(ctx, p, redirect, st, cb)
	case "yandex":
		return r.yandex(ctx, p, redirect, cb.Code)
	case "google":
		return r.google(ctx, p, redirect, cb.Code)
	case "github":
		return r.github(ctx, p, redirect, cb.Code)
	case "apple":
		return r.apple(ctx, p, redirect, cb)
	}
	return Profile{}, errors.New("unknown provider")
}

func (r *Registry) postForm(ctx context.Context, u string, form url.Values, headers map[string]string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return r.do(req)
}

func (r *Registry) getJSON(ctx context.Context, u string, headers map[string]string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return r.do(req)
}

func (r *Registry) do(req *http.Request) (map[string]any, error) {
	req.Header.Set("User-Agent", "Racion/1.0 (+https://racion.app)")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%s: http %d, not json", req.URL.Host, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("%s: http %d: %s", req.URL.Host, resp.StatusCode, firstString(out, "error_description", "error", "message"))
	}
	if e := firstString(out, "error"); e != "" {
		return out, fmt.Errorf("%s: %s", req.URL.Host, firstString(out, "error_description", "error"))
	}
	return out, nil
}

func str(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case json.Number:
		return v.String()
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := str(m, k); s != "" {
			return s
		}
	}
	return ""
}

func sub(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

// --- VK ID (OAuth 2.1 + PKCE)

func (r *Registry) vk(ctx context.Context, p Provider, redirect string, st State, cb Callback) (Profile, error) {
	tok, err := r.postForm(ctx, "https://id.vk.com/oauth2/auth", url.Values{
		"grant_type": {"authorization_code"}, "code": {cb.Code}, "code_verifier": {st.Verifier}, "client_id": {p.ClientID},
		"device_id": {cb.DeviceID}, "redirect_uri": {redirect}, "state": {st.Nonce},
	}, nil)
	if err != nil {
		return Profile{}, err
	}
	access := str(tok, "access_token")
	info, err := r.postForm(ctx, "https://id.vk.com/oauth2/user_info", url.Values{"client_id": {p.ClientID}, "access_token": {access}}, nil)
	if err != nil {
		return Profile{}, err
	}
	u := sub(info, "user")
	if u == nil {
		return Profile{}, errors.New("vk: no user")
	}
	pr := Profile{ID: str(u, "user_id"), Email: str(u, "email"), Avatar: str(u, "avatar"),
		Name: strings.TrimSpace(str(u, "first_name") + " " + str(u, "last_name"))}
	if pr.ID == "" {
		pr.ID = str(tok, "user_id")
	}
	if pr.Email == "" {
		pr.Email = str(tok, "email")
	}
	return pr, nil
}

// --- Яндекс

func (r *Registry) yandex(ctx context.Context, p Provider, redirect, code string) (Profile, error) {
	tok, err := r.postForm(ctx, "https://oauth.yandex.ru/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {p.ClientID}, "client_secret": {p.Secret},
	}, nil)
	if err != nil {
		return Profile{}, err
	}
	info, err := r.getJSON(ctx, "https://login.yandex.ru/info?format=json", map[string]string{"Authorization": "OAuth " + str(tok, "access_token")})
	if err != nil {
		return Profile{}, err
	}
	pr := Profile{ID: str(info, "id"), Email: str(info, "default_email"), Name: firstString(info, "real_name", "display_name", "login")}
	if av := str(info, "default_avatar_id"); av != "" && info["is_avatar_empty"] != true {
		pr.Avatar = "https://avatars.yandex.net/get-yapic/" + av + "/islands-200"
	}
	return pr, nil
}

// --- Google

func (r *Registry) google(ctx context.Context, p Provider, redirect, code string) (Profile, error) {
	tok, err := r.postForm(ctx, "https://oauth2.googleapis.com/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {p.ClientID}, "client_secret": {p.Secret}, "redirect_uri": {redirect},
	}, nil)
	if err != nil {
		return Profile{}, err
	}
	info, err := r.getJSON(ctx, "https://openidconnect.googleapis.com/v1/userinfo", map[string]string{"Authorization": "Bearer " + str(tok, "access_token")})
	if err != nil {
		return Profile{}, err
	}
	return Profile{ID: str(info, "sub"), Email: str(info, "email"), Name: str(info, "name"), Avatar: str(info, "picture")}, nil
}

// --- GitHub

func (r *Registry) github(ctx context.Context, p Provider, redirect, code string) (Profile, error) {
	tok, err := r.postForm(ctx, "https://github.com/login/oauth/access_token", url.Values{
		"code": {code}, "client_id": {p.ClientID}, "client_secret": {p.Secret}, "redirect_uri": {redirect},
	}, nil)
	if err != nil {
		return Profile{}, err
	}
	h := map[string]string{"Authorization": "Bearer " + str(tok, "access_token")}
	info, err := r.getJSON(ctx, "https://api.github.com/user", h)
	if err != nil {
		return Profile{}, err
	}
	pr := Profile{ID: str(info, "id"), Email: str(info, "email"), Name: firstString(info, "name", "login"), Avatar: str(info, "avatar_url")}
	if pr.Email == "" {
		// почта может быть скрыта в профиле, но доступна по user:email
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
		req.Header.Set("Authorization", h["Authorization"])
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Racion/1.0")
		if resp, err := r.client.Do(req); err == nil {
			defer resp.Body.Close()
			var list []map[string]any
			if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&list) == nil {
				for _, e := range list {
					if e["primary"] == true && e["verified"] == true {
						pr.Email = str(e, "email")
						break
					}
				}
			}
		}
	}
	return pr, nil
}

// --- Apple

func appleSecret(p Provider) (string, error) {
	block, _ := pem.Decode([]byte(p.AppleKey))
	if block == nil {
		return "", errors.New("apple: bad key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("apple: %w", err)
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return "", errors.New("apple: key is not EC")
	}
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": p.AppleTeamID, "iat": now.Unix(), "exp": now.Add(24 * time.Hour).Unix(), "aud": "https://appleid.apple.com", "sub": p.ClientID,
	})
	t.Header["kid"] = p.AppleKeyID
	return t.SignedString(ec)
}

func (r *Registry) apple(ctx context.Context, p Provider, redirect string, cb Callback) (Profile, error) {
	secret, err := appleSecret(p)
	if err != nil {
		return Profile{}, err
	}
	tok, err := r.postForm(ctx, "https://appleid.apple.com/auth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {cb.Code}, "client_id": {p.ClientID}, "client_secret": {secret}, "redirect_uri": {redirect},
	}, nil)
	if err != nil {
		return Profile{}, err
	}
	// id_token подписан Apple; мы получили его напрямую по TLS от appleid.apple.com, поэтому читаем без проверки подписи
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(str(tok, "id_token"), claims); err != nil {
		return Profile{}, fmt.Errorf("apple: id_token: %w", err)
	}
	pr := Profile{ID: str(claims, "sub"), Email: str(claims, "email")}
	if cb.User != "" {
		var u struct {
			Name struct{ FirstName, LastName string } `json:"name"`
		}
		if json.Unmarshal([]byte(cb.User), &u) == nil {
			pr.Name = strings.TrimSpace(u.Name.FirstName + " " + u.Name.LastName)
		}
	}
	return pr, nil
}
