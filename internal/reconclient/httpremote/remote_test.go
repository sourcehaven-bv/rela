package httpremote

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
)

func mustBearer(t *testing.T, v string) Authenticator {
	t.Helper()
	a, err := Bearer(v)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func serve(t *testing.T, h http.HandlerFunc) (*Remote, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	r, err := New(Config{BaseURL: srv.URL + "/sub/", Auth: mustBearer(t, "s3cret")})
	if err != nil {
		t.Fatal(err)
	}
	return r, srv
}

func TestReconcile_SendsRequest(t *testing.T) {
	var got v1.ReconcileRequest
	r, _ := serve(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/sub/api/v1/_reconcile" {
			t.Errorf("%s %s", req.Method, req.URL.Path)
		}
		if a := req.Header.Get("Authorization"); a != "Bearer s3cret" {
			t.Errorf("Authorization = %q", a)
		}
		if req.Header.Get("Cookie") != "" || req.Header.Get("Origin") != "" {
			t.Error("cookie or origin sent")
		}
		_ = json.NewDecoder(req.Body).Decode(&got)
		n := uint64(1)
		_ = json.NewEncoder(w).Encode(v1.ReconcileResponse{Ranges: []v1.ReconcileResult{
			{Status: "ok", Count: &n, Fingerprint: strings.Repeat("0", 32)},
		}})
	})
	out, err := r.Reconcile(t.Context(), v1.ReconcileRequest{Ranges: []v1.ReconcileRange{
		{Kind: "entity", Entities: []string{"ticket"}, Op: "fingerprint"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Ranges) != 1 || got.Ranges[0].Entities[0] != "ticket" || len(out.Ranges) != 1 {
		t.Errorf("sent %+v, got %+v", got, out)
	}
}

func TestReconcile_Refusals(t *testing.T) {
	cases := []struct {
		name  string
		h     http.HandlerFunc
		check func(t *testing.T, err error)
	}{
		{"429 seconds", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
		}, func(t *testing.T, err error) {
			t.Helper()
			var re *reconclient.RefusedError
			if !errors.As(err, &re) || re.RetryAfter != 7*time.Second || re.Status != 429 {
				t.Errorf("err = %v", err)
			}
		}},
		{"503 date", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", time.Now().Add(30*time.Second).UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusServiceUnavailable)
		}, func(t *testing.T, err error) {
			t.Helper()
			var re *reconclient.RefusedError
			if !errors.As(err, &re) || re.RetryAfter < 25*time.Second || re.RetryAfter > 31*time.Second {
				t.Errorf("err = %v", err)
			}
		}},
		{"422 with range", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			i := 2
			_ = json.NewEncoder(w).Encode(v1.Error{
				Type: "https://rela.dev/errors/relation_meta_gated", Detail: "range 2: \x1b[2Jno", Range: &i,
			})
		}, func(t *testing.T, err error) {
			t.Helper()
			var re *reconclient.RangeError
			if !errors.As(err, &re) || re.Index != 2 || !re.MetaGated || strings.ContainsRune(re.Detail, 0x1b) {
				t.Errorf("err = %#v", err)
			}
		}},
		{"422 without range", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(v1.Error{Type: "https://rela.dev/errors/relation_meta_gated"})
		}, func(t *testing.T, err error) {
			t.Helper()
			var re *reconclient.RangeError
			if !errors.As(err, &re) || re.Index != -1 {
				t.Errorf("err = %v", err)
			}
		}},
		{"400 detail", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(v1.Error{Title: "Invalid", Detail: `entity type "x" is not declared`})
		}, func(t *testing.T, err error) {
			t.Helper()
			if err == nil || !strings.Contains(err.Error(), `entity type "x" is not declared`) {
				t.Errorf("err = %v", err)
			}
		}},
		{"401", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
			func(t *testing.T, err error) {
				t.Helper()
				if err == nil || !strings.Contains(err.Error(), "authentication required") {
					t.Errorf("err = %v", err)
				}
			}},
		{"redirect not followed", func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "https://elsewhere.example/", http.StatusTemporaryRedirect)
		}, func(t *testing.T, err error) {
			t.Helper()
			if err == nil || !strings.Contains(err.Error(), "redirect") {
				t.Errorf("err = %v", err)
			}
		}},
		{"oversized answer", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, `{"ranges":[`+strings.Repeat(" ", maxAnswer)+`]}`)
		}, func(t *testing.T, err error) {
			t.Helper()
			if !errors.Is(err, reconclient.ErrProtocol) {
				t.Errorf("err = %v", err)
			}
		}},
		{"not json", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "<html>") },
			func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, reconclient.ErrProtocol) {
					t.Errorf("err = %v", err)
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, srv := serve(t, tc.h)
			_, err := r.Reconcile(t.Context(), v1.ReconcileRequest{})
			tc.check(t, err)
			if err != nil && strings.Contains(err.Error(), "s3cret") {
				t.Error("the credential reached an error")
			}
			if err != nil && strings.Contains(err.Error(), "/sub/") {
				t.Errorf("the error names more than the origin %s: %v", srv.URL, err)
			}
		})
	}
}

