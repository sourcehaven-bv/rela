package idporg_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/idporg"
)

const testSecret = "operator-secret"

// verify checks r the way the operator API does: the date within 300 s and
// the HMAC over the canonical string, rebuilt here independently of the
// client.
func verify(t *testing.T, r *http.Request) bool {
	t.Helper()
	date := r.Header.Get("X-Pratique-Date")
	secs, err := strconv.ParseInt(date, 10, 64)
	if err != nil {
		return false
	}
	if d := time.Now().Unix() - secs; d > 300 || d < -300 {
		return false
	}
	empty := sha256.Sum256(nil)
	canonical := r.Method + "\n" + r.URL.Path + "\n" + date + "\n" + hex.EncodeToString(empty[:])
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(canonical))
	got, err := base64.StdEncoding.DecodeString(r.Header.Get("X-Pratique-Signature"))
	return err == nil && hmac.Equal(mac.Sum(nil), got)
}

// operatorAPI answers GET /api/v1/orgs/{id} from orgs, as pratique does.
func operatorAPI(t *testing.T, orgs map[string]string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/orgs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !verify(t, r) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		id := r.PathValue("id")
		status, ok := orgs[id]
		if !ok {
			http.Error(w, `{"error":"org not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + id + `","slug":"s","name":"n","status":"` + status +
			`","entitlements":[],"created_at":"2026-01-01T00:00:00Z"}`))
	})
	return mux
}

func TestOrgStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(operatorAPI(t, map[string]string{
		"org-a": "active", "org-s": "suspended", "org-e": "", "org-x": "frozen",
	}))
	t.Cleanup(srv.Close)
	c, err := idporg.New(srv.URL, []byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, org string
		want      idporg.Status
		wantErr   error
	}{
		{name: "active", org: "org-a", want: idporg.StatusActive},
		{name: "suspended", org: "org-s", want: idporg.StatusSuspended},
		{name: "empty status", org: "org-e", wantErr: idporg.ErrUnavailable},
		{name: "unknown org", org: "org-z", wantErr: idporg.ErrNotFound},
		{name: "unknown status", org: "org-x", wantErr: idporg.ErrUnavailable},
		{name: "id needing escaping", org: "a/b", wantErr: idporg.ErrInvalidOrgID},
		{name: "empty id", org: "", wantErr: idporg.ErrInvalidOrgID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := c.OrgStatus(t.Context(), tc.org)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("OrgStatus(%q) = %q, %v; want %v", tc.org, got, err, tc.wantErr)
				}
				// An id the client refuses says nothing about the org, so it
				// must not read as not found, which revokes the tenant.
				if errors.Is(tc.wantErr, idporg.ErrInvalidOrgID) && errors.Is(err, idporg.ErrNotFound) {
					t.Fatalf("OrgStatus(%q) = %v; an invalid id must not be ErrNotFound", tc.org, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("OrgStatus(%q) = %q, %v; want %q", tc.org, got, err, tc.want)
			}
		})
	}
}

func TestOrgStatus_WrongSecretIsUnavailable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(operatorAPI(t, map[string]string{"org-a": "active"}))
	t.Cleanup(srv.Close)
	c, err := idporg.New(srv.URL, []byte("another secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.OrgStatus(t.Context(), "org-a"); !errors.Is(err, idporg.ErrUnavailable) {
		t.Fatalf("OrgStatus with a wrong secret = %v; want ErrUnavailable", err)
	}
}

func TestOrgStatus_BadAnswers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "server error", handler: func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusBadGateway)
		}},
		{name: "redirect not followed", handler: func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		}},
		{name: "not json", handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("<html>"))
		}},
		{name: "missing status", handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":"org-a"}`))
		}},
		{name: "another org", handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":"org-b","status":"active"}`))
		}},
		{name: "oversized", handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":"org-a","status":"active","name":"` + strings.Repeat("x", 70<<10) + `"}`))
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(tc.handler)
			t.Cleanup(srv.Close)
			c, err := idporg.New(srv.URL, []byte(testSecret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.OrgStatus(t.Context(), "org-a"); !errors.Is(err, idporg.ErrUnavailable) {
				t.Fatalf("OrgStatus = %v; want ErrUnavailable", err)
			}
		})
	}
}

func TestOrgStatus_Unreachable(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	c, err := idporg.New("http://"+addr, []byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.OrgStatus(t.Context(), "org-a"); !errors.Is(err, idporg.ErrUnavailable) {
		t.Fatalf("OrgStatus against a closed port = %v; want ErrUnavailable", err)
	}
}

func TestOrgStatus_UnixSocket(t *testing.T) {
	t.Parallel()
	// A short path: a unix socket path is limited to about 100 bytes, and
	// t.TempDir on macOS is longer.
	socket := fmt.Sprintf("/tmp/idporg-%d.sock", os.Getpid())
	_ = os.Remove(socket)
	t.Cleanup(func() { _ = os.Remove(socket) })
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(operatorAPI(t, map[string]string{"org-s": "suspended"}))
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	c, err := idporg.New("unix://"+socket, []byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.OrgStatus(t.Context(), "org-s")
	if err != nil || got != idporg.StatusSuspended {
		t.Fatalf("OrgStatus over a unix socket = %q, %v; want suspended", got, err)
	}
}

func TestNew_RefusesBadConfig(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, base, secret string }{
		{name: "no secret", base: "http://127.0.0.1:1"},
		{name: "no base", secret: "s"},
		{name: "ftp", base: "ftp://host", secret: "s"},
		{name: "user info", base: "http://u:p@host", secret: "s"},
		{name: "path", base: "https://host/api", secret: "s"},
		{name: "query", base: "https://host?x=1", secret: "s"},
		{name: "no host", base: "https://", secret: "s"},
		{name: "plain http to a remote host", base: "http://idp.internal:8090", secret: "s"},
		{name: "plain http to a public IP", base: "http://203.0.113.7", secret: "s"},
		{name: "plain http to a private IP", base: "http://10.0.0.5:8090", secret: "s"},
		{name: "plain http to a name ending in localhost", base: "http://evil.localhost", secret: "s"},
		{name: "unix with host", base: "unix://host/sock", secret: "s"},
		{name: "unix without path", base: "unix://", secret: "s"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := idporg.New(tc.base, []byte(tc.secret)); err == nil {
				t.Fatalf("New(%q) accepted", tc.base)
			}
		})
	}
	for _, ok := range []string{
		"http://127.0.0.1:8090", "http://[::1]:8090", "http://localhost:8090", "http://127.0.0.2",
		"https://idp.internal/", "unix:///run/pratique/op.sock",
	} {
		if _, err := idporg.New(ok, []byte("s")); err != nil {
			t.Errorf("New(%q) = %v", ok, err)
		}
	}
}
