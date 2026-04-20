// Package ai provides an OpenAI-compatible LLM Provider (chat + embed)
// that is exposed to Lua scripts as the `ai` global.
//
// # Configuration
//
// AI is opt-in: requires .rela/ai.yaml (gitignored, per-user). Without it,
// bindings return a `not_configured` error. api_key_env is optional —
// absent means no Authorization header (supports Ollama, apfel, LM Studio).
// The env var is read at call time, not at startup.
//
// # Wiring
//
// Single wiring path: lua.LoadContextOptions(cacheDir, scriptPath) loads
// AI + secrets; script.NewWriterRuntime calls it. Do not add parallel
// ai.LoadProvider call sites — go through NewWriterRuntime.
//
// # Convention deviation
//
// ai.chat / ai.complete / ai.embed are the only rela Lua bindings that
// return (nil, err_table) instead of raising. AI is network-bound;
// scripts should handle failures inline rather than wrap every call in
// pcall. Stable err.kind values live in errors.go; full rationale in
// internal/lua/ai.go.
//
// # Security
//
// Treat Lua scripts as trusted code. A malicious script with AI access
// can exfiltrate project data through the user's own provider. Validation
// deliberately does not get AI wired — a per-entity ai.chat call during
// `analyze` would hit the provider with no quota or kill switch; AI-
// powered validation needs its own design.
//
// Mitigations: key redaction in logs and errors, credential-URL rejection,
// 10 MiB response cap, structured logs that never include headers,
// API keys, or message content.
package ai

import "context"

// Provider is the unified interface for AI capabilities.
//
// Implementations must be safe to share across goroutines. Concurrency
// at the Lua-binding layer is constrained: gopher-lua *lua.LState is
// not safe for concurrent use, so callers should ensure ai.chat /
// ai.embed are called serially per LState. The same Provider instance
// can serve any number of LStates concurrently as long as the
// implementation itself is thread-safe (the OpenAI-compat provider is,
// because http.Client is).
type Provider interface {
	// Chat sends a chat completion request and returns the response.
	// On failure it returns a *Error (typed via ErrKind) — callers
	// can use errors.As(err, &aiErr) to inspect the kind.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// Embed computes vector embeddings for one or more input texts.
	// On failure it returns a *Error (typed via ErrKind).
	Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}

// Message is one entry in a chat conversation.
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// ChatRequest is the input to Provider.Chat.
//
// Temperature and MaxTokens are pointers so callers can distinguish
// "explicitly set to zero" from "unset" — temperature=0 is the most
// common deterministic-sampling setting and must round-trip correctly.
// Nil pointers are omitted from the wire JSON; non-nil pointers are
// always sent.
type ChatRequest struct {
	Messages    []Message
	Model       string   // optional; provider falls back to Config.Model
	Temperature *float64 // optional
	MaxTokens   *int     // optional
}

// ChatResponse is the response from Provider.Chat.
//
// Optional upstream fields (Usage, FinishReason, Model) are zero-valued
// when the upstream provider omits them. Implementations tolerate
// providers that diverge from the canonical OpenAI shape.
type ChatResponse struct {
	Content      string
	Model        string
	FinishReason string
	Usage        Usage
}

// Usage is the token accounting block. Zero values mean the provider
// did not report usage. For embeddings, CompletionTokens is always 0.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// MaxEmbedInputs is the maximum number of texts accepted in a single
// Embed call. The OpenAI API caps at 2048; most other providers are
// lower. This prevents accidental resource exhaustion from Lua scripts
// that build large input tables.
const MaxEmbedInputs = 2048

// EmbedRequest is the input to Provider.Embed.
type EmbedRequest struct {
	// Input is one or more texts to embed. Must be non-empty and
	// contain no empty strings. Limited to MaxEmbedInputs entries.
	Input []string
	// Model is optional; the provider falls back to
	// Config.EmbeddingModel, then Config.Model.
	Model string
}

// EmbedResponse is the response from Provider.Embed.
type EmbedResponse struct {
	// Embeddings contains one vector per input, in the same order as
	// EmbedRequest.Input. Vectors are float64 to avoid silent
	// precision loss at the JSON→Go→Lua boundary (gopher-lua's
	// LNumber is float64).
	Embeddings [][]float64
	Model      string
	Usage      Usage
}
