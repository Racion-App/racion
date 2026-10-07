package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/messenger"
)

// loginAccounts — аккаунты для входа через бота: привязок нет, EnsureExternal заводит одного человека.
type loginAccounts struct{ ensured []string }

func (a *loginAccounts) ByLink(context.Context, string, string) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}
func (a *loginAccounts) Link(context.Context, string, string, string, bool) error { return nil }
func (a *loginAccounts) EnsureExternal(_ context.Context, provider, id string) (domain.User, error) {
	a.ensured = append(a.ensured, provider+":"+id)
	return domain.User{ID: "u1"}, nil
}

func TestMessengerLoginFlow(t *testing.T) {
	ctx := context.Background()
	repo, client, acc := newMemBotRepo(), &fakeClient{}, &loginAccounts{}
	b := NewBots(BotDeps{Repo: repo, Plans: fakePlans{}, Accounts: acc}, "https://racion.app", zap.NewNop())
	b.Add(client, false)

	start := func() (token, secret string) {
		link, secret, err := b.LoginStart(ctx, "telegram", "ru", "", "Chrome, Windows")
		if err != nil {
			t.Fatal(err)
		}
		_, token, ok := strings.Cut(link, "start=l")
		if !ok || token == "" || secret == "" {
			t.Fatalf("link %q secret %q", link, secret)
		}
		return token, secret
	}
	tap := func(data string) {
		if err := b.Handle(ctx, messenger.Update{Platform: messenger.Telegram, ChatID: "7", UserID: "42", Callback: "cb", Data: data, MessageID: "m1"}); err != nil {
			t.Fatal(err)
		}
	}

	token, secret := start()
	if uid, _, err := b.LoginTake(ctx, secret); err != nil || uid != "" {
		t.Fatalf("before confirm: %q %v", uid, err)
	}
	// человек нажал «Начать» по ссылке: бот переспрашивает и показывает браузер
	if err := b.Handle(ctx, messenger.Update{Platform: messenger.Telegram, ChatID: "7", UserID: "42", Start: true, Payload: "l" + token}); err != nil {
		t.Fatal(err)
	}
	ask := client.sent[len(client.sent)-1]
	if !strings.Contains(ask.Text, "Chrome, Windows") || len(ask.Rows) != 1 || ask.Rows[0][0].Data != "a:y:"+token || ask.Rows[0][1].Data != "a:n:"+token {
		t.Fatalf("ask %+v", ask)
	}
	tap("a:y:" + token)
	if len(acc.ensured) != 1 || acc.ensured[0] != "telegram:42" {
		t.Fatalf("ensured %v", acc.ensured)
	}
	if c := repo.chats["telegram7"]; c.UserID == nil || *c.UserID != "u1" {
		t.Fatalf("chat does not know the account: %+v", c)
	}
	if uid, _, err := b.LoginTake(ctx, secret); err != nil || uid != "u1" {
		t.Fatalf("after confirm: %q %v", uid, err)
	}
	if _, _, err := b.LoginTake(ctx, secret); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("login must burn after use, got %v", err)
	}
	// повторное нажатие старой кнопки ничего не открывает
	tap("a:y:" + token)
	if len(acc.ensured) != 1 {
		t.Fatal("used token confirmed twice")
	}

	// «Это не я»: запрос удаляется, браузер получает отказ
	token2, secret2 := start()
	tap("a:n:" + token2)
	if _, _, err := b.LoginTake(ctx, secret2); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("declined login still waits: %v", err)
	}
	if _, _, err := b.LoginTake(ctx, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("empty secret must not match")
	}
}
