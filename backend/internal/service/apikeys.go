package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"

	"racion/internal/domain"
)

// Ключи API: «rk_» + 32 случайных байта. В базе — sha256, сам ключ отдаётся один раз. Ключ действует от имени
// владельца с его правами (админ → полный доступ к /api/admin/*), поэтому создавать его может только админ.

type APIKeyRepo interface {
	Create(ctx context.Context, userID, name, prefix, hash string) (domain.APIKey, error)
	List(ctx context.Context, userID string) ([]domain.APIKey, error)
	Delete(ctx context.Context, userID, id string) error
	UserByHash(ctx context.Context, hash string) (*domain.User, error)
}

type APIKeys struct{ repo APIKeyRepo }

func NewAPIKeys(repo APIKeyRepo) *APIKeys { return &APIKeys{repo: repo} }

const keyPrefix = "rk_"

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func (a *APIKeys) Create(ctx context.Context, userID, name string) (domain.APIKey, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n == 0 || n > 40 {
		return domain.APIKey{}, domain.Invalid("admin.keys.err.name")
	}
	if list, err := a.repo.List(ctx, userID); err == nil && len(list) >= 10 {
		return domain.APIKey{}, domain.Invalid("admin.keys.err.limit")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return domain.APIKey{}, err
	}
	key := keyPrefix + base64.RawURLEncoding.EncodeToString(raw)
	k, err := a.repo.Create(ctx, userID, name, key[:8]+"…", hashKey(key))
	if err != nil {
		return k, err
	}
	k.Key = key
	return k, nil
}

func (a *APIKeys) List(ctx context.Context, userID string) ([]domain.APIKey, error) {
	return a.repo.List(ctx, userID)
}

func (a *APIKeys) Delete(ctx context.Context, userID, id string) error {
	return a.repo.Delete(ctx, userID, id)
}

// Auth — владелец ключа из заголовка Authorization: Bearer rk_…; nil, если это не наш ключ.
func (a *APIKeys) Auth(ctx context.Context, header string) (*domain.User, error) {
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))
	if !strings.HasPrefix(token, keyPrefix) || len(token) < 20 {
		return nil, nil
	}
	u, err := a.repo.UserByHash(ctx, hashKey(token))
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	return u, err
}
