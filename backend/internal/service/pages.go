package service

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/i18n"
)

// PageVersionRepo — хеши страниц и даты их смены.
type PageVersionRepo interface {
	All(ctx context.Context) (map[string]domain.PageVersion, error)
	Save(ctx context.Context, rows map[string]domain.PageVersion) error
}

// Page — страница для сверки: адрес без языкового префикса, хеш содержимого и языки, на которых она есть.
// Created — когда страница появилась (у рецепта created_at): дата для первой сверки.
type Page struct {
	Path    string
	Hash    string
	Langs   []i18n.Lang
	Created time.Time
}

// PageVersions — когда страница менялась по-настоящему. Сайт сверяет хеши страниц с записанными при старте и
// раз в несколько минут: новая или изменившаяся страница получает текущую дату, а её адреса на всех её языках
// уходят в IndexNow. Так каждый деплой с новыми рецептами сам сообщает о них Яндексу и Bing, и правки из
// админки тоже. Первая сверка на пустой таблице только запоминает состояние: даты — дни появления, без пинга.
type PageVersions struct {
	repo   PageVersionRepo
	index  *IndexNow
	log    *zap.Logger
	mu     sync.RWMutex
	m      map[string]domain.PageVersion
	loaded bool
}

func NewPageVersions(repo PageVersionRepo, index *IndexNow, log *zap.Logger) *PageVersions {
	return &PageVersions{repo: repo, index: index, log: log, m: map[string]domain.PageVersion{}}
}

// Sync сверяет страницы и возвращает, сколько записано новых и изменившихся.
func (p *PageVersions) Sync(ctx context.Context, pages []Page) (int, error) {
	if p == nil || p.repo == nil {
		return 0, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.loaded {
		m, err := p.repo.All(ctx)
		if err != nil {
			return 0, err
		}
		p.m, p.loaded = m, true
	}
	baseline := len(p.m) == 0
	now := time.Now().UTC()
	upd := map[string]domain.PageVersion{}
	var notify []string
	for _, pg := range pages {
		if old, ok := p.m[pg.Path]; ok && old.Hash == pg.Hash {
			continue
		}
		at := now
		if baseline && !pg.Created.IsZero() {
			at = pg.Created.UTC()
		}
		upd[pg.Path] = domain.PageVersion{Hash: pg.Hash, Changed: at}
		if !baseline {
			for _, l := range pg.Langs {
				notify = append(notify, langPath(l, pg.Path))
			}
		}
	}
	if len(upd) == 0 {
		return 0, nil
	}
	if err := p.repo.Save(ctx, upd); err != nil {
		return 0, err
	}
	for k, v := range upd {
		p.m[k] = v
	}
	if len(notify) > 0 && p.index != nil {
		p.index.Notify(notify...)
	}
	return len(upd), nil
}

// Changed — когда страница менялась в последний раз; нулевое время — неизвестно.
func (p *PageVersions) Changed(path string) time.Time {
	if p == nil {
		return time.Time{}
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.m[path].Changed
}

// langPath — адрес на языке: русский без префикса, остальные — /en/…
func langPath(l i18n.Lang, path string) string {
	if l == i18n.RU {
		return path
	}
	return "/" + string(l) + path
}
