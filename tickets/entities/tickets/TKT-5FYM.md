---
id: TKT-5FYM
type: ticket
title: 'Add embeddings support: ai.embed Lua binding and Provider.Embed'
kind: enhancement
priority: high
effort: m
status: done
---

## Goal

Add an `Embed` method to the `ai.Provider` interface, an OpenAI-compatible HTTP
implementation, and a Lua binding `ai.embed()` that lets scripts compute vector
embeddings for text. This is the second slice of `FEAT-ER3Y` (AI integration via
OpenAI-compatible API) and the foundation for everything in Tier 3
(suggest-links, semantic search, duplicate detection, ask).

## Background

`TKT-YBKB` deliberately named the interface `Provider` (not `Client`) so
embeddings could be added later without parallel wiring. The Lua runtime takes a
single `ai.Provider` via `WithAIProvider`, so adding `Embed` to the interface is
a one-method change at the consumer side plus a new test stub. The 4 entry
points (cli/script, cli/flow, script/executor, mcp/tools_lua) need no wiring
changes.

Embeddings are also genuinely valuable on their own: combined with the `rela`
graph, they unlock "find entities semantically similar to this one" — a
differentiating capability for a traceability tool.

## In Scope

### Provider interface

Add `Embed` to `ai.Provider`:

```go
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}

type EmbedRequest struct {
    Input []string  // batch input — embeddings APIs are per-text but accept arrays
    Model string    // optional; defaults to Config.EmbeddingModel (or Config.Model if unset)
}

type EmbedResponse struct {
    Embeddings [][]float32  // one vector per input, in the same order
    Model      string
    Usage      Usage         // reuses the existing Usage struct (prompt_tokens, total_tokens)
}
```

### OpenAI-compat HTTP implementation

POST to `{base_url}/embeddings` with body:

```json
{
  "model": "text-embedding-3-small",
  "input": ["first text", "second text"]
}
```

Response shape:

```json
{
  "object": "list",
  "data": [
    {"object": "embedding", "index": 0, "embedding": [0.123, ...]},
    {"object": "embedding", "index": 1, "embedding": [0.456, ...]}
  ],
  "model": "text-embedding-3-small",
  "usage": {"prompt_tokens": 10, "total_tokens": 10}
}
```

Same hardening as Chat:

- Read API key at call time, not construction
- Same redirect-refusal policy
- Same Content-Type validation
- Same body cap
- Same error taxonomy (`ErrAuth`, `ErrRateLimited`, `ErrServerError`,
`ErrTimeout`, `ErrNetwork`, `ErrBadResponse`)
- Same `redactKey` defense in depth
- Same operational logging via `slog`
- Same sentinel-key leak test extended to cover the new code paths

### Config

Add an optional `embedding_model` field to `.rela/ai.yaml`. If unset, `Embed`
falls back to `model`. Most providers use a *different* default for chat vs
embeddings (e.g. OpenAI uses `gpt-4o-mini` for chat but `text-embedding-3-small`
for embeddings), so a single `model` field isn't enough.

```yaml
base_url: https://api.openai.com/v1
model: gpt-4o-mini
embedding_model: text-embedding-3-small
api_key_env: OPENAI_API_KEY
```

### Lua binding

```lua
-- Single text
local vec, err = ai.embed("hello world")
-- vec is a Lua table of numbers (the embedding vector)

-- Batch (more efficient — one HTTP call for many texts)
local vecs, err = ai.embed({"first", "second", "third"})
-- vecs is a Lua table of tables (one inner table per input)

-- Optional model override
local vec, err = ai.embed("text", {model = "text-embedding-3-large"})
```

Returns `(result, nil)` on success or `(nil, err_table)` on failure (same
convention split as `ai.chat`). The result is either a flat array of numbers
(single input) or an array of arrays (batch input).

### Tests

- `httptest.Server` based unit tests in `internal/ai/openai_test.go` for
every error path (auth, rate-limited, server, network, timeout, malformed JSON,
missing data, content-type validation, redirect refusal, body cap)
- Extend the sentinel-key leak test to cover Embed
- Lua-level integration tests in `internal/lua/ai_test.go` for both
single and batch input, success and error paths
- Live smoke test against ollama with `nomic-embed-text:latest` (already
pulled in the dev environment)

## Out of Scope

- **Caching layer** — strongly recommended as a follow-up (without it,
every analyze run that uses semantic search re-embeds everything). Tracked
separately as `Add content-hash-keyed cache for AI responses`.
- **Vector store / similarity index** — `ai.embed` is the primitive;
building a persistent vector index for the rela graph is its own ticket.
- **Suggest-links / improve / ask CLI commands** — these consume
embeddings but are separate user-facing tickets.
- **Embedding-powered validation rules** — same reasoning as
`AI in validation rules` follow-up: needs cost guardrails first.

## Acceptance Criteria

1. `ai.Provider` interface has both `Chat` and `Embed` methods.
2. `OpenAICompatProvider.Embed` POSTs to `{base_url}/embeddings` with
the OpenAI-compatible request shape and parses the response.
3. `Embed` honors the configured timeout, redirect-refusal, body cap,
and Content-Type validation, identical to `Chat`.
4. `Embed` returns typed errors via the existing `Error` / `ErrKind`
taxonomy.
5. `Config.EmbeddingModel` is optional. When unset, `Embed` uses
`Config.Model`.
6. Lua binding `ai.embed(string)` returns `({float, ...}, nil)` on
success.
7. Lua binding `ai.embed({string, string, ...})` returns
`({{float, ...}, {float, ...}}, nil)` on success — one vector per input in
order.
8. Lua binding `ai.embed(input, {model = "..."})` accepts an optional
options table for model override.
9. Sentinel-key leak test extended: poisoned API key never appears in
error or log output across all Embed code paths.
10. Live smoke test against ollama `nomic-embed-text` returns a vector
of the expected dimensionality (384 for nomic) for both single and batch input.
11. `internal/ai` coverage stays at 90 %+ after the changes.
12. The `lua.WithAIProvider` wiring at the 4 entry points needs zero
changes — the `Embed` method is automatically available via the same
`ai.Provider` instance.

## Notes

- Use the existing `chatRequestWire` / `chatResponseWire` pattern for
the embed wire types (`embedRequestWire`, `embedResponseWire`).
- Ollama's OpenAI-compat `/v1/embeddings` endpoint is well-tested with
`nomic-embed-text` and should work out of the box.
- The OpenAI embedding response shape has `data: [{embedding: [...]}]`
not `embeddings: [...]` directly. The wire type needs to flatten this to
`[][]float32`.
- Float32 (not float64) for vectors — that's what every embedding
provider returns and it halves the memory cost.
- This unblocks `TKT-LK1J` (logger DI) being applied to the new methods
too — do `TKT-LK1J` first if possible so Embed inherits the cleaner test
pattern.
