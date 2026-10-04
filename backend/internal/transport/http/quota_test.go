package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

type fakeQuota struct{ n map[string]int }

func (f *fakeQuota) Hit(_ context.Context, key string) (int, int, error) {
	f.n[key]++
	return f.n[key], f.n[key], nil
}

func TestFirstParty(t *testing.T) {
	s := &Server{}
	req := func(h map[string]string) *http.Request {
		r := httptest.NewRequest("GET", "https://racion.app/api/recipes", nil)
		for k, v := range h {
			r.Header.Set(k, v)
		}
		return r
	}
	cases := []struct {
		h    map[string]string
		want bool
	}{
		{map[string]string{"Sec-Fetch-Site": "same-origin"}, true},
		{map[string]string{"Sec-Fetch-Site": "cross-site", "Referer": "https://racion.app/"}, false}, // браузер главнее подделанного Referer
		{map[string]string{"Origin": "https://racion.app"}, true},
		{map[string]string{"Referer": "https://racion.app/plan/1"}, true},
		{map[string]string{"Origin": "https://example.com"}, false},
		{map[string]string{}, false}, // curl, MCP, сервер разработчика
	}
	for i, c := range cases {
		if got := s.firstParty(req(c.h)); got != c.want {
			t.Errorf("case %d %v: %v, want %v", i, c.h, got, c.want)
		}
	}
}

func TestOverQuota(t *testing.T) {
	q := &fakeQuota{n: map[string]int{}}
	s := &Server{quota: q, log: zap.NewNop()}
	third := func() (*httptest.ResponseRecorder, bool) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "https://racion.app/api/recipes", nil)
		r.RemoteAddr = "203.0.113.7:5000"
		return w, s.overQuota(w, r)
	}
	for i := 1; i <= apiHourQuota; i++ {
		if w, over := third(); over {
			t.Fatalf("request %d rejected: %d", i, w.Code)
		}
	}
	w, over := third()
	if !over || w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" || w.Header().Get("X-RateLimit-Remaining-Hour") != "0" {
		t.Fatalf("over the hour quota: over=%v code=%d headers=%v", over, w.Code, w.Header())
	}
	// сайт квоту не тратит
	r := httptest.NewRequest("GET", "https://racion.app/api/recipes", nil)
	r.RemoteAddr = "203.0.113.7:5000"
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	if s.overQuota(httptest.NewRecorder(), r) {
		t.Fatal("first-party request must not be limited")
	}
	if quotaKey("2001:db8:1:2:aa::1") != quotaKey("2001:db8:1:2:bb::9") || quotaKey("203.0.113.7") != "203.0.113.7" {
		t.Fatal("IPv6 must be counted per /64")
	}
}
