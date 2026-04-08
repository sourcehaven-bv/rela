---
id: PLAN-FOOU
type: planning-checklist
title: 'Planning: Add OpenAI-compatible AI client config and Lua bindings'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Addendum — real-world findings from apfel and ollama

After completing the design review, we probed two real OpenAI-compatible
endpoints (apfel v0.9.1 on Apple FoundationModels, ollama 0.20.3 with gemma3:12b)
to validate assumptions before implementation. Findings supersede the original
plan where they conflict.

### Confirmed assumptions

- OpenAI error envelope shape: `{"error": {"message": "...", "type": "..."}}`
- HTTP 503 with `type: server_error` for transient unavailability
- HTTP 400 with `type: invalid_request_error` for client errors
- `Content-Type: application/json` on errors (always)
- `/v1/chat/completions` and `/v1/models` paths are standard
- Default port 11434 collides between apfel and ollama (not our problem, but worth noting in docs)

### New findings that change the design

**Finding A — Optional authentication (was a real design hole).**
Most local providers (ollama, apfel, LM Studio) ship with NO authentication by
default. The original plan made `api_key_env` required, which would force users
to invent a fake env var. **Fix:** `api_key_env` is now optional in
`.rela/ai.yaml`. When absent, no `Authorization` header is sent. When present,
the env var must be set to a non-empty value at `Chat()` call time or `ErrAuth`
is returned. New ACs #42-#44 below.

**Finding B — `error.type` is more reliable than HTTP status for classification.**
SSE responses return HTTP 200 even when the body contains `data: {"error":...}`.
Some providers return 503 for both transient unavailability AND model loading.
The plan's `classify()` should prefer `error.type` over HTTP status when present:
`invalid_request_error` → `ErrBadRequest`, `server_error` → `ErrServerError`,
`authentication_error` → `ErrAuth`, etc. Falls back to status code when `type` is
absent or unrecognized.

**Finding C — Provider may publish unsupported parameters in `/v1/models`.**
apfel's model metadata includes an `unsupported_parameters` array. We deliberately
send only `model`, `messages`, `stream`, `temperature?`, `max_tokens?` — but
adding a **negative test** asserting we never accidentally send `logprobs`, `n`,
`presence_penalty`, `frequency_penalty`, `stop` (the common offenders) would
catch regression. Added as AC #45.

**Finding D — Streamed error responses arrive on HTTP 200 + SSE.**
When apfel was probed with `stream: true` while the model was loading, it
returned `HTTP 200 OK` with `Content-Type: text/event-stream` and the SSE body
contained `data: {"error": {...}}` followed by `data: [DONE]`. The plan's
Content-Type check correctly catches this as `ErrStreamingUnsupported` because
we never request streaming, but the test for AC #17 should use exactly this
shape (200 + SSE + error chunk + DONE) since it's what real providers do under
edge conditions.

