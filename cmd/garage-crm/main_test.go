package main

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/garage-crm/internal/app"
)

// Exercises the real listener, private bootstrap, browser cookies, persistence,
// and Quit-to-database-close lifecycle rather than a mocked web callback.
func TestRealLocalSaveQuitAndRestart(t *testing.T) {
	root := t.TempDir()
	finished := make(chan error, 1)
	go func() { finished <- serve(root, 0, true) }()
	var starter string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		matches, err := filepath.Glob(filepath.Join(root, "garage-unlock-*.html"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) == 1 {
			b, err := os.ReadFile(matches[0])
			if err != nil {
				t.Fatal(err)
			}
			starter = string(b)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if starter == "" {
		t.Fatal("server did not create private starter")
	}
	bootURL := regexp.MustCompile(`action="(http://127\.0\.0\.1:[0-9]+/bootstrap)"`).FindStringSubmatch(starter)
	token := regexp.MustCompile(`name="token" value="([a-f0-9]+)"`).FindStringSubmatch(starter)
	if len(bootURL) != 2 || len(token) != 2 {
		t.Fatalf("unexpected starter page: %q", starter)
	}
	origin := strings.TrimSuffix(bootURL[1], "/bootstrap")
	client := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	post := func(path string, values url.Values, cookie *http.Cookie) *http.Response {
		t.Helper()
		r, err := http.NewRequest(http.MethodPost, origin+path, strings.NewReader(values.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "null")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	bootstrap := post("/bootstrap", url.Values{"token": {token[1]}}, nil)
	defer bootstrap.Body.Close()
	if bootstrap.StatusCode != http.StatusSeeOther || len(bootstrap.Cookies()) != 1 {
		t.Fatalf("bootstrap status %d, cookies %d", bootstrap.StatusCode, len(bootstrap.Cookies()))
	}
	cookie := bootstrap.Cookies()[0]
	dashboardReq, err := http.NewRequest(http.MethodGet, origin+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	dashboardReq.AddCookie(cookie)
	dashboard, err := client.Do(dashboardReq)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(dashboard.Body)
	dashboard.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	csrf := regexp.MustCompile(`name="_csrf" value="([a-f0-9]+)"`).FindStringSubmatch(string(body))
	if dashboard.StatusCode != http.StatusOK || len(csrf) != 2 {
		t.Fatalf("dashboard status %d, CSRF found %v", dashboard.StatusCode, len(csrf) == 2)
	}
	blocked := post("/save", url.Values{"type": {"people"}, "name": {"Blocked"}}, cookie)
	blocked.Body.Close()
	if blocked.StatusCode != http.StatusForbidden {
		t.Fatalf("missing CSRF accepted: %d", blocked.StatusCode)
	}
	saved := post("/save", url.Values{"_csrf": {csrf[1]}, "type": {"people"}, "name": {"Saved after repair"}}, cookie)
	saved.Body.Close()
	if saved.StatusCode != http.StatusSeeOther {
		t.Fatalf("save refused: %d", saved.StatusCode)
	}
	quit := post("/quit", url.Values{"_csrf": {csrf[1]}}, cookie)
	quitPage, err := io.ReadAll(quit.Body)
	quit.Body.Close()
	if err != nil || quit.StatusCode != http.StatusOK || !strings.Contains(string(quitPage), "has closed") {
		t.Fatalf("quit failed: status %d, error %v", quit.StatusCode, err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Quit did not shut down the listener and database")
	}
	db, err := openRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := (&app.Service{DB: db}).Lists("people", "", "Saved after repair", 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("save not persisted across restart: rows %d, error %v", len(rows), err)
	}
}
