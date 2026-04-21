---
id: TKT-V8UQC
type: ticket
title: Add cache API for Lua scripts (get/set + memoize, process-wide with per-script namespace)
kind: enhancement
priority: medium
effort: m
status: done
---

## Goal

Add a general-purpose key/value cache to the Lua runtime so scripts can memoize
expensive work (AI calls, filtered lookups, computed views) without each author
re-inventing a caching layer. rela manages the lifetime (TTL, LRU cap, eviction)
so scripts don't leak memory.

Exposes `rela.cache` with three functions:

```lua
rela.cache.get(key)                    -- returns cached value or nil
rela.cache.set(key, value, opts?)      -- stores value (nil deletes)
rela.cache.memoize(key, fn, opts?)     -- get-or-compute via fn
```

The cache is a **process-wide singleton, namespaced by script path**. Two
scripts that both cache `"filtered-tickets"` get two distinct entries — no
accidental cross-script communication. Scheduled jobs keep their cache warm
across runs inside the same scheduler process.

## Background

The Lua runtime is invoked from many places:

- `rela script foo.lua` (one-shot) — runtime lifetime ~= process lifetime
- `rela scheduler` (long-lived) — runs many Lua scripts sequentially;
wants to cache per-task across repeat executions
- `rela-server` data-entry actions — new `Runtime` per HTTP request,
but same server process; wants to cache per-action across requests
- `rela mcp` (long-lived) — reuses a single `Runtime` for tool calls;
already has a natural cache lifetime but benefits from namespacing
- Validation rules (`internal/validation/lua.go`) — re-run on every
`analyze`; each rule is a script-like snippet whose compute shouldn't be
repeated needlessly within one analyze pass

A per-runtime cache dies with the runtime — useless for three of those four
callers. A global cache with no namespace bleeds state across unrelated scripts.
**A process-wide cache namespaced by script path fits all four without
confusion.**

This ticket explicitly does **not** replace `TKT-135Q` (AI response cache). That
ticket caches across *processes* on disk, keyed by provider+model+prompt hash.
This one is a faster in-process layer keyed by whatever the script author wants.
They complement.

## Strategy

**One cache, many namespaces.** The cache is a `Cache` value created by each
entry point (CLI, server, MCP, scheduler) and passed into Lua runtimes via
`lua.WithCache(cache)`. Each runtime also knows its own `scriptPath` (set by
`RunFile`) and prefixes all cache operations with it under the hood:

```go
// Under the hood:
cache.Get(scriptPath + "\x00" + key)
```

The separator is a null byte — illegal in POSIX paths, so no ambiguity between
`foo.lua\x00k` and some other encoding.

**No cache for inline / REPL / `lua_eval`.** Runtimes constructed via
`RunString` or the MCP `lua_eval` tool have no script path; `rela.cache` calls
raise a Lua error `cache: not available in inline/eval contexts`. This prevents
accidental cross-session bleed in MCP sessions.

**Caller-owned instance.** `lua.WithCache(cache)` follows the same wiring
pattern as `lua.WithAIProvider`. Four entry points create the cache (or inject
`nil` for cases that don't want caching). Tests get a fresh cache via
`lua.NewCache()` per test.

## API surface

```lua
-- All three functions are namespaced to the current script's path.
local v = rela.cache.get(key)                -- returns value or nil
rela.cache.set(key, value, opts?)            -- nil value deletes
local r = rela.cache.memoize(key, fn, opts?) -- get-or-compute
```

Options table:

| Field | Default | Applies to | Effect |
|---|---|---|---|
| `ttl` | `3600` (1h) | `set`, `memoize` | Seconds until expiry. `0` or negative = no expiry (still bounded by LRU cap) |
| `bypass` | `false` | `memoize` only | Skip read, still compute and store |
| `refresh` | `false` | `memoize` only | Alias for `bypass` (reserved for future semantic split if needed) |

`get` accepts no options — the signature is `get(key)`. `set` accepts only
`ttl`. Unknown options raise a Lua error (so typos like `refersh` surface loudly
instead of silently ignoring).

`memoize`'s `fn` may return multiple values; rela caches and re-emits **all** of
them. This matches the Lua `ai.chat` convention which returns `(result, err)`.

## Limits and eviction (the part rela owns)

| Limit | Value | Behavior on breach |
|---|---|---|
| Key length | ≤ 512 bytes | Lua error at API boundary |
| Value type | representable via `luaValueToGo` (strings, numbers, bool, nil, nested tables) | Lua error; functions/userdata/coroutines rejected |
| Global entry count | 10,000 across all namespaces | LRU eviction by `last_access_at` on `set` |
| Per-namespace entry count | none in v1 | (add if a runaway script starves others) |
| TTL | default 1h per `set`/`memoize` call; `0`/negative = no expiry | Expired entries return nil on `get` and are deleted lazily |

No config file. Constants live in `internal/lua/cache.go`. Revisit if real use
shows them wrong.

Eviction walks the global map looking for the lowest `last_access_at` (O(n),
n=10,000 — still microseconds on any real machine). If a list/heap becomes
justified by profiling, it's a drop-in replacement.

## Concurrency

This cache is a real multi-goroutine store:

- `rela-server` handles requests on goroutines
- `rela mcp` handles tool calls on goroutines
- `rela scheduler` runs tasks sequentially today but that's a
single-ticket change away from parallel

The `Cache` wraps a `sync.RWMutex`: reads take `RLock`, writes take `Lock`.
`memoize` releases the lock across `fn()` — two concurrent misses on the same
key both run `fn` and the second write wins. This is strictly correct for a
cache (values should be deterministic) and avoids the recursive-call deadlock
that holding the lock across `fn` would create.

## Bypass / invalidation

Three mechanisms:

1. **Per-call**: `{bypass = true}` on `memoize` — skip read, store result
2. **Delete one entry**: `rela.cache.set(key, nil)` — removes from the
current namespace
3. **Nuke**: restart the process

No `rela.cache.clear()` in v1 — defer until a concrete need lands. If added
later, it clears only the current script's namespace (per-namespace by design; a
global clear is never exposed to scripts).

