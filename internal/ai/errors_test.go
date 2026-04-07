package ai

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestError_Error(t *testing.T) {
	withStatus := &Error{Kind: ErrAuth, Status: 401, Message: "bad key"}
	if got := withStatus.Error(); !strings.Contains(got, "auth") || !strings.Contains(got, "401") {
		t.Errorf("Error() = %q", got)
	}

	noStatus := &Error{Kind: ErrNotConfigured, Message: "missing config"}
	if got := noStatus.Error(); !strings.Contains(got, "not_configured") || strings.Contains(got, "0") {
		t.Errorf("Error() = %q", got)
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("inner")
	e := &Error{Kind: ErrNetwork, Message: "wrapped", cause: cause}
	if !errors.Is(e, cause) {
		t.Error("errors.Is should find the cause through Unwrap")
	}
	noCause := &Error{Kind: ErrNetwork}
	if errors.Unwrap(noCause) != nil {
		t.Error("Unwrap should return nil when no cause")
	}
}

func TestWrapNetworkError_Nil(t *testing.T) {
	if got := wrapNetworkError(nil, ""); got != nil {
		t.Errorf("wrapNetworkError(nil) = %v", got)
	}
}

func TestWrapNetworkError_DeadlineExceeded(t *testing.T) {
	got := wrapNetworkError(context.DeadlineExceeded, "")
	if got.Kind != ErrTimeout {
		t.Errorf("Kind = %q", got.Kind)
	}
}

func TestWrapNetworkError_Canceled(t *testing.T) {
	got := wrapNetworkError(context.Canceled, "")
	if got.Kind != ErrTimeout {
		t.Errorf("Kind = %q", got.Kind)
	}
}

func TestWrapNetworkError_Generic(t *testing.T) {
	got := wrapNetworkError(errors.New("dns failed"), "")
	if got.Kind != ErrNetwork {
		t.Errorf("Kind = %q", got.Kind)
	}
}

func TestKindFromEnvelopeType(t *testing.T) {
	cases := map[string]ErrKind{
		"":                      "",
		"unknown_thing":         "",
		"invalid_request_error": ErrBadRequest,
		"authentication_error":  ErrAuth,
		"permission_error":      ErrAuth,
		"unauthorized":          ErrAuth,
		"rate_limit_error":      ErrRateLimited,
		"rate_limit_exceeded":   ErrRateLimited,
		"server_error":          ErrServerError,
		"api_error":             ErrServerError,
		"internal_server_error": ErrServerError,
	}
	for input, want := range cases {
		if got := kindFromEnvelopeType(input); got != want {
			t.Errorf("kindFromEnvelopeType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestKindFromStatus(t *testing.T) {
	cases := map[int]ErrKind{
		401: ErrAuth,
		403: ErrAuth,
		429: ErrRateLimited,
		408: ErrTimeout,
		504: ErrTimeout,
		400: ErrBadRequest,
		404: ErrBadRequest,
		500: ErrServerError,
		502: ErrServerError,
		200: ErrBadResponse, // unexpected; classify never called for 2xx
	}
	for status, want := range cases {
		if got := kindFromStatus(status); got != want {
			t.Errorf("kindFromStatus(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	if d := parseRetryAfter(""); d != 0 {
		t.Errorf("empty = %v", d)
	}
	if d := parseRetryAfter("30"); d != 30*time.Second {
		t.Errorf("30 = %v", d)
	}
	if d := parseRetryAfter("0"); d != 0 {
		t.Errorf("0 = %v", d)
	}
	if d := parseRetryAfter("garbage"); d != 0 {
		t.Errorf("garbage = %v", d)
	}
	// HTTP-date in the past → 0
	if d := parseRetryAfter("Wed, 21 Oct 2015 07:28:00 GMT"); d != 0 {
		t.Errorf("past date = %v", d)
	}
}

func TestParseErrorEnvelope(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantType    string
		wantMessage string
	}{
		{"empty", "", "", ""},
		{"valid", `{"error":{"type":"x","message":"y"}}`, "x", "y"},
		{"no_error", `{"choices":[]}`, "", ""},
		{"malformed", `{not json`, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotMsg := parseErrorEnvelope([]byte(tc.body))
			if gotType != tc.wantType {
				t.Errorf("type = %q, want %q", gotType, tc.wantType)
			}
			if gotMsg != tc.wantMessage {
				t.Errorf("message = %q, want %q", gotMsg, tc.wantMessage)
			}
		})
	}
}

func TestSnippet(t *testing.T) {
	if got := snippet(nil); got != "" {
		t.Errorf("nil = %q", got)
	}
	if got := snippet([]byte("short")); got != "short" {
		t.Errorf("short = %q", got)
	}
	long := strings.Repeat("a", errBodySnippetBytes+50)
	got := snippet([]byte(long))
	if len(got) > errBodySnippetBytes {
		t.Errorf("long snippet too long: %d", len(got))
	}
}

func TestClassify_FallsBackToStatus(t *testing.T) {
	// No error envelope → status fallback
	headers := http.Header{}
	headers.Set("Retry-After", "5")
	got := classify(http.StatusTooManyRequests, headers, []byte(""), "")
	if got.Kind != ErrRateLimited {
		t.Errorf("Kind = %q", got.Kind)
	}
	if got.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter = %v", got.RetryAfter)
	}
}
