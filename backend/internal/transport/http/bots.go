package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"

	"racion/internal/domain"
	"racion/internal/messenger"
	"racion/internal/service"
)

// botUpdate — вебхук Telegram или MAX. Подпись проверяет клиент мессенджера (секрет в заголовке).
// На всё, кроме чужой подписи, отвечаем 200: на ошибку мессенджер шлёт обновление повторно,
// и человек получил бы список дважды. Ошибку пишем в лог.
func (s *Server) botUpdate(w http.ResponseWriter, r *http.Request) {
	c, ok := s.svc.Bots.Client(r.PathValue("platform"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	u, ok, err := c.Parse(r)
	switch {
	case errors.Is(err, messenger.ErrForged):
		w.WriteHeader(http.StatusUnauthorized)
		return
	case err != nil:
		s.log.Warn("bot update", zap.String("platform", string(c.Platform())), zap.Error(err))
	case ok:
		// мессенджер может оборвать запрос, а отметку о покупке надо довести до конца
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 20*time.Second)
		defer cancel()
		if err := s.svc.Bots.Handle(ctx, u); err != nil {
			s.log.Warn("bot update", zap.String("platform", string(u.Platform)), zap.Error(err))
		}
	}
	w.WriteHeader(http.StatusOK)
}

// webAppAuth — сайт открыт мини-приложением внутри мессенджера: вход по подписи мессенджера. Сайт зовёт
// это при каждом открытии, а с create=true — когда человек сам нажал «Войти через Telegram».
func (s *Server) webAppAuth(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	var body struct {
		InitData string `json:"initData"`
		Create   bool   `json:"create"`
		Plan     string `json:"plan"`
	}
	if !decode(w, r, 16<<10, &body) {
		return
	}
	wu, err := s.svc.Bots.WebApp(platform, body.InitData)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	viewer := currentUser(r)
	res, err := s.svc.Accounts.LoginExternal(r.Context(), service.OAuthProfile{Provider: platform, ID: wu.ID, Name: wu.Name, Avatar: wu.Photo}, viewer, body.Create, body.Plan)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if res.Session != nil {
		setSessionCookie(w, r, *res.Session)
	}
	switch {
	case res.User != nil:
		_ = s.svc.Bots.Known(r.Context(), platform, wu.ID, res.User.ID)
	case res.Status == "linked":
		_ = s.svc.Bots.Known(r.Context(), platform, wu.ID, viewer.ID)
	}
	writeJSON(w, 200, map[string]any{"status": res.Status, "user": res.User, "open": s.svc.Bots.StartPath(wu.StartParam)})
}

// webAppList — «Список в Telegram» внутри мини-приложения: бот присылает список прямо в чат. Человек
// ещё не начинал чат с ботом — 409, и сайт открывает обычную ссылку на бота.
func (s *Server) webAppList(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	var body struct {
		InitData string `json:"initData"`
		Plan     string `json:"plan"`
	}
	if !decode(w, r, 16<<10, &body) {
		return
	}
	wu, err := s.svc.Bots.WebApp(platform, body.InitData)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	err = s.svc.Bots.SendList(r.Context(), platform, wu, body.Plan)
	if errors.Is(err, messenger.ErrBlocked) {
		writeJSON(w, http.StatusConflict, map[string]bool{"open": true})
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"sent": true})
}

// linkMessenger — одноразовая ссылка на бота: по ней бот привяжет мессенджер к аккаунту.
func (s *Server) linkMessenger(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	link, err := s.svc.Bots.LinkURL(r.Context(), r.PathValue("platform"), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"url": link})
}

// unlinkMessenger — отвязать мессенджер от аккаунта. Если это единственный способ войти, сервис откажет.
func (s *Server) unlinkMessenger(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	platform := r.PathValue("platform")
	if _, ok := s.svc.Bots.Client(platform); !ok {
		s.fail(w, r, domain.ErrNotFound)
		return
	}
	if err := s.svc.Accounts.Unlink(r.Context(), u.ID, platform); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.svc.Bots.Unlinked(r.Context(), platform, u.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
