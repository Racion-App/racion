package service

import (
	"context"
	"strings"

	"racion/internal/domain"
)

// Admin — панель администратора: кто админ решает список почт из ADMIN_EMAILS; данные — агрегаты из хранилища.

type AdminRepo interface {
	Counters(ctx context.Context) (domain.AdminCounters, error)
	Daily(ctx context.Context, days int) ([]domain.AdminDay, error)
	Events(ctx context.Context, days int) ([]domain.AdminEvent, error)
	Top(ctx context.Context) (domain.AdminTop, error)
	Errors(ctx context.Context, limit int) ([]domain.AdminError, error)
	Users(ctx context.Context, limit int) ([]domain.AdminUser, error)
}

// Права: admin — всё; moderator — обзор, рецепты базы, модерация, ошибки. Роли и логи — только admin.
const (
	PermStats      = "stats"
	PermUsers      = "users"
	PermRoles      = "roles"
	PermLogs       = "logs"
	PermRecipes    = "recipes"
	PermModeration = "moderation"
	PermErrors     = "errors"
	PermPartners   = "partners"
)

var rolePerms = map[string][]string{
	"admin":     {PermStats, PermUsers, PermRoles, PermLogs, PermRecipes, PermModeration, PermErrors, PermPartners},
	"moderator": {PermStats, PermRecipes, PermModeration, PermErrors},
}

type Admin struct {
	repo   AdminRepo
	users  UserRepo
	emails map[string]bool
}

func NewAdmin(repo AdminRepo, users UserRepo, emails string) *Admin {
	a := &Admin{repo: repo, users: users, emails: map[string]bool{}}
	for _, e := range strings.Split(emails, ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			a.emails[e] = true
		}
	}
	return a
}

// Role — роль с учётом ADMIN_EMAILS (они всегда admin).
func (a *Admin) Role(u *domain.User) string {
	if a == nil || u == nil {
		return ""
	}
	if a.emails[strings.ToLower(u.Email)] {
		return "admin"
	}
	return u.Role
}

// Perms — права пользователя; пусто у обычного аккаунта.
func (a *Admin) Perms(u *domain.User) []string {
	p := rolePerms[a.Role(u)]
	if p == nil {
		return []string{}
	}
	return p
}

// Can — есть ли право.
func (a *Admin) Can(u *domain.User, perm string) bool {
	for _, p := range a.Perms(u) {
		if p == perm {
			return true
		}
	}
	return false
}

// Is — есть ли вообще доступ в админку.
func (a *Admin) Is(u *domain.User) bool { return len(a.Perms(u)) > 0 }

// SetRole — назначить роль; себя разжаловать нельзя, ADMIN_EMAILS остаются админами всегда.
func (a *Admin) SetRole(ctx context.Context, actor domain.User, userID, role string) error {
	if _, ok := rolePerms[role]; !ok && role != "" {
		return domain.Invalid("admin.role.bad")
	}
	if userID == actor.ID {
		return domain.Invalid("admin.role.self")
	}
	return a.users.SetRole(ctx, userID, role)
}

type AdminOverview struct {
	Counters domain.AdminCounters `json:"counters"`
	Daily    []domain.AdminDay    `json:"daily"`
	Events   []domain.AdminEvent  `json:"events"`
	Top      domain.AdminTop      `json:"top"`
}

func (a *Admin) Overview(ctx context.Context, days int) (AdminOverview, error) {
	if days <= 0 || days > 90 {
		days = 14
	}
	var out AdminOverview
	var err error
	if out.Counters, err = a.repo.Counters(ctx); err != nil {
		return out, err
	}
	if out.Daily, err = a.repo.Daily(ctx, days); err != nil {
		return out, err
	}
	if out.Events, err = a.repo.Events(ctx, days); err != nil {
		return out, err
	}
	out.Top, err = a.repo.Top(ctx)
	if out.Daily == nil {
		out.Daily = []domain.AdminDay{}
	}
	if out.Events == nil {
		out.Events = []domain.AdminEvent{}
	}
	return out, err
}

func (a *Admin) Errors(ctx context.Context) ([]domain.AdminError, error) {
	out, err := a.repo.Errors(ctx, 100)
	if out == nil {
		out = []domain.AdminError{}
	}
	return out, err
}

func (a *Admin) Users(ctx context.Context) ([]domain.AdminUser, error) {
	out, err := a.repo.Users(ctx, 100)
	if out == nil {
		out = []domain.AdminUser{}
	}
	return out, err
}
