// Package logger — единый логгер приложения на zap. JSON для продакшена, цветная консоль для разработки.
// LOG_LEVEL: debug | info | warn | error (по умолчанию info). LOG_FORMAT: json | console (по умолчанию json).
package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(level, format string) *zap.Logger {
	lvl := zapcore.InfoLevel
	_ = lvl.UnmarshalText([]byte(strings.ToLower(strings.TrimSpace(level))))
	cfg := zap.NewProductionConfig()
	if strings.EqualFold(format, "console") {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Sampling = nil // логи запросов нужны все, без сэмплирования
	l, err := cfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		return zap.NewNop()
	}
	return l
}
