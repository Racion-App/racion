package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"racion/internal/domain"
	"racion/internal/media"
)

// Media — загрузка фото пользователей: квота по человеку, виды recipe | comment | avatar.

const uploadsPerHour = 40

type Media struct {
	store *media.Store
	mu    sync.Mutex
	usage map[string][]time.Time
}

func NewMedia(store *media.Store) *Media {
	return &Media{store: store, usage: map[string][]time.Time{}}
}

func (m *Media) Enabled() bool { return m != nil && m.store.Enabled() }

// Owns — ссылка ведёт в наше хранилище (для рецептов, комментариев, аватара).
func (m *Media) Owns(url string) bool { return m != nil && m.store.Owns(url) }

func (m *Media) Upload(ctx context.Context, userID, kind string, r io.Reader) (media.Photo, error) {
	if !m.Enabled() {
		return media.Photo{}, domain.Invalid("photo.off")
	}
	switch kind {
	case "recipe", "comment", "avatar", "offer": // offer — картинка товара для рекламы, только админ (проверка в транспорте)
	default:
		return media.Photo{}, domain.Invalid("photo.bad")
	}
	if !m.allow(userID) {
		return media.Photo{}, domain.Invalid("photo.limit")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	p, err := m.store.Upload(ctx, kind, r)
	switch {
	case errors.Is(err, media.ErrBadImage):
		return p, domain.Invalid("photo.bad")
	case errors.Is(err, media.ErrTooBig):
		return p, domain.Invalid("photo.big")
	}
	return p, err
}

func (m *Media) allow(userID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	keep := m.usage[userID][:0]
	for _, t := range m.usage[userID] {
		if now.Sub(t) < time.Hour {
			keep = append(keep, t)
		}
	}
	if len(keep) >= uploadsPerHour {
		m.usage[userID] = keep
		return false
	}
	m.usage[userID] = append(keep, now)
	return true
}

// Import — фото по ссылке или data-URI (для API: скрипт отдаёт адрес картинки или base64, сервер сам
// скачивает и пережимает). Ссылки в наше хранилище возвращаются как есть.
func (m *Media) Import(ctx context.Context, userID, kind, src string) (media.Photo, error) {
	src = strings.TrimSpace(src)
	if m.Owns(src) {
		return media.Photo{URL: src}, nil
	}
	if strings.HasPrefix(src, "data:") {
		comma := strings.IndexByte(src, ',')
		if comma < 0 {
			return media.Photo{}, domain.Invalid("photo.bad")
		}
		raw, err := base64.StdEncoding.DecodeString(src[comma+1:])
		if err != nil {
			return media.Photo{}, domain.Invalid("photo.bad")
		}
		return m.Upload(ctx, userID, kind, bytes.NewReader(raw))
	}
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		return media.Photo{}, domain.Invalid("photo.bad")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return media.Photo{}, domain.Invalid("photo.bad")
	}
	req.Header.Set("User-Agent", "Racion/1.0 (+https://racion.app)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return media.Photo{}, fmt.Errorf("photo fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return media.Photo{}, fmt.Errorf("photo fetch: http %d", resp.StatusCode)
	}
	return m.Upload(ctx, userID, kind, io.LimitReader(resp.Body, media.MaxUpload+1))
}