func TestNew_ChecksURL(t *testing.T) {
	tok := mustBearer(t, "x")
	cases := []struct {
		cfg Config
		ok  bool
	}{
		{Config{BaseURL: "https://rela.example"}, true},
		{Config{BaseURL: "https://rela.example", Auth: tok}, true},
		{Config{BaseURL: "http://localhost:8080", Auth: tok}, true},
		{Config{BaseURL: "http://127.0.0.1:8080", Auth: tok}, true},
		{Config{BaseURL: "http://[::1]:8080", Auth: tok}, true},
		{Config{BaseURL: "http://rela.example", Auth: tok}, false},
		{Config{BaseURL: "http://rela.example", Auth: tok, AllowInsecureHTTP: true}, true},
		{Config{BaseURL: "http://rela.example"}, false},
		{Config{BaseURL: "http://rela.example", AllowInsecureHTTP: true}, true},
		{Config{BaseURL: "ftp://rela.example"}, false},
		{Config{BaseURL: "https://user:pw@rela.example"}, false},
		{Config{BaseURL: "https://rela.example/?a=b"}, false},
		{Config{BaseURL: "https://rela.example/#x"}, false},
		{Config{BaseURL: "rela.example"}, false},
		{Config{BaseURL: "https://"}, false},
		{Config{BaseURL: "%%"}, false},
	}
	for _, tc := range cases {
		_, err := New(tc.cfg)
		if (err == nil) != tc.ok {
			t.Errorf("%s (auth %v, insecure %v): err = %v", tc.cfg.BaseURL, tc.cfg.Auth != nil,
				tc.cfg.AllowInsecureHTTP, err)
		}
	}
}

func TestAuthenticators(t *testing.T) {
	a := mustBearer(t, " tok \n")
	h := http.Header{}
	a.Authenticate(h)
	if h.Get("Authorization") != "Bearer tok" {
		t.Errorf("header %q", h.Get("Authorization"))
	}
	for _, s := range []string{fmt.Sprint(a), fmt.Sprintf("%v %+v %#v %s", a, a, a, a)} {
		if strings.Contains(s, "tok") && !strings.Contains(s, "redacted") || strings.Contains(s, " tok") {
			t.Errorf("credential printed: %s", s)
		}
	}
	x, err := Header("x-remote-user", "alice")
	if err != nil {
		t.Fatal(err)
	}
	h = http.Header{}
	x.Authenticate(h)
	if h.Get("X-Remote-User") != "alice" {
		t.Errorf("header %v", h)
	}
	if b, err := Header("authorization", "t"); err != nil || fmt.Sprint(b) != "Authorization: [redacted]" {
		t.Errorf("Header(authorization) = %v, %v", b, err)
	}
	for _, bad := range []struct{ name, value string }{
		{"Cookie", "a"}, {"Host", "a"}, {"Content-Type", "a"}, {"bad name", "a"}, {"", "a"},
		{"X-A", ""}, {"X-A", "a\r\nX-B: b"}, {"X-A", "é"}, {"Connection", "a"}, {"Transfer-Encoding", "a"},
		{"Proxy-Authorization", "a"}, {"Upgrade", "a"},
	} {
		if _, err := Header(bad.name, bad.value); err == nil {
			t.Errorf("Header(%q, %q) accepted", bad.name, bad.value)
		}
	}
}

func TestBearer_StripsPrefix(t *testing.T) {
	for _, v := range []string{"tok", "Bearer tok", "bearer tok", " BEARER tok "} {
		h := http.Header{}
		mustBearer(t, v).Authenticate(h)
		if got := h.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Bearer(%q) sends %q", v, got)
		}
	}
	if _, err := Bearer("Bearer "); err == nil {
		t.Error("an empty bearer token was accepted")
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for v, want := range map[string]time.Duration{
		"": 0, "3": 3 * time.Second, "-1": 0, "soon": 0, "999999": 24 * time.Hour,
		now.Add(-time.Minute).Format(http.TimeFormat): 0,
		now.Add(time.Minute).Format(http.TimeFormat):  time.Minute,
	} {
		if got := retryAfter(v, now); got != want {
			t.Errorf("retryAfter(%q) = %s, want %s", v, got, want)
		}
	}
}
