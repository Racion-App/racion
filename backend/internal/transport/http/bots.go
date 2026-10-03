package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"

	"racion/internal/messenger"
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
