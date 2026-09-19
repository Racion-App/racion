package service

import (
	"context"
	"errors"
	"io"
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
