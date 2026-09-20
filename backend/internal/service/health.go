package service

import (
	"context"
	"net"
	"racion/internal/domain"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Монитор состояния для страницы /status: раз в минуту проверяет базу, хранилище фото и почтовый сервер,
// пишет замеры в health_samples (30 дней) и держит последний результат в памяти. Push и индексация
// поисковиков — статические признаки: ключи есть или нет.

type HealthRepo interface {
	Add(ctx context.Context, component string, ok bool, ms int) error
	Days(ctx context.Context, days int) ([]domain.HealthDay, error)
	Latency(ctx context.Context, component string, since time.Time) (avgMs int, n int, err error)
	Prune(ctx context.Context, keep time.Duration) error
}

type Check struct {
	Key   string
	Fn    func(ctx context.Context) error
	Every time.Duration // как часто; 0 — каждую минуту
}

type ComponentState struct {
	Key     string    `json:"key"`
	OK      bool      `json:"ok"`
	Ms      int       `json:"ms"`
	At      time.Time `json:"at"`
	Static  bool      `json:"static,omitempty"` // без замеров: настроено или нет
	Message string    `json:"message,omitempty"`
}

type Health struct {
	repo    HealthRepo
	log     *zap.Logger
	checks  []Check
	static  []ComponentState
	started time.Time
	mu      sync.RWMutex
	state   map[string]ComponentState
	lastRun map[string]time.Time
}

func NewHealth(repo HealthRepo, log *zap.Logger) *Health {
	return &Health{repo: repo, log: log, started: time.Now(), state: map[string]ComponentState{}, lastRun: map[string]time.Time{}}
}

func (h *Health) AddCheck(c Check) { h.checks = append(h.checks, c) }

// AddStatic — компонент без проверок: есть настройка или нет
func (h *Health) AddStatic(key string, ok bool, message string) {
	h.static = append(h.static, ComponentState{Key: key, OK: ok, Static: true, Message: message, At: time.Now()})
}

func (h *Health) Started() time.Time { return h.started }

// Latency — средний отклик компонента с момента since
func (h *Health) Latency(ctx context.Context, key string, since time.Time) (int, int, error) {
	return h.repo.Latency(ctx, key, since)
}

// Run — цикл проверок; первая сразу, дальше раз в минуту
func (h *Health) Run(ctx context.Context) {
	h.tick(ctx)
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	prune := time.NewTicker(6 * time.Hour)
	defer prune.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.tick(ctx)
		case <-prune.C:
			_ = h.repo.Prune(ctx, 31*24*time.Hour)
		}
	}
}

func (h *Health) tick(ctx context.Context) {
	now := time.Now()
	for _, c := range h.checks {
		every := c.Every
		if every == 0 {
			every = time.Minute
		}
		if last, ok := h.lastRun[c.Key]; ok && now.Sub(last) < every-5*time.Second {
			continue
		}
		h.lastRun[c.Key] = now
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		t0 := time.Now()
		err := c.Fn(cctx)
		cancel()
		ms := int(time.Since(t0) / time.Millisecond)
		st := ComponentState{Key: c.Key, OK: err == nil, Ms: ms, At: now}
		if err != nil {
			st.Message = err.Error()
			h.log.Warn("health check failed", zap.String("component", c.Key), zap.Error(err))
		}
		h.mu.Lock()
		h.state[c.Key] = st
		h.mu.Unlock()
		if err := h.repo.Add(ctx, c.Key, st.OK, ms); err != nil {
			h.log.Warn("health sample", zap.Error(err))
		}
	}
}

// Snapshot — текущее состояние всех компонентов в порядке добавления
func (h *Health) Snapshot() []ComponentState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]ComponentState, 0, len(h.checks)+len(h.static))
	for _, c := range h.checks {
		if st, ok := h.state[c.Key]; ok {
			out = append(out, st)
		} else {
			out = append(out, ComponentState{Key: c.Key, OK: true, At: h.started})
		}
	}
	out = append(out, h.static...)
	return out
}

// History — по компонентам: дни (последние n, включая сегодня) и доля удачных проверок; нет данных — Total 0
func (h *Health) History(ctx context.Context, days int) (map[string][]domain.HealthDay, error) {
	rows, err := h.repo.Days(ctx, days)
	if err != nil {
		return nil, err
	}
	byKey := map[string]map[string]domain.HealthDay{}
	for _, r := range rows {
		if byKey[r.Component] == nil {
			byKey[r.Component] = map[string]domain.HealthDay{}
		}
		byKey[r.Component][r.Day.Format("2006-01-02")] = r
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	out := map[string][]domain.HealthDay{}
	for _, c := range h.checks {
		list := make([]domain.HealthDay, 0, days)
		for i := days - 1; i >= 0; i-- {
			d := today.AddDate(0, 0, -i)
			row, ok := byKey[c.Key][d.Format("2006-01-02")]
			if !ok {
				row = domain.HealthDay{Component: c.Key, Day: d}
			}
			list = append(list, row)
		}
		out[c.Key] = list
	}
	return out, nil
}

// Uptime — доля удачных проверок за период по всем измеряемым компонентам, в процентах
func Uptime(hist map[string][]domain.HealthDay) float64 {
	total, ok := 0, 0
	keys := make([]string, 0, len(hist))
	for k := range hist {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, d := range hist[k] {
			total += d.Total
			ok += d.OK
		}
	}
	if total == 0 {
		return 100
	}
	return float64(ok) * 100 / float64(total)
}

// TCPCheck — проверка «порт отвечает» (почтовый сервер)
func TCPCheck(addr string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		return c.Close()
	}
}