## Out of scope

- Disk persistence (v2 could add `lua.WithCacheBackend(diskBackend)`
behind the same `Cache` interface — that's why the caller-owned pattern matters)
- Per-namespace entry cap / per-namespace LRU
- `rela.cache.clear()` for the current namespace
- Cross-process sharing (`TKT-135Q` covers that for AI)
- Hit-rate metrics / Prometheus counters — `slog.Debug` lines only
- Background sweep goroutine for expired entries — lazy expire only
- Typed Go-callable cache API for non-Lua code

## Acceptance criteria

1. New `internal/lua/cache.go` exports `type Cache` with methods `Get`,
`Set`, `Memoize`, `Delete` — plus `NewCache() *Cache` constructor
2. New `lua.WithCache(*Cache) Option` wires a cache onto a `Runtime`
3. `NewReader`/`NewWriter` without `WithCache` leave `rela.cache.*`
unregistered; calling those from Lua raises "attempt to call a nil value"
(matches the pattern for mutation bindings on reader runtimes)
4. `RunFile(path, args)` stores `filepath.Clean(path)` as the runtime's
script path; `RunString` and the MCP `lua_eval` tool leave it empty
5. When script path is empty AND `rela.cache.*` is called, the binding
raises a Lua error `cache: not available in inline/eval contexts`
6. All `rela.cache.*` operations prefix keys with
`scriptPath + "\x00"` before hitting the underlying `Cache`
7. `rela.cache.get(key)` returns the cached value on hit, `nil` on
miss or expiry. Expired entries are deleted lazily on read
8. `rela.cache.set(key, value, opts?)` stores the value; nil value
deletes; TTL default 1h; `ttl <= 0` means no expiry
9. Key length limit: 512 bytes, enforced at API boundary, Lua error
on violation
10. Value must be representable via `luaValueToGo` (scalars, tables of
those); functions/userdata/coroutines raise a Lua error at `set` time naming the
offending type
11. `rela.cache.memoize(key, fn, opts?)` returns cached value on hit;
on miss calls `fn()` and stores **all return values** (not just the first); on
next hit, re-emits all values. `fn` errors propagate without caching anything
12. Per-call options honored: `ttl` (set + memoize), `bypass`
(memoize only). Unknown option keys raise a Lua error
13. `Cache` uses `sync.RWMutex`; `Memoize` releases the lock across
`fn` execution (concurrent misses on the same key both run `fn`, last write wins
— documented explicitly)
14. Global entry count capped at 10,000 across all namespaces; LRU
eviction by `last_access_at` on `set` when at cap
15. `slog.Debug` logging with fields `cache=hit|miss|store|evict|expire`,
`namespace_hash=<sha256[:16]>`, `key_hash=<sha256[:16]>` — raw script paths and
raw keys never logged, never included in any Lua error message (test-enforced)
16. Entry points wired: `cmd/rela/main.go` (or `internal/cli/root.go`)
creates one `Cache`, stores on shared state; `internal/cli/script.go`,
`internal/cli/flow.go`, `internal/mcp/tools_lua.go`, `internal/dataentry`
server, `internal/scheduler`, and `internal/script/executor.go` all pass it to
`lua.WithCache`. Validation (`internal/validation/lua.go`) also gets it — rules
need the cache too
17. Tests cover:
    - namespaced isolation: two runtimes with different paths don't
see each other's entries
    - `rela.cache.*` in inline context raises the correct error
    - LRU eviction at 10,000-entry cap
    - TTL expiry (time source injected — no `time.Sleep`)
    - memoize with multi-return `fn` round-trips all return values
    - memoize with `fn` raising leaves nothing cached
    - concurrent memoize on same key: both `fn` calls observed, last
write wins, no deadlock (uses `sync.WaitGroup` with two goroutines)
    - disk-filename-hash-mismatch test is N/A (no disk in v1)
    - key length > 512 rejected with Lua error
    - function value rejected with Lua error naming the type
    - unknown option key rejected with Lua error
    - logs contain `namespace_hash`/`key_hash`, never raw path or key
    - set(nil) deletes; get after set(nil) returns nil
    - `RunFile` with absolute and relative paths cache-isolate by the
cleaned path (sanity check — `filepath.Clean` is stable)
18. `internal/lua` coverage: add a package floor of 85% to
`.testcoverage.yml` as part of this ticket so AC 13 / future regressions are
actually gated
19. Documentation:
    - `docs/lua-scripting.md` (or closest existing user-facing Lua doc
— confirm during implementation) gets a `rela.cache` section with API reference
+ one memoize-an-AI-call example
  - `CLAUDE.md` gets a one-paragraph note pointing at the full docs
so agents working on rela see it exists

## Notes

- The `Cache` type is deliberately exported from `internal/lua` rather
than put in a standalone `internal/cache` package. The logic wraps Lua semantics
(multi-return, representable values) and doesn't generalize. Extract if a second
caller emerges
- `WithCache(nil)` is a valid no-op: the `rela.cache.*` functions
don't get registered. This lets callers that deliberately want no caching (e.g.,
the test runner) opt out without a branch
- `luaMemoize` uses `PCall(0, lua.MultRet, nil)` and records the stack
delta to capture all returns — same pattern `RunActionString` uses
(`internal/lua/runtime.go:291`). Store as `[]interface{}`
- The scriptPath used for namespacing is `filepath.Clean(path)`, not
`filepath.Abs`, so the namespace is stable across different working directories
(relative path "foo.lua" from the script's parent dir is the stable name; the
CLI already `chdir`s to project root)
- `namespace_hash` and `key_hash` in logs are sha256 prefixes. We
don't log the script path directly because paths can reveal project structure;
we don't log keys because keys may contain user data (entity properties, AI
prompts)
- JSON encoding is only needed for the (future) disk backend; in-memory
v1 holds `interface{}` values directly. The "value representability" check is a
walk over the Lua value rejecting unsupported types, not a JSON round-trip
- Numeric precision: Lua numbers are float64; cached round-trip is
lossless. Users should not use the cache for int64 IDs > 2^53 — that's a Lua
limitation, not a cache one (document in the Lua docs)
- Two Cache instances in one process (tests, or someone calling
`cli.Execute` programmatically twice) are completely independent. The
"process-wide" framing refers to the default wiring, not a singleton

