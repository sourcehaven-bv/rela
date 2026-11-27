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
	rt := NewWriter(ws.services(t.TempDir()), &buf, WithAIProvider(provider),
		WithCapabilities(Capabilities{AI: true}))
	t.Cleanup(rt.Close)
	return rt
}

// newAIRuntimeNoProvider builds a Runtime with no AI provider wired.
func newAIRuntimeNoProvider(t *testing.T) *Runtime {
	t.Helper()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	rt := NewWriter(ws.services(t.TempDir()), &buf,
		WithCapabilities(Capabilities{AI: true}))
	t.Cleanup(rt.Close)
	return rt
}

func TestLuaAI_NoProviderScripts(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{name: "GlobalAlwaysRegistered", code: `
			assert(type(ai) == "table", "ai global must be a table, got " .. type(ai))
			assert(type(ai.chat) == "function", "ai.chat must be a function")
			assert(type(ai.complete) == "function", "ai.complete must be a function")
		`},
		{name: "NotConfiguredError", code: `
			local result, err = ai.chat({messages = {{role="user", content="hi"}}})
			assert(result == nil, "result should be nil")
			assert(type(err) == "table", "err should be a table, got " .. type(err))
			assert(err.kind == "not_configured", "err.kind = " .. tostring(err.kind))
			assert(string.find(err.message, ".rela/ai.yaml"),
				"message should mention .rela/ai.yaml, got: " .. err.message)
		`},
		{name: "NotConfiguredError_Complete", code: `
			local result, err = ai.complete("hi")
			assert(result == nil)
			assert(err.kind == "not_configured")
		`},
		{name: "EmbedGlobalRegistered", code: `
			assert(type(ai.embed) == "function", "ai.embed must be a function")
		`},
		{name: "EmbedNotConfigured", code: `
			local result, err = ai.embed("hello")
			assert(result == nil, "result should be nil")
			assert(err.kind == "not_configured", "err.kind = " .. tostring(err.kind))
		`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rt := newAIRuntimeNoProvider(t)
			if err := rt.RunString(tc.code); err != nil {
				t.Fatalf("RunString: %v", err)
			}
		})
	}
}

// TestLuaAI_ResponseScripts serves one canned response per case and runs a
// script that asserts how the binding surfaces it.
func TestLuaAI_ResponseScripts(t *testing.T) {
	const serverErrorBody = `{"error":{"type":"server_error","message":"oops"}}`
	tests := []struct {
		name       string
		status     int
		retryAfter string
		body       string
		code       string
	}{
		{name: "ChatSuccess", body: aiSuccessBody, code: `
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
		`},
		{name: "CompleteSuccess", body: aiSuccessBody, code: `
			local text, err = ai.complete("hi")
			assert(err == nil)
			assert(text == "hello from gemma", "text = " .. tostring(text))
		`},
		{name: "HTTPErrorReturnsTypedTable", status: http.StatusInternalServerError, body: serverErrorBody, code: `
			local result, err = ai.chat({messages = {{role="user", content="hi"}}})
			assert(result == nil)
			assert(type(err) == "table")
			assert(err.kind == "server_error", "kind = " .. tostring(err.kind))
			assert(err.status == 500, "status = " .. tostring(err.status))
			assert(string.find(err.message, "oops"), "message = " .. tostring(err.message))
		`},
		{
			name:       "RateLimitedHasRetryAfter",
			status:     http.StatusTooManyRequests,
			retryAfter: "60",
			body:       `{"error":{"type":"rate_limit_error","message":"slow down"}}`,
			code: `
				local result, err = ai.chat({messages = {{role="user", content="hi"}}})
				assert(err.kind == "rate_limited")
				assert(err.status == 429)
				assert(err.retry_after == 60, "retry_after = " .. tostring(err.retry_after))
			`,
		},
		{name: "EmbedSingleString", body: embedSuccessBody, code: `
			local result, err = ai.embed("hello")
			assert(err == nil, "expected nil err, got " .. tostring(err))
			assert(type(result) == "table", "result should be a table")
			-- Always returns array-of-arrays, even for single input
			assert(#result == 1, "#result = " .. #result)
			assert(type(result[1]) == "table", "result[1] should be a table")
			assert(#result[1] == 3, "vector length = " .. #result[1])
			assert(result[1][1] == 0.1, "first value = " .. tostring(result[1][1]))
			assert(result[1][2] == 0.2)
			assert(result[1][3] == 0.3)
		`},
		{name: "EmbedBatch", body: embedBatchBody, code: `
			local result, err = ai.embed({"first", "second"})
			assert(err == nil, "expected nil err")
			assert(#result == 2, "#result = " .. #result)
			assert(result[1][1] == 0.1)
			assert(result[2][1] == 0.3)
		`},
		{name: "EmbedHTTPError", status: http.StatusInternalServerError, body: serverErrorBody, code: `
			local result, err = ai.embed("hello")
			assert(result == nil)
			assert(err.kind == "server_error", "kind = " .. tostring(err.kind))
		`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)

			rt := newAIRuntime(t, server, "TEST_KEY")
			if err := rt.RunString(tc.code); err != nil {
				t.Fatalf("RunString: %v", err)
			}
		})
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

