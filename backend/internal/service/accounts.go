package service

import (
	"racion/internal/i18n"
	"crypto/sha256"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"racion/internal/domain"
	"racion/internal/planner"
)

const SessionTTL = 90 * 24 * time.Hour

// Accounts — регистрация, вход, профиль, нелюбимые рецепты, история покупок.
type Accounts struct {
	users     UserRepo
	resets    ResetRepo
	sessions  SessionRepo
	mailer    Mailer // nil — письма не шлём (восстановление пароля недоступно)
	baseURL   string
	plans     PlanRepo
	dislikes  DislikeRepo
	purchases PurchaseRepo
	catalog   *planner.CatalogRef
	mediaOwns func(url string) bool // ссылка ведёт в наше хранилище фото
	importPic func(ctx context.Context, userID, src string) (string, error) // аватар от внешнего сервиса → наше хранилище
}

// SetAvatarImporter — как забирать аватар у провайдера входа (CSP не пускает чужие картинки).
func (a *Accounts) SetAvatarImporter(f func(ctx context.Context, userID, src string) (string, error)) { a.importPic = f }

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (c *Credentials) clean() error {
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	c.Name = strings.TrimSpace(c.Name)
	if _, err := mail.ParseAddress(c.Email); err != nil || len(c.Email) > 200 {
		return domain.Invalid("auth.bad_email")
	}
	if len(c.Password) < 8 {
		return domain.Invalid("auth.short_password")
	}
	if len(c.Password) > 200 {
		return domain.Invalid("auth.long_password")
	}
	if len(c.Name) > 80 {
		c.Name = c.Name[:80]
	}
	return nil
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Register создаёт аккаунт и сессию; claimPlan — гостевой план, который надо привязать к аккаунту.
func (a *Accounts) Register(ctx context.Context, c Credentials, claimPlan string) (domain.User, domain.Session, error) {
	if err := c.clean(); err != nil {
		return domain.User{}, domain.Session{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, domain.Session{}, domain.Internal("hash", err)
	}
	u, err := a.users.Create(ctx, c.Email, string(hash), c.Name)
	if errors.Is(err, domain.ErrConflict) {
		return domain.User{}, domain.Session{}, domain.Invalid("auth.exists")
	}
	if err != nil {
		return domain.User{}, domain.Session{}, err
	}
	s, err := a.startSession(ctx, u.ID, claimPlan)
	return u, s, err
}

// OAuthProfile — что рассказал внешний сервис.
type OAuthProfile struct {
	Provider, ID, Email, Name, Avatar string
}

// LoginOAuth — вход через внешний сервис: знакомый аккаунт → сессия; незнакомый с известной почтой →
// привязка к существующему пользователю; иначе новый пользователь без пароля (его можно задать через
// «забыли пароль»). Без почты от провайдера — служебный адрес, который человек потом меняет в кабинете.
func (a *Accounts) LoginOAuth(ctx context.Context, pr OAuthProfile, claimPlan string) (domain.User, domain.Session, error) {
	if pr.ID == "" {
		return domain.User{}, domain.Session{}, domain.Invalid("auth.oauth.failed")
	}
	u, err := a.users.ByOAuth(ctx, pr.Provider, pr.ID)
	if errors.Is(err, domain.ErrNotFound) {
		email := strings.ToLower(strings.TrimSpace(pr.Email))
		if _, perr := mail.ParseAddress(email); perr != nil {
			email = pr.Provider + "-" + pr.ID + "@login.racion.app"
		}
		u, _, err = a.users.ByEmail(ctx, email)
		if errors.Is(err, domain.ErrNotFound) {
			name := strings.TrimSpace(pr.Name)
			if len(name) > 80 {
				name = name[:80]
			}
			u, err = a.users.Create(ctx, email, "", name)
			if err == nil && pr.Avatar != "" && a.importPic != nil {
				if url, perr := a.importPic(ctx, u.ID, pr.Avatar); perr == nil && url != "" {
					_ = a.users.SetAvatar(ctx, u.ID, url)
					u.Avatar = url
				}
			}
		}
		if err != nil {
			return domain.User{}, domain.Session{}, err
		}
		if err := a.users.LinkOAuth(ctx, pr.Provider, pr.ID, u.ID, email); err != nil {
			return domain.User{}, domain.Session{}, err
		}
	} else if err != nil {
		return domain.User{}, domain.Session{}, err
	}
	s, err := a.startSession(ctx, u.ID, claimPlan)
	return u, s, err
}

// Login проверяет пароль и открывает сессию. Неверная почта и неверный пароль неразличимы.
func (a *Accounts) Login(ctx context.Context, c Credentials, claimPlan string) (domain.User, domain.Session, error) {
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	u, hash, err := a.users.ByEmail(ctx, c.Email)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && (hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil)) {
		return domain.User{}, domain.Session{}, domain.ErrUnauthorized
	}
	if err != nil {
		return domain.User{}, domain.Session{}, err
	}
	s, err := a.startSession(ctx, u.ID, claimPlan)
	return u, s, err
}

// SetMailer подключает отправку писем
func (a *Accounts) SetMailer(m Mailer) { a.mailer = m }

const ResetTTL = time.Hour

// Forgot шлёт письмо со ссылкой на смену пароля. Ответ одинаковый, есть адрес в базе или нет.
func (a *Accounts) Forgot(ctx context.Context, email string, lang i18n.Lang) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || a.mailer == nil {
		return nil
	}
	u, _, err := a.users.ByEmail(ctx, email)
	if err != nil {
		return nil // неизвестный адрес: молчим, чтобы по ответу нельзя было проверить, есть ли аккаунт
	}
	token := newToken()
	if err := a.resets.Create(ctx, hashToken(token), u.ID, time.Now().Add(ResetTTL)); err != nil {
		return err
	}
	link := a.baseURL + "/login?reset=" + token
	if lang != "" && lang != "ru" {
		link = a.baseURL + "/login?reset=" + token + "&lang=" + string(lang)
	}
	return a.mailer.Send(email, i18n.T(lang, "mail.reset.subject"), i18n.T(lang, "mail.reset.body", link))
}

