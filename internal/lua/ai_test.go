package lua

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/ai"
)

const aiSuccessBody = `{
  "id":"chatcmpl-1",
  "object":"chat.completion",
  "model":"gemma3:12b",
  "choices":[{"index":0,"message":{"role":"assistant","content":"hello from gemma"},"finish_reason":"stop"}],
  "usage":{"prompt_tokens":20,"completion_tokens":5,"total_tokens":25}
}`

// newAIRuntime builds a Runtime wired to a fake AI provider pointed at
// the given test server. apiKeyEnv may be empty for no-auth providers.
func newAIRuntime(t *testing.T, server *httptest.Server, apiKeyEnv string) *Runtime {
	t.Helper()
	if apiKeyEnv != "" {
		t.Setenv(apiKeyEnv, "test-key-value")
	}
	cfg := &ai.Config{
		BaseURL:        server.URL + "/v1",
		Model:          "test-model",
		APIKeyEnv:      apiKeyEnv,
		TimeoutSeconds: 5,
	}
	provider, err := ai.NewOpenAICompatProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatProvider: %v", err)
	}

	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	rt := New(ws, ws.Meta(), t.TempDir(), &buf, WithAIProvider(provider))
	t.Cleanup(rt.Close)
	return rt
}

// newAIRuntimeNoProvider builds a Runtime with no AI provider wired.
func newAIRuntimeNoProvider(t *testing.T) *Runtime {
	t.Helper()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	rt := New(ws, ws.Meta(), t.TempDir(), &buf)
	t.Cleanup(rt.Close)
	return rt
}

