package ai

import "context"

// Provider is the unified interface for AI capabilities.
//
// Today it has only Chat. A future ticket will add Embed for embeddings,
// at which point implementations grow a method and test fakes need a
// stub. The Lua runtime field and option are deliberately named after
// Provider (not Client) so adding capabilities does not require a
// parallel wiring path.
//
// Implementations must be safe to share across goroutines. Concurrency
// at the Lua-binding layer is constrained: gopher-lua *lua.LState is
// not safe for concurrent use, so callers should ensure ai.chat is
// called serially per LState. The same Provider instance can serve any
// number of LStates concurrently as long as the implementation itself
// is thread-safe (the OpenAI-compat provider is, because http.Client
// is).
type Provider interface {
	// Chat sends a chat completion request and returns the response.
	// On failure it returns a *Error (typed via ErrKind) — callers
	// can use errors.As(err, &aiErr) to inspect the kind.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
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
// did not report usage.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
