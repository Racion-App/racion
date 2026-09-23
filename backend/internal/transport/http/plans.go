package http

import (
	"net/http"
	"strconv"

	"racion/internal/domain"
	"racion/internal/i18n"
	"racion/internal/planner"
)

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	var p planner.Params
	if !decode(w, r, 64<<10, &p) {
		return
	}
	plan, err := s.svc.Plans.Create(r.Context(), p, langOf(r, p.Lang), currentUser(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, plan)
}

func (s *Server) getPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := s.svc.Plans.Get(r.Context(), r.PathValue("id"), i18n.FromRequest(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	plan.Family = s.svc.Plans.Family(r.Context(), plan.ID)
	writeJSON(w, 200, plan)
}

func (s *Server) swap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Day  int    `json:"day"`
		Slot string `json:"slot"`
	}
	if !decode(w, r, 4<<10, &body) {
		return
	}
	plan, err := s.svc.Plans.Swap(r.Context(), r.PathValue("id"), body.Day, body.Slot, i18n.FromRequest(r), currentUser(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, plan)
}

func (s *Server) swapSide(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Day  int    `json:"day"`
		Slot string `json:"slot"`
	}
	if !decode(w, r, 4<<10, &body) {
		return
	}
	plan, err := s.svc.Plans.SwapSide(r.Context(), r.PathValue("id"), body.Day, body.Slot, i18n.FromRequest(r), currentUser(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, plan)
}

func (s *Server) skipDay(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Day  int  `json:"day"`
		Skip bool `json:"skip"`
	}
	if !decode(w, r, 4<<10, &body) {
		return
	}
	plan, err := s.svc.Plans.SetSkip(r.Context(), r.PathValue("id"), body.Day, body.Skip, i18n.FromRequest(r), currentUser(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, plan)
}

func (s *Server) moveDish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		From int    `json:"from"`
		To   int    `json:"to"`
		Slot string `json:"slot"`
	}
	if !decode(w, r, 4<<10, &body) {
		return
	}
	plan, err := s.svc.Plans.Move(r.Context(), r.PathValue("id"), body.From, body.To, body.Slot, i18n.FromRequest(r), currentUser(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, plan)
}

// repeatPlan — та же неделя на новые даты; план становится своим у того, кто повторил.
func (s *Server) repeatPlan(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		StartDate string `json:"startDate"`
	}
	if !decode(w, r, 4<<10, &body) {
		return
	}
	plan, err := s.svc.Plans.Repeat(r.Context(), r.PathValue("id"), body.StartDate, i18n.FromRequest(r), *u)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, plan)
}

func (s *Server) renamePlan(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Title string `json:"title"`
	}
	if !decode(w, r, 4<<10, &body) {
		return
	}
	if err := s.svc.Plans.Rename(r.Context(), r.PathValue("id"), u.ID, body.Title); err != nil {
		writeErr(w, 404, "not found")
		return
	}
	w.WriteHeader(204)
}

func (s *Server) deletePlan(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	if err := s.svc.Plans.Delete(r.Context(), r.PathValue("id"), u.ID); err != nil {
		writeErr(w, 404, "not found")
		return
	}
	w.WriteHeader(204)
}

// ── Отметки и свои товары: доступны и гостю по ссылке на план ─────────────

func (s *Server) planChecks(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.Plans.Checks(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) setCheck(w http.ResponseWriter, r *http.Request) {
	var in domain.CheckInput
	if !decode(w, r, 4<<10, &in) {
		return
	}
	if err := s.svc.Plans.SetCheck(r.Context(), r.PathValue("id"), in, currentUser(r)); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) listExtras(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.Plans.Extras(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) addExtra(w http.ResponseWriter, r *http.Request) {
	var e domain.Extra
	if !decode(w, r, 4<<10, &e) {
		return
	}
	out, err := s.svc.Plans.AddExtra(r.Context(), r.PathValue("id"), e)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 201, out)
}

func (s *Server) deleteExtra(w http.ResponseWriter, r *http.Request) {
	eid, err := strconv.ParseInt(r.PathValue("extra"), 10, 64)
	if err != nil {
		writeErr(w, 404, "not found")
		return
	}
	_ = s.svc.Plans.DeleteExtra(r.Context(), r.PathValue("id"), eid)
	w.WriteHeader(204)
}

// planChat — вопрос помощнику по неделе: ответ словами и применённые действия (замены, «не дома», перестановки).
func (s *Server) planChat(w http.ResponseWriter, r *http.Request) {
	u := requireUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Message string `json:"message"`
	}
	if !decode(w, r, 8<<10, &body) {
		return
	}
	out, err := s.svc.PlanChat.Ask(r.Context(), r.PathValue("id"), body.Message, i18n.FromRequest(r), u)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, 200, out)
}

// langOf — язык ответа: из тела запроса, если он там задан, иначе как обычно (?lang=, cookie, Accept-Language).
// Клиенты API шлют язык в параметрах плана — в openapi.json это описано, и обещание надо держать.
func langOf(r *http.Request, want string) i18n.Lang {
	if l, ok := i18n.Valid(want); ok {
		return l
	}
	return i18n.FromRequest(r)
}
