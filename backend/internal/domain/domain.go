// Package domain — сущности и ошибки, общие для хранилища, сервисов и транспортов.
// Рецепты, продукты и планы живут в planner; здесь то, что вокруг них: люди, сессии, отметки, покупки.
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"racion/internal/planner"
)

// ── Ошибки ─────────────────────────────────────────────────────────────────
// Сервисы возвращают только их (плюс обёрнутые ошибки БД); транспорт переводит в статус и текст.

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrBadInput     = errors.New("bad input")
)

// ValidationError — ошибка пользовательского ввода с ключом i18n; транспорт покажет текст на языке запроса.
type ValidationError struct {
	Key string
}

func (e *ValidationError) Error() string { return "validation: " + e.Key }

func Invalid(key string) error { return &ValidationError{Key: key} }

// Internal оборачивает сбой инфраструктуры, чтобы транспорт отдал 500 без деталей.
func Internal(op string, err error) error { return fmt.Errorf("%s: %w", op, err) }

// ── Люди и сессии ──────────────────────────────────────────────────────────

type User struct {
	ID       string          `json:"id"`
	Email    string          `json:"email"`
	Name     string          `json:"name"`
	Nick     string          `json:"nick"`     // публичное имя в комментариях и у своих рецептов
	Avatar   string          `json:"avatar"`   // ссылка на фото профиля (S3), пусто — монограмма
	Role     string          `json:"role"`     // "" | moderator | admin (админом делает и ADMIN_EMAILS)
	Defaults json.RawMessage `json:"defaults"` // последние ответы квиза
}

type Session struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

// ── Планы ──────────────────────────────────────────────────────────────────

// PlanRecord — план в хранилище: сам план плюс владелец.
type PlanRecord struct {
	ID      string
	Plan    planner.Plan
	OwnerID *string
}

type PlanSummary struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	StartDate string          `json:"startDate"`
	Store     string          `json:"store"`
	Cost      float64         `json:"cost"`
	Country   planner.Country `json:"country"`
	Portions  float64         `json:"portions"`
	CreatedAt string          `json:"createdAt"`
	Checked   int             `json:"checked"`
	Items     int             `json:"items"`
	Shared    bool            `json:"shared"` // чужой план, к которому присоединились
}

// CheckInput — отметка «куплено» по позиции списка.
type CheckInput struct {
	ItemID  string  `json:"itemId"`
	Checked bool    `json:"checked"`
	Name    string  `json:"name"`
	Qty     string  `json:"qty"`
	Cost    float64 `json:"cost"`
}

type Purchase struct {
	ID       int64   `json:"id"`
	PlanID   *string `json:"planId"`
	ItemID   string  `json:"itemId"`
	Name     string  `json:"name"`
	Qty      string  `json:"qty"`
	Cost     float64 `json:"cost"`
	BoughtAt string  `json:"boughtAt"`
}

// Extra — свой товар в списке покупок плана.
type Extra struct {
	ID   int64   `json:"id"`
	Name string  `json:"name"`
	Qty  string  `json:"qty"`
	Due  *string `json:"due"` // YYYY-MM-DD
	Note string  `json:"note"`
}

// ── Свои рецепты ───────────────────────────────────────────────────────────

// OwnRecipeInput — то, что пользователь присылает при создании и правке своего рецепта.
type OwnRecipeInput struct {
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Slot        string                     `json:"slot"`
	TimeMin     int                        `json:"timeMin"`
	Equipment   []string                   `json:"equipment"`
	Tags        []string                   `json:"tags"`
	Steps       []string                   `json:"steps"`
	Ingredients []planner.RecipeIngredient `json:"ingredients"`
	Image       string                     `json:"image"` // ссылка на фото из нашего хранилища
	Lang        string                     `json:"-"`     // язык текста: ставит транспорт по языку интерфейса
}

// ── Семья ──────────────────────────────────────────────────────────────────

type Household struct {
	ID          string
	OwnerID     string
	Name        string
	Adults      []planner.Member
	Kids        []planner.Child
	InviteToken string
}

type HouseholdAccount struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Nick   string `json:"nick"`
	Owner  bool   `json:"owner"`
	You    bool   `json:"you"`
}

