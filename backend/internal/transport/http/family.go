package http

import (
	"errors"
	"go.uber.org/zap"
	"net/http"

	"racion/internal/domain"
	"racion/internal/i18n"
)

// ── Семья: присоединиться к плану по ссылке ────────────────────────────────

func (s *Server) joinPlan(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if err := s.svc.Plans.Join(r.Context(), r.PathValue("id"), *u); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// ── Push-уведомления ───────────────────────────────────────────────────────

func (s *Server) pushKey(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"key": s.svc.Notify.PublicKey()})
}

func (s *Server) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var sub domain.PushSubscription
	if !decode(w, r, 8<<10, &sub) {
		return
	}
	sub.Lang = string(i18n.FromRequest(r))
	if err := s.svc.Notify.Subscribe(r.Context(), u.ID, sub); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) pushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if !decode(w, r, 4<<10, &body) {
		return
	}
	_ = s.svc.Notify.Unsubscribe(r.Context(), body.Endpoint)
	w.WriteHeader(204)
}

// pushDevice — напоминания без регистрации: подписка этого устройства и неделя, по которой напоминать.
// Устройство, уже подписанное, присылает так и каждую новую неделю.
func (s *Server) pushDevice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		domain.PushSubscription
		PlanID string `json:"planId"`
		Tz     int    `json:"tz"`
		Today  bool   `json:"today"`
	}
	if !decode(w, r, 8<<10, &body) {
		return
	}
	sub := body.PushSubscription
	sub.Lang = string(i18n.FromRequest(r))
	if err := s.svc.Notify.SubscribeDevice(r.Context(), sub, body.PlanID, body.Tz, body.Today); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) pushDeviceDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if !decode(w, r, 4<<10, &body) {
		return
	}
	_ = s.svc.Notify.UnsubscribeDevice(r.Context(), body.Endpoint)
	w.WriteHeader(204)
}

func (s *Server) notifySettings(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	st, devices, err := s.svc.Notify.Settings(r.Context(), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"settings": st, "devices": devices})
}

func (s *Server) setNotifySettings(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var st domain.NotifySettings
	if !decode(w, r, 4<<10, &st) {
		return
	}
	if err := s.svc.Notify.SetSettings(r.Context(), u.ID, st); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// notifyTestResult — устройство сообщает, чем кончилась проверка (shown/hidden/lost): виден обрыв цепочки
func (s *Server) notifyTestResult(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Result string `json:"result"`
		UA     string `json:"ua"`
	}
	if !decode(w, r, 4<<10, &body) {
		return
	}
	s.log.Info("push test result", zap.String("user", u.ID), zap.String("result", body.Result), zap.String("ua", body.UA))
	w.WriteHeader(204)
}

func (s *Server) notifyTest(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if err := s.svc.Notify.Test(r.Context(), u.ID, i18n.FromRequest(r)); err != nil {
		// подписок нет или push-сервис их отверг: не «план не найден», а понятное объяснение
		if errors.Is(err, domain.ErrNotFound) {
			writeErr(w, 404, i18n.T(i18n.FromRequest(r), "notify.test.nodevice"))
			return
		}
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}
