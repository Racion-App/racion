package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"racion/internal/ai"
	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

// Помощник по неделе: человек пишет «не нравится вторник» или «сделай ужины легче», нейросеть видит план
// (дни, блюда, ккал, цена, цель, бюджет) и отвечает словами плюс списком действий, которые сервер применяет
// теми же ручками, что и кнопки на чеке: заменить блюдо, день «не дома», переставить. Числа нейросеть не
// считает — они приходят из плана. Квота — та же, что у помощника рецептов.

const chatMaxActions = 8

type PlanChat struct {
	pool  *ai.Pool
	plans *Plans
	mu    sync.Mutex
	usage map[string][]time.Time
}

func NewPlanChat(pool *ai.Pool, plans *Plans) *PlanChat {
	return &PlanChat{pool: pool, plans: plans, usage: map[string][]time.Time{}}
}

func (c *PlanChat) Enabled() bool { return c != nil && c.pool != nil && c.pool.Enabled() }

type ChatAction struct {
	Type string `json:"type"` // swap | skip | unskip | move
	Day  int    `json:"day"`
	Slot string `json:"slot,omitempty"`
	To   int    `json:"to,omitempty"`
}

type ChatReply struct {
	Reply   string       `json:"reply"`
	Actions []ChatAction `json:"actions"`
	Applied []string     `json:"applied"` // что реально сделали, словами
	Plan    planner.Plan `json:"plan"`
	Model   string       `json:"model,omitempty"`
}

const chatSystem = `You are the assistant inside a weekly meal-planning app. The user has a ready plan (below) and asks about it in %s.
Answer briefly and concretely in %s, like a friend who cooks: refer to real dishes and days from the plan, never invent dishes that are not in it.
You can change the plan with actions the app will apply:
- {"type":"swap","day":D,"slot":S} — replace the dish of that day and slot with another one (the app picks it; you cannot choose which);
- {"type":"skip","day":D} — the person will not be home that day (dishes stay, but are not bought); {"type":"unskip","day":D} undoes it;
- {"type":"move","day":D,"to":D2,"slot":S} — swap the dishes of the same slot between two days.
Days are 0..6 (0 = Monday), slots: breakfast | lunch | dinner | snack. If the user dislikes a day, swap its dishes (up to 4 actions). If they ask a question, answer without actions.
Never promise things outside these actions (no new recipes, no prices you do not see). Return JSON: {"reply": "...", "actions": [ ... ]} with at most %d actions.`

