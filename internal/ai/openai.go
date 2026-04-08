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
	"os"
	"strings"
	"time"
)

// maxResponseBytes caps the response body to prevent OOM from a
// malicious or misconfigured server.
const maxResponseBytes = 10 * 1024 * 1024 // 10 MiB

// openAICompatProvider implements Provider using the OpenAI Chat
// Completions HTTP wire format. It works against any provider that
// honors the OpenAI shape, including OpenAI itself, ollama, apfel, LM
// Studio, Groq, and Anthropic-compat layers.
type openAICompatProvider struct {
	cfg        *Config
	httpClient *http.Client
	// logger is the destination for operational logs (request start,
	// success, failure). Defaults to slog.Default() when no
	// WithLogger option is supplied. Per-provider rather than
	// per-package so tests can use per-test loggers via WithLogger
	// instead of swapping slog.SetDefault, which is process-global
	// and blocks t.Parallel().
	logger *slog.Logger
}

// Option configures an openAICompatProvider at construction time.
type Option func(*openAICompatProvider)

// WithLogger sets the *slog.Logger used for operational logs emitted
// by this provider. Passing nil leaves the default
// (slog.Default()) in place — matches the "zero means default" pattern
// used by http.Client.Timeout and friends.
//
// Typical production callers do NOT pass WithLogger and inherit
// slog.Default(), which the CLI/MCP entry points configure via
// --verbose / --quiet wiring. Tests use WithLogger to route logs into
// a per-test buffer so multiple tests can run under t.Parallel()
// without contaminating each other's captured output.
func WithLogger(l *slog.Logger) Option {
	return func(p *openAICompatProvider) {
		if l != nil {
			p.logger = l
		}
	}
}

