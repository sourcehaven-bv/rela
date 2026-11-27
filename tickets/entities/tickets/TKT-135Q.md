---
id: TKT-135Q
type: ticket
title: Add content-hash-keyed cache for AI responses
kind: enhancement
priority: high
effort: m
status: ready
---

## Goal

Add a content-hash-keyed disk cache for AI responses (chat and embeddings) so
repeated runs against the same input don't burn tokens or wait on the network.
Implemented as a decorator over `ai.Provider` so it composes cleanly without
touching the underlying transport code.

## Background

Once embeddings (`TKT-5FYM`) ship, every analyze run that uses semantic search
will re-embed every entity unless we cache. For a 200-entity project that's
potentially thousands of API calls per run — too expensive to use casually, even
against a local Ollama instance, where the bottleneck is GPU latency rather than
dollars.

Chat completions also benefit, especially for deterministic prompts
(`temperature=0`) used in golden tests, evals, or "improve this entity
description" workflows where the same input will be retried multiple times
during iteration.

## Strategy

**Decorator over `ai.Provider`.** A new `internal/ai/cache` subpackage exposes:

```go
type CachingProvider struct {
    inner ai.Provider
    store CacheStore
}

func NewCachingProvider(inner ai.Provider, dir string) (*CachingProvider, error)

func (c *CachingProvider) Chat(ctx, req) (*ai.ChatResponse, error)
func (c *CachingProvider) Embed(ctx, req) (*ai.EmbedResponse, error)
```

`CachingProvider` implements `ai.Provider`, so callers see no API change. Wire
it into `ai.LoadProvider` so every entry point gets caching for free.

## Cache key

For chat:

sha256(provider_id || model || temperature || max_tokens || canonical(messages))

For embed:

sha256(provider_id || model || canonical(input))

`canonical(messages)` and `canonical(input)` are JSON marshals with sorted map
keys. `provider_id` is a stable string derived from `Config.BaseURL` (so a cache
built against `localhost:11434` doesn't get reused when the user switches to
OpenAI). `temperature=0` is explicit, `nil` is `"unset"`, so the same key only
matches truly identical requests.

## Cache layout

```
.rela/ai-cache/
  chat/
    <sha256-prefix>/<sha256>.json    # serialized ChatResponse + metadata
  embed/
    <sha256-prefix>/<sha256>.json    # serialized EmbedResponse + metadata
```

Two-level fan-out (`<sha256-prefix>/<sha256>`) so directories don't get
hot-spotted with thousands of entries.

`.rela/ai-cache/` is gitignored (`.rela/` is already gitignored as a whole —
verified).

## Cache entry format

```json
{
  "version": 1,
  "created_at": "2026-04-08T...",
  "request_summary": {
    "model": "gemma3:12b",
    "kind": "chat",
    "prompt_tokens": 20
  },
  "response": { /* ChatResponse or EmbedResponse */ }
}
```

The `request_summary` is *not* the cache key (the key is the sha256 in the
filename) — it's just human-readable metadata for debugging ("what's in this
cache entry?").

The cache file contains the **response** (model output) but not the **request**
(prompt). The hash-only filename means you can't enumerate the prompts from the
cache, but the response payload itself is stored in cleartext. Privacy story is
the same as `.rela/cache.json`: gitignored, local to the user, treat as you
would any local-only project state.

## Invalidation

**TTL-less by default** — responses to deterministic prompts (`temperature=0`)
don't go stale. Users can `rm -rf .rela/ai-cache/` to invalidate manually.

A future ticket may add an optional `cache.ttl_days` config field for users who
want time-based eviction. **Out of scope for v1.**

## Bypass

Users sometimes need to force a fresh call (e.g. after switching models or to
refresh stale embeddings). Two mechanisms:

1. **Global**: `.rela/ai.yaml` has `cache: false` to disable entirely.
2. **Per-call**: Lua scripts can pass `bypass_cache = true` in the
options table for `ai.chat` and `ai.embed`. Defaults to false.

## Out of Scope

- LRU eviction or size limits — this is a leaf-fanout disk cache, not
an in-memory LRU. Users who care can `du -sh .rela/ai-cache/` and `rm -rf` if it
gets big. Add eviction in v2 if it becomes a real problem.
- Distributed cache (Redis, S3, etc.). Local disk only.
- Cache hit/miss metrics — operational logging emits `cache=hit` /
`cache=miss` fields in the existing `slog.Info` line so users can grep, but no
Prometheus-style counters.
- Cache warming (precomputing embeddings for the whole graph). Useful
but a separate ticket.
- Sharing cache between projects — keyed by `provider_id` which is per
`BaseURL`, not per-project. If two projects use the same provider, they share
cache entries. That's a feature, not a bug.

## Acceptance Criteria

1. New `internal/ai/cache` package with a `CachingProvider` that
implements `ai.Provider` (decorator over an inner provider).
2. `Chat` and `Embed` first check the cache, then fall through to the
inner provider on miss, then write the response back to disk.
3. Cache key is sha256 over (provider_id, model, temperature,
max_tokens, canonical-JSON of input). Different providers/models/ parameters
never collide.
4. Cache directory is `.rela/ai-cache/{chat,embed}/<prefix>/<sha>.json`
with two-level fan-out.
5. `ai.LoadProvider` wraps the underlying provider in
`CachingProvider` by default. Disabled if `.rela/ai.yaml` has `cache: false`.
6. Lua bindings accept an optional `bypass_cache = true` in the
options table for both `ai.chat` and `ai.embed`. When set, the cache layer is
skipped for that one call (no read, no write).
7. Operational logging includes `cache=hit` / `cache=miss` fields in
the existing `slog.Info` line for AI requests.
8. Cache hits still produce a log line at `slog.Debug` so users can
verify caching is working without enabling full debug.
9. Cache misses are written atomically (write to `.tmp` then rename)
so a crash mid-write doesn't leave a half-file that fails to parse on next read.
10. Tests cover: cache miss + write, cache hit, bypass_cache,
concurrent reads/writes don't corrupt entries, atomic writes, cache disabled via
config, key collision avoidance across parameters.
11. Live smoke test against ollama: first call to a deterministic
prompt is slow (~1s gemma3 inference); second call with the same prompt is <50ms
(cache hit) and produces a `cache=hit` log line.
12. `internal/ai` coverage stays at 90 %+ including the new cache
package.

## Notes

- Implement the cache as `internal/ai/cache.New(ai.Provider, dir) ai.Provider`
rather than a method on `ai.Config` so the decorator pattern is obvious and
tests can stub the inner provider with a fake.
- The `provider_id` derivation should be `sha256(BaseURL)[:16]` rather
than the raw URL to avoid pathological filenames if a user has a weird
`BaseURL`.
- Use `os.WriteFile` to a `.tmp` sibling then `os.Rename` for atomicity.
POSIX guarantees rename is atomic on the same filesystem.
- Cache reads should be `io.LimitReader` capped to 100 MiB per entry.
An attacker who can drop a giant file into the cache dir already owns your
machine, but the cap prevents accidental OOM if a future bug writes a malformed
huge entry.
- The `request_summary` metadata field is for `rela ai cache inspect`
in some future ticket, not for runtime use. Keep it lightweight.
- This ticket explicitly **depends on** `TKT-5FYM` (embeddings) for
the Embed code path. The Chat caching can ship first if needed, but the whole
point is making embeddings affordable, so don't split unless required.