// TestLuaAI_ProgrammingErrorsRaise covers argument mistakes: each raises a
// Lua error before any request reaches the provider.
func TestLuaAI_ProgrammingErrorsRaise(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr string
	}{
		{
			name:    "EmptyMessagesRaisesProgrammingError",
			code:    `ai.chat({messages={}})`,
			wantErr: "messages must not be empty",
		},
		{
			name:    "MessagesNotTableRaises",
			code:    `ai.chat({messages="not a table"})`,
			wantErr: "messages must be a table",
		},
		{name: "MessageMissingRoleRaises", code: `ai.chat({messages={{content="hi"}}})`, wantErr: "role"},
		// gopher-lua's CheckString reports the type mismatch like
		// "string expected, got table". Assert a specific substring so a
		// regression that breaks ai.complete entirely (e.g. global no longer
		// registered) doesn't accidentally satisfy this test.
		{name: "CompleteRejectsNonString", code: `ai.complete({})`, wantErr: "string expected"},
		{name: "EmbedEmptyStringRaises", code: `ai.embed("")`, wantErr: "empty string"},
		{name: "EmbedEmptyTableRaises", code: `ai.embed({})`, wantErr: "empty"},
		{name: "EmbedNonStringInTableRaises", code: `ai.embed({"hello", 42})`, wantErr: "input[2] must be a string"},
		{name: "EmbedWrongArgTypeRaises", code: `ai.embed(42)`, wantErr: "string or table"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("server should not be called")
			}))
			t.Cleanup(server.Close)

			rt := newAIRuntime(t, server, "TEST_KEY")
			err := rt.RunString(tc.code)
			if err == nil {
				t.Fatal("expected Lua error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected %q in error, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestLuaAI_NoAuthHeader(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{
			name: "NoAuthHeaderWhenAPIKeyEnvEmpty",
			body: aiSuccessBody,
			code: `ai.chat({messages={{role="user",content="hi"}}})`,
		},
		{name: "EmbedNoAuthHeader", body: embedSuccessBody, code: `ai.embed("hello")`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var receivedAuth string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)

			rt := newAIRuntime(t, server, "")
			if err := rt.RunString(tc.code); err != nil {
				t.Fatalf("RunString: %v", err)
			}
			if receivedAuth != "" {
				t.Errorf("expected no Authorization header, got %q", receivedAuth)
			}
		})
	}
}

const embedSuccessBody = `{
  "object": "list",
  "data": [{"object":"embedding","index":0,"embedding":[0.1,0.2,0.3]}],
  "model": "nomic-embed-text",
  "usage": {"prompt_tokens": 4, "total_tokens": 4}
}`

const embedBatchBody = `{
  "object": "list",
  "data": [
    {"object":"embedding","index":0,"embedding":[0.1,0.2]},
    {"object":"embedding","index":1,"embedding":[0.3,0.4]}
  ],
  "model": "nomic-embed-text",
  "usage": {"prompt_tokens": 8, "total_tokens": 8}
}`

// TestLuaAI_RequestBodyPassthrough checks that call options reach the
// request body the provider receives.
func TestLuaAI_RequestBodyPassthrough(t *testing.T) {
	tests := []struct {
		name     string
		respBody string
		code     string
		wantBody []string
	}{
		{
			name:     "EmbedModelOverride",
			respBody: embedSuccessBody,
			code:     `ai.embed("hello", {model = "custom-embed-model"})`,
			wantBody: []string{`"model":"custom-embed-model"`},
		},
		{
			name:     "AdditionalParametersPassthrough",
			respBody: aiSuccessBody,
			code: `
				ai.chat({
					messages = {{role="user", content="hi"}},
					model = "override-model",
					temperature = 0.5,
					max_tokens = 100,
				})
			`,
			wantBody: []string{`"model":"override-model"`, `"temperature":0.5`, `"max_tokens":100`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var bodySeen []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				bodySeen, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.respBody))
			}))
			t.Cleanup(server.Close)

			rt := newAIRuntime(t, server, "TEST_KEY")
			if err := rt.RunString(tc.code); err != nil {
				t.Fatalf("RunString: %v", err)
			}
			body := string(bodySeen)
			for _, want := range tc.wantBody {
				if !strings.Contains(body, want) {
					t.Errorf("expected %s in body, got %s", want, body)
				}
			}
		})
	}
}
