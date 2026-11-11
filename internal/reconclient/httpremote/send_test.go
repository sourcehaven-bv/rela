package httpremote

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/reconclient"
)

func TestSend(t *testing.T) {
	var gotMethod, gotIfMatch, gotType, gotAuth, gotBody string
	r, _ := serve(t, func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		gotMethod, gotIfMatch, gotBody = req.Method, req.Header.Get("If-Match"), string(b)
		gotType, gotAuth = req.Header.Get("Content-Type"), req.Header.Get("Authorization")
		switch req.URL.Path {
		case "/sub/api/v1/things/T-1":
			w.Header().Set("ETag", `"abc"`)
			_, _ = w.Write([]byte(`{"id":"T-1"}`))
		case "/sub/api/v1/longtag":
			w.Header().Set("ETag", strings.Repeat("x", maxETag+1))
			_, _ = w.Write([]byte(`{}`))
		case "/sub/api/v1/stale":
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = w.Write([]byte(`{"title":"Precondition Failed","detail":"changed\u001b[31m"}`))
		case "/sub/api/v1/titled":
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"title":"Invalid"}`))
		case "/sub/api/v1/big":
			_, _ = w.Write([]byte(strings.Repeat("x", 100)))
		case "/sub/api/v1/anon":
			w.WriteHeader(http.StatusUnauthorized)
		case "/sub/api/v1/busy":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/sub/api/v1/moved":
			http.Redirect(w, req, "/elsewhere", http.StatusFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	ctx := context.Background()

	ans, err := r.Send(ctx, http.MethodPatch, "things/T-1", []byte(`{"a":1}`), `"old"`, 1024)
	if err != nil || ans.Status != http.StatusOK || ans.ETag != `"abc"` || string(ans.Body) != `{"id":"T-1"}` {
		t.Fatalf("Send = %+v, %v", ans, err)
	}
	if gotMethod != http.MethodPatch || gotIfMatch != `"old"` || gotBody != `{"a":1}` ||
		gotType != "application/json" || gotAuth != "Bearer s3cret" {

		t.Fatalf("request %s if-match %q body %q type %q auth %q", gotMethod, gotIfMatch, gotBody, gotType, gotAuth)
	}
	if _, err = r.Send(ctx, http.MethodGet, "things/T-1", nil, "", 1024); err != nil || gotIfMatch != "" ||
		gotType != "" {

		t.Fatalf("GET: %v, if-match %q type %q", err, gotIfMatch, gotType)
	}
	if ans, _ = r.Send(ctx, http.MethodGet, "longtag", nil, "", 1024); ans.ETag != "" {
		t.Errorf("an oversize ETag was kept: %q", ans.ETag)
	}
	ans, err = r.Send(ctx, http.MethodPatch, "stale", nil, `"x"`, 1024)
	if err != nil || ans.Status != http.StatusPreconditionFailed || strings.Contains(string(ans.Body), "\x1b") ||
		!strings.HasPrefix(string(ans.Body), "changed") {

		t.Errorf("412: %+v %q, %v", ans, ans.Body, err)
	}
	if ans, err := r.Send(ctx, http.MethodPost, "titled", nil, "", 1024); err != nil || string(ans.Body) != "Invalid" {
		t.Errorf("422: %+v, %v", ans, err)
	}
	if ans, err := r.Send(ctx, http.MethodPost, "boom", nil, "", 1024); err != nil ||
		ans.Status != http.StatusInternalServerError {

		t.Errorf("500: %+v, %v", ans, err)
	}
	if _, err := r.Send(ctx, http.MethodGet, "big", nil, "", 10); !errors.Is(err, reconclient.ErrProtocol) {
		t.Errorf("oversize: %v", err)
	}
	if _, err := r.Send(ctx, http.MethodPatch, "anon", nil, "", 10); !errors.Is(err, ErrDenied) ||
		strings.Contains(err.Error(), "s3cret") {

		t.Errorf("401: %v", err)
	}
	var refused *reconclient.RefusedError
	if _, err := r.Send(ctx, http.MethodPatch, "busy", nil, "", 10); !errors.As(err, &refused) {
		t.Errorf("429: %v", err)
	}
	if _, err := r.Send(ctx, http.MethodGet, "moved", nil, "", 10); err == nil {
		t.Error("a redirect was followed")
	}
}

func TestSend_RejectsBadInput(t *testing.T) {
	r, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	ctx := context.Background()
	cases := map[string]func() error{
		"method": func() error { _, err := r.Send(ctx, http.MethodDelete, "a", nil, "", 1); return err },
		"path":   func() error { _, err := r.Send(ctx, http.MethodGet, "../a", nil, "", 1); return err },
		"if-match newline": func() error {
			_, err := r.Send(ctx, http.MethodPatch, "a", nil, "\"x\"\r\nX-Evil: 1", 1)
			return err
		},
		"if-match long": func() error {
			_, err := r.Send(ctx, http.MethodPatch, "a", nil, strings.Repeat("x", maxETag+1), 1)
			return err
		},
	}
	for name, f := range cases {
		if f() == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
