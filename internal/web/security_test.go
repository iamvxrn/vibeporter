package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibeporter/internal/models"
)

// A page on another origin must not be able to drive the local API. A POST with
// a "simple" content type is delivered without a CORS preflight, so before the
// same-origin guard a visited web page could call /api/handoff with a `target`
// of its choosing and write a file anywhere the user can write, even though it
// could not read the response.
func TestAPIRefusesCrossOriginWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", "")

	src := filepath.Join(home, "src.jsonl")
	conv := &models.Conversation{ID: "victim", Title: "T", AgentSource: "gemini",
		Messages: []models.Message{models.NewMessage(models.RoleUser, []models.Part{models.TextPart("private chat data")})}}
	if _, err := injectors["gemini"].Inject(conv, src); err != nil {
		t.Fatal(err)
	}

	victim := filepath.Join(home, "pwned.jsonl")
	body, _ := json.Marshal(map[string]string{
		"from": "gemini", "to": "gemini", "source": src,
		"target": victim, "compact": "200k", "strategy": "recent",
	})

	for _, tc := range []struct {
		name        string
		contentType string
		origin      string
		secFetch    string
	}{
		{"no-cors simple content type", "text/plain;charset=UTF-8", "https://evil.example", "cross-site"},
		{"forged json content type", "application/json", "https://evil.example", "cross-site"},
		{"origin only", "application/json", "https://evil.example", ""},
		{"sec-fetch-site only", "application/json", "", "cross-site"},
		{"form post", "application/x-www-form-urlencoded", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Remove(victim)
			req := httptest.NewRequest(http.MethodPost, "/api/handoff", strings.NewReader(string(body)))
			req.Host = "127.0.0.1:8080"
			req.Header.Set("Content-Type", tc.contentType)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.secFetch != "" {
				req.Header.Set("Sec-Fetch-Site", tc.secFetch)
			}
			rr := httptest.NewRecorder()
			newMux().ServeHTTP(rr, req)

			if rr.Code == http.StatusOK {
				t.Errorf("cross-origin POST accepted (status 200): %s", rr.Body.String())
			}
			if _, err := os.Stat(victim); err == nil {
				t.Errorf("cross-origin POST wrote %s", victim)
			}
		})
	}
}

// The bundled UI must still work: same-origin JSON requests pass the guard.
func TestAPIAllowsSameOrigin(t *testing.T) {
	for _, origin := range []string{"", "http://127.0.0.1:8080"} {
		req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
		req.Host = "127.0.0.1:8080"
		if origin != "" {
			req.Header.Set("Origin", origin)
			req.Header.Set("Sec-Fetch-Site", "same-origin")
		}
		rr := httptest.NewRecorder()
		newMux().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("same-origin GET (origin=%q) rejected: %d %s", origin, rr.Code, rr.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/handoff", strings.NewReader(`{}`))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rr := httptest.NewRecorder()
	newMux().ServeHTTP(rr, req)
	// Reaches the handler (which rejects the empty body on its own merits),
	// rather than being blocked by the guard.
	if rr.Code == http.StatusForbidden || rr.Code == http.StatusUnsupportedMediaType {
		t.Fatalf("same-origin POST blocked by guard: %d %s", rr.Code, rr.Body.String())
	}
}

// serve must expose every agent the CLI does. dsh was documented, had a full
// adapter and its own docs page, but was missing from the web maps and from the
// hardcoded agent lists, so `vibeporter serve` silently showed no DSH chats.
func TestWebAgentListCoversExtractors(t *testing.T) {
	listed := map[string]bool{}
	for _, a := range canonicalAgents() {
		listed[a] = true
		if _, ok := extractors[a]; !ok {
			t.Errorf("agent %q listed by the UI has no extractor", a)
		}
		if _, ok := injectors[a]; !ok {
			t.Errorf("agent %q listed by the UI has no injector", a)
		}
	}
	aliases := map[string]bool{"ag": true, "kimi": true, "dhs": true, "wind": true}
	for k := range extractors {
		if !listed[k] && !aliases[k] {
			t.Errorf("extractor %q is registered but never listed to the UI", k)
		}
	}
}

// Every /api route must sit behind sameOriginOnly. A route wired straight to
// its handler would reopen the cross-site write above.
func TestEveryAPIRouteIsGuarded(t *testing.T) {
	routes := []string{
		"/api/agents", "/api/chats", "/api/conversation", "/api/search",
		"/api/diff", "/api/migrate", "/api/handoff", "/api/handoff/preview", "/api/stats",
	}
	mux := newMux()
	for _, route := range routes {
		req := httptest.NewRequest(http.MethodPost, route, strings.NewReader("{}"))
		req.Host = "127.0.0.1:8080"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://evil.example")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s: cross-origin POST got %d, want 403 (route not behind sameOriginOnly?)", route, rr.Code)
		}
	}
}