// Reset меняет пароль по одноразовой ссылке и сразу открывает сессию.
func (a *Accounts) Reset(ctx context.Context, token, password, claimPlan string) (domain.User, domain.Session, error) {
	if len(token) != 64 || len(password) < 8 || len(password) > 200 {
		return domain.User{}, domain.Session{}, domain.ErrBadInput
	}
	userID, err := a.resets.Take(ctx, hashToken(token))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, domain.Session{}, domain.Invalid("auth.reset.bad")
	}
	if err != nil {
		return domain.User{}, domain.Session{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, domain.Session{}, domain.Internal("hash", err)
	}
	if err := a.users.SetPassword(ctx, userID, string(hash)); err != nil {
		return domain.User{}, domain.Session{}, err
	}
	s, err := a.startSession(ctx, userID, claimPlan)
	if err != nil {
		return domain.User{}, domain.Session{}, err
	}
	u, err := a.sessions.UserByToken(ctx, s.Token)
	return u, s, err
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

func (a *Accounts) startSession(ctx context.Context, userID, claimPlan string) (domain.Session, error) {
	s := domain.Session{Token: newToken(), UserID: userID, ExpiresAt: time.Now().Add(SessionTTL)}
	if err := a.sessions.Create(ctx, s.Token, userID, s.ExpiresAt); err != nil {
		return domain.Session{}, err
	}
	if IsPlanID(claimPlan) {
		_ = a.plans.Claim(ctx, claimPlan, userID)
	}
	return s, nil
}

func (a *Accounts) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return a.sessions.Delete(ctx, token)
}

// UserByToken — пользователь по сессии; пусто и nil, если сессии нет.
func (a *Accounts) UserByToken(ctx context.Context, token string) (*domain.User, error) {
	if token == "" {
		return nil, nil
	}
	u, err := a.sessions.UserByToken(ctx, token)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

type ProfileUpdate struct {
	Name     *string         `json:"name"`
	Nick     *string         `json:"nick"`
	Avatar   *string         `json:"avatar"` // ссылка из нашего хранилища или "" (убрать)
	Defaults json.RawMessage `json:"defaults"`
}

// SetMedia подключает проверку ссылок на фото (после сборки сервисов).
func (a *Accounts) SetMedia(owns func(url string) bool) { a.mediaOwns = owns }

// UpdateProfile — имя и сохранённые ответы квиза.
func (a *Accounts) UpdateProfile(ctx context.Context, u domain.User, upd ProfileUpdate) (domain.User, error) {
	if upd.Name != nil {
		n := strings.TrimSpace(*upd.Name)
		if len(n) > 80 {
			n = n[:80]
		}
		if err := a.users.SetName(ctx, u.ID, n); err != nil {
			return u, err
		}
		u.Name = n
	}
	if upd.Nick != nil {
		nick, err := NormalizeNick(*upd.Nick)
		if err != nil {
			return u, err
		}
		if err := a.users.SetNick(ctx, u.ID, nick); err != nil {
			if errors.Is(err, domain.ErrConflict) {
				return u, domain.Invalid("nick.taken")
			}
			return u, err
		}
		u.Nick = nick
	}
	if upd.Avatar != nil {
		if *upd.Avatar != "" && (a.mediaOwns == nil || !a.mediaOwns(*upd.Avatar)) {
			return u, domain.Invalid("photo.bad")
		}
		if err := a.users.SetAvatar(ctx, u.ID, *upd.Avatar); err != nil {
			return u, err
		}
		u.Avatar = *upd.Avatar
	}
	if len(upd.Defaults) > 0 && json.Valid(upd.Defaults) {
		if err := a.users.SetDefaults(ctx, u.ID, upd.Defaults); err != nil {
			return u, err
		}
		u.Defaults = upd.Defaults
	}
	return u, nil
}

// ── Нелюбимые рецепты ──────────────────────────────────────────────────────

type DislikedRecipe struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Slot  string `json:"slot"`
}

// DislikeIDs — id нелюбимых рецептов; при ошибке БД — пусто, план соберётся и без них.
func (a *Accounts) DislikeIDs(ctx context.Context, userID string) []string {
	ids, _ := a.dislikes.List(ctx, userID)
	return ids
}

func (a *Accounts) Dislikes(ctx context.Context, userID string) ([]DislikedRecipe, error) {
	ids, err := a.dislikes.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]DislikedRecipe, 0, len(ids))
	for _, id := range ids {
		if rc, ok := a.catalog.Load().RecipeByID[id]; ok {
			out = append(out, DislikedRecipe{id, rc.Title, rc.Slot})
		}
	}
	return out, nil
}

func (a *Accounts) AddDislike(ctx context.Context, userID, recipeID string) error {
	if _, ok := a.catalog.Load().RecipeByID[recipeID]; !ok {
		return domain.ErrNotFound
	}
	return a.dislikes.Add(ctx, userID, recipeID)
}

func (a *Accounts) RemoveDislike(ctx context.Context, userID, recipeID string) error {
	return a.dislikes.Remove(ctx, userID, recipeID)
}

// Purchases — история покупок за days дней (1…365, по умолчанию 60).
func (a *Accounts) Purchases(ctx context.Context, userID string, days int) ([]domain.Purchase, error) {
	if days <= 0 || days > 365 {
		days = 60
	}
	return a.purchases.Recent(ctx, userID, days)
}