// ── Лайки, избранное, комментарии ──────────────────────────────────────────

type RecipeStats struct {
	Likes     int  `json:"likes"`
	Comments  int  `json:"comments"`
	Favorites int  `json:"favorites"` // сколько людей добавили в избранное
	Liked     bool `json:"liked"`
	Favorite  bool `json:"favorite"`
}

type Comment struct {
	ID        int64  `json:"id"`
	UserID    string `json:"userId"`
	Nick      string `json:"nick"`
	Name      string `json:"name"`
	Body      string `json:"body"`
	HTML      string `json:"html"`             // тело в HTML из подмножества Markdown (сервер)
	Image     string `json:"image,omitempty"`  // фото к комментарию
	Avatar    string `json:"avatar,omitempty"` // аватар автора
	CreatedAt string `json:"createdAt"`
	Mine      bool   `json:"mine"`
}

// ── Аналитика ──────────────────────────────────────────────────────────────

type Event struct {
	Name  string
	Props json.RawMessage
	At    time.Time
}

// ── Уведомления ────────────────────────────────────────────────────────────

type PushSubscription struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
	Lang     string `json:"lang"`
}

// NotifySettings — напоминания: магазин в день недели и час (по местному времени, tz — смещение в минутах),
// вечером «что готовим завтра», в воскресенье «собрать неделю».
type NotifySettings struct {
	ShopDay  int  `json:"shopDay"`  // 0 — воскресенье … 6 — суббота
	ShopHour int  `json:"shopHour"` // 0…23
	Prep     bool `json:"prep"`
	Week     bool `json:"week"`
	NoAsk    bool `json:"noAsk"` // не спрашивать вечером «как было?» (по умолчанию спрашиваем)
	Tz       int  `json:"tz"`    // минуты к UTC, как -(new Date()).getTimezoneOffset()
}

func DefaultNotify() NotifySettings {
	return NotifySettings{ShopDay: 0, ShopHour: 12, Prep: true, Week: true, Tz: 180}
}

type NotifyUser struct {
	UserID   string
	Settings NotifySettings
}

// PlanReminderInfo — план для напоминаний: даты, блюда по дням, сколько куплено.
type PlanReminderInfo struct {
	ID        string
	StartDate string
	Items     int
	Checked   int
	Dishes    map[string][]string // дата → названия блюд
	Dinner    map[string]DishRef  // дата → ужин (для вопроса «как было?»)
}

// DishRef — блюдо плана для уведомления.
type DishRef struct {
	RecipeID string
	Title    string
}

// Notification — что уходит на устройство.
type Notification struct {
	Title   string               `json:"title"`
	Body    string               `json:"body"`
	URL     string               `json:"url"`
	Tag     string               `json:"tag"`
	Actions []NotificationAction `json:"actions,omitempty"` // кнопки в уведомлении
	Recipe  string               `json:"recipe,omitempty"`  // для «как было?»: какой рецепт оценивают
}

// NotificationAction — кнопка уведомления: action уходит в service worker, title видит человек.
type NotificationAction struct {
	Action string `json:"action"`
	Title  string `json:"title"`
}

// CatalogRecipeInput — рецепт базы из админки; пустой ID — новый.
type CatalogRecipeInput struct {
	ID          string                     `json:"id"`
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Slot        string                     `json:"slot"`
	TimeMin     int                        `json:"timeMin"`
	Batch       bool                       `json:"batch"`
	Equipment   []string                   `json:"equipment"`
	Tags        []string                   `json:"tags"`
	Steps       []string                   `json:"steps"`
	Ingredients []planner.RecipeIngredient `json:"ingredients"`
	Image       string                     `json:"image"`
}

// Collection — папка рецептов пользователя.
type Collection struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Recipes      []string          `json:"recipes"`
	CreatedAt    string            `json:"createdAt"`
	Public       bool              `json:"public"`
	Curated      bool              `json:"curated"` // редакционная: видна в каталоге
	Slug         string            `json:"slug,omitempty"`
	Description  string            `json:"description"`
	Cover        string            `json:"cover"`
	CoverAuto    string            `json:"coverAuto,omitempty"`    // обложка по умолчанию: фото первого рецепта
	Names        map[string]string `json:"names,omitempty"`        // переводы названия по языкам (редакционные)
	Descriptions map[string]string `json:"descriptions,omitempty"` // переводы описания
	Author       string            `json:"author,omitempty"`       // ник владельца для публичной страницы
	SEO          map[string]CollectionText `json:"seo,omitempty"`  // редакционный текст по языкам
}

