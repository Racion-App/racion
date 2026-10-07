package http

import (
	"errors"
	"net/http"
	"strings"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/service"
)

// Вход на сайт через бота Telegram или MAX (см. service/bots_login.go). Страница входа получает
// ссылку на бота, а секрет ожидания уходит в cookie только этому браузеру: дождаться подтверждения
// и получить сессию может лишь тот, кто начал вход.

const messengerLoginCookie = "racion_mlogin"

// messengerLoginStart — запрос входа: ссылка на бота и cookie с секретом ожидания.
func (s *Server) messengerLoginStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Plan string `json:"plan"`
	}
	if r.ContentLength > 0 && !decode(w, r, 4<<10, &body) {
		return
	}
	link, secret, err := s.svc.Bots.LoginStart(r.Context(), r.PathValue("platform"), i18n.FromRequest(r), body.Plan, agentOf(r.UserAgent()))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: messengerLoginCookie, Value: secret, Path: "/api/auth/messenger", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"), MaxAge: int(service.LoginTTL.Seconds()),
	})
	writeJSON(w, 200, map[string]string{"url": link})
}

// messengerLoginPoll — подтвердили ли вход в боте: 202 — ждём, 200 — сессия открыта, 410 — запроса
// больше нет (истёк или человек ответил «это не я»).
func (s *Server) messengerLoginPoll(w http.ResponseWriter, r *http.Request) {
	ck, _ := r.Cookie(messengerLoginCookie)
	secret := ""
	if ck != nil {
		secret = ck.Value
	}
	userID, planID, err := s.svc.Bots.LoginTake(r.Context(), secret)
	if errors.Is(err, domain.ErrNotFound) {
		writeErr(w, 410, i18n.T(i18n.FromRequest(r), "auth.messenger.expired"))
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if userID == "" {
		writeJSON(w, 202, map[string]string{"status": "pending"})
		return
	}
	u, sess, err := s.svc.Accounts.SessionFor(r.Context(), userID, planID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: messengerLoginCookie, Value: "", Path: "/api/auth/messenger", MaxAge: -1})
	setSessionCookie(w, r, sess)
	writeJSON(w, 200, u)
}

// agentOf — «Chrome, Windows» из User-Agent: бот показывает, откуда входят, чтобы человек узнал свой
// браузер. Неизвестное — пусто.
func agentOf(ua string) string {
	browser := ""
	for _, b := range []struct{ key, name string }{
		{"YaBrowser", "Яндекс Браузер"}, {"Edg/", "Edge"}, {"OPR/", "Opera"}, {"Firefox/", "Firefox"},
		{"Chrome/", "Chrome"}, {"CriOS", "Chrome"}, {"FxiOS", "Firefox"}, {"Safari/", "Safari"},
	} {
		if strings.Contains(ua, b.key) {
			browser = b.name
			break
		}
	}
	system := ""
	for _, o := range []struct{ key, name string }{
		{"iPhone", "iPhone"}, {"iPad", "iPad"}, {"Android", "Android"}, {"Windows", "Windows"},
		{"Mac OS X", "macOS"}, {"CrOS", "ChromeOS"}, {"Linux", "Linux"},
	} {
		if strings.Contains(ua, o.key) {
			system = o.name
			break
		}
	}
	switch {
	case browser != "" && system != "":
		return browser + ", " + system
	case browser != "":
		return browser
	}
	return system
}
