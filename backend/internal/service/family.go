package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"racion/internal/domain"
	"racion/internal/planner"
)

// Семья: состав (взрослые с целями, дети) и аккаунты, которые её делят. Состав подставляется в квиз,
// планы любого аккаунта семьи видны всем и правятся всеми.

type HouseholdRepo interface {
	ByUser(ctx context.Context, userID string) (domain.Household, error)
	ByToken(ctx context.Context, token string) (domain.Household, error)
	Create(ctx context.Context, ownerID string) (domain.Household, error)
	Save(ctx context.Context, h domain.Household) error
	SetInvite(ctx context.Context, id, token string) error
	Accounts(ctx context.Context, id string) ([]domain.HouseholdAccount, error)
	AddUser(ctx context.Context, id, userID string) error
	RemoveUser(ctx context.Context, id, userID string) error
	MemberCount(ctx context.Context, id string) (int, error)
	SameHousehold(ctx context.Context, a, b string) (bool, error)
}

type Family struct {
	repo HouseholdRepo
}

// FamilyView — семья глазами аккаунта: состав, аккаунты, ссылка-приглашение (только владельцу).
type FamilyView struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Adults      []planner.Member          `json:"adults"`
	Kids        []planner.Child           `json:"kids"`
	Accounts    []domain.HouseholdAccount `json:"accounts"`
	Owner       bool                      `json:"owner"`
	InviteToken string                    `json:"inviteToken,omitempty"`
}

// Get — семья аккаунта; если её ещё нет — пустая с Owner=true.
func (f *Family) Get(ctx context.Context, userID string) (FamilyView, error) {
	h, err := f.repo.ByUser(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return FamilyView{Adults: []planner.Member{}, Kids: []planner.Child{}, Accounts: []domain.HouseholdAccount{}, Owner: true}, nil
	}
	if err != nil {
		return FamilyView{}, err
	}
	return f.view(ctx, h, userID)
}

func (f *Family) view(ctx context.Context, h domain.Household, userID string) (FamilyView, error) {
	accounts, err := f.repo.Accounts(ctx, h.ID)
	if err != nil {
		return FamilyView{}, err
	}
	for i := range accounts {
		accounts[i].You = accounts[i].UserID == userID
	}
	v := FamilyView{ID: h.ID, Name: h.Name, Adults: h.Adults, Kids: h.Kids, Accounts: accounts, Owner: h.OwnerID == userID}
	if v.Adults == nil {
		v.Adults = []planner.Member{}
	}
	if v.Kids == nil {
		v.Kids = []planner.Child{}
	}
	if v.Owner {
		v.InviteToken = h.InviteToken
	}
	return v, nil
}

// ensure — семья аккаунта; создаётся при первой записи.
func (f *Family) ensure(ctx context.Context, userID string) (domain.Household, error) {
	h, err := f.repo.ByUser(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return f.repo.Create(ctx, userID)
	}
	return h, err
}

type FamilyInput struct {
	Name   string           `json:"name"`
	Adults []planner.Member `json:"adults"`
	Kids   []planner.Child  `json:"kids"`
}

// Save — состав семьи; правит любой её аккаунт.
func (f *Family) Save(ctx context.Context, userID string, in FamilyInput) (FamilyView, error) {
	if len(in.Adults) > 8 || len(in.Kids) > 8 {
		return FamilyView{}, domain.Invalid("family.err.size")
	}
	h, err := f.ensure(ctx, userID)
	if err != nil {
		return FamilyView{}, err
	}
	h.Name = strings.TrimSpace(in.Name)
	if len([]rune(h.Name)) > 40 {
		h.Name = string([]rune(h.Name)[:40])
	}
	// нормализуем тем же кодом, что и квиз, чтобы в семье не было мусора
	p := planner.Params{Members: in.Adults, Kids: in.Kids, Slots: planner.SlotOrder}
	p = planner.NormalizeFamily(p)
	h.Adults, h.Kids = p.Members, p.Kids
	if err := f.repo.Save(ctx, h); err != nil {
		return FamilyView{}, err
	}
	return f.view(ctx, h, userID)
}

// Invite — ссылка-приглашение: токен создаётся один раз и живёт, пока владелец не сбросит.
func (f *Family) Invite(ctx context.Context, userID string, reset bool) (string, error) {
	h, err := f.ensure(ctx, userID)
	if err != nil {
		return "", err
	}
	if h.OwnerID != userID {
		return "", domain.ErrForbidden
	}
	if h.InviteToken != "" && !reset {
		return h.InviteToken, nil
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	if err := f.repo.SetInvite(ctx, h.ID, tok); err != nil {
		return "", err
	}
	return tok, nil
}

// Join — присоединиться по ссылке. Владелец семьи с другими аккаунтами перейти не может.
func (f *Family) Join(ctx context.Context, userID, token string) (FamilyView, error) {
	if len(token) != 32 {
		return FamilyView{}, domain.ErrNotFound
	}
	target, err := f.repo.ByToken(ctx, token)
	if err != nil {
		return FamilyView{}, err
	}
	if cur, err := f.repo.ByUser(ctx, userID); err == nil {
		if cur.ID == target.ID {
			return f.view(ctx, target, userID)
		}
		if cur.OwnerID == userID {
			if n, _ := f.repo.MemberCount(ctx, cur.ID); n > 1 {
				return FamilyView{}, domain.Invalid("family.err.owner")
			}
		}
	}
	if err := f.repo.AddUser(ctx, target.ID, userID); err != nil {
		return FamilyView{}, err
	}
	return f.view(ctx, target, userID)
}

// Leave — выйти из семьи (владелец не выходит: он может только пересобрать состав).
func (f *Family) Leave(ctx context.Context, userID string) error {
	h, err := f.repo.ByUser(ctx, userID)
	if err != nil {
		return err
	}
	if h.OwnerID == userID {
		return domain.Invalid("family.err.owner")
	}
	return f.repo.RemoveUser(ctx, h.ID, userID)
}

// Remove — владелец убирает аккаунт из семьи.
func (f *Family) Remove(ctx context.Context, ownerID, userID string) error {
	h, err := f.repo.ByUser(ctx, ownerID)
	if err != nil {
		return err
	}
	if h.OwnerID != ownerID || userID == ownerID {
		return domain.ErrForbidden
	}
	return f.repo.RemoveUser(ctx, h.ID, userID)
}

// Together — состоят ли два аккаунта в одной семье (для прав на планы).
func (f *Family) Together(ctx context.Context, a, b string) bool {
	ok, _ := f.repo.SameHousehold(ctx, a, b)
	return ok
}

// AccountNames — имена аккаунтов семьи для подписи на чеке.
func (f *Family) AccountNames(ctx context.Context, userID string) []string {
	h, err := f.repo.ByUser(ctx, userID)
	if err != nil {
		return nil
	}
	acc, _ := f.repo.Accounts(ctx, h.ID)
	if len(acc) < 2 {
		return nil
	}
	out := make([]string, 0, len(acc))
	for _, a := range acc {
		out = append(out, a.Name)
	}
	return out
}
