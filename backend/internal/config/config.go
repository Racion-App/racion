// Package config читает настройки из окружения.
package config

import (
	"os"
	"time"
)

type Config struct {
	DatabaseURL     string
	HTTPAddr        string
	LogLevel        string // debug | info | warn | error
	LogFormat       string // json | console
	BaseURL         string // публичный адрес сайта для canonical и sitemap; пусто — из заголовков запроса
	MetrikaID       string // счётчик Яндекс Метрики на SSR-страницах; пусто — не подключать
	LegalEmail      string // почта для жалоб и вопросов о данных
	ImagesDir       string // фото блюд для карточек превью ссылок (IMAGES_DIR; в докере — том фронтенда)
	GeoDir          string // папка файла базы городов DB-IP (пусто — только страна)
	PushContact     string // контакт оператора для VAPID (mailto:…), его видят push-сервисы
	VkusvillMCP     string // MCP-сервер ВкусВилла для корзины ссылкой; пусто — mcp.vkusvill.ru
	OpenAIKey       string // ключ OpenAI для переводов и улучшения рецептов; пусто — функции ИИ выключены
	OpenAIModel     string // модель для ИИ; по умолчанию gpt-5-nano
	MistralKey      string // бесплатные уровни провайдеров: любой из ключей включает помощника и переводы
	GeminiKey       string
	GroqKey         string
	OpenRouterKey   string
	LocalAIURL      string // OpenAI-совместимый прокси без ключа (ima2), пусто — не используется
	AIOrder         string // порядок провайдеров через запятую: mistral,gemini,groq,openrouter,openai,local
	AIModels        string // переопределить модели: mistral=ministral-14b-latest,gemini=gemini-2.5-flash
	AdminEmails     string // почты администраторов через запятую: им открыта /admin
	MailHost        string // SMTP для писем (восстановление пароля); пусто — письма только в лог
	MailPort        string
	MailUser        string
	MailPass        string
	MailFrom        string
	S3Endpoint      string // minio:9000 или s3.example.com; пусто — фото выключены
	S3AccessKey     string
	S3SecretKey     string
	S3Bucket        string
	S3Secure        bool   // https к хранилищу
	S3PublicURL     string // база публичных ссылок на фото: /media (nginx → MinIO) или адрес CDN
	OAuth           OAuth
	Bots            Bots
	ShutdownTimeout time.Duration
}

// Bots — боты в Telegram и MAX: список покупок по отделам. Пустой токен выключает мессенджер.
type Bots struct {
	TelegramToken   string // от @BotFather
	TelegramName    string // имя бота без @: из него ссылка t.me/<имя>
	MaxToken        string // из кабинета «MAX для бизнеса»
	MaxName         string // ник бота: ссылка max.ru/<ник>
	Secret          string // секрет вебхуков (A-Z, a-z, 0-9, _ и -); пусто — выводится из токена
	TelegramAPI     string // другой адрес Bot API для локального стенда; пусто — боевой
	TelegramUpdates string // poll (по умолчанию) — сервер сам забирает обновления; webhook — Telegram шлёт их на сервер
	MaxAPI          string
}

// OAuth — ключи входа через внешние сервисы; пустая пара выключает провайдера.
type OAuth struct {
	VKID, VKSecret         string
	YandexID, YandexSecret string
	GoogleID, GoogleSecret string
	GitHubID, GitHubSecret string
	AppleClientID          string // Services ID вида app.racion.web
	AppleTeamID            string
	AppleKeyID             string
	AppleKey               string // содержимое .p8 одной строкой с

}

func Load() Config {
	return Config{
		DatabaseURL:   env("DATABASE_URL", "postgres://racion:racion@localhost:5432/racion?sslmode=disable"),
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		LogLevel:      env("LOG_LEVEL", "info"),
		LogFormat:     env("LOG_FORMAT", "json"),
		BaseURL:       env("BASE_URL", ""),
		MetrikaID:     env("METRIKA_ID", ""),
		LegalEmail:    env("LEGAL_EMAIL", "info@racion.app"),
		VkusvillMCP:   env("VKUSVILL_MCP", ""),
		ImagesDir:     env("IMAGES_DIR", "../frontend/public/images"),
		GeoDir:        env("GEO_DIR", os.TempDir()),
		PushContact:   env("PUSH_CONTACT", "mailto:admin@racion.app"),
		OpenAIKey:     env("OPENAI_API_KEY", ""),
		OpenAIModel:   env("OPENAI_MODEL", "gpt-5-nano"),
		MistralKey:    env("MISTRAL_API_KEY", ""),
		GeminiKey:     env("GEMINI_API_KEY", ""),
		GroqKey:       env("GROQ_API_KEY", ""),
		OpenRouterKey: env("OPENROUTER_API_KEY", ""),
		LocalAIURL:    env("LOCAL_AI_URL", ""),
		AIOrder:       env("AI_ORDER", ""),
		AIModels:      env("AI_MODELS", ""),
		AdminEmails:   env("ADMIN_EMAILS", ""),
		MailHost:      env("MAIL_HOST", ""),
		MailPort:      env("MAIL_PORT", "587"),
		MailUser:      env("MAIL_USER", ""),
		MailPass:      env("MAIL_PASS", ""),
		MailFrom:      env("MAIL_FROM", "Racion <info@racion.app>"),
		S3Endpoint:    env("S3_ENDPOINT", ""),
		S3AccessKey:   env("S3_ACCESS_KEY", ""),
		S3SecretKey:   env("S3_SECRET_KEY", ""),
		S3Bucket:      env("S3_BUCKET", "racion"),
		S3Secure:      env("S3_SECURE", "") == "1",
		S3PublicURL:   env("S3_PUBLIC_URL", "/media"),
		OAuth: OAuth{
			VKID: env("OAUTH_VK_ID", ""), VKSecret: env("OAUTH_VK_SECRET", ""),
			YandexID: env("OAUTH_YANDEX_ID", ""), YandexSecret: env("OAUTH_YANDEX_SECRET", ""),
			GoogleID: env("OAUTH_GOOGLE_ID", ""), GoogleSecret: env("OAUTH_GOOGLE_SECRET", ""),
			GitHubID: env("OAUTH_GITHUB_ID", ""), GitHubSecret: env("OAUTH_GITHUB_SECRET", ""),
			AppleClientID: env("OAUTH_APPLE_CLIENT_ID", ""), AppleTeamID: env("OAUTH_APPLE_TEAM_ID", ""), AppleKeyID: env("OAUTH_APPLE_KEY_ID", ""), AppleKey: env("OAUTH_APPLE_KEY", ""),
		},
		Bots: Bots{
			TelegramToken: env("TELEGRAM_BOT_TOKEN", ""), TelegramName: env("TELEGRAM_BOT_NAME", ""),
			MaxToken: env("MAX_BOT_TOKEN", ""), MaxName: env("MAX_BOT_NAME", ""),
			Secret:      env("BOT_SECRET", ""),
			TelegramAPI: env("TELEGRAM_API", ""), MaxAPI: env("MAX_API", ""),
			TelegramUpdates: env("TELEGRAM_UPDATES", "poll"),
		},
		ShutdownTimeout: 10 * time.Second,
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
