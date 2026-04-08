package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// sentinelKey is a unique string we set as the API key in tests so we
// can assert it never appears in any error or log message.
const sentinelKey = "SENTINEL_KEY_ZZZZZ_DO_NOT_LEAK"

// TestMain sets the leak-test env vars for the entire test binary so
// individual tests can call t.Parallel() without conflicting with
// t.Setenv (which panics when called from a parallel test). Setting
// env vars once at startup is safe because all leak tests use the
// same sentinel value and neither env var is consumed elsewhere in
// the test binary. A Setenv failure (rare but possible, e.g. on a
// read-only process environment) makes the whole suite meaningless,
// so panic instead of silently degrading.
func TestMain(m *testing.M) {
	for _, envVar := range []string{"TEST_KEY", "LEAK_TEST_KEY"} {
		if err := os.Setenv(envVar, sentinelKey); err != nil {
			panic(fmt.Sprintf("TestMain: failed to set %s: %v", envVar, err))
		}
	}
	os.Exit(m.Run())
}

// canonicalSuccessBody is the canonical OpenAI response shape, taken
// from a real ollama gemma3:12b round trip.
const canonicalSuccessBody = `{
  "id":"chatcmpl-366",
  "object":"chat.completion",
  "created":1775560657,
  "model":"gemma3:12b",
  "system_fingerprint":"fp_ollama",
  "choices":[
    {"index":0,"message":{"role":"assistant","content":"hello from gemma"},"finish_reason":"stop"}
  ],
  "usage":{"prompt_tokens":20,"completion_tokens":5,"total_tokens":25}
}`

// newTestProvider builds a Provider pointing at the given test server.
// The env var named by apiKeyEnv (TEST_KEY or LEAK_TEST_KEY) is
// pre-set to sentinelKey by TestMain so this helper is safe to call
// from parallel tests. Variadic opts lets tests pass construction-time
// options such as WithLogger for log-capture assertions.
func newTestProvider(t *testing.T, server *httptest.Server, apiKeyEnv string, opts ...Option) Provider {
	t.Helper()
	cfg := &Config{
		BaseURL:        server.URL + "/v1",
		Model:          "test-model",
		APIKeyEnv:      apiKeyEnv,
		TimeoutSeconds: 5,
	}
	p, err := NewOpenAICompatProvider(cfg, opts...)
	if err != nil {
		t.Fatalf("NewOpenAICompatProvider: %v", err)
	}
	return p
}

// newCapturedLogger returns a fresh *slog.Logger that writes to the
// returned bytes.Buffer, and the buffer. Used by log-capture tests to
// avoid the process-global slog.Default() entirely — safe under
// t.Parallel() because each call produces a fresh independent pair.
//
// Level is LevelDebug so DEBUG-level request-start lines are captured
// alongside INFO success and WARN failure lines.
func newCapturedLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, &buf
}

// hiMessages returns a single user message with content "hi". Used by
// tests that exercise transport behavior and don't care about the
// prompt content.
func hiMessages() []Message {
	return []Message{{Role: "user", Content: "hi"}}
}

func TestProvider_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	resp, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "hello from gemma" {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.Model != "gemma3:12b" {
		t.Errorf("Model = %q", resp.Model)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason = %q", resp.FinishReason)
	}
	if resp.Usage.PromptTokens != 20 || resp.Usage.CompletionTokens != 5 || resp.Usage.TotalTokens != 25 {
		t.Errorf("Usage = %+v", resp.Usage)
	}
}

func TestProvider_Chat_AuthHeaderSent(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	want := "Bearer " + sentinelKey
	if receivedAuth != want {
		t.Errorf("Authorization = %q, want %q", receivedAuth, want)
	}
}

func TestProvider_Chat_NoAuthHeaderWhenAPIKeyEnvEmpty(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "")
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if receivedAuth != "" {
		t.Errorf("expected no Authorization header, got %q", receivedAuth)
	}
}

func TestProvider_Chat_AuthErrorWhenEnvVarMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called when env var missing")
	}))
	t.Cleanup(server.Close)

	// Provider construction must succeed even with missing env var.
	cfg := &Config{
		BaseURL:   server.URL + "/v1",
		Model:     "test-model",
		APIKeyEnv: "DEFINITELY_NOT_SET_VAR_ZZZ",
	}
	// Make sure it really isn't set.
	t.Setenv("DEFINITELY_NOT_SET_VAR_ZZZ", "")

	p, err := NewOpenAICompatProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatProvider should succeed even with unset env var: %v", err)
	}

	_, chatErr := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	if chatErr == nil {
		t.Fatal("expected ErrAuth")
	}
	var aiErr *Error
	if !errors.As(chatErr, &aiErr) {
		t.Fatalf("expected *Error, got %T: %v", chatErr, chatErr)
	}
	if aiErr.Kind != ErrAuth {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrAuth)
	}
	if !strings.Contains(aiErr.Message, "DEFINITELY_NOT_SET_VAR_ZZZ") {
		t.Errorf("error should name the env var, got %q", aiErr.Message)
	}
}