func (c *PlanChat) Ask(ctx context.Context, planID, message string, lang i18n.Lang, viewer *domain.User) (ChatReply, error) {
	if !c.Enabled() {
		return ChatReply{}, domain.Invalid("api.ai.off")
	}
	if viewer == nil {
		return ChatReply{}, domain.ErrUnauthorized
	}
	message = strings.TrimSpace(message)
	if message == "" || utf8.RuneCountInString(message) > 500 {
		return ChatReply{}, domain.Invalid("api.ai.empty")
	}
	if !c.allow(viewer.ID) {
		return ChatReply{}, domain.Invalid("api.ai.quota")
	}
	plan, err := c.plans.Get(ctx, planID, lang)
	if err != nil {
		return ChatReply{}, err
	}
	// описание плана для модели: только то, что ей нужно для ответа
	type dishLine struct {
		Slot  string `json:"slot"`
		Title string `json:"title"`
		Kcal  int    `json:"kcal"`
		Cost  int    `json:"cost"`
		Min   int    `json:"min"`
	}
	type dayLine struct {
		Day    int        `json:"day"`
		Label  string     `json:"label"`
		Date   string     `json:"date"`
		Away   bool       `json:"away,omitempty"`
		Dishes []dishLine `json:"dishes"`
	}
	var days []dayLine
	for _, d := range plan.Days {
		dl := dayLine{Day: d.Index, Label: d.Label, Date: d.Date, Away: d.Skipped}
		for _, x := range d.Dishes {
			slot := x.Slot
			if x.Course != "" {
				slot = x.Course
			}
			dl.Dishes = append(dl.Dishes, dishLine{Slot: slot, Title: x.Title, Kcal: int(x.Kcal), Cost: int(x.Cost), Min: x.TimeMin})
		}
		days = append(days, dl)
	}
	desc := map[string]any{
		"days":         days,
		"goal":         plan.Goal.Label,
		"kcalPerDay":   plan.Totals.KcalPerDay,
		"weekCost":     int(plan.Totals.Cost),
		"currency":     plan.Country.Symbol,
		"budgetTarget": int(plan.Budget.TargetWeek),
		"adults":       plan.Params.Adults,
		"kids":         len(plan.Params.Kids),
		"occasion":     plan.Occasion != nil,
		"question":     message,
	}
	user, _ := json.Marshal(desc)
	var out struct {
		Reply   string       `json:"reply"`
		Actions []ChatAction `json:"actions"`
	}
	model, err := c.pool.JSONModel(ctx, fmt.Sprintf(chatSystem, i18n.Meta(lang).English, i18n.Meta(lang).English, chatMaxActions), string(user), &out)
	if err != nil {
		return ChatReply{}, domain.Invalid("api.ai.busy")
	}
	reply := ChatReply{Reply: strings.TrimSpace(out.Reply), Actions: out.Actions, Applied: []string{}, Model: model}
	if len(reply.Actions) > chatMaxActions {
		reply.Actions = reply.Actions[:chatMaxActions]
	}
	// применяем действия теми же сервисами, что и кнопки — с проверкой прав на план
	for _, a := range reply.Actions {
		if a.Day < 0 || a.Day >= len(plan.Days) {
			continue
		}
		var perr error
		var next planner.Plan
		switch a.Type {
		case "swap":
			next, perr = c.plans.Swap(ctx, planID, a.Day, a.Slot, lang, viewer)
		case "skip":
			next, perr = c.plans.SetSkip(ctx, planID, a.Day, true, lang, viewer)
		case "unskip":
			next, perr = c.plans.SetSkip(ctx, planID, a.Day, false, lang, viewer)
		case "move":
			next, perr = c.plans.Move(ctx, planID, a.Day, a.To, a.Slot, lang, viewer)
		default:
			continue
		}
		if perr != nil {
			continue // одно неудачное действие (нет альтернативы, чужой план) не ломает остальные
		}
		plan = next
		reply.Applied = append(reply.Applied, describeAction(a, plan, lang))
	}
	reply.Plan = plan
	return reply, nil
}

func describeAction(a ChatAction, plan planner.Plan, lang i18n.Lang) string {
	day := ""
	if a.Day < len(plan.Days) {
		day = plan.Days[a.Day].Label
	}
	switch a.Type {
	case "swap":
		title := ""
		for _, d := range plan.Days[a.Day].Dishes {
			if d.Slot == a.Slot {
				title = d.Title
				break
			}
		}
		return i18n.T(lang, "chat.did.swap", day, planner.SlotLabel(lang, a.Slot), title)
	case "skip":
		return i18n.T(lang, "chat.did.skip", day)
	case "unskip":
		return i18n.T(lang, "chat.did.unskip", day)
	case "move":
		to := ""
		if a.To < len(plan.Days) {
			to = plan.Days[a.To].Label
		}
		return i18n.T(lang, "chat.did.move", planner.SlotLabel(lang, a.Slot), day, to)
	}
	return ""
}

func (c *PlanChat) allow(userID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	keep := c.usage[userID][:0]
	for _, t := range c.usage[userID] {
		if now.Sub(t) < time.Hour {
			keep = append(keep, t)
		}
	}
	if len(keep) >= assistantPerHour {
		c.usage[userID] = keep
		return false
	}
	c.usage[userID] = append(keep, now)
	return true
}
