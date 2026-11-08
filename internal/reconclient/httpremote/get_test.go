package httpremote

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/reconclient"
)

func TestGet(t *testing.T) {
	var gotPath, gotAuth string
	r, _ := serve(t, func(w http.ResponseWriter, req *http.Request) {
		gotPath, gotAuth = req.URL.EscapedPath(), req.Header.Get("Authorization")
		switch req.URL.Path {
		case "/sub/api/v1/things/T-1":
			_, _ = w.Write([]byte(`{"id":"T-1"}`))
		case "/sub/api/v1/big":
			_, _ = w.Write([]byte(strings.Repeat("x", 100)))
		case "/sub/api/v1/gone":
			http.NotFound(w, req)
		case "/sub/api/v1/secret":
			w.WriteHeader(http.StatusForbidden)
		case "/sub/api/v1/anon":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	ctx := context.Background()
	body, err := r.Get(ctx, "things/T-1", 1024)
	if err != nil || string(body) != `{"id":"T-1"}` {
		t.Fatalf("Get = %q, %v", body, err)
	}
	if gotPath != "/sub/api/v1/things/T-1" || gotAuth != "Bearer s3cret" {
		t.Fatalf("request path %q auth %q", gotPath, gotAuth)
	}
	if _, err := r.Get(ctx, "big", 10); !errors.Is(err, reconclient.ErrProtocol) {
		t.Errorf("oversize: %v", err)
	}
	if _, err := r.Get(ctx, "gone", 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	for _, p := range []string{"secret", "anon"} {
		_, err := r.Get(ctx, p, 10)
		if !errors.Is(err, ErrDenied) || strings.Contains(err.Error(), "s3cret") {
			t.Errorf("%s: %v", p, err)
		}
	}
	if _, err := r.Get(ctx, "boom", 10); err == nil || errors.Is(err, ErrDenied) || errors.Is(err, ErrNotFound) {
		t.Errorf("500: %v", err)
	}
}

func TestValidPath(t *testing.T) {
	for _, p := range []string{"a", "a/b", "a%20b/c", "_schema"} {
		if !validPath(p) {
			t.Errorf("%q should be valid", p)
		}
	}
	for _, p := range []string{"", "/a", "a/", "a//b", "../a", "a/./b", "a?x=1", "a#f", `a\b`,
		"%2e%2e/a", "a/%2E", "a%2Fb", "a%5cb", "%zz"} {
		if validPath(p) {
			t.Errorf("%q should be invalid", p)
		}
	}
}