func TestProvider_Chat_TemperatureZeroSentDistinctly(t *testing.T) {
	var bodySeen []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodySeen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")

	// temperature=0 should be sent.
	zero := 0.0
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages(), Temperature: &zero}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(string(bodySeen), `"temperature":0`) {
		t.Errorf("expected temperature:0 in body, got %s", bodySeen)
	}

	// temperature absent should NOT be sent.
	bodySeen = nil
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if strings.Contains(string(bodySeen), `"temperature"`) {
		t.Errorf("expected no temperature key in body, got %s", bodySeen)
	}
}

func TestProvider_Chat_StreamFalseAlwaysSent(t *testing.T) {
	var bodySeen []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodySeen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(string(bodySeen), `"stream":false`) {
		t.Errorf("expected stream:false in body, got %s", bodySeen)
	}
}

func TestProvider_Chat_NoUnsupportedParameters(t *testing.T) {
	var bodySeen []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodySeen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	temp := 0.5
	maxTok := 100
	if _, err := p.Chat(context.Background(), ChatRequest{
		Messages: hiMessages(), Temperature: &temp, MaxTokens: &maxTok,
	}); err != nil {
		t.Fatalf("Chat: %v", err)
	}

	body := string(bodySeen)
	for _, banned := range []string{"logprobs", `"n":`, "presence_penalty", "frequency_penalty", `"stop":`} {
		if strings.Contains(body, banned) {
			t.Errorf("body contains unsupported parameter %q: %s", banned, body)
		}
	}
}

func TestProvider_Chat_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrRateLimited {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
	if aiErr.Status != 429 {
		t.Errorf("Status = %d", aiErr.Status)
	}
	if aiErr.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v", aiErr.RetryAfter)
	}
}

func TestProvider_Chat_AuthFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"bad key"}}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrAuth {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
}

func TestProvider_Chat_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"unknown model"}}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadRequest {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
	if !strings.Contains(aiErr.Message, "unknown model") {
		t.Errorf("expected upstream message in error, got %q", aiErr.Message)
	}
}

func TestProvider_Chat_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"type":"server_error","message":"oops"}}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrServerError {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
	if aiErr.Status != 500 {
		t.Errorf("Status = %d", aiErr.Status)
	}
}

func TestProvider_Chat_NetworkError(t *testing.T) {
	// Point at a port that's almost certainly closed.
	cfg := &Config{
		BaseURL:        "http://127.0.0.1:1/v1",
		Model:          "x",
		TimeoutSeconds: 2,
	}
	p, err := NewOpenAICompatProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatProvider: %v", err)
	}
	_, chatErr := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, chatErr)
	if aiErr.Kind != ErrNetwork {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrNetwork)
	}
}

func TestProvider_Chat_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	cfg := &Config{
		BaseURL:        server.URL + "/v1",
		Model:          "test-model",
		TimeoutSeconds: 1,
	}
	p, _ := NewOpenAICompatProvider(cfg)
	// Use a context with a tight deadline so we don't wait the full
	// 1-second client timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.Chat(ctx, ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrTimeout {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
}

func TestProvider_Chat_StreamingResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrStreamingUnsupported {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
}

func TestProvider_Chat_HTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>proxy error</body></html>"))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
	if !strings.Contains(aiErr.Message, "proxy error") {
		t.Errorf("expected body snippet in error, got %q", aiErr.Message)
	}
}

func TestProvider_Chat_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
}

func TestProvider_Chat_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"x","choices":[]}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
}

func TestProvider_Chat_ContentAsArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "model":"x",
  "choices":[{"message":{"role":"assistant","content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]},"finish_reason":"stop"}]
}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	resp, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("Content = %q", resp.Content)
	}
}