func TestLuaAI_GlobalAlwaysRegistered(t *testing.T) {
	rt := newAIRuntimeNoProvider(t)
	if err := rt.RunString(`
		assert(type(ai) == "table", "ai global must be a table, got " .. type(ai))
		assert(type(ai.chat) == "function", "ai.chat must be a function")
		assert(type(ai.complete) == "function", "ai.complete must be a function")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaAI_NotConfiguredError(t *testing.T) {
	rt := newAIRuntimeNoProvider(t)
	if err := rt.RunString(`
		local result, err = ai.chat({messages = {{role="user", content="hi"}}})
		assert(result == nil, "result should be nil")
		assert(type(err) == "table", "err should be a table, got " .. type(err))
		assert(err.kind == "not_configured", "err.kind = " .. tostring(err.kind))
		assert(string.find(err.message, ".rela/ai.yaml"), "message should mention .rela/ai.yaml, got: " .. err.message)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaAI_NotConfiguredError_Complete(t *testing.T) {
	rt := newAIRuntimeNoProvider(t)
	if err := rt.RunString(`
		local result, err = ai.complete("hi")
		assert(result == nil)
		assert(err.kind == "not_configured")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaAI_ChatSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(aiSuccessBody))
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")
	if err := rt.RunString(`
		local result, err = ai.chat({messages = {{role="user", content="hi"}}})
		assert(err == nil, "expected nil err, got " .. tostring(err))
		assert(type(result) == "table", "result should be a table")
		assert(result.content == "hello from gemma", "content = " .. tostring(result.content))
		assert(result.model == "gemma3:12b", "model = " .. tostring(result.model))
		assert(result.finish_reason == "stop", "finish_reason = " .. tostring(result.finish_reason))
		assert(type(result.usage) == "table", "usage should be a table")
		assert(result.usage.prompt_tokens == 20, "prompt_tokens = " .. tostring(result.usage.prompt_tokens))
		assert(result.usage.completion_tokens == 5)
		assert(result.usage.total_tokens == 25)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaAI_CompleteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(aiSuccessBody))
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")
	if err := rt.RunString(`
		local text, err = ai.complete("hi")
		assert(err == nil)
		assert(text == "hello from gemma", "text = " .. tostring(text))
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaAI_HTTPErrorReturnsTypedTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"type":"server_error","message":"oops"}}`))
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")
	if err := rt.RunString(`
		local result, err = ai.chat({messages = {{role="user", content="hi"}}})
		assert(result == nil)
		assert(type(err) == "table")
		assert(err.kind == "server_error", "kind = " .. tostring(err.kind))
		assert(err.status == 500, "status = " .. tostring(err.status))
		assert(string.find(err.message, "oops"), "message = " .. tostring(err.message))
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaAI_RateLimitedHasRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")
	if err := rt.RunString(`
		local result, err = ai.chat({messages = {{role="user", content="hi"}}})
		assert(err.kind == "rate_limited")
		assert(err.status == 429)
		assert(err.retry_after == 60, "retry_after = " .. tostring(err.retry_after))
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaAI_TemperatureZeroPropagated(t *testing.T) {
	var bodyWithTemp, bodyWithoutTemp []byte
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if calls == 0 {
			bodyWithTemp = body
		} else {
			bodyWithoutTemp = body
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(aiSuccessBody))
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")

	if err := rt.RunString(`
		ai.chat({messages={{role="user",content="hi"}}, temperature=0})
		ai.chat({messages={{role="user",content="hi"}}})
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}

	if !strings.Contains(string(bodyWithTemp), `"temperature":0`) {
		t.Errorf("first call should have temperature:0, got %s", bodyWithTemp)
	}
	if strings.Contains(string(bodyWithoutTemp), `"temperature"`) {
		t.Errorf("second call should not have temperature key, got %s", bodyWithoutTemp)
	}
}

func TestLuaAI_EmptyMessagesRaisesProgrammingError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called for programming error")
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")
	err := rt.RunString(`ai.chat({messages={}})`)
	if err == nil {
		t.Fatal("expected Lua error")
	}
	if !strings.Contains(err.Error(), "messages must not be empty") {
		t.Errorf("error = %v", err)
	}
}

func TestLuaAI_MessagesNotTableRaises(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called")
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")
	err := rt.RunString(`ai.chat({messages="not a table"})`)
	if err == nil {
		t.Fatal("expected Lua error")
	}
	if !strings.Contains(err.Error(), "messages must be a table") {
		t.Errorf("error = %v", err)
	}
}

func TestLuaAI_MessageMissingRoleRaises(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called")
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")
	err := rt.RunString(`ai.chat({messages={{content="hi"}}})`)
	if err == nil {
		t.Fatal("expected Lua error")
	}
	if !strings.Contains(err.Error(), "role") {
		t.Errorf("error should mention role, got: %v", err)
	}
}

func TestLuaAI_CompleteRejectsNonString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called")
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")
	err := rt.RunString(`ai.complete({})`)
	if err == nil {
		t.Fatal("expected Lua error")
	}
}

func TestLuaAI_NoAuthHeaderWhenAPIKeyEnvEmpty(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(aiSuccessBody))
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "")
	if err := rt.RunString(`ai.chat({messages={{role="user",content="hi"}}})`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if receivedAuth != "" {
		t.Errorf("expected no Authorization header, got %q", receivedAuth)
	}
}

func TestLuaAI_AdditionalParametersPassthrough(t *testing.T) {
	var bodySeen []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodySeen, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(aiSuccessBody))
	}))
	t.Cleanup(server.Close)

	rt := newAIRuntime(t, server, "TEST_KEY")
	if err := rt.RunString(`
		ai.chat({
			messages = {{role="user", content="hi"}},
			model = "override-model",
			temperature = 0.5,
			max_tokens = 100,
		})
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	body := string(bodySeen)
	if !strings.Contains(body, `"model":"override-model"`) {
		t.Errorf("expected override-model in body, got %s", body)
	}
	if !strings.Contains(body, `"temperature":0.5`) {
		t.Errorf("expected temperature:0.5, got %s", body)
	}
	if !strings.Contains(body, `"max_tokens":100`) {
		t.Errorf("expected max_tokens:100, got %s", body)
	}
}
