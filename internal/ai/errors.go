package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrKind classifies an AI error so callers (and Lua scripts) can branch
// on the kind of failure without parsing prose error messages.
type ErrKind string

const (
	// ErrNotConfigured indicates the runtime has no AI provider wired
	// (no .rela/ai.yaml or it failed to load).
	ErrNotConfigured ErrKind = "not_configured"

	// ErrAuth indicates the request was rejected for authentication
	// reasons: missing API key, wrong key, expired key, etc.
	ErrAuth ErrKind = "auth"

	// ErrBadRequest indicates the upstream provider rejected the
	// request as invalid: unknown model, unsupported parameter,
	// malformed body, etc.
	ErrBadRequest ErrKind = "bad_request"

	// ErrRateLimited indicates the upstream provider returned 429.
	// RetryAfter is populated when the response includes a usable
	// Retry-After header.
	ErrRateLimited ErrKind = "rate_limited"

	// ErrServerError indicates an upstream 5xx response.
	ErrServerError ErrKind = "server_error"

	// ErrTimeout indicates the request exceeded its deadline.
	ErrTimeout ErrKind = "timeout"

	// ErrNetwork indicates a network-level failure (DNS, connection
	// refused, TLS handshake, etc.) — anything that prevented the
	// HTTP exchange from completing.
	ErrNetwork ErrKind = "network"

	// ErrBadResponse indicates the upstream returned a response we
	// could not understand: wrong Content-Type, malformed JSON,
	// missing required fields, unrecognized content shape.
	ErrBadResponse ErrKind = "bad_response"

	// ErrStreamingUnsupported indicates the upstream returned a
	// streaming response when we requested non-streamed.
	ErrStreamingUnsupported ErrKind = "streaming_unsupported"
)

// Error is the typed error returned by Provider implementations.
//
// Message is human-readable and is safe to display to users — it never
// contains API keys (callers must use redactKey for any text derived
// from secrets-adjacent sources).
type Error struct {
	Kind       ErrKind
	Status     int           // HTTP status code, 0 if not applicable
	Message    string        // human-readable, never contains secrets
	RetryAfter time.Duration // 0 if unknown; populated for ErrRateLimited
	cause      error
}

func (e *Error) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("%s (status %d): %s", e.Kind, e.Status, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *Error) Unwrap() error {
	return e.cause
}

// errBodySnippetBytes caps how much of an unrecognized response body is
// included in error messages for diagnostics.
const errBodySnippetBytes = 200

// wrapNetworkError converts a network/transport-level error from
// http.Client.Do into a typed Error. It distinguishes between deadline
// exceeded (timeout) and other network failures.
func wrapNetworkError(err error, apiKey string) *Error {
	if err == nil {
		return nil
	}
	msg := redactKey(err.Error(), apiKey)
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Kind: ErrTimeout, Message: msg, cause: err}
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Kind: ErrTimeout, Message: msg, cause: err}
	}
	// net.Error timeout (e.g., dial timeout)
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return &Error{Kind: ErrTimeout, Message: msg, cause: err}
	}
	return &Error{Kind: ErrNetwork, Message: msg, cause: err}
}

// classify maps an HTTP response (and its already-read body) to a
// typed Error. It prefers the upstream error envelope's "type" field
// when present, falling back to HTTP status.
//
// apiKey is used solely for redaction of any body snippet included in
// the error message — pass an empty string when no key is in scope.
func classify(status int, headers http.Header, body []byte, apiKey string) *Error {
	envType, envMessage := parseErrorEnvelope(body)

	kind := kindFromEnvelopeType(envType)
	if kind == "" {
		kind = kindFromStatus(status)
	}

	message := envMessage
	if message == "" {
		message = fmt.Sprintf("upstream returned status %d: %s", status, snippet(body))
	}
	message = redactKey(message, apiKey)

	out := &Error{Kind: kind, Status: status, Message: message}
	if kind == ErrRateLimited {
		out.RetryAfter = parseRetryAfter(headers.Get("Retry-After"))
	}
	return out
}

// kindFromEnvelopeType maps OpenAI-style error.type strings to ErrKind.
// Returns "" when the type is unknown or absent.
func kindFromEnvelopeType(t string) ErrKind {
	switch t {
	case "":
		return ""
	case "invalid_request_error":
		return ErrBadRequest
	case "authentication_error", "permission_error", "unauthorized":
		return ErrAuth
	case "rate_limit_error", "rate_limit_exceeded":
		return ErrRateLimited
	case "server_error", "api_error", "internal_server_error":
		return ErrServerError
	}
	// Unknown type — fall through to status-based classification.
	return ""
}

// kindFromStatus maps HTTP status codes to ErrKind.
func kindFromStatus(status int) ErrKind {
	switch {
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return ErrAuth
	case status == http.StatusTooManyRequests:
		return ErrRateLimited
	case status == http.StatusRequestTimeout, status == http.StatusGatewayTimeout:
		return ErrTimeout
	case status >= 400 && status < 500:
		return ErrBadRequest
	case status >= 500 && status < 600:
		return ErrServerError
	}
	return ErrBadResponse
}

// parseErrorEnvelope extracts (type, message) from an OpenAI-style
// error envelope. It does not use encoding/json to avoid pulling errors
// when the body is malformed; it does a tolerant scan instead.
func parseErrorEnvelope(body []byte) (errType, message string) {
	if len(body) == 0 {
		return "", ""
	}
	// Tolerant approach: try a strict JSON decode first; on failure
	// return empty so the classifier falls back to status.
	var env struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", ""
	}
	return env.Error.Type, env.Error.Message
}

// parseRetryAfter parses an HTTP Retry-After header value, supporting
// both delta-seconds and HTTP-date forms. Returns 0 on any parse error.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	// delta-seconds
	if secs, err := strconv.Atoi(value); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	// HTTP-date
	if t, err := http.ParseTime(value); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

// snippet returns up to errBodySnippetBytes of body, truncated on a
// UTF-8 rune boundary so the result is always valid UTF-8.
func snippet(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if len(body) <= errBodySnippetBytes {
		return string(body)
	}
	// Walk back from errBodySnippetBytes to find the last rune boundary.
	end := errBodySnippetBytes
	for end > 0 && (body[end]&0xC0) == 0x80 {
		end--
	}
	return string(body[:end])
}
