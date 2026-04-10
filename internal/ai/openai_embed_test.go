package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// canonicalEmbedBody is a realistic OpenAI embeddings response with two
// 3-dimensional vectors. Indices are intentionally out of order to test
// the sort-by-index logic.
const canonicalEmbedBody = `{
  "object": "list",
  "data": [
    {"object": "embedding", "index": 1, "embedding": [0.4, 0.5, 0.6]},
    {"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]}
  ],
  "model": "nomic-embed-text",
  "usage": {"prompt_tokens": 8, "total_tokens": 8}
}`

// singleEmbedBody is a response with a single embedding vector.
const singleEmbedBody = `{
  "object": "list",
  "data": [
    {"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]}
  ],
  "model": "nomic-embed-text",
  "usage": {"prompt_tokens": 4, "total_tokens": 4}
}`

func TestProvider_Embed_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(singleEmbedBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	resp, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hello"}})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if resp.Model != "nomic-embed-text" {
		t.Errorf("Model = %q, want nomic-embed-text", resp.Model)
	}
	if len(resp.Embeddings) != 1 {
		t.Fatalf("Embeddings count = %d, want 1", len(resp.Embeddings))
	}
	if len(resp.Embeddings[0]) != 3 {
		t.Fatalf("vector length = %d, want 3", len(resp.Embeddings[0]))
	}
	if resp.Embeddings[0][0] != 0.1 {
		t.Errorf("first value = %f, want 0.1", resp.Embeddings[0][0])
	}
	if resp.Usage.PromptTokens != 4 {
		t.Errorf("PromptTokens = %d, want 4", resp.Usage.PromptTokens)
	}
	if resp.Usage.TotalTokens != 4 {
		t.Errorf("TotalTokens = %d, want 4", resp.Usage.TotalTokens)
	}
}

func TestProvider_Embed_BatchWithIndexReordering(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalEmbedBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	resp, err := p.Embed(context.Background(), EmbedRequest{
		Input: []string{"first", "second"},
	})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(resp.Embeddings) != 2 {
		t.Fatalf("Embeddings count = %d, want 2", len(resp.Embeddings))
	}
	// After sort-by-index, index 0 should be first.
	if resp.Embeddings[0][0] != 0.1 {
		t.Errorf("first embedding first value = %f, want 0.1 (index 0)", resp.Embeddings[0][0])
	}
	if resp.Embeddings[1][0] != 0.4 {
		t.Errorf("second embedding first value = %f, want 0.4 (index 1)", resp.Embeddings[1][0])
	}
}

func TestProvider_Embed_UsesEmbeddingModel(t *testing.T) {
	t.Parallel()
	var receivedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var wire embedRequestWire
		_ = json.Unmarshal(body, &wire)
		receivedModel = wire.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(singleEmbedBody))
	}))
	t.Cleanup(server.Close)

	cfg := &Config{
		BaseURL:        server.URL + "/v1",
		Model:          "chat-model",
		EmbeddingModel: "embed-model",
		APIKeyEnv:      "TEST_KEY",
		TimeoutSeconds: 5,
	}
	p, err := NewOpenAICompatProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatProvider: %v", err)
	}
	if _, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if receivedModel != "embed-model" {
		t.Errorf("model = %q, want embed-model", receivedModel)
	}
}

func TestProvider_Embed_FallsBackToModel(t *testing.T) {
	t.Parallel()
	var receivedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var wire embedRequestWire
		_ = json.Unmarshal(body, &wire)
		receivedModel = wire.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(singleEmbedBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	if _, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if receivedModel != "test-model" {
		t.Errorf("model = %q, want test-model (fallback)", receivedModel)
	}
}

func TestProvider_Embed_ModelOverrideInRequest(t *testing.T) {
	t.Parallel()
	var receivedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var wire embedRequestWire
		_ = json.Unmarshal(body, &wire)
		receivedModel = wire.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(singleEmbedBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	if _, err := p.Embed(context.Background(), EmbedRequest{
		Input: []string{"hi"},
		Model: "override-model",
	}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if receivedModel != "override-model" {
		t.Errorf("model = %q, want override-model", receivedModel)
	}
}

func TestProvider_Embed_EmptyInputReturnsError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called for empty input")
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Embed(context.Background(), EmbedRequest{Input: nil})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadRequest {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadRequest)
	}
	if !strings.Contains(aiErr.Message, "empty") {
		t.Errorf("message should mention empty, got %q", aiErr.Message)
	}
}

func TestProvider_Embed_EmptyStringInInputReturnsError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called for empty string input")
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hello", ""}})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadRequest {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadRequest)
	}
	if !strings.Contains(aiErr.Message, "input[1]") {
		t.Errorf("message should mention index, got %q", aiErr.Message)
	}
}