**Finding E — Error messages may contain non-ASCII (smart quotes).**
apfel returned `isn't` as `isn’t` (U+2019). Our string operations and the
`redactKey` helper must be UTF-8 safe (Go's are by default), and the body-snippet
truncation must not split mid-rune. Added as AC #46.

### Findings deferred to future tickets

- **`/health` endpoint with model availability** — apfel exposes rich diagnostics. A future `ai.health()` Lua function could probe this. Out of scope.
- **`unsupported_parameters` discovery via `/v1/models`** — could let scripts feature-detect, but adds complexity. Defer until a use case appears.
- **JSON pretty-printing tolerance** — stdlib `json.Decoder` already handles it; nothing to change.

### Effort impact

The optional-auth change is one config field + one branch in the request
builder + 3 new ACs. The negative-parameter test is one table-driven test. Total
~1 hour of additional work. Effort stays at `l`.

---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** rela has no way to invoke LLMs from scripts. Users want to write
Lua scripts that call out to OpenAI-compatible providers (OpenAI, Ollama, LM
Studio, Anthropic compat layer, Groq, etc.) for content generation, analysis,
and transformation. This ticket lays the foundation: config + provider + Lua
bindings + typed errors + operational logging. Future tickets layer embeddings,
CLI commands, MCP tools, caching, and UI integration on top.

**Scope (in):**

- `internal/ai` package owning `Config`, `Provider` interface, `OpenAICompatProvider` impl
- `.rela/ai.yaml` config file: `provider`, `base_url`, `model`, `api_key_env?`, `timeout_seconds?`
- **`api_key_env` is OPTIONAL.** When absent, no `Authorization` header is sent (supports local auth-free providers like ollama, apfel, LM Studio). When present, the named env var must be set at `Chat()` call time or `ErrAuth` is returned.
- API key sourced from env var named in config; **read at call time**, not at construction
- Lua bindings: `ai.chat({messages, model?, temperature?, max_tokens?})` and `ai.complete(string)`
- Both bindings return `(result, nil)` on success, `(nil, err_table)` on failure
- **Typed error table** with stable `kind` enum: `not_configured`, `bad_request`, `auth`, `rate_limited`, `server_error`, `timeout`, `network`, `bad_response`, `streaming_unsupported`. Plus `message`, `status`, `retry_after` fields.
- Optional types for `temperature` and `max_tokens` so `0` is distinguishable from "unset"
- Wired into the Lua runtime via a new `lua.WithAIProvider` option
- Operational logging (debug/info/warn) using stdlib `log` to match codebase
- Provider-divergence handling: tolerate missing `usage`, `finish_reason`; handle `content` as string OR array of content parts
- Stream defense: send `"stream": false`; reject `text/event-stream` responses with a typed error
- Content-Type validation before JSON decoding
- API key redaction helper used at every error and log construction site
- Unit tests for `internal/ai` using `httptest.Server`
- Integration test for the Lua binding (sandbox + fake HTTP server)
- Document network-egress + provider-side exfiltration in CLAUDE.md and the `ai-integration` concept

**Scope (out):**

- Embeddings (`ai.embed`) — separate ticket; the `Provider` interface design supports adding a method without breaking the wiring
- Caching layer — separate ticket
- CLI commands (`rela ai ...`) — separate ticket
- MCP tool wrappers — separate ticket
- Web UI integration — separate ticket
- Streaming responses (just rejected as unsupported) — separate ticket
- Tool use / function calling
- Multi-provider abstraction beyond OpenAI-compat
- Pre-built example scripts (suggest-links, validate-with-ai, etc.)
- Audit logging (every prompt/response stored) — separate ticket
- Per-call timeout argument (`ai.chat({..., timeout=60})`) — defer
- Record-and-replay test mode — defer
- HTTP retries with backoff — defer (the typed `rate_limited` error gives scripts the data to do this themselves)
- CI precommit hook scanning for API key prefixes — separate ticket; generic secret-scanning concern, not AI-specific
- Opt-in `--enable-ai` flag — explicitly rejected; AI is always available when configured
- One-time warning on first AI invocation from a script — deferred to follow-up if it needs UX design

**Acceptance Criteria:**

1. **Config loads from `.rela/ai.yaml`** — Test: write a fixture YAML file in a temp dir, call `ai.LoadConfig`, assert all fields parse correctly.
2. **Missing config returns a sentinel `nil` config without error** — Test: call `ai.LoadConfig` on a temp dir with no `ai.yaml`, assert `(nil, nil)`.
3. **Malformed config returns an error** — Test: write invalid YAML, assert `LoadConfig` returns a wrapped error.
4. **Required config fields validated** — Test: missing `base_url` / `model` / `api_key_env` each return a clear error naming the missing field.
5. **`base_url` must include scheme** — Test: `base_url: api.openai.com` returns an error; `https://...` and `http://...` accepted.
6. **`Provider.Chat` succeeds against an OpenAI-compatible endpoint** — Test: `httptest.Server` returning a canned OpenAI response, assert `Response.Content` non-empty and `Response.Usage` populated.
7. **`Provider.Chat` returns typed `auth` error when API key env var unset** — Test: unset env var, call `Chat`, assert error has `Kind == ErrAuth` and message names the env var. **Construction does NOT read the env var.** Construction succeeds even with unset env var.
8. **`Provider.Chat` returns typed `auth` error when API key env var empty string** — Same handling as unset.
9. **`Provider.Chat` returns typed `rate_limited` error on HTTP 429 with `Retry-After`** — Test: fake server returns 429 with `Retry-After: 30`, assert error `Kind == ErrRateLimited`, `Status == 429`, `RetryAfter == 30 * time.Second`.
10. **`Provider.Chat` returns typed `auth` error on HTTP 401/403** — Test asserts `Kind == ErrAuth`.
11. **`Provider.Chat` returns typed `bad_request` error on HTTP 400** — Test asserts `Kind == ErrBadRequest`, message includes upstream error envelope.
12. **`Provider.Chat` returns typed `server_error` on HTTP 5xx** — Test asserts `Kind == ErrServerError`, status preserved.
13. **`Provider.Chat` returns typed `network` error on connection refused** — Test: client points at `127.0.0.1:1`, assert `Kind == ErrNetwork`.
14. **`Provider.Chat` returns typed `timeout` error when context deadline exceeded** — Test: fake server sleeps longer than `timeout_seconds`, assert `Kind == ErrTimeout`.
15. **`Provider.Chat` honors `temperature=0` distinctly from unset** — Test: pass `Temperature: ptr(0.0)`, assert request body contains `"temperature": 0`. Pass nil, assert key absent. Same for `MaxTokens`.
16. **`Provider.Chat` explicitly sends `"stream": false`** — Test: assert request body contains `"stream": false`.
17. **`Provider.Chat` rejects `text/event-stream` responses** — Test: fake server responds with `Content-Type: text/event-stream`, assert `Kind == ErrStreamingUnsupported`.
18. **`Provider.Chat` validates Content-Type before decoding** — Test: fake server returns HTML with `Content-Type: text/html`, assert error `Kind == ErrBadResponse`, message includes status and snippet of body (first 200 bytes).
19. **`Provider.Chat` handles missing optional response fields** — Tests: missing `usage`, missing `finish_reason`, missing `model` — all succeed with zero values, no error.
20. **`Provider.Chat` handles `content` as a JSON array of parts** — Test: response has `"content": [{"type":"text","text":"hi"}]`, assert `Response.Content == "hi"`.
21. **`Provider.Chat` returns `bad_response` for unrecognized content shape** — Test: response has `"content": 42`, assert `Kind == ErrBadResponse` with a clear message.
22. **`Provider.Chat` returns `bad_response` for empty `choices`** — Test asserts the same.
23. **`Provider.Chat` returns `bad_response` for malformed JSON** — Test asserts the same; message includes first 200 bytes of body.
24. **API key never appears in any error message** — **Table-driven test**: poison the env var with a unique sentinel string (`SENTINEL_KEY_ZZZZZ`), exercise *every* error path (auth, rate_limited, network, timeout, bad_response, server_error, streaming_unsupported, bad content-type), assert the sentinel appears in NO returned error's `.Error()` or `.Message`.
25. **`base_url` with trailing slash works** — Test: `https://api.openai.com/v1/`, assert request goes to `.../v1/chat/completions` (no double slash).
26. **`base_url` with `user:pass@host` is rejected** — Test asserts config validation error (defends against URL credential leakage).
27. **Lua: `ai.chat` returns a result table on success** — Test: spin up fake server, build provider, run Lua script `local r, err = ai.chat({messages={{role="user", content="hi"}}}); rela.output(r)`, assert output has `content`, `model`, `finish_reason`, `usage.prompt_tokens`, `usage.completion_tokens`, `usage.total_tokens`.
28. **Lua: `ai.chat` returns `(nil, err_table)` on HTTP error** — Test: fake server returns 500, assert `r == nil`, `type(err) == "table"`, `err.kind == "server_error"`, `err.status == 500`, `err.message ~= ""`.
29. **Lua: `ai.chat` returns typed `not_configured` error when no provider wired** — Test: construct runtime *without* `WithAIProvider`, call `ai.chat`, assert `err.kind == "not_configured"`, message mentions `.rela/ai.yaml`.
30. **Lua: `ai.chat` propagates `temperature=0` distinctly from absent** — Test: Lua `ai.chat({messages=..., temperature=0})` produces a request body with `"temperature": 0`. Lua `ai.chat({messages=...})` (no temperature key) produces a request body without the `temperature` field. Verify by having the fake server echo the received body.
31. **Lua: empty `messages` list raises a Lua error** — Test: `ai.chat({messages={}})` raises (programming error, uses `RaiseError`).
32. **Lua: `messages` not a table raises a Lua error** — Test: `ai.chat({messages="hi"})` raises.
33. **Lua: `messages[i]` missing `role` or `content` raises a Lua error** — Test asserts.
34. **Lua: `ai.complete(string)` returns a string on success** — Test: fake server returns canned content, assert `text == "<canned>"`.
35. **Lua: `ai.complete` returns `(nil, err_table)` on failure** — Test: fake server returns 429, assert `err.kind == "rate_limited"`.
36. **Lua: `ai.complete` rejects non-string argument** — Test: `ai.complete({})` raises a Lua type error (programming error).
37. **The `ai` global is always registered** — Test: with no `WithAIProvider`, assert `type(ai) == "table"` and `type(ai.chat) == "function"`.
38. **Operational logging on success** — Test: capture log output, run `Provider.Chat`, assert one info-level line contains `status=200`, `model=...`, `latency_ms=...`, `tokens=...`. Assert NO log line contains the API key sentinel or any message content.
39. **Operational logging on failure** — Test: capture log output, run failing `Provider.Chat`, assert one warn-level line contains the status code and a body snippet. Assert no API key in log.
40. **Existing Lua tests pass unchanged** — `go test ./internal/lua/...` passes.
41. **Coverage ratchet passes** — `just coverage-check` passes.
42. **Config without `api_key_env` loads successfully** — Test: `.rela/ai.yaml` with only `base_url` and `model`, assert `LoadConfig` succeeds and `Config.APIKeyEnv == ""`.
43. **`Provider.Chat` sends NO `Authorization` header when `api_key_env` is empty** — Test: fake server records request headers, build provider with `Config.APIKeyEnv == ""`, call `Chat`, assert no `Authorization` header was received.
44. **`Provider.Chat` sends `Authorization: Bearer <env value>` when `api_key_env` is set** — Test: fake server records request headers, set env var to a sentinel, call `Chat`, assert `Authorization == "Bearer <sentinel>"`.
45. **`Provider.Chat` never sends unsupported parameters** — Table-driven test: capture the request body sent to a fake server across all `ChatRequest` configurations (with/without temperature, with/without max_tokens), assert the JSON body contains NO keys named `logprobs`, `n`, `presence_penalty`, `frequency_penalty`, `stop`. Locks in our minimal-payload promise.
46. **Body snippet truncation is UTF-8 safe** — Test: fake server returns an error body containing multi-byte runes (e.g., `isn’t` with U+2019) at byte position 199. Assert the resulting `Error.Message` snippet does not contain a half-rune and decodes as valid UTF-8.
47. **`error.type` from response body is preferred over HTTP status for classification** — Test: fake server returns HTTP 200 with `{"error": {"type": "invalid_request_error", "message": "..."}}` (a real apfel quirk), assert `Kind == ErrBadRequest`. Test: fake server returns HTTP 503 with `{"error": {"type": "server_error", ...}}`, assert `Kind == ErrServerError`. Test: fake server returns HTTP 401 with no error envelope, assert `Kind == ErrAuth` (status fallback).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **OpenAI Go SDK (`github.com/sashabaranov/go-openai`)**: mature, but heavy dependency tree, opinionated request shape, won't bend gracefully for compat-layer divergences. **Rejected** — the OpenAI Chat Completions API is one POST endpoint; hand-rolling with `net/http` is cleaner, lighter, and lets us tolerate provider quirks.
- **`github.com/anthropics/anthropic-sdk-go`**: locks us into one provider; defeats the OpenAI-compat strategy. **Rejected.**
- **`langchaingo`**: massive surface area for "POST a JSON payload, parse the response." **Rejected.**

**Codebase patterns reused:**

- **Lua submodule registration**: `internal/lua/markdown.go:41` (`registerMarkdownModule`). The new `internal/lua/ai.go` follows the same shape: a `registerAIModule()` method on `*Runtime` that builds a table and assigns it.
- **Lua runtime options**: `internal/lua/runtime.go:67` (`Option func(*Runtime)`) plus `WithTimeout`, `WithOutputDir`. New `WithAIProvider(ai.Provider) Option` follows the same shape.
- **Lua test helpers**: `internal/lua/runtime_test.go:206` (`TestRunFile_BasicOutput`). Pattern: build mock workspace, construct runtime, run script via `RunString`, parse `rela.output` JSON, assert.
- **`.rela/` lifecycle**: `.rela/` is gitignored as a whole (`.gitignore` line 14). No new gitignore entry needed.
- **YAML config loading**: `internal/dataentry/app.go:234` (`loadUserDefaults`) shows the silent-on-missing pattern. We're slightly stricter — `LoadConfig` returns `(nil, nil)` on missing file but errors on malformed YAML or missing required fields.
- **Logging**: codebase uses stdlib `log` (`internal/workspace/workspace.go:11`), not `slog`. Match that.

**Convention deviation — error handling (deliberate):**

All existing rela Lua bindings raise errors via `ls.RaiseError(...)`. The new
`ai.*` bindings will instead return `(nil, err_table)` as a deliberate
exception. **Rationale:** AI calls are network-bound; failure is expected as
normal operation (transient network errors, rate limits, upstream 500s).
Wrapping every call in `pcall(function() ... end)` is verbose and obscures
intent. The two-return pattern lets scripts write idiomatic Lua: `local result,
err = ai.chat({...}); if not result then ... end`.

**Programming errors still raise** (wrong arg type, empty messages list,
malformed messages entry). The convention is: "expected runtime failures return
`nil, err_table`; programming errors raise."

This deviation will be documented in `internal/lua/ai.go` with a top-of-file
comment so future maintainers don't "fix" it.

**Relevant concepts:**

- `ai-integration` (new) — this ticket is the first slice
- `lua-scripting` — this ticket extends the Lua surface
- `FEAT-i5ji` — Lua scripting feature, history of incremental Lua API additions

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Package layout

```text
internal/ai/
  config.go         Config struct, LoadConfig
  config_test.go
  provider.go       Provider interface, types, errors
  errors.go         ErrKind enum, Error struct, error classifier
  errors_test.go
  openai.go         OpenAICompatProvider implementation
  openai_test.go    httptest.Server-based unit tests
  redact.go         redactKey helper (used by errors and logging)
```

### Provider interface (forward-compatible for embeddings)

```go
// internal/ai/provider.go
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    // Embed(...) will be added in a future ticket without breaking the
    // wiring — implementations grow a method, the interface widens, and
    // every existing test that uses a fake Provider will need to add a
    // stub. Since fakes are constrained to internal/lua/ai_test.go and
    // internal/ai/*_test.go, this is acceptable. The alternative
    // (composition via Chatter + Embedder small interfaces) was
    // considered and rejected: it forces every consumer to know about
    // both interfaces and parameterize on the right one.
}

type ChatRequest struct {
    Messages    []Message
    Model       string   // optional; defaults to Config.Model
    Temperature *float64 // nil = unset; explicit 0 sent as 0
    MaxTokens   *int     // nil = unset
}

type Message struct {
    Role    string // "system" | "user" | "assistant"
    Content string
}

type ChatResponse struct {
    Content      string
    Model        string
    FinishReason string
    Usage        Usage
}

type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

### Typed errors

```go
// internal/ai/errors.go
type ErrKind string

const (
    ErrNotConfigured       ErrKind = "not_configured"
    ErrAuth                ErrKind = "auth"
    ErrBadRequest          ErrKind = "bad_request"
    ErrRateLimited         ErrKind = "rate_limited"
    ErrServerError         ErrKind = "server_error"
    ErrTimeout             ErrKind = "timeout"
    ErrNetwork             ErrKind = "network"
    ErrBadResponse         ErrKind = "bad_response"
    ErrStreamingUnsupported ErrKind = "streaming_unsupported"
)

type Error struct {
    Kind       ErrKind
    Status     int           // HTTP status, 0 if not applicable
    Message    string        // human-readable; never contains secrets
    RetryAfter time.Duration // 0 if unknown; populated for ErrRateLimited
    cause      error         // wrapped underlying error, optional
}

func (e *Error) Error() string  // human-readable, never includes API key
func (e *Error) Unwrap() error  // returns cause
```

A package-level `classify(resp *http.Response, body []byte) *Error` function
maps HTTP responses to typed errors. Used by `OpenAICompatProvider.Chat`.

### Config

```go
// internal/ai/config.go
type Config struct {
    Provider       string `yaml:"provider"`        // optional, informational; if set, must be "openai-compatible"
    BaseURL        string `yaml:"base_url"`        // required, must include scheme, no userinfo
    Model          string `yaml:"model"`           // required
    APIKeyEnv      string `yaml:"api_key_env"`     // OPTIONAL — name of env var holding API key; absent = no auth
    TimeoutSeconds int    `yaml:"timeout_seconds"` // optional, default 30
}

// LoadConfig reads .rela/ai.yaml from the given .rela directory.
// Returns (nil, nil) if the file does not exist (treated as "AI not configured").
// Returns (nil, err) on parse failure or invalid required fields.
func LoadConfig(relaDir string) (*Config, error)

// Validate checks required fields and rejects URL credentials, missing scheme, etc.
func (c *Config) Validate() error
```

### Provider construction (no env var read)

```go
// NewOpenAICompatProvider constructs a provider from a Config.
// It does NOT read the API key. The key is read at Chat() call time
// from os.Getenv(cfg.APIKeyEnv). This means commands that don't use AI
// will not fail to start when the env var is unset.
func NewOpenAICompatProvider(cfg *Config) (Provider, error)
```

### Wire format

POST `{base_url}/chat/completions`:

```json
{
  "model": "gpt-4o-mini",
  "messages": [{"role": "user", "content": "hi"}],
  "stream": false,
  "temperature": 0.2,
  "max_tokens": 500
}
```

`stream: false` is **always** sent. `temperature` and `max_tokens` are omitted
from the JSON (using `omitempty` on pointer fields) when nil.

Headers: `Content-Type: application/json`. `Authorization: Bearer <key>` is sent ONLY if `Config.APIKeyEnv` is non-empty AND the named env var is set to a non-empty value at call time. If `Config.APIKeyEnv` is set but the env var is unset/empty, return `ErrAuth` before making the HTTP call. If `Config.APIKeyEnv` is empty, no Authorization header is sent at all (supports auth-free local providers).

Response handling:

1. Read up to 10 MiB via `io.LimitReader`. If reader hits the limit, return `ErrBadResponse`.
2. Validate `Content-Type` includes `json`. If `text/event-stream`, return `ErrStreamingUnsupported`. Otherwise return `ErrBadResponse` with status + first 200 bytes of body.
3. If status is non-2xx, classify via `classify()` → typed error.
4. Decode JSON into a tolerant response struct that uses `json.RawMessage` for `choices[0].message.content`.
5. Parse `content`: try string first, then array of `{type, text}` parts (concatenate `text` fields), else `ErrBadResponse`.
6. `usage`, `finish_reason`, `model` are all optional — missing → zero values.
7. If `choices` is empty, return `ErrBadResponse`.

### API key redaction

`redactKey(s string, key string) string` replaces all occurrences of `key` (and
the substring `Bearer <key>`) with `<REDACTED>`. Used at every error
construction site that *could* see the key, and in logging. Belt and braces —
the key shouldn't be passed to error construction in the first place, but the
helper is the safety net.

### Operational logging

Use stdlib `log` (matching `internal/workspace/workspace.go`). Logger is the
package-level default; no DI in this slice.

| Event | Level | Fields | NOT logged |
|---|---|---|---|
| Request start | DEBUG | base_url (no path), model, message_count | headers, content, key |
| Success | INFO | status, model, latency_ms, prompt_tokens, completion_tokens | content, key |
| HTTP error | WARN | status, kind, latency_ms, body snippet (200 bytes, key-redacted) | content, key |
| Network error | WARN | kind, latency_ms, error message (key-redacted) | content, key |

Tests assert: API key sentinel never appears in captured log output.

### Lua binding

```text
internal/lua/
  ai.go       registerAIModule, luaAIChat, luaAIComplete, errorToLuaTable
  ai_test.go  Integration tests with httptest.Server
```

Top-of-file comment in `internal/lua/ai.go`:

```go
// Package lua bindings for ai.* — DELIBERATE CONVENTION DEVIATION.
//
// All other rela Lua bindings raise errors via ls.RaiseError. The ai.*
// bindings instead return (nil, err_table) for *expected runtime failures*
// (network errors, HTTP errors, missing config, rate limits) because AI
// calls are inherently network-bound and scripts should be able to handle
// failure inline rather than wrap every call in pcall.
//
// PROGRAMMING ERRORS still raise via RaiseError (wrong argument type,
// empty messages list, malformed messages entry). The taxonomy is:
//   - expected runtime failure  -> (nil, err_table)
//   - programming error         -> RaiseError
//
// CONCURRENCY: ai.chat assumes single-threaded LState use. gopher-lua
// *lua.LState is NOT safe for concurrent goroutine use. ai.Provider
// implementations must be safe to share across runtimes (the default
// OpenAICompatProvider is, because http.Client is safe).
//
// Do not "fix" this convention without reading the planning document.
```

```go
type Runtime struct {
    // ... existing fields ...
    aiProvider ai.Provider  // nil means AI is not configured
}

func WithAIProvider(p ai.Provider) Option {
    return func(r *Runtime) { r.aiProvider = p }
}

func (r *Runtime) registerAIModule() {
    aiTable := r.L.NewTable()
    r.L.SetField(aiTable, "chat", r.L.NewFunction(r.luaAIChat))
    r.L.SetField(aiTable, "complete", r.L.NewFunction(r.luaAIComplete))
    r.L.SetGlobal("ai", aiTable)
}
```

`luaAIChat`:

1. If `r.aiProvider == nil`, push `(nil, errorTable(ErrNotConfigured, "AI is not configured: create .rela/ai.yaml"))`, return 2.
2. Type-check the argument is a table. If not, `RaiseError`.
3. Type-check `messages` is a non-empty table-of-tables. Each entry must have `role` (string) and `content` (string). On any failure, `RaiseError` with a clear message.
4. Read optional `model` (string), `temperature` (number), `max_tokens` (integer). **Use `LGetField + LNil check`** to distinguish "key absent" from "key set to 0". Build `*float64` / `*int` accordingly.
5. Call `r.aiProvider.Chat(r.ctx, req)`.
6. On error: type-assert to `*ai.Error`, build a Lua table `{kind=..., status=..., message=..., retry_after=...}`, push `(nil, err_table)`, return 2. If the error is somehow not `*ai.Error` (programming bug), `RaiseError` (we want a loud failure during dev).
7. On success: build a Lua result table with flat fields (`content`, `model`, `finish_reason`, `usage` sub-table), push `(result, nil)`, return 2.

`luaAIComplete`:

1. Type-check argument is a string. If not, `RaiseError`.
2. Build a `ChatRequest` with one user message.
3. Call `r.aiProvider.Chat(...)`.
4. On error: same error-table marshaling as `luaAIChat`. Note: `complete` returns `(nil, err_table)`, NOT `(nil, "string err")` — consistent with `chat`.
5. On success: push `(content_string, nil)`, return 2.

### Wiring into entry points (concrete)

`grep "lua.New("` finds 5 sites. **AI is wired into 4 of the 5**, deliberately
excluding `internal/validation/lua.go`:

| File | AI wired? | Notes |
|---|---|---|
| `internal/cli/script.go` | YES | `rela script` command — has project context, load AI config |
| `internal/cli/flow.go` | YES | `rela flow` command — has project context, load AI config |
| `internal/script/executor.go` | YES | shared script executor for automation Lua actions |
| `internal/mcp/tools_lua.go` | YES | MCP `lua_run` / `lua_eval` tools — has project context, load AI config |
| `internal/validation/lua.go` | **NO** | Lua validation rules — see "Why not validation" below |

**Why not validation:** an AI-powered validation rule would call out to a
provider on **every entity on every analyze run** with no quota, no kill
switch, and no cost warning. The 5-second validation timeout would also
silently clip slow AI calls without informing the rule author. Wiring
AI into validations responsibly needs its own design (per-rule opt-in,
cost guardrails, longer per-rule budget) — tracked as a follow-up
ticket.

Each AI-wired site loads `.rela/ai.yaml` via `ai.LoadConfig(ctx.CacheDir)`. If
`ErrConfigNotFound`, no provider wired (Lua functions return `not_configured`).
If a `*Config` is returned, build provider via
`ai.NewOpenAICompatProvider(cfg)` and pass `lua.WithAIProvider(p)`. If config
load fails for any other reason, log a warning and continue without a provider
(don't fail the whole command for a malformed AI config).

**Files to modify/create:**

| File | Action | Purpose |
|---|---|---|
| `internal/ai/config.go` | new | Config struct + LoadConfig + Validate |
| `internal/ai/config_test.go` | new | Loader tests (missing/present/malformed/required-field/url-creds) |
| `internal/ai/provider.go` | new | Provider interface + ChatRequest/Response/Message/Usage types |
| `internal/ai/errors.go` | new | ErrKind constants, Error struct, classify() |
| `internal/ai/errors_test.go` | new | Classifier tests |
| `internal/ai/openai.go` | new | OpenAICompatProvider implementation |
| `internal/ai/openai_test.go` | new | httptest.Server tests covering all AC scenarios |
| `internal/ai/redact.go` | new | redactKey helper |
| `internal/ai/redact_test.go` | new | Redaction tests including edge cases (empty key, key in URL) |
| `internal/lua/ai.go` | new | registerAIModule + luaAIChat + luaAIComplete + errorToLuaTable + top-of-file convention doc |
| `internal/lua/ai_test.go` | new | Integration tests through Lua sandbox |
| `internal/lua/runtime.go` | edit | Add `aiProvider` field, `WithAIProvider` option, call `registerAIModule()` from `registerBindings` |
| `internal/cli/script.go` | edit | Load AI config + wire into runtime |
| `internal/cli/flow.go` | edit | Same |
| `internal/script/executor.go` | edit | Same |
| `internal/validation/lua.go` | (no edit) | AI deliberately not wired — see Wiring section |
| `internal/mcp/tools_lua.go` | edit | Same |
| `CLAUDE.md` | edit | Document AI integration: config, Lua API, network egress + provider exfiltration risk |
| `tickets/entities/concepts/ai-integration.md` | edit | Add explicit network-egress + exfiltration section |

**Alternatives considered:**

| Alternative | Why rejected |
|---|---|
| Use `go-openai` SDK | Heavy dependency, opinionated, fragile across compat layers |
| `RaiseError` for AI errors | Forces `pcall` for every call; AI failures are expected, not exceptional |
| String error returns to Lua | Locks API into prose-parsing forever; typed errors are a one-way improvement |
| `ai.Client` instead of `ai.Provider` (single-purpose) | Forces parallel `aiClient` + `aiEmbedder` Lua plumbing later |
| Keep `Temperature float64` and document `0` ambiguity | Ships a known correctness footgun; pointers are 30 minutes of work |
| Read API key at construction (fail fast at startup) | Breaks unrelated commands when env var unset; conflicts with AC #7 |
| Hang AI config off `project.Context` | Pollutes Context with AI concerns; harder to test |
| Conditional `ai.*` registration (only if provider configured) | Scripts can't feature-detect cleanly; better to always register |
| `rela.ai.*` namespace | Less ergonomic; AI calls are conceptually independent of rela |
| Streaming responses | Out of scope; requires coroutine API design |
| Caching | Out of scope; needs its own design |
| HTTP retries with backoff | Out of scope; the typed `rate_limited` error gives scripts what they need to do this themselves |
| Per-call timeout argument | Out of scope; runtime context handles common case |
| Linter rule enforcing `(nil, err_table)` for AI bindings | Brittle; top-of-file comment + code review is sufficient |
| Composition interfaces (`Chatter` + `Embedder`) instead of `Provider` aggregate | More flexible but forces every consumer to parameterize on the right small interface; aggregate is simpler for this codebase |
| `ai.complete(prompt, opts?)` with system prompt support | Defeats the purpose of the helper; if you need a system prompt, use `ai.chat` |

**Dependencies:**

- `gopkg.in/yaml.v3` (already in go.mod)
- `net/http`, `encoding/json`, `context`, `log`, `strings`, `time`, `io`, `errors`, `net/url`, `os` (stdlib)
- No new third-party dependencies

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Source | Validation | On Invalid |
|---|---|---|
| `.rela/ai.yaml` (file content) | YAML parse + required field check (`base_url`, `model`) + URL scheme + reject userinfo. `api_key_env` is OPTIONAL. | `LoadConfig` returns wrapped error |
| API key (env var) | Non-empty check at `Chat()` call time IFF `api_key_env` is set in config | Return `ErrAuth` referencing the env var name (NOT the key) |
| Lua `messages` arg | Type-check non-empty table-of-tables; each entry has `role` (string) and `content` (string) | `RaiseError` (programming error) |
| Lua `model`, `temperature`, `max_tokens` | Optional; type-check if present | `RaiseError` |
| Lua `complete(string)` arg | Type-check string | `RaiseError` |
| Upstream HTTP response | Status code, Content-Type, JSON shape, response size | Return typed `Error` (`ErrBadResponse`, `ErrStreamingUnsupported`, etc.) |

**Security-Sensitive Operations:**

### 1. API key handling

- Key is read from env var, never from a file
- Key is **read at call time**, never stored long-term in the provider
- Key is never logged, never echoed in errors, never written to disk
- Error messages reference the env var *name*, not the value
- **Optional**: when `api_key_env` is absent from config, no auth is used at all (local providers like ollama, apfel, LM Studio)
- **`redactKey` helper** is used at every error construction and log site as a belt-and-braces measure (no-op when key is empty)
- **Table-driven leak test** (AC #24): poison the env var with a sentinel string and assert it appears in NO error or log line across every code path

**Key leak surface considered:**

- Error wrapping (`fmt.Errorf("%w", err)`) — `redactKey` wraps any error message we construct from upstream data
- Stack traces / panics — we don't `recover` and log request objects; if a panic happens, gopher-lua's recover wraps it but doesn't have the key in scope
- URL credentials (`https://user:key@host`) — config validation **rejects** any `base_url` with userinfo
- Test fixtures — addressed by AC #24 (any committed fixture using the sentinel will fail tests if it leaks)
- CI precommit hook for `sk-` prefix scanning — **deferred** to a separate ticket, generic concern

### 2. Network egress (NEW capability for the Lua sandbox)

- The Lua sandbox previously had no outbound network access (io/os/debug blocked)
- `ai.chat` is the first sanctioned outbound network call from the sandbox
- This is a **genuine new capability**, not a no-op against the existing threat model

### 3. Script-level data exfiltration (NEW threat class)

This was undersold in the first draft of the plan. Reframing:

**Before AI integration:** A malicious Lua script can read all entities
(`rela.list_entities`) and write files (`rela.write_file` within the project
root). The damage is contained to your local filesystem.

**After AI integration:** A malicious script can additionally call
`ai.chat({messages={{role="user", content=<entire project dump>}}})`, sending
project content to **the user's own legitimate provider**. The data lands in the
provider's logs, possibly in training data, possibly readable by junior staff,
possibly billed to the user. The script needs no malicious config — it uses the
user's own working setup. This is a meaningful escalation: contained-to-local
damage becomes silent third-party data egress.

**Mitigations in this slice:**

- Documentation in CLAUDE.md and the `ai-integration` concept explicitly calling out this threat
- Operational logging makes it visible *if anyone looks* — scripts that suddenly start hammering the AI endpoint will show up
- Users are advised: treat Lua scripts as trusted code, the same way you'd treat any code that runs in your project
- AI is opt-in by virtue of requiring `.rela/ai.yaml` to exist and an API key to be set

**Mitigations deferred:**

- One-time warning on first AI invocation per-script or per-session (needs UX design)
- Allowlist of script files permitted to use `ai.*`
- Per-script token / call budgets
- Network egress audit log

### 4. URL validation

- `base_url` is used as-is for HTTP POST after stripping trailing slash
- Required: scheme is `http://` or `https://`
- Required: no userinfo (no `user:pass@host`)
- **Not** restricted to specific hosts — users want to point at internal endpoints, Ollama on localhost, etc.

### 5. Prompt injection

- Entity content passed into AI prompts is user-controlled
- **Out of scope** for this ticket: the `internal/ai` layer is just a transport
- Future tickets that build AI-powered features (suggest-links, etc.) must address this in their own designs

### 6. Response size

- Cap upstream body read at **10 MiB** via `io.LimitReader`
- If the limit is hit, return `ErrBadResponse` rather than truncating silently

### 7. TLS

- Use the default `http.Client` TLS config (system roots)
- No `insecure_skip_verify` option in this slice

### 8. SSRF considerations

- Users configure `base_url` themselves; SSRF in the traditional sense (attacker-controlled URL) requires the attacker to first control the config file, which requires filesystem access
- We do not protect against `base_url: http://169.254.169.254/...` (cloud metadata) because that would prevent legitimate use of `localhost` and internal endpoints
- Documented as user responsibility

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria above — each criterion has a
concrete test.

**Edge Cases:**

| Case | Expected Behavior |
|---|---|
| Empty `.rela/ai.yaml` | `LoadConfig` returns error (required fields missing) |
| `.rela/ai.yaml` missing | `LoadConfig` returns `(nil, nil)`; runtime gets nil provider; Lua returns `not_configured` |
| Env var named in `api_key_env` is unset | `Chat()` returns `ErrAuth` (NOT construction failure) |
| Env var is set but empty string | Same — treat empty as unset |
| `base_url` with trailing slash | Normalize: strip trailing slash before joining `/chat/completions` |
| `base_url` missing scheme | `Validate()` returns error (require `http://` or `https://`) |
| `base_url` with `user:pass@host` | `Validate()` returns error |
| `temperature=0` from Lua | Sent as `"temperature": 0` (pointer is non-nil) |
| `temperature` absent from Lua | Field omitted from JSON (pointer is nil) |
| `max_tokens=0` from Lua | Sent as `"max_tokens": 0` |
| `max_tokens` absent | Field omitted |
| Empty `messages` | `RaiseError` (programming error) |
| `messages` not a table | `RaiseError` |
| `messages` entry missing `role` or `content` | `RaiseError` |
| Upstream returns SSE stream | `ErrStreamingUnsupported` |
| Upstream returns HTML error page | `ErrBadResponse` with status + body snippet |
| Upstream returns 200 with empty `choices` | `ErrBadResponse` |
| Upstream returns 200 with `content` as string | Success, normal path |
| Upstream returns 200 with `content` as `[{type, text}]` array | Success, parts concatenated |
| Upstream returns 200 with `content` as integer | `ErrBadResponse` |
| Upstream returns 200 with malformed JSON | `ErrBadResponse` with snippet |
| Upstream returns 200 missing `usage` | Success, `Usage{}` zero values |
| Upstream returns 200 missing `finish_reason` | Success, empty string |
| Upstream returns 401 with OpenAI envelope | `ErrAuth`, message includes upstream `error.message` |
| Upstream returns 403 | `ErrAuth` |
| Upstream returns 400 with envelope | `ErrBadRequest`, includes upstream message |
| Upstream returns 429 with `Retry-After: 30` | `ErrRateLimited`, `RetryAfter == 30s` |
| Upstream returns 429 without `Retry-After` | `ErrRateLimited`, `RetryAfter == 0` |
| Upstream returns 5xx | `ErrServerError` |
| Network error (DNS, connection refused) | `ErrNetwork` |
| Timeout exceeded | `ErrTimeout` |
| Response body > 10 MiB | `ErrBadResponse` |
| API key sentinel test | Sentinel never appears in any error or log |
| Concurrent providers across runtimes | Safe; `http.Client` is safe to share |
| Single LState used from one goroutine at a time | Safe; documented invariant |
| Two runtimes constructed without `WithAIProvider` | Both have `ai` global; both `chat` and `complete` return `not_configured` |

**Negative Tests:**

- Invalid YAML → loader returns wrapped error
- Missing required config field → loader returns error naming the field
- Missing API key env var → `Chat()` returns `ErrAuth` naming the env var
- HTTP 401 → `ErrAuth` to Lua
- HTTP 429 → `ErrRateLimited` with `RetryAfter`
- HTTP 500 → `ErrServerError`
- Network unreachable → `ErrNetwork`
- Timeout → `ErrTimeout`
- SSE response → `ErrStreamingUnsupported`
- HTML response → `ErrBadResponse`
- Lua arg type errors → `RaiseError`
- `ai.chat` with no provider wired → `ErrNotConfigured`

**Integration Test Approach:**

`internal/lua/ai_test.go` runs the full stack end-to-end:

1. Spin up `httptest.NewServer` returning canned OpenAI-compatible JSON
2. Build `*ai.Config` pointing at the test server's URL
3. Set the API key env var via `t.Setenv` to a known value
4. Build `Provider` via `ai.NewOpenAICompatProvider(cfg)`
5. Construct `lua.Runtime` with `WithAIProvider(p)`
6. Run a Lua script via `Runtime.RunString(...)` that calls `ai.chat` and writes to `rela.output`
7. Parse the output JSON, assert all fields

Plus a parallel suite that exercises every error path through the Lua surface to
confirm the error tables marshal correctly.

We do NOT mock at the `Provider` interface level for the Lua integration tests —
the whole point is to verify the wire format, HTTP plumbing, and error
marshaling.

Provider-level unit tests (`internal/ai/openai_test.go`) DO use
`httptest.Server` directly without involving Lua, for finer-grained coverage of
HTTP edge cases.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| OpenAI-compat providers diverge in response shape | High | Medium | Tests for: missing usage, missing finish_reason, content-as-array, content-as-string. Tolerant `json.RawMessage` decoding. |
| The `(nil, err_table)` convention split causes confusion | Medium | Low | Top-of-file comment + this planning doc as the durable rationale |
| Network egress + script-level exfiltration is a real new risk class | Medium | Medium | Documented honestly in CLAUDE.md and `ai-integration` concept; operational logging makes it visible |
| API key leaks via some path we didn't think of | Low | High | `redactKey` helper used everywhere + table-driven sentinel test across all error paths + reject `user:pass@host` URLs |
| Test flakiness from `httptest.Server` race conditions | Low | Low | `t.Cleanup(server.Close)` |
| `gopher-lua` LState concurrent use bug | Low | High | Documented invariant; existing rela code does not use Lua coroutines across goroutines |
| Five wiring sites mean one is missed | Medium | Low | Concrete list in plan; grep `lua.New(` during impl as a final check |
| Embeddings ticket forces interface refactor | Low | Low | `ai.Provider` aggregate interface is forward-compatible; only test fakes need a stub method |
| Operational log volume is too high in scripts that call AI in a loop | Low | Low | Default level is INFO; DEBUG-level events are off by default |

**Effort:** **m+** (slightly larger than the original `m` estimate). Roughly:

- `internal/ai/config.go` + tests: small
- `internal/ai/provider.go`: small (interface only)
- `internal/ai/errors.go` + tests: small
- `internal/ai/redact.go` + tests: small
- `internal/ai/openai.go` + tests: medium-large (the bulk of the work — HTTP plumbing, content-shape handling, error classification, edge cases)
- `internal/lua/ai.go` + tests: medium (error table marshaling, table-key-presence checks for optional fields)
- `internal/lua/runtime.go` edit: trivial
- 5 entry-point wirings: small (one line each, plus error handling)
- CLAUDE.md update + concept update: small
- Total: **~1 day** of focused work, larger than the original `m` estimate but justified by the design review fixes.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] CLAUDE.md — new section: `.rela/ai.yaml` config, Lua `ai.*` API, error taxonomy table, **network egress + script-level exfiltration security note**
- [x] `ai-integration` concept — add explicit script-level exfiltration threat
- [x] Top-of-file comment in `internal/lua/ai.go` documenting the `(nil, err_table)` convention split, the programming-error vs runtime-error taxonomy, AND the LState concurrency invariant
- [x] ~~User guide / reference docs~~ (N/A: no Lua API reference docs exist yet; TKT-CVG6 covers documenting them)
- [x] ~~CLI help text~~ (N/A: no CLI commands added in this slice)
- [x] ~~README.md~~ (N/A: project-level readme is unaffected)
- [x] ~~API docs~~ (N/A: no public API docs to update)
- Optional: a small example script (deferred unless a natural home appears)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| RR | Severity | Title | Status |
|---|---|---|---|
| RR-JV6M | significant | AC #15 contains unfinished editing residue | addressed (AC list rewritten) |
| RR-Z5XJ | critical | temperature=0 cannot be distinguished from unset | addressed (`*float64` / `*int` end-to-end) |
| RR-8HSQ | critical | No typed errors or rate-limit handling | addressed (error taxonomy with stable `kind` enum, marshaled as Lua table) |
| RR-U833 | critical | API key read at construction | addressed (env var read deferred to `Chat()` call) |
| RR-SNJ8 | significant | Client interface too narrow for embeddings | addressed (renamed to `Provider`, single wiring point) |
| RR-LUXQ | significant | Lua sandbox concurrency invariants unstated | addressed (top-of-file comment in `internal/lua/ai.go`) |
| RR-6FMU | significant | Provider-divergence handling hand-waved | addressed (tolerant `json.RawMessage`, content-as-array, missing fields) |
| RR-TH21 | significant | Streamed response confusion | addressed (`stream: false` always sent, SSE rejected with typed error) |
| RR-GIK4 | significant | Content-Type validation missing | addressed (validated before decode) |
| RR-LQ1R | significant | Threat model dismissively framed | addressed (security section rewritten with script-level exfiltration as distinct threat) |
| RR-N3OU | significant | API key leak surface broader than one test | addressed (`redactKey` helper + table-driven sentinel test; CI scanning deferred) |
| RR-QIEC | significant | No operational logging | addressed (debug/info/warn requirements added) |