// CollectionText — текст страницы подборки: вступление (абзацы), как пользоваться (абзацы), вопросы-ответы.
type CollectionText struct {
	Intro []string `json:"intro,omitempty"`
	How   []string `json:"how,omitempty"`
	FAQ   []QA     `json:"faq,omitempty"`
}

type QA struct {
	Q string `json:"q"`
	A string `json:"a"`
}

// TextFor — текст подборки на языке с запасом en → ru.
func (c Collection) TextFor(lang string) CollectionText {
	for _, code := range []string{lang, "en", "ru"} {
		if t, ok := c.SEO[code]; ok && (len(t.Intro) > 0 || len(t.FAQ) > 0) {
			return t
		}
	}
	return CollectionText{}
}

// ── Админка ─────────────────────────────────────────────────────────────────

type AdminCounters struct {
	Users         int `json:"users"`
	UsersWeek     int `json:"usersWeek"`
	ActiveWeek    int `json:"activeWeek"` // входили за 7 дней
	Plans         int `json:"plans"`
	PlansWeek     int `json:"plansWeek"`
	PlansOwned    int `json:"plansOwned"`
	OwnRecipes    int `json:"ownRecipes"`
	Households    int `json:"households"`
	PushUsers     int `json:"pushUsers"`
	Comments      int `json:"comments"`
	Feedback      int `json:"feedback"`
	PurchasesWeek int `json:"purchasesWeek"`
	ErrorsWeek    int `json:"errorsWeek"`
}

type AdminDay struct {
	Day        string `json:"day"`
	Users      int    `json:"users"`
	Plans      int    `json:"plans"`
	Visitors   int    `json:"visitors"` // сессий аналитики
	QuizStarts int    `json:"quizStarts"`
	Errors     int    `json:"errors"`
}

type AdminEvent struct {
	Name     string `json:"name"`
	Count    int    `json:"count"`
	Sessions int    `json:"sessions"`
}

type AdminPair struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type AdminTop struct {
	Stores  []AdminPair `json:"stores"`
	Recipes []AdminPair `json:"recipes"`
}

type AdminError struct {
	At      string `json:"at"`
	Sid     string `json:"sid"`
	Message string `json:"message"`
	URL     string `json:"url"`
	Stack   string `json:"stack"`
	UA      string `json:"ua"`
}

type AdminUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	Plans     int    `json:"plans"`
	LastSeen  string `json:"lastSeen,omitempty"`
	Role      string `json:"role"`
}

// Localized — копия подборки с названием и описанием на языке lang (если перевода нет — как есть).
func (c Collection) Localized(lang string) Collection {
	// нет перевода на язык — английский, потом русский (основные поля)
	for _, code := range []string{lang, "en"} {
		if code == "ru" {
			break
		}
		if n := c.Names[code]; n != "" {
			c.Name = n
			if d := c.Descriptions[code]; d != "" {
				c.Description = d
			}
			return c
		}
	}
	return c
}

// ── Переводы своих рецептов ──────────────────────────────────────────────────

type Translation struct {
	Lang      string `json:"lang"`
	Status    string `json:"status"` // queued | running | done | error
	Model     string `json:"model,omitempty"`
	Error     string `json:"error,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

// TranslationSummary — строка в списке своих рецептов: «готово 5 из 14, сейчас es (mistral/…)».
type TranslationSummary struct {
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Errors  int    `json:"errors"`
	Running string `json:"running,omitempty"` // язык, который переводится сейчас
	Model   string `json:"model,omitempty"`   // последняя модель, что переводила
}

type TranslationStatus struct {
	Source    string        `json:"source"`
	Enabled   bool          `json:"enabled"`
	Items     []Translation `json:"items"`
	Providers []string      `json:"providers"` // доступные сейчас provider/model
}