func TestProvider_Chat_ContentUnrecognizedShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"x","choices":[{"message":{"role":"assistant","content":42},"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
}

func TestProvider_Chat_OptionalFieldsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	resp, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "hi" {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.Usage != (Usage{}) {
		t.Errorf("expected zero usage, got %+v", resp.Usage)
	}
	if resp.FinishReason != "" {
		t.Errorf("FinishReason = %q", resp.FinishReason)
	}
}

// TestProvider_Chat_ChoiceWithoutMessage covers the edge case where the
// upstream returns a 2xx JSON body containing a choice that omits the
// "message" object entirely. Some providers have been observed to do
// this under load. Without explicit handling we would silently surface
// empty content as success — see review finding F5.
func TestProvider_Chat_ChoiceWithoutMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"x","choices":[{"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadResponse)
	}
	if !strings.Contains(aiErr.Message, "no message") {
		t.Errorf("expected 'no message' in error, got %q", aiErr.Message)
	}
}

// TestProvider_Chat_NoRedirectFollow verifies that the provider does
// not follow HTTP redirects. None of the OpenAI-compat providers we
// target use redirects in their normal flow; following them would let
// a misconfigured proxy or DNS cache redirect a request to a path the
// user did not authorize. Review finding F7.
func TestProvider_Chat_NoRedirectFollow(t *testing.T) {
	var followed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			followed = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(canonicalSuccessBody))
			return
		}
		http.Redirect(w, r, "/elsewhere", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	if err == nil {
		t.Fatal("expected error when upstream returns a redirect")
	}
	if followed {
		t.Error("client followed the redirect; CheckRedirect should have refused")
	}
	// The 307 has no Content-Type, so we surface it as ErrBadResponse.
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadResponse)
	}
}

func TestProvider_Chat_ErrorEnvelopeOn200(t *testing.T) {
	// apfel quirk: streaming endpoint returns 200 + SSE body containing
	// {"error":...}. We catch this via the Content-Type check.
	// This test covers a different quirk: 200 + JSON body that is an
	// error envelope (some compat layers do this).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"weird"}}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadRequest {
		t.Errorf("Kind = %q", aiErr.Kind)
	}
	if !strings.Contains(aiErr.Message, "weird") {
		t.Errorf("expected upstream message, got %q", aiErr.Message)
	}
}

func TestProvider_Chat_BaseURLTrailingSlash(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	cfg := &Config{
		BaseURL:        server.URL + "/v1/",
		Model:          "test-model",
		TimeoutSeconds: 5,
	}
	p, _ := NewOpenAICompatProvider(cfg)
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if path != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", path)
	}
}

// TestProvider_Chat_KeyNeverLeaks is the table-driven sentinel test that
// exercises every error path and asserts the API key sentinel string
// appears in NO returned error message and NO captured log line.
func TestProvider_Chat_KeyNeverLeaks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			"401_auth",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprintf(w, `{"error":{"type":"authentication_error","message":"received key %s"}}`, sentinelKey)
			},
		},
		{
			"429_rate_limited",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = fmt.Fprintf(w, `{"error":{"type":"rate_limit_error","message":"key %s rate limited"}}`, sentinelKey)
			},
		},
		{
			"500_server_error",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, `{"error":{"type":"server_error","message":"failure with key %s"}}`, sentinelKey)
			},
		},
		{
			"400_bad_request",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"error":{"type":"invalid_request_error","message":"key %s is unknown"}}`, sentinelKey)
			},
		},
		{
			"html_proxy_error",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, _ = fmt.Fprintf(w, "<html>received key: %s</html>", sentinelKey)
			},
		},
		{
			"malformed_json_with_key_in_body",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{not json key=%s`, sentinelKey)
			},
		},
		{
			"sse_stream",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprintf(w, "data: {\"error\":{\"message\":\"key %s\"}}\n\n", sentinelKey)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(tc.handler)
			t.Cleanup(server.Close)

			logger, logBuf := newCapturedLogger()
			p := newTestProvider(t, server, "LEAK_TEST_KEY", WithLogger(logger))
			_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), sentinelKey) {
				t.Errorf("API key leaked into error: %q", err.Error())
			}
			if strings.Contains(logBuf.String(), sentinelKey) {
				t.Errorf("API key leaked into log: %q", logBuf.String())
			}
		})
	}
}

// TestProvider_Chat_KeyNeverLeaks_SuccessPath complements the
// error-path leak test by exercising the happy path: logRequestStart
// and logRequestSuccess must not embed the API key in their structured
// log fields, even though they have access to base_url and model.
func TestProvider_Chat_KeyNeverLeaks_SuccessPath(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	logger, logBuf := newCapturedLogger()
	p := newTestProvider(t, server, "LEAK_TEST_KEY", WithLogger(logger))
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if strings.Contains(logBuf.String(), sentinelKey) {
		t.Errorf("API key leaked into log on success path: %q", logBuf.String())
	}
	// Sanity check: we should at least have logged something on the
	// success path, otherwise the leak check is meaningless.
	if !strings.Contains(logBuf.String(), "ai request ok") {
		t.Errorf("expected success log line, got %q", logBuf.String())
	}
}