func TestProvider_Embed_BatchLimitExceeded(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called for oversized batch")
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	bigInput := make([]string, MaxEmbedInputs+1)
	for i := range bigInput {
		bigInput[i] = "text"
	}
	_, err := p.Embed(context.Background(), EmbedRequest{Input: bigInput})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadRequest {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadRequest)
	}
	if !strings.Contains(aiErr.Message, "maximum") {
		t.Errorf("message should mention maximum, got %q", aiErr.Message)
	}
}

func TestProvider_Embed_NoEmbeddingsReturned(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"model":"x","usage":{"prompt_tokens":1,"total_tokens":1}}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadResponse)
	}
}

func TestProvider_Embed_CountMismatch(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalEmbedBody)) // returns 2 embeddings
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"only one"}})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadResponse)
	}
	if !strings.Contains(aiErr.Message, "expected 1") {
		t.Errorf("message should mention expected count, got %q", aiErr.Message)
	}
}

func TestProvider_Embed_EmptyEmbeddingVector(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[]}],"model":"x"}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadResponse)
	}
	if !strings.Contains(aiErr.Message, "empty embedding") {
		t.Errorf("message should mention empty embedding, got %q", aiErr.Message)
	}
}

func TestProvider_Embed_MalformedJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not json`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrBadResponse {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrBadResponse)
	}
}

func TestProvider_Embed_HTTPError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	_, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}})
	aiErr := assertAIError(t, err)
	if aiErr.Kind != ErrRateLimited {
		t.Errorf("Kind = %q, want %q", aiErr.Kind, ErrRateLimited)
	}
}

func TestProvider_Embed_PostsToCorrectEndpoint(t *testing.T) {
	t.Parallel()
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(singleEmbedBody))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	if _, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if path != "/v1/embeddings" {
		t.Errorf("path = %q, want /v1/embeddings", path)
	}
}

// TestProvider_Embed_KeyNeverLeaks mirrors the Chat sentinel test,
// exercising every error path for Embed.
func TestProvider_Embed_KeyNeverLeaks(t *testing.T) {
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
				_, _ = fmt.Fprintf(w, `{"error":{"type":"authentication_error","message":"key %s"}}`, sentinelKey)
			},
		},
		{
			"429_rate_limited",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = fmt.Fprintf(w, `{"error":{"type":"rate_limit_error","message":"key %s"}}`, sentinelKey)
			},
		},
		{
			"500_server_error",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, `{"error":{"type":"server_error","message":"key %s"}}`, sentinelKey)
			},
		},
		{
			"html_proxy_error",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, _ = fmt.Fprintf(w, "<html>key: %s</html>", sentinelKey)
			},
		},
		{
			"malformed_json",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{not json key=%s`, sentinelKey)
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
			_, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}})
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

func TestProvider_Embed_KeyNeverLeaks_SuccessPath(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(singleEmbedBody))
	}))
	t.Cleanup(server.Close)

	logger, logBuf := newCapturedLogger()
	p := newTestProvider(t, server, "LEAK_TEST_KEY", WithLogger(logger))
	if _, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if strings.Contains(logBuf.String(), sentinelKey) {
		t.Errorf("API key leaked into log on success path: %q", logBuf.String())
	}
	if !strings.Contains(logBuf.String(), "ai embed ok") {
		t.Errorf("expected embed success log line, got %q", logBuf.String())
	}
}

func TestProvider_Embed_LogsEmbedStart(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(singleEmbedBody))
	}))
	t.Cleanup(server.Close)

	logger, logBuf := newCapturedLogger()
	p := newTestProvider(t, server, "TEST_KEY", WithLogger(logger))
	if _, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	logs := logBuf.String()
	if !strings.Contains(logs, "ai embed start") {
		t.Errorf("expected embed start log, got %q", logs)
	}
	if !strings.Contains(logs, "inputs=1") {
		t.Errorf("expected inputs=1 in log, got %q", logs)
	}
}

func TestProvider_Embed_UsageOmittedGracefully(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2]}],"model":"x"}`))
	}))
	t.Cleanup(server.Close)

	p := newTestProvider(t, server, "TEST_KEY")
	resp, err := p.Embed(context.Background(), EmbedRequest{Input: []string{"hi"}})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if resp.Usage.PromptTokens != 0 {
		t.Errorf("expected zero PromptTokens when usage omitted, got %d", resp.Usage.PromptTokens)
	}
}
