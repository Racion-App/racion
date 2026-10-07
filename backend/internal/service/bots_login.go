package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"html"
	"strings"
	"time"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/messenger"
)

// Вход на сайт через бота: страница входа заводит запрос (LoginStart) и открывает ссылку на бота
// с параметром «l<токен>»; бот переспрашивает «войти на racion.app?» и по «Войти» узнаёт или заводит
// аккаунт этого человека в мессенджере; страница опрашивает LoginTake своим секретом из cookie и
// получает аккаунт. Переспрос нужен, чтобы чужая ссылка, нажатая по ошибке, не отдала аккаунт тому,
// кто её прислал: опрашивать может только браузер, который начал вход.

// LoginTTL — сколько ждём подтверждения в боте.
const LoginTTL = 10 * time.Minute

// LoginStart — запрос входа: ссылка на бота для человека и секрет для браузера, который будет ждать.
func (b *Bots) LoginStart(ctx context.Context, platform string, lang i18n.Lang, planID, agent string) (link, secret string, err error) {
	c, ok := b.Client(platform)
	if !ok {
		return "", "", domain.ErrNotFound
	}
	token, err := randomToken()
	if err != nil {
		return "", "", err
	}
	if secret, err = randomToken(); err != nil {
		return "", "", err
	}
	if !IsPlanID(planID) {
		planID = ""
	}
	l := domain.MessengerLogin{TokenHash: tokenHash(token), PollHash: tokenHash(secret), Platform: platform, Lang: string(lang), PlanID: planID, Agent: agent}
	if err := b.repo.CreateLogin(ctx, l, time.Now().Add(LoginTTL)); err != nil {
		return "", "", err
	}
	return c.StartLink("l" + token), secret, nil
}

// LoginTake — ждёт ли браузер ещё (userID пуст), подтвердили ли вход (userID и неделя) или запроса
// больше нет (ErrNotFound: просрочен или человек ответил «это не я»).
func (b *Bots) LoginTake(ctx context.Context, secret string) (userID, planID string, err error) {
	if secret == "" {
		return "", "", domain.ErrNotFound
	}
	return b.repo.TakeLogin(ctx, tokenHash(secret))
}

// loginAsk — человек пришёл по ссылке входа: переспросить, он ли это входит.
func (b *Bots) loginAsk(ctx context.Context, c messenger.Client, u messenger.Update, token string, lang i18n.Lang) error {
	l, err := b.repo.PendingLogin(ctx, tokenHash(token))
	if errors.Is(err, domain.ErrNotFound) || (err == nil && (u.UserID == "" || b.accounts == nil)) {
		return b.send(ctx, c, u.ChatID, messenger.Message{Text: html.EscapeString(i18n.T(lang, "bot.login.expired"))})
	}
	if err != nil {
		return err
	}
	lang = langOr(l.Lang, lang)
	text := "<b>" + html.EscapeString(i18n.T(lang, "bot.login.ask")) + "</b>"
	if l.Agent != "" {
		text += "\n" + html.EscapeString(l.Agent)
	}
	text += "\n\n" + html.EscapeString(i18n.T(lang, "bot.login.warn"))
	return b.send(ctx, c, u.ChatID, messenger.Message{Text: text, Rows: [][]messenger.Button{{
		{Text: i18n.T(lang, "bot.login.yes"), Data: "a:y:" + token},
		{Text: i18n.T(lang, "bot.login.no"), Data: "a:n:" + token},
	}}})
}

// loginTap — ответ на переспрос: «Войти» узнаёт или заводит аккаунт и отдаёт его странице входа,
// «Это не я» удаляет запрос.
func (b *Bots) loginTap(ctx context.Context, c messenger.Client, u messenger.Update, lang i18n.Lang) error {
	parts := strings.SplitN(u.Data, ":", 3)
	if len(parts) != 3 || u.UserID == "" || b.accounts == nil {
		return nil
	}
	hash := tokenHash(parts[2])
	reply := func(key string) error {
		m := messenger.Message{Text: html.EscapeString(i18n.T(lang, key))}
		if u.MessageID != "" {
			return b.edit(ctx, c, u.ChatID, u.MessageID, m)
		}
		return b.send(ctx, c, u.ChatID, m)
	}
	if parts[1] != "y" {
		if err := b.repo.DropLogin(ctx, hash); err != nil {
			return err
		}
		return reply("bot.login.declined")
	}
	if _, err := b.repo.PendingLogin(ctx, hash); errors.Is(err, domain.ErrNotFound) {
		return reply("bot.login.expired")
	} else if err != nil {
		return err
	}
	usr, err := b.accounts.EnsureExternal(ctx, string(c.Platform()), u.UserID)
	if err != nil {
		return err
	}
	if err := b.repo.ConfirmLogin(ctx, hash, usr.ID); errors.Is(err, domain.ErrNotFound) {
		return reply("bot.login.expired")
	} else if err != nil {
		return err
	}
	// чат с ботом тоже узнаёт аккаунт: списки и напоминания пойдут уже как аккаунту
	if err := b.repo.SetUser(ctx, string(c.Platform()), u.ChatID, &usr.ID); err != nil {
		return err
	}
	return reply("bot.login.done")
}

func randomToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
