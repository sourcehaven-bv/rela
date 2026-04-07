package dataentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestSecurity(t *testing.T, allowedOrigins ...string) *security {
	t.Helper()
	s, err := newSecurity(SecurityConfig{
		BindAddress:    "127.0.0.1:8080",
		AllowedOrigins: allowedOrigins,
	})
	if err != nil {
		t.Fatalf("newSecurity: %v", err)
	}
	return s
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestRequireLocalHost_AllowsLoopbackHosts(t *testing.T) {
	s := newTestSecurity(t)
	h := s.requireLocalHost(okHandler())

	for _, host := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080"} {
		t.Run(host, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			r.Host = host
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("host %q: expected 200, got %d", host, w.Code)
			}
		})
	}
}

func TestRequireLocalHost_RejectsSpoofedHost(t *testing.T) {
	s := newTestSecurity(t)
	h := s.requireLocalHost(okHandler())

	for _, host := range []string{
		"evil.example",
		"evil.example:8080",
		"127.0.0.1",      // missing port
		"127.0.0.1:9999", // wrong port
		"",               // empty host
		"LOCALHOST:9999", // mixed case but wrong port
	} {
		t.Run(host, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			r.Host = host
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("host %q: expected 403, got %d", host, w.Code)
			}
			if !strings.Contains(w.Body.String(), "host_not_allowed") {
				t.Fatalf("host %q: expected reason in body, got %q", host, w.Body.String())
			}
		})
	}
}

func TestRequireSameOrigin_AllowsExemptPath(t *testing.T) {
	s := newTestSecurity(t)
	h := s.requireSameOrigin(okHandler())

	// Static / SPA paths bypass the Origin check entirely.
	for _, path := range []string{"/", "/index.html", "/static/favicon.ico"} {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			r.Header.Set("Origin", "https://evil.example")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("path %q: expected 200, got %d", path, w.Code)
			}
		})
	}
}

func TestRequireSameOrigin_RejectsCrossOriginOnSensitivePath(t *testing.T) {
	s := newTestSecurity(t)
	h := s.requireSameOrigin(okHandler())

	cases := []struct {
		name   string
		method string
		path   string
		origin string
	}{
		{"POST entities", http.MethodPost, "/api/v1/tickets/", "https://evil.example"},
		{"GET command (img-tag CSRF)", http.MethodGet, "/api/command/run", "https://evil.example"},
		{"GET SSE", http.MethodGet, "/api/events", "https://evil.example"},
		{"DELETE relation", http.MethodDelete, "/api/v1/tickets/T-1/relations/affects/T-2", "https://evil.example"},
		{"Origin: null", http.MethodPost, "/api/v1/tickets/", "null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, http.NoBody)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d", w.Code)
			}
		})
	}
}

func TestRequireSameOrigin_AllowsSameOrigin(t *testing.T) {
	s := newTestSecurity(t)
	h := s.requireSameOrigin(okHandler())

	for _, origin := range []string{
		"http://127.0.0.1:8080",
		"http://localhost:8080",
		"HTTP://LOCALHOST:8080",  // case normalised
		"http://localhost:8080/", // trailing slash tolerated
		"http://[::1]:8080",
	} {
		t.Run(origin, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/", http.NoBody)
			r.Header.Set("Origin", origin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("origin %q: expected 200, got %d (body=%s)", origin, w.Code, w.Body.String())
			}
		})
	}
}

func TestRequireSameOrigin_FallsBackToReferer(t *testing.T) {
	s := newTestSecurity(t)
	h := s.requireSameOrigin(okHandler())

	r := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/", http.NoBody)
	r.Header.Set("Referer", "http://localhost:8080/some/page")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 via Referer fallback, got %d", w.Code)
	}
}

func TestRequireSameOrigin_RejectsMissingOriginAndReferer(t *testing.T) {
	s := newTestSecurity(t)
	h := s.requireSameOrigin(okHandler())

	r := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 with no Origin/Referer, got %d", w.Code)
	}
}

func TestRequireSameOrigin_AcceptsExtraAllowedOrigin(t *testing.T) {
	// Vue dev workflow: Vite on :5173 proxying to :8080.
	s := newTestSecurity(t, "http://localhost:5173")
	h := s.requireSameOrigin(okHandler())

	r := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/", http.NoBody)
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for allowed dev origin, got %d", w.Code)
	}
}

func TestNormaliseOrigin(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"http://localhost:8080", "http://localhost:8080", false},
		{"HTTP://LOCALHOST:8080", "http://localhost:8080", false},
		{"http://localhost", "http://localhost:80", false},
		{"https://example.com", "https://example.com:443", false},
		{"http://localhost:8080/", "http://localhost:8080", false},
		{"http://localhost:8080/path", "", true},
		{"http://localhost:8080?q=1", "", true},
		{"file:///etc/passwd", "", true},
		{"javascript:alert(1)", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := normaliseOrigin(tc.in)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("normaliseOrigin(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitBindAddress(t *testing.T) {
	cases := []struct {
		in       string
		wantHost string
		wantPort string
		err      bool
	}{
		{":8080", "0.0.0.0", "8080", false},
		{"127.0.0.1:8080", "127.0.0.1", "8080", false},
		{"[::1]:8080", "::1", "8080", false},
		{"0.0.0.0:8080", "0.0.0.0", "8080", false},
		{"nonsense", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			h, p, err := splitBindAddress(tc.in)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if h != tc.wantHost || p != tc.wantPort {
				t.Fatalf("got (%q,%q), want (%q,%q)", h, p, tc.wantHost, tc.wantPort)
			}
		})
	}
}

func TestNewSecurity_NonLoopbackBindOnlyAcceptsItself(t *testing.T) {
	s, err := newSecurity(SecurityConfig{BindAddress: "10.0.0.5:9000"})
	if err != nil {
		t.Fatalf("newSecurity: %v", err)
	}
	if _, ok := s.allowedHosts["10.0.0.5:9000"]; !ok {
		t.Fatalf("expected bound host in allowlist")
	}
	if _, ok := s.allowedHosts["127.0.0.1:9000"]; ok {
		t.Fatalf("loopback should NOT be in allowlist when bound to non-loopback")
	}
}

func TestNewSecurity_UnspecifiedBindAcceptsAnyHost(t *testing.T) {
	// Operator passed -bind 0.0.0.0; the Host header check must be relaxed
	// because we cannot enumerate the legitimate hostnames a client might
	// send (LAN IPs, mDNS names, /etc/hosts entries, …).
	for _, addr := range []string{"0.0.0.0:8080", ":8080", "[::]:8080"} {
		t.Run(addr, func(t *testing.T) {
			s, err := newSecurity(SecurityConfig{BindAddress: addr})
			if err != nil {
				t.Fatalf("newSecurity(%q): %v", addr, err)
			}
			if s.allowedHosts != nil {
				t.Fatalf("expected nil allowedHosts when bound to unspecified, got %v", s.allowedHosts)
			}

			h := s.requireLocalHost(okHandler())
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			r.Host = "my-laptop.local:8080"
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("unspecified bind must accept arbitrary Host, got %d", w.Code)
			}
		})
	}
}

func TestRequireLocalHost_AllowsCaseInsensitiveHost(t *testing.T) {
	s := newTestSecurity(t)
	h := s.requireLocalHost(okHandler())

	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.Host = "LOCALHOST:8080"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected mixed-case Host to be accepted, got %d", w.Code)
	}
}
