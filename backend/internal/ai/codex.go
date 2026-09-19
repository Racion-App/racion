package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Codex — тот же Client, но вместо API ключа ходит через Codex CLI по подписке ChatGPT:
// `codex exec --ephemeral --skip-git-repo-check -s read-only -m <model> "<prompt>"`, ответ — последнее
// сообщение агента (файл -o). Для локальных утилит (cmd/racionai): переводы и доработка рецептов
// без API-кредитов. У ChatGPT-аккаунта доступна только основная модель (gpt-5.5), поэтому
// размышления ставим на минимум — для переводов их не нужно.

type codexBackend struct {
	bin    string // путь к codex.exe (или codex в PATH)
	model  string // пусто — модель из ~/.codex/config.toml
	effort string // model_reasoning_effort: minimal | low | medium
}

// NewCodex — клиент поверх Codex CLI. bin пустой — ищем codex в PATH.
func NewCodex(bin, model, effort string) *Client {
	if bin == "" {
		bin = "codex"
	}
	if effort == "" {
		effort = "low"
	}
	return &Client{key: "codex", model: model, codex: &codexBackend{bin: bin, model: model, effort: effort}}
}

func (b *codexBackend) run(ctx context.Context, system, user string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "racion-codex-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "out.json")
	prompt := system + "\n\nInput:\n" + user + "\n\nAnswer with the JSON object only, no prose, no code fences."
	args := []string{"exec", "--ephemeral", "--skip-git-repo-check", "-s", "read-only", "-C", dir, "-c", "model_reasoning_effort=" + b.effort, "-o", out}
	if b.model != "" {
		args = append(args, "-m", b.model)
	}
	args = append(args, prompt)
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, b.bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = nil // иначе codex ждёт ввод
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codex: %w: %s", err, truncate(stderr.Bytes()))
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("codex: no answer: %s", truncate(stderr.Bytes()))
	}
	// на всякий случай срезаем ```json … ``` и текст вокруг объекта
	s := strings.TrimSpace(string(raw))
	if i := strings.Index(s, "{"); i > 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 {
		s = s[:j+1]
	}
	if s == "" {
		return nil, errors.New("codex: empty answer")
	}
	return []byte(s), nil
}

func (c *Client) codexJSON(ctx context.Context, system, user string, out any) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		raw, err := c.codex.run(ctx, system, user)
		if err == nil {
			if err = json.Unmarshal(raw, out); err == nil {
				return nil
			}
			err = fmt.Errorf("codex: bad json in answer: %w: %s", err, truncate(raw))
		}
		lastErr = err
		if ctx.Err() != nil {
			return lastErr
		}
		time.Sleep(time.Duration(attempt+1) * 3 * time.Second)
	}
	return lastErr
}