## Design Review Findings Addressed

This plan revision addresses the design review findings on the original
per-runtime approach:

- ✅ **Process-wide + namespaced** — replaces the per-runtime scope
that made the cache useless for 3 of 4 callers
- ✅ **No disk in v1** — eliminates the sha-mismatch / TOCTOU / atomic
rename / dir-creation / `io.LimitReader` sizing concerns from the original
plan's `{persist = true}` design. Revisit for v2
- ✅ **Memoize multi-return** — AC 11 explicitly captures all return
values instead of silently dropping past the first
- ✅ **Mutex across `fn()`** — AC 13 spells out the semantic: lock
released, concurrent double-compute allowed, no deadlock
- ✅ **Options per function** — AC 12: `get` accepts none, `set`
accepts `ttl`, `memoize` accepts all; unknown keys raise
- ✅ **`refresh` semantics collapsed into `bypass`** — the split was
speculative, removed
- ✅ **Inline/eval not cacheable** — AC 5 raises a loud error instead
of silently sharing a namespace
- ✅ **Time injection** — AC 17 bullet explicitly forbids `time.Sleep`;
the Cache gets a `now func() time.Time` field defaulted to `time.Now`
- ✅ **Coverage floor** — AC 18 adds the floor rather than claiming
AC 13 in the original "stays at floor" form (which was a no-op because
`internal/lua` had no floor)
- ✅ **Docs target** — AC 19 points at the user-facing Lua doc, not
CLAUDE.md
- ⏭️ **Error-message leakage test** — deferred to implementation:
AC 15's "test-enforced" clause covers it, but the plan doesn't prescribe exact
assertions; the test will pattern-match a recognizable key/path substring
against all emitted errors and log lines
