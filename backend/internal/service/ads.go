package service

import (
	"context"
	"sync"
	"time"
)

// Ads — общий выключатель рекламы: партнёрские ссылки «где купить», доставки в корзине и точечные
// предложения. Пока договорённостей с рекламодателями нет, всё выключено (ключ settings ads.enabled,
// по умолчанию «0»). Включается во вкладке «Реклама» админки.
type Ads struct {
	settings SettingsRepo
	mu       sync.Mutex
	on       bool
	at       time.Time
}

func NewAds(settings SettingsRepo) *Ads { return &Ads{settings: settings} }

const adsKey = "ads.enabled"

// Enabled — кэш на 30 секунд: проверка идёт на каждой странице рецепта и в корзине.
func (a *Ads) Enabled(ctx context.Context) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if time.Since(a.at) < 30*time.Second {
		return a.on
	}
	v, err := a.settings.Get(ctx, adsKey)
	a.on, a.at = err == nil && v == "1", time.Now()
	return a.on
}

func (a *Ads) SetEnabled(ctx context.Context, on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	if err := a.settings.Set(ctx, adsKey, v); err != nil {
		return err
	}
	a.mu.Lock()
	a.on, a.at = on, time.Now()
	a.mu.Unlock()
	return nil
}
