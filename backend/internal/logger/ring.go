package logger

import (
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Ring — последние записи лога в памяти для админки: без внешнего хранилища, обнуляется при рестарте.
// Держим 1000 записей уровня info и выше; поле stacktrace не сохраняем, оно есть в основном логе.

type Entry struct {
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Logger  string         `json:"logger,omitempty"`
	Message string         `json:"msg"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type Ring struct {
	mu   sync.Mutex
	buf  []Entry
	head int
	full bool
}

func NewRing(size int) *Ring { return &Ring{buf: make([]Entry, size)} }

func (r *Ring) add(e Entry) {
	r.mu.Lock()
	r.buf[r.head] = e
	r.head = (r.head + 1) % len(r.buf)
	if r.head == 0 {
		r.full = true
	}
	r.mu.Unlock()
}

// Last — до n последних записей, свежие первыми; minLevel — info | warn | error.
func (r *Ring) Last(n int, minLevel string) []Entry {
	rank := map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3, "dpanic": 4, "panic": 5, "fatal": 6}
	min := rank[minLevel]
	r.mu.Lock()
	defer r.mu.Unlock()
	total := r.head
	if r.full {
		total = len(r.buf)
	}
	out := make([]Entry, 0, n)
	for i := 1; i <= total && len(out) < n; i++ {
		e := r.buf[(r.head-i+len(r.buf))%len(r.buf)]
		if rank[e.Level] >= min {
			out = append(out, e)
		}
	}
	return out
}

// core — zapcore.Core, который пишет в кольцо параллельно основному выводу.
type core struct {
	zapcore.LevelEnabler
	ring   *Ring
	fields []zapcore.Field
	name   string
}

func (c *core) With(fields []zapcore.Field) zapcore.Core {
	return &core{LevelEnabler: c.LevelEnabler, ring: c.ring, fields: append(append([]zapcore.Field{}, c.fields...), fields...)}
}

func (c *core) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}
	return ce
}

func (c *core) Write(e zapcore.Entry, fields []zapcore.Field) error {
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range c.fields {
		f.AddTo(enc)
	}
	for _, f := range fields {
		f.AddTo(enc)
	}
	// значения приводим к тому, что переживёт json: ошибки и time.Duration
	out := map[string]any{}
	for k, v := range enc.Fields {
		switch x := v.(type) {
		case error:
			out[k] = x.Error()
		case time.Duration:
			out[k] = x.String()
		default:
			if _, err := json.Marshal(v); err == nil {
				out[k] = v
			}
		}
	}
	c.ring.add(Entry{Time: e.Time.UTC().Format(time.RFC3339), Level: e.Level.String(), Logger: e.LoggerName, Message: e.Message, Fields: out})
	return nil
}

func (c *core) Sync() error { return nil }

// WithRing — логгер, который дублирует записи уровня info+ в кольцо.
func WithRing(l *zap.Logger, ring *Ring) *zap.Logger {
	return l.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, &core{LevelEnabler: zapcore.InfoLevel, ring: ring})
	}))
}
