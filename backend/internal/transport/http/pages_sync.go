package http

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"racion/internal/i18n"
	"racion/internal/service"
)

// pagesLoop — сверка страниц с записанными версиями (service.PageVersions) при старте и раз в 10 минут:
// так дату и пинг в IndexNow получают и рецепты нового деплоя, и правки из админки, и готовые переводы.
func (s *Server) pagesLoop() {
	if s.svc.Pages == nil {
		return
	}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		if n, err := s.svc.Pages.Sync(ctx, s.pageList()); err != nil {
			s.log.Warn("pages sync", zap.Error(err))
		} else if n > 0 {
			s.log.Info("pages changed", zap.Int("count", n))
		}
		cancel()
		time.Sleep(10 * time.Minute)
	}
}

// pageList — рецепты базы и страницы банок с хешем содержимого. Хеш рецепта — весь его JSON с переводами:
// поправили шаг, фото или перевод — страница сменилась. Хеш страницы банок — хеши её рецептов по порядку.
func (s *Server) pageList() []service.Page {
	cat := s.svc.Catalog.Base()
	hashes := make(map[string]string, len(cat.Recipes))
	pages := make([]service.Page, 0, len(cat.Recipes)+len(jarHubs))
	for _, rc := range cat.Recipes {
		if rc.Hidden {
			continue
		}
		raw, _ := json.Marshal(rc)
		sum := sha1.Sum(raw)
		h := hex.EncodeToString(sum[:])
		hashes[rc.ID] = h
		var langs []i18n.Lang
		for _, l := range i18n.Langs {
			if hasLang(rc, l) {
				langs = append(langs, l)
			}
		}
		pages = append(pages, service.Page{Path: "/recipe/" + rc.ID, Hash: h, Langs: langs, Created: rc.Created})
	}
	for _, hub := range jarHubs {
		sum := sha1.New()
		for _, sd := range jarSections[hub.Key] {
			for _, id := range sd.Recipes {
				sum.Write([]byte(id + hashes[id]))
			}
		}
		pages = append(pages, service.Page{Path: hub.Path, Hash: hex.EncodeToString(sum.Sum(nil)), Langs: topicLangs})
	}
	return pages
}