// TestProvider_Chat_KeyNeverLeaks_NetworkError covers the network-failure
// log/error path. wrapNetworkError pulls the underlying error string into
// the typed *Error.Message field; that string could in principle contain
// the URL, which on a misconfigured base_url could embed credentials.
// Defense in depth: assert the sentinel never appears even when the
// transport itself fails.
func TestProvider_Chat_KeyNeverLeaks_NetworkError(t *testing.T) {
	t.Parallel()
	// LEAK_TEST_KEY is pre-set to sentinelKey in TestMain so this
	// test is safe to run in parallel.
	cfg := &Config{
		BaseURL:        "http://127.0.0.1:1/v1",
		Model:          "x",
		APIKeyEnv:      "LEAK_TEST_KEY",
		TimeoutSeconds: 2,
	}

	logger, logBuf := newCapturedLogger()
	p, err := NewOpenAICompatProvider(cfg, WithLogger(logger))
	if err != nil {
		t.Fatalf("NewOpenAICompatProvider: %v", err)
	}

	_, chatErr := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	if chatErr == nil {
		t.Fatal("expected network error")
	}
	if strings.Contains(chatErr.Error(), sentinelKey) {
		t.Errorf("API key leaked into network error: %q", chatErr.Error())
	}
	if strings.Contains(logBuf.String(), sentinelKey) {
		t.Errorf("API key leaked into network failure log: %q", logBuf.String())
	}
}

func TestProvider_Chat_BodySnippetUTF8Safe(t *testing.T) {
	// Build a body where a multi-byte rune (’ = U+2019, 3 bytes in
	// UTF-8) starts before byte 200 but ends after.
	prefix := strings.Repeat("a", 199)
	bodyText := prefix + "’" + " trailing"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(bodyText))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	// The snippet should be valid UTF-8 — it must not contain a half rune.
	for i, r := range aiErr.Message {
		if r == '\uFFFD' {
			t.Errorf("snippet contains replacement char at byte %d: %q", i, aiErr.Message)
			break
		}
	}
}

func TestProvider_Chat_LogsSuccessAndFailure(t *testing.T) {
	t.Parallel()
	// Success path
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalSuccessBody))
	}))
	t.Cleanup(server.Close)

	logger, logBuf := newCapturedLogger()
	p := newTestProvider(t, server, "TEST_KEY", WithLogger(logger))
	if _, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	logs := logBuf.String()
	if !strings.Contains(logs, `msg="ai request ok"`) {
		t.Errorf("expected success log line, got %q", logs)
	}
	if !strings.Contains(logs, "level=INFO") {
		t.Errorf("expected INFO level in log, got %q", logs)
	}
	if !strings.Contains(logs, "status=200") {
		t.Errorf("expected status=200 in log, got %q", logs)
	}
	if !strings.Contains(logs, "prompt_tokens=20") {
		t.Errorf("expected prompt_tokens=20 in log, got %q", logs)
	}
}

func TestProvider_Chat_LogsFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"type":"server_error","message":"oops"}}`))
	}))
	t.Cleanup(server.Close)

	logger, logBuf := newCapturedLogger()
	p := newTestProvider(t, server, "TEST_KEY", WithLogger(logger))
	_, _ = p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	logs := logBuf.String()
	if !strings.Contains(logs, `msg="ai request failed"`) {
		t.Errorf("expected failure log, got %q", logs)
	}
	if !strings.Contains(logs, "level=WARN") {
		t.Errorf("expected WARN level in log, got %q", logs)
	}
	if !strings.Contains(logs, "status=500") {
		t.Errorf("expected status=500 in log, got %q", logs)
	}
}

func TestProvider_Chat_ResponseTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Write > 10 MiB
		big := make([]byte, maxResponseBytes+10)
		for i := range big {
			big[i] = 'a'
		}
		_, _ = w.Write(big)
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Chat(context.Background(), ChatRequest{Messages: hiMessages()})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadResponse)
	}
	if !strings.Contains(aiErr.Message, "exceeded") {
		t.Errorf("expected size limit message, got %q", aiErr.Message)
	}
}

// --- helpers ---

func assertAIError(t *testing.T, err error) *Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var aiErr *Error
	if !errors.As(err, &aiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	return aiErr
}

// Static check: chatRequestWire should round-trip with omitempty for
// pointer fields. This is a regression guard against accidentally
// changing the field types.
func TestChatRequestWire_OmitEmpty(t *testing.T) {
	body, err := json.Marshal(chatRequestWire{
		Model:    "x",
		Messages: []messageWire{{Role: "user", Content: "hi"}},
		Stream:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if strings.Contains(s, "temperature") {
		t.Errorf("temperature should be omitted when nil, got %s", s)
	}
	if strings.Contains(s, "max_tokens") {
		t.Errorf("max_tokens should be omitted when nil, got %s", s)
	}

	zero := 0.0
	body2, _ := json.Marshal(chatRequestWire{
		Messages:    []messageWire{{Role: "user", Content: "hi"}},
		Temperature: &zero,
	})
	if !strings.Contains(string(body2), `"temperature":0`) {
		t.Errorf("temperature=0 should be sent as 0, got %s", body2)
	}
}
