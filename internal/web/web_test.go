package web

import (
	"github.com/jonathanselby/garage-crm/internal/app"
	"github.com/jonathanselby/garage-crm/internal/sqlite"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func serverTest(t *testing.T) *Server {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = app.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return New(&app.Service{DB: db}, "127.0.0.1:43812")
}
func req(t *testing.T, s *Server, method, path string, values url.Values, authenticated bool, origin string) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if values != nil {
		body = strings.NewReader(values.Encode())
	} else {
		body = strings.NewReader("")
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:43812"+path, body)
	r.RemoteAddr = "127.0.0.1:49002"
	if values != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if authenticated {
		r.AddCookie(&http.Cookie{Name: "garage_session", Value: s.Session})
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestProtectedServerAndCSRF(t *testing.T) {
	s := serverTest(t)
	w := req(t, s, "GET", "/", nil, false, "")
	if w.Code != 401 {
		t.Fatalf("unauthenticated GET: %d", w.Code)
	}
	w = req(t, s, "POST", "/bootstrap", url.Values{"token": {"bad"}}, false, "null")
	if w.Code != 403 {
		t.Fatalf("invalid bootstrap: %d", w.Code)
	}
	w = req(t, s, "POST", "/bootstrap", url.Values{"token": {s.Bootstrap}}, false, "null")
	if w.Code != 303 {
		t.Fatalf("bootstrap %d: %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/bootstrap", url.Values{"token": {s.Bootstrap}}, false, "null")
	if w.Code != 403 {
		t.Fatal("replayed bootstrap allowed")
	}
	w = req(t, s, "POST", "/save", url.Values{"type": {"people"}, "name": {"Mara"}, "_csrf": {s.CSRF}}, true, "https://evil.invalid")
	if w.Code != 403 {
		t.Fatalf("cross-site origin accepted %d", w.Code)
	}
	w = req(t, s, "POST", "/save", url.Values{"type": {"people"}, "name": {"Mara"}}, true, "http://127.0.0.1:43812")
	if w.Code != 403 {
		t.Fatalf("missing CSRF accepted %d", w.Code)
	}
	w = req(t, s, "GET", "/", nil, true, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Good to have you back") {
		t.Fatalf("dashboard %d: %s", w.Code, w.Body.String())
	}
}
func TestFormsAndXSS(t *testing.T) {
	s := serverTest(t)
	w := req(t, s, "GET", "/record?type=brands", nil, true, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Save record") {
		t.Fatalf("new brand form: %s", w.Body.String())
	}
	w = req(t, s, "POST", "/save", url.Values{"_csrf": {s.CSRF}, "type": {"brands"}, "name": {"StagZero"}, "currency": {"AUD"}, "colour": {"#3e7ac4"}}, true, "http://127.0.0.1:43812")
	if w.Code != 303 {
		t.Fatalf("save brand %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/save", url.Values{"_csrf": {s.CSRF}, "type": {"people"}, "name": {"<script>alert(1)</script>"}}, true, "http://127.0.0.1:43812")
	if w.Code != 303 {
		t.Fatalf("save person %d %s", w.Code, w.Body.String())
	}
	w = req(t, s, "GET", "/list?type=people", nil, true, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "<script>alert(1)</script>") || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
		t.Fatalf("unescaped HTML or wrong output: %s", w.Body.String())
	}
	w = req(t, s, "GET", "/record?type=leads", nil, true, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Choose") {
		t.Fatalf("new lead form %d: %s", w.Code, w.Body.String())
	}
}
func TestHostAndAttachmentHeaders(t *testing.T) {
	s := serverTest(t)
	r := httptest.NewRequest("GET", "http://evil.invalid/", nil)
	r.RemoteAddr = "127.0.0.1:40001"
	r.AddCookie(&http.Cookie{Name: "garage_session", Value: s.Session})
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("DNS rebinding Host accepted")
	}
	w = req(t, s, "GET", "/static/site.css", nil, false, "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/css; charset=utf-8" {
		t.Fatalf("CSS %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'none'") {
		t.Fatal("missing strict CSP")
	}
}

// Regression: browser form posts must retain their same-origin context while
// opaque and cross-site origins remain forbidden.
func TestReferrerPolicyKeepsSameOriginPostsWorking(t *testing.T) {
	s := serverTest(t)
	w := req(t, s, "GET", "/", nil, true, "")
	if got := w.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Fatalf("referrer policy = %q; want same-origin", got)
	}
	values := url.Values{"type": {"people"}, "name": {"Mara"}, "_csrf": {s.CSRF}}
	w = req(t, s, "POST", "/save", values, true, "http://127.0.0.1:43812")
	if w.Code != http.StatusSeeOther {
		t.Fatalf("valid same-origin save: status %d, body %s", w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/save", values, true, "null")
	if w.Code != http.StatusForbidden {
		t.Fatalf("opaque origin accepted: status %d", w.Code)
	}
	w = req(t, s, "POST", "/save", values, true, "https://evil.invalid")
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin request accepted: status %d", w.Code)
	}
}
