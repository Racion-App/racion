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
	PushContact     string // контакт оператора для VAPID (mailto:…), его видят push-сервисы
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
	S3Endpoint      string // minio:9000 или s3.example.com; пусто — фото выключены
	S3AccessKey     string
	S3SecretKey     string
	S3Bucket        string
	S3Secure        bool   // https к хранилищу
	S3PublicURL     string // база публичных ссылок на фото: /media (nginx → MinIO) или адрес CDN
	ShutdownTimeout time.Duration
}

func Load() Config {
	return Config{
		DatabaseURL:     env("DATABASE_URL", "postgres://racion:racion@localhost:5432/racion?sslmode=disable"),
		HTTPAddr:        env("HTTP_ADDR", ":8080"),
		LogLevel:        env("LOG_LEVEL", "info"),
		LogFormat:       env("LOG_FORMAT", "json"),
		BaseURL:         env("BASE_URL", ""),
		MetrikaID:       env("METRIKA_ID", ""),
		PushContact:     env("PUSH_CONTACT", "mailto:admin@racion.app"),
		OpenAIKey:       env("OPENAI_API_KEY", ""),
		OpenAIModel:     env("OPENAI_MODEL", "gpt-5-nano"),
		MistralKey:      env("MISTRAL_API_KEY", ""),
		GeminiKey:       env("GEMINI_API_KEY", ""),
		GroqKey:         env("GROQ_API_KEY", ""),
		OpenRouterKey:   env("OPENROUTER_API_KEY", ""),
		LocalAIURL:      env("LOCAL_AI_URL", ""),
		AIOrder:         env("AI_ORDER", ""),
		AIModels:        env("AI_MODELS", ""),
		AdminEmails:     env("ADMIN_EMAILS", ""),
		S3Endpoint:      env("S3_ENDPOINT", ""),
		S3AccessKey:     env("S3_ACCESS_KEY", ""),
		S3SecretKey:     env("S3_SECRET_KEY", ""),
		S3Bucket:        env("S3_BUCKET", "racion"),
		S3Secure:        env("S3_SECURE", "") == "1",
		S3PublicURL:     env("S3_PUBLIC_URL", "/media"),
		ShutdownTimeout: 10 * time.Second,
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
