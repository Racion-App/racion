// Точка сборки: конфиг, логгер, БД, каталог, фоновые синхронизации, сервисы и транспорты.
// Транспорт здесь один — HTTP; очередь или пакетное задание подключаются рядом из тех же service.Services.
package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"racion/internal/ai"
	"racion/internal/config"
	"racion/internal/db"
	"racion/internal/geo"
	"racion/internal/localprices"
	"racion/internal/logger"
	"racion/internal/media"
	"racion/internal/planner"
	"racion/internal/rosstat"
	"racion/internal/seed"
	"racion/internal/service"
	"racion/internal/storage/postgres"
	transport "racion/internal/transport/http"
)

func main() {
	cfg := config.Load()
	ring := logger.NewRing(1000)
	log := logger.WithRing(logger.New(cfg.LogLevel, cfg.LogFormat), ring)
	defer func() { _ = log.Sync() }()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("connect", zap.Error(err))
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal("migrate", zap.Error(err))
	}
	if err := seed.Run(ctx, pool); err != nil {
		log.Fatal("seed", zap.Error(err))
	}
	planner.SetOccasions(seed.Occasions())
	catalog, err := planner.LoadCatalog(ctx, pool)
	if err != nil {
		log.Fatal("catalog", zap.Error(err))
	}
	log.Info("catalog", zap.Int("recipes", len(catalog.Recipes)), zap.Int("ingredients", len(catalog.Ingredients)), zap.Int("stores", len(catalog.StoreList)))

	// Страна по IP: база DB-IP в БД, обновление раз в месяц в фоне.
	geoResolver := &geo.Resolver{Dir: cfg.GeoDir}
	geoResolver.Start(ctx, pool, log.Named("geo"))

	// Ценник Росстата: что есть в БД — сразу; обновление — в фоне, раз в сутки, без блокировки старта.
	if pb, err := planner.LoadPriceBook(ctx, pool); err != nil {
		log.Error("pricebook", zap.Error(err))
	} else if pb != nil {
		catalog.SetPriceBook(pb)
		log.Info("pricebook", zap.String("period", pb.Period), zap.Int("regions", len(pb.Regions)))
	}
	go every(ctx, 6*time.Hour, func() {
		if !rosstat.Stale(ctx, pool) {
			return
		}
		sctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		if err := rosstat.Sync(sctx, pool, log.Named("rosstat")); err != nil {
			log.Warn("rosstat sync failed, keeping previous prices", zap.Error(err))
			return
		}
		if pb, err := planner.LoadPriceBook(ctx, pool); err == nil && pb != nil {
			catalog.SetPriceBook(pb)
		}
	})

	// Живые цены других стран (BLS, Белстат, Бюро нацстатистики Казахстана, Eurostat, ONS) — тем же способом.
	go every(ctx, 6*time.Hour, func() {
		packs := map[string]localprices.PackInfo{}
		for id, ing := range catalog.Ingredients {
			packs[id] = localprices.PackInfo{Pack: ing.Pack, Unit: ing.Unit, Category: ing.Category, Local: ing.Prices}
		}
		sctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
		if updated := localprices.Sync(sctx, pool, packs, log.Named("localprices")); len(updated) > 0 {
			if fresh, err := planner.LoadCatalog(ctx, pool); err == nil {
				for _, c := range updated {
					if lp := fresh.LocalPrices(c); lp != nil {
						catalog.SetLocalPrices(lp)
					}
				}
			}
		}
	})

	// Сервисы поверх хранилища; транспорты получают их целиком.
	store := postgres.New(pool)

	// Уборка: просроченные сессии и старая аналитика, раз в сутки.
	go every(ctx, 24*time.Hour, func() {
		sctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if n, err := store.Cleanup(sctx, 180*24*time.Hour); err != nil {
			log.Warn("cleanup", zap.Error(err))
		} else if n > 0 {
			log.Info("cleanup", zap.Int64("rows", n))
		}
	})
	// Каталог — через атомарную ссылку: админка после правки рецептов перечитывает его из БД и подменяет целиком.
	catalogRef := planner.NewCatalogRef(catalog)
	services := service.New(service.Repos{
		Users: store.Users, Sessions: store.Sessions, Plans: store.Plans, Dislikes: store.Dislikes, Checks: store.Checks,
		Purchases: store.Purchases, Extras: store.Extras, UserRecipes: store.UserRecipes, Events: store.Events,
		PlanMembers: store.PlanMembers, Push: store.Push, Settings: store.Settings, Social: store.Social, Households: store.Households, Admin: store.Admin, Collections: store.Collections, Partners: store.Partners, APIKeys: store.APIKeys,
	}, catalogRef, cfg.PushContact, cfg.BaseURL)
	services.Admin = service.NewAdmin(store.Admin, store.Users, cfg.AdminEmails)
	// замены продуктов: таблица из seed/data/substitutes.json
	subsTable := map[string][]service.SubEntry{}
	for id, list := range seed.Substitutes() {
		for _, e := range list {
			subsTable[id] = append(subsTable[id], service.SubEntry{ID: e.ID, Ratio: e.Ratio, Note: e.Note, NoteEn: e.NoteEn, Not: e.Not})
		}
	}
	services.Subs = service.NewSubstitutes(subsTable, services.Recipes)
	// Рецепты базы из админки: после правки каталог перечитывается из БД с теми же ценниками
	services.CatalogAdmin = service.NewCatalogAdmin(store.CatalogRecipes, catalogRef, func(ctx context.Context) error {
		fresh, err := catalogRef.Load().Reload(ctx, pool)
		if err != nil {
			return err
		}
		catalogRef.Store(fresh)
		return nil
	})
	// Нейросети: пул провайдеров (бесплатные уровни Mistral/Gemini/Groq/OpenRouter, OpenAI, локальный прокси);
	// один пул на модерацию, помощника и очередь переводов своих рецептов
	aiPool := ai.NewPoolFromKeys(ai.ProviderKeys{Order: splitList(cfg.AIOrder), Mistral: cfg.MistralKey, Gemini: cfg.GeminiKey, Groq: cfg.GroqKey, OpenRouter: cfg.OpenRouterKey, OpenAI: cfg.OpenAIKey, LocalURL: cfg.LocalAIURL, Models: parseModels(cfg.AIModels, cfg.OpenAIModel)})
	var checker service.RecipeChecker
	if aiPool.Enabled() {
		checker = aiPool
	}
	services.Moderation = service.NewModeration(store.UserRecipes, store.UserRecipes, store.Users, checker, log.Named("moderation"))
	services.Translations = service.NewTranslations(store.Translations, store.UserRecipes, store.UserRecipes, aiPool, log.Named("translations"))
	services.Recipes.SetTranslations(services.Translations)
	services.PlanChat = service.NewPlanChat(aiPool, services.Plans)
	go services.Translations.Run(ctx)
	if err := services.Partners.Seed(ctx); err != nil {
		log.Warn("partners seed", zap.Error(err))
	}
	// Фото в S3/MinIO: без S3_ENDPOINT загрузка выключена, всё остальное работает
	if mediaStore, err := media.New(media.Config{Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, Bucket: cfg.S3Bucket, Secure: cfg.S3Secure, PublicURL: cfg.S3PublicURL}); err != nil {
		log.Warn("media", zap.Error(err))
	} else if mediaStore != nil {
		if err := mediaStore.Init(ctx); err != nil {
			log.Warn("media init", zap.Error(err))
		} else {
			services.Media = service.NewMedia(mediaStore)
			services.Accounts.SetMedia(services.Media.Owns)
			services.Recipes.SetMedia(services.Media.Owns)
			services.Social.SetMedia(services.Media.Owns)
			log.Info("media on", zap.String("bucket", cfg.S3Bucket))
		}
	}
	if aiPool.Enabled() {
		services.AI = service.NewAssistant(aiPool)
		names := []string{}
		for _, p := range aiPool.Status() {
			names = append(names, p.Name+"/"+p.Model)
		}
		log.Info("ai on", zap.Strings("providers", names))
	}

	// Push: ключи VAPID и проход по напоминаниям раз в 10 минут.
	if err := services.Notify.Init(ctx); err != nil {
		log.Warn("push init", zap.Error(err))
	}
	go every(ctx, 10*time.Minute, func() {
		sctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if n, err := services.Notify.Tick(sctx, time.Now()); err != nil {
			log.Warn("push tick", zap.Error(err))
		} else if n > 0 {
			log.Info("push sent", zap.Int("count", n))
		}
	})

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: transport.New(transport.Deps{
			Services: services, Log: log.Named("http"), Geo: geoResolver,
			Health:  func() error { return store.Ping(context.Background()) },
			BaseURL: cfg.BaseURL,
			Metrika: cfg.MetrikaID, Contact: cfg.LegalEmail, Images: cfg.ImagesDir,
			Logs: ring,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	go func() {
		log.Info("listen", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("serve", zap.Error(err))
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// every запускает fn сразу и затем по тикеру, пока не отменён контекст.
func every(ctx context.Context, d time.Duration, fn func()) {
	fn()
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn()
		}
	}
}

func splitList(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// parseModels — «mistral=ministral-14b-latest,gemini=…» → map; OPENAI_MODEL — как раньше.
func parseModels(s, openai string) map[string]string {
	m := map[string]string{"openai": openai}
	for _, kv := range splitList(s) {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m
}