// NewOpenAICompatProvider builds a Provider from a Config and optional
// construction-time options (e.g. WithLogger).
//
// It does NOT read the API key. The key (if any) is read at Chat() call
// time so commands that never use AI can still start when the env var
// is unset.
func NewOpenAICompatProvider(cfg *Config, opts ...Option) (Provider, error) {
	if cfg == nil {
		return nil, errors.New("ai: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	p := &openAICompatProvider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout()) * time.Second,
			// Disable redirect-following. None of the OpenAI-compat
			// providers we target use redirects in their normal flow.
			// Following them would let a misconfigured proxy or DNS
			// cache redirect a request to a path the user did not
			// authorize, and Go's stdlib only strips Authorization
			// on cross-host redirects (not same-host), which is a
			// thinner defense than just refusing to follow.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

// chatRequestWire is the JSON shape sent to the upstream. We deliberately
// keep this minimal: only the parameters we explicitly set. omitempty on
// the pointer fields means temperature=0 is sent (pointer is non-nil)
// while absent temperature is omitted entirely.
type chatRequestWire struct {
	Model       string        `json:"model"`
	Messages    []messageWire `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

type messageWire struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponseWire mirrors the OpenAI response. Content is read as
// json.RawMessage so we can tolerate providers that return either a
// string or an array of content parts.
type chatResponseWire struct {
	Model   string       `json:"model"`
	Choices []choiceWire `json:"choices"`
	Usage   *usageWire   `json:"usage"`
}

type choiceWire struct {
	// Message is a pointer so we can tell when the upstream omitted it
	// entirely (vs. sending an empty message object). Some providers
	// have been observed to return choices without a message under
	// edge conditions; that should be ErrBadResponse, not silently
	// produce empty content.
	Message      *messageRawWire `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type messageRawWire struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type usageWire struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Chat sends a chat completion request and returns the response.
func (p *openAICompatProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	apiKey, authErr := p.resolveAPIKey()
	if authErr != nil {
		return nil, authErr
	}

	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}

	httpReq, err := p.buildHTTPRequest(ctx, model, req, apiKey)
	if err != nil {
		return nil, err
	}

	p.logRequestStart(p.cfg.BaseURL, model, len(req.Messages))
	start := time.Now()

	resp, doErr := p.httpClient.Do(httpReq)
	if doErr != nil {
		netErr := wrapNetworkError(doErr, apiKey)
		p.logRequestFailure(string(netErr.Kind), 0, time.Since(start), netErr.Message)
		return nil, netErr
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, readErr := readLimitedBody(resp.Body)
	if readErr != nil {
		var aiErr *Error
		if errors.Is(readErr, errBodyTooLarge) {
			aiErr = &Error{
				Kind:    ErrBadResponse,
				Status:  resp.StatusCode,
				Message: fmt.Sprintf("upstream response exceeded %d bytes", maxResponseBytes),
				cause:   readErr,
			}
		} else {
			aiErr = wrapNetworkError(readErr, apiKey)
		}
		p.logRequestFailure(string(aiErr.Kind), resp.StatusCode, time.Since(start), aiErr.Message)
		return nil, aiErr
	}

	return p.parseResponse(resp, respBody, apiKey, start)
}

// buildHTTPRequest constructs the *http.Request including headers and
// JSON body. Returns a typed *Error on any failure.
func (p *openAICompatProvider) buildHTTPRequest(
	ctx context.Context, model string, req ChatRequest, apiKey string,
) (*http.Request, error) {
	wire := chatRequestWire{
		Model:       model,
		Messages:    toMessageWire(req.Messages),
		Stream:      false,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	body, err := json.Marshal(wire)
	if err != nil {
		// Unreachable in practice; chatRequestWire has no fields that fail to marshal.
		return nil, &Error{Kind: ErrBadRequest, Message: "marshal request: " + redactKey(err.Error(), apiKey), cause: err}
	}

	endpoint := strings.TrimRight(p.cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &Error{Kind: ErrNetwork, Message: redactKey(err.Error(), apiKey), cause: err}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return httpReq, nil
}

// parseResponse handles all post-HTTP response processing: Content-Type
// validation, status classification, error envelope detection, JSON
// decoding, and content shape handling. Returns a typed *Error on any
// failure path.
func (p *openAICompatProvider) parseResponse(
	resp *http.Response, respBody []byte, apiKey string, start time.Time,
) (*ChatResponse, error) {
	// Validate Content-Type before doing anything with the body.
	contentType := resp.Header.Get("Content-Type")
	if isStreamContentType(contentType) {
		streamErr := &Error{
			Kind:    ErrStreamingUnsupported,
			Status:  resp.StatusCode,
			Message: fmt.Sprintf("upstream returned streaming response (Content-Type %q) but stream=false was requested", contentType),
		}
		p.logRequestFailure(string(streamErr.Kind), resp.StatusCode, time.Since(start), streamErr.Message)
		return nil, streamErr
	}
	if !isJSONContentType(contentType) {
		badResp := &Error{
			Kind:    ErrBadResponse,
			Status:  resp.StatusCode,
			Message: fmt.Sprintf("upstream returned non-JSON response (Content-Type %q, status %d): %s", contentType, resp.StatusCode, redactKey(snippet(respBody), apiKey)),
		}
		p.logRequestFailure(string(badResp.Kind), resp.StatusCode, time.Since(start), badResp.Message)
		return nil, badResp
	}

	// Non-2xx → classify and return.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		classified := classify(resp.StatusCode, resp.Header, respBody, apiKey)
		p.logRequestFailure(string(classified.Kind), resp.StatusCode, time.Since(start), classified.Message)
		return nil, classified
	}

	// 2xx with JSON body — error envelope masquerading as success?
	if envType, envMessage := parseErrorEnvelope(respBody); envType != "" || envMessage != "" {
		classified := classify(resp.StatusCode, resp.Header, respBody, apiKey)
		if envType != "" {
			if k := kindFromEnvelopeType(envType); k != "" {
				classified.Kind = k
			}
		}
		if envMessage != "" {
			classified.Message = redactKey(envMessage, apiKey)
		}
		p.logRequestFailure(string(classified.Kind), resp.StatusCode, time.Since(start), classified.Message)
		return nil, classified
	}

	out, parseErr := decodeChatResponse(respBody, apiKey)
	if parseErr != nil {
		parseErr.Status = resp.StatusCode
		p.logRequestFailure(string(parseErr.Kind), resp.StatusCode, time.Since(start), parseErr.Message)
		return nil, parseErr
	}

	p.logRequestSuccess(resp.StatusCode, out.Model, time.Since(start), out.Usage)
	return out, nil
}

// decodeChatResponse parses a successful 2xx JSON body into a
// *ChatResponse. Returns a typed *Error on any decode failure.
func decodeChatResponse(respBody []byte, apiKey string) (*ChatResponse, *Error) {
	var parsed chatResponseWire
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, &Error{
			Kind:    ErrBadResponse,
			Message: fmt.Sprintf("decode response: %v: %s", err, redactKey(snippet(respBody), apiKey)),
			cause:   err,
		}
	}

	if len(parsed.Choices) == 0 {
		return nil, &Error{Kind: ErrBadResponse, Message: "upstream returned no choices"}
	}

	if parsed.Choices[0].Message == nil {
		return nil, &Error{Kind: ErrBadResponse, Message: "upstream returned choice with no message"}
	}

	content, contentErr := decodeContent(parsed.Choices[0].Message.Content)
	if contentErr != nil {
		return nil, &Error{Kind: ErrBadResponse, Message: contentErr.Error(), cause: contentErr}
	}

	out := &ChatResponse{
		Content:      content,
		Model:        parsed.Model,
		FinishReason: parsed.Choices[0].FinishReason,
	}
	if parsed.Usage != nil {
		out.Usage = Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		}
	}
	return out, nil
}

// resolveAPIKey reads the API key from the configured env var. Returns
// ("", nil) when no auth is configured. Returns ("", *Error) when auth
// is configured but the env var is missing/empty.
func (p *openAICompatProvider) resolveAPIKey() (string, *Error) {
	if p.cfg.APIKeyEnv == "" {
		return "", nil
	}
	key := os.Getenv(p.cfg.APIKeyEnv)
	if key == "" {
		return "", &Error{
			Kind:    ErrAuth,
			Message: fmt.Sprintf("environment variable %s is unset or empty", p.cfg.APIKeyEnv),
		}
	}
	return key, nil
}

// errBodyTooLarge is returned by readLimitedBody when the upstream
// response exceeds maxResponseBytes. Distinguished as a sentinel so
// Chat() can classify it as ErrBadResponse (an upstream protocol
// violation) rather than ErrNetwork (a transport failure).
var errBodyTooLarge = errors.New("response body exceeded size limit")

// readLimitedBody reads up to maxResponseBytes from r. If the limit is
// hit it returns errBodyTooLarge so the caller can surface
// ErrBadResponse.
func readLimitedBody(r io.Reader) ([]byte, error) {
	limited := io.LimitReader(r, maxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, errBodyTooLarge
	}
	return body, nil
}

// decodeContent parses the message.content field which can be either a
// string or an array of content parts ({"type":"text","text":"..."}).
func decodeContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	// Try string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	// Try array of parts.
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type == "" || p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("unrecognized content shape: %s", snippet(raw))
}

// toMessageWire converts the public Message slice to the wire form.
func toMessageWire(in []Message) []messageWire {
	out := make([]messageWire, len(in))
	for i, m := range in {
		out[i] = messageWire(m)
	}
	return out
}

// isJSONContentType returns true if the Content-Type header indicates a
// JSON body. Tolerant: matches application/json, application/vnd...+json, etc.
func isJSONContentType(ct string) bool {
	ct = strings.ToLower(ct)
	if ct == "" {
		return false
	}
	// Strip parameters.
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return strings.Contains(ct, "json")
}

// isStreamContentType returns true for SSE responses.
func isStreamContentType(ct string) bool {
	ct = strings.ToLower(ct)
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct == "text/event-stream"
}

// logRequestStart emits a debug log when an AI request begins.
// We log the base URL (which has no path), model, and message count —
// never headers, content, or the API key.
func (p *openAICompatProvider) logRequestStart(baseURL, model string, messageCount int) {
	p.logger.Debug("ai request start",
		"base_url", baseURL,
		"model", model,
		"messages", messageCount)
}

// logRequestSuccess emits an info log on successful response.
func (p *openAICompatProvider) logRequestSuccess(status int, model string, latency time.Duration, usage Usage) {
	p.logger.Info("ai request ok",
		"status", status,
		"model", model,
		"latency_ms", latency.Milliseconds(),
		"prompt_tokens", usage.PromptTokens,
		"completion_tokens", usage.CompletionTokens,
		"total_tokens", usage.TotalTokens)
}

// logRequestFailure emits a warn log on any error path. The message has
// already been redacted.
func (p *openAICompatProvider) logRequestFailure(kind string, status int, latency time.Duration, message string) {
	p.logger.Warn("ai request failed",
		"kind", kind,
		"status", status,
		"latency_ms", latency.Milliseconds(),
		"message", message)
}
