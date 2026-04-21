---
id: PLAN-8TL60
type: planning-checklist
title: 'Planning: Add cache API for Lua scripts (get/set + memoize with TTL and size limits)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope (v1):

- `rela.cache.get(key)`, `rela.cache.set(key, value, opts?)`,
`rela.cache.memoize(key, fn, opts?)` Lua bindings
- Process-wide `Cache` singleton namespaced by script path
- Caller-owned `Cache` instance injected via `lua.WithCache(*Cache)`
- Per-call TTL (default 1h, `0`/negative = no expiry)
- Global LRU cap of 10,000 entries across all namespaces
- Per-call `bypass` option for `memoize`
- Available on both reader and writer runtimes
- `slog.Debug` operational logging with `namespace_hash` and `key_hash`
- `filepath.Clean(path)` as the namespace key; inline/eval runtimes
have no cache and raise a loud Lua error on `rela.cache.*`
- Coverage floor for `internal/lua` added to `.testcoverage.yml`
- Time source injection for deterministic TTL/LRU tests

Out of scope (v1):

- Disk persistence (deferred to v2; `Cache` interface designed to
accept future backends)
- Per-namespace entry cap / LRU
- `rela.cache.clear()`
- Cross-process sharing
- Metrics / hit-rate counters
- Background sweep goroutine for expired entries
- Config file
- Typed Go-callable cache API

**Acceptance Criteria:**

See ticket TKT-V8UQC for the full list (19 ACs). Test scenario mapping is in
**Test Plan** below.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **gopher-lua ecosystem**: no built-in cache. A thin domain-specific
binding is the right call — needs to interop with the rela-specific
`luaValueToGo` / `GoToLuaValue` paths
- **Go in-memory caches**: `hashicorp/golang-lru/v2`,
`patrickmn/go-cache`, `dgraph-io/ristretto`. All overkill for ≤10k entries with
simple TTL. A `map[string]*entry` + sync.RWMutex is ~100 lines and zero new deps
- **Codebase prior art**:
  - `internal/lua/runtime.go:74-78` — `Runtime` already holds per-run
state (`params`, `secrets`, `aiProvider`); cache handle fits the same pattern
but as a *reference* to a process-wide instance
  - `internal/lua/runtime.go:145-149` — `WithAIProvider` is the exact
wiring model for `WithCache`
  - `internal/lua/runtime.go:447-464` — `registerContextBindings`
shows where to add the sub-table registration
  - `internal/lua/runtime.go:291-320` — `RunActionString` shows the
stack-delta pattern for capturing multi-return values from `fn`
  - `internal/lua/runtime.go:828-940` — `luaValueToGo` /
`GoToLuaValue` / `luaTableToGo` are the conversion functions we reuse
  - `internal/ai/loader.go` — pattern for entry-point-wired optional
capability (`LoadProvider` returns nil if not configured)
- **Reference**: `TKT-135Q` (AI response cache) is disk-focused
cross-process. We're in-memory intra-process. They complement; no dependency
between them
- **Concept reviewed**: `lua-scripting` exists; ticket affects-linked

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

New file `internal/lua/cache.go`:

```go
package lua

type Cache struct {
    mu       sync.RWMutex
    entries  map[string]*cacheEntry   // key is "namespace\x00userKey"
    now      func() time.Time          // injected for tests
}

type cacheEntry struct {
    values     []interface{}  // multi-return from memoize; or single
    expiresAt  time.Time      // zero = never
    lastAccess time.Time
}

func NewCache() *Cache
func (c *Cache) get(namespacedKey string) ([]interface{}, bool)
func (c *Cache) set(namespacedKey string, values []interface{}, ttl time.Duration)
func (c *Cache) delete(namespacedKey string)
```

New `lua.Option`:

```go
func WithCache(c *Cache) Option
```

`Runtime` grows two fields:

```go
type Runtime struct {
    ...
    cache      *Cache   // nil when not wired
    scriptPath string   // set by RunFile(filepath.Clean(path))
}
```

`RunFile(path, args)` adds one line: `r.scriptPath = filepath.Clean(path)`
before loading the script.

`registerCacheBindings(rela)` is called from `registerContextBindings` only when
`r.cache != nil`. Each binding starts with a script-path guard that raises when
`scriptPath == ""`.

`memoize` uses `PCall(0, lua.MultRet, nil)` and records stack delta to capture
all return values (same pattern as `RunActionString` at `runtime.go:291`). Store
as `[]interface{}`. Release the mutex across `fn()` — concurrent misses on same
key both run `fn`, last write wins (AC 13).

`set`/`get` are simpler: `set` validates type then stores; `get` looks up,
checks TTL, updates `lastAccess`, returns.

Eviction on `set` when at 10,000 cap: walk the map, find minimum `lastAccess`,
delete that one entry.

**Files to modify:**

- `internal/lua/cache.go` — NEW (~200 lines)
- `internal/lua/cache_test.go` — NEW (~350 lines)
- `internal/lua/runtime.go` — add fields, option, registration, scriptPath
- `internal/cli/root.go` — create the process-wide `Cache`
- `internal/cli/script.go`, `internal/cli/flow.go`,
`internal/mcp/tools_lua.go`, `internal/script/executor.go`,
`internal/scheduler/`, `internal/dataentry/`, `internal/validation/lua.go` —
pass cache via `WithCache`
- `.testcoverage.yml` — add `internal/lua: 85` floor
- `docs/lua-scripting.md` (confirm path during implementation) — add
`rela.cache` section
- `CLAUDE.md` — one-paragraph pointer to docs

**Alternatives considered:**

- **Per-runtime cache** (original v1 design) — rejected; dies with
the runtime, useless for 3 of 4 callers
- **Unnamespaced global cache** — rejected; cross-script collisions
- **Namespace by script-content hash** — rejected; churns on every edit
- **Closure-based auto-keying** — rejected; explicit keys safer
- **Disk backend in v1** — rejected; eliminates ~6 design-review
findings, halves effort
- **Separate `internal/cache` package** — rejected for v1; Lua-specific
- **`hashicorp/golang-lru/v2`** — rejected; ~30 lines saved, one new dep

**Dependencies:** stdlib only (`crypto/sha256`, `sync`, `time`, `log/slog`,
`path/filepath`). No new external modules.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `key` | Lua string | Length ≤ 512 bytes | Lua error at API boundary |
| `value` | Lua value (scalar or table) | Walk rejecting function/userdata/coroutine/channel | Lua error naming the offending type |
| `opts.ttl` | Lua number | Coerced to `time.Duration` (seconds); `≤ 0` = no expiry | Documented; not rejected |
| `opts.bypass` | Lua bool | Coerced | None |
| Unknown option keys | Lua table | Allowlist: `ttl` (set/memoize), `bypass` (memoize) | Lua error listing recognized keys |
| `scriptPath` (set internally by `RunFile`) | Caller of `RunFile` | `filepath.Clean` applied; never hashed directly in the cache key, only in logs | N/A |

**Security-Sensitive Operations:**

- **No disk access in v1** — eliminates all filesystem concerns
- **Logging**: `namespace_hash=<sha256(path)[:16]>` and
`key_hash=<sha256(userKey)[:16]>`. Raw script path never logged (reveals project
structure); raw key never logged (may contain user data)
- **Lua error messages**: never format `%v` over raw key or path.
Messages like `cache: key length 513 exceeds limit 512`. Test-enforced (AC 15,
AC 17)
- **Cross-script isolation**: namespace separator is `"\x00"`, illegal
in POSIX paths and rare in Lua usage. No cross-namespace collision possible

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (mapped to ACs):**

| AC | Test |
|---|---|
| 1 | `TestNewCacheReturnsValidInstance` |
| 2 | `TestWithCacheOptionWires` |
| 3 | `TestCacheUnregisteredWithoutOption` — "attempt to call a nil value" |
| 4 | `TestRunFileSetsScriptPath` |
| 5 | `TestCacheInInlineRaisesError` |
| 6 | `TestNamespacedKeysIsolated` |
| 7 | `TestCacheGetHitMiss`, `TestCacheGetLazilyDeletesExpired` |
| 8 | `TestCacheSetDefaultTTL`, `TestCacheSetZeroTTLNoExpiry`, `TestCacheSetNilDeletes` |
| 9 | `TestCacheRejectsLongKey` (513 bytes) |
| 10 | `TestCacheRejectsFunctionValue`, `...Userdata...`, `...Coroutine...`, `TestCacheRejectsNestedFunction` |
| 11 | `TestMemoizeRoundTripsAllReturns`, `TestMemoizeFnRaisesNotCached` |
| 12 | `TestCacheGetRejectsAnyOpts`, `TestCacheSetRejectsBypass`, `TestMemoizeRejectsUnknownOption` |
| 13 | `TestMemoizeConcurrentBothRunFnLastWriteWins` |
| 14 | `TestCacheLRUEvictionAtCap` (injected time) |
| 15 | `TestCacheLoggingHasHashes`, `TestCacheLoggingNeverLeaksRawKey` |
| 16 | Integration tests per entry point |
| 18 | `just coverage-check` with new floor |
| 19 | Manual review |

**Edge Cases:**

- Empty key `""` — allowed; tested
- Key with newlines/null/unicode — round-trips (no further encoding)
- Exactly-512-byte key — accepted; 513-byte rejected
- TTL = 0/-1/math.huge — all treated as no-expiry
- fn returns zero values — cached as empty list; next hit returns 0
- fn returns nil explicitly — cached as `[]{nil}`; distinguishable
from miss
- Recursive memoize same key — not deadlocked (mutex released);
outer overwrites inner
- Same-key concurrent memoize — both run fn, last write wins (AC 13)

**Negative Tests:**

- Function as value → `cache: cannot cache value of type function`
- Userdata/coroutine/channel → same shape
- Key > 512 bytes → `cache: key length 513 exceeds limit 512`
- Inline context → `cache: not available in inline/eval contexts`
- Unknown option → `cache: unknown option "refersh"; recognized: ttl, bypass`
- `WithCache(nil)` + call → `attempt to call a nil value`

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| Numeric precision past 2^53 | Lua limitation, documented; cache itself is lossless |
| Memory growth in long-lived processes | 10,000-entry cap + LRU; ~10 MiB worst case |
| Eviction walk O(n) slow under load | n=10k is microseconds; swap for heap if profile shows |
| Namespace pollution via symlinks | Each symlink has own path → own namespace |
| Adding state to Runtime clashing with tests | `cache` nil, `scriptPath` empty by default |
| Validation runtime churn | Stable pseudo-paths per rule; cache hits within one analyze |
| Cache holding resources (fd, socket) alive | Value representability check rejects functions/userdata |

**Effort: m** — ~200 LoC cache.go, ~350 LoC tests, ~15 LoC runtime.go, 7
entry-point patches, 40 LoC docs. ~one day with code review.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — `docs/lua-scripting.md` (or closest
existing equivalent — confirm path during implementation) gets a `rela.cache`
section with API reference and a memoize-an-AI-call example
- [x] ~~CLI help text~~ (N/A: no new commands)
- [x] CLAUDE.md — one-paragraph pointer to docs
- [x] ~~README.md~~ (N/A: too low-level for project README)
- [x] ~~API docs~~ (N/A: no HTTP API)

Enhancement ticket → `docs-checklist` created manually when entering review
(validator doesn't require it for merge; we add for rigor).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Critical (3):

- `RR-AE716` — Disk cache: no verification stored key matches filename
hash. **Resolved by scope change** (no disk in v1)
- `RR-YX8B5` — Recursive memoize on same key deadlocks under
sync.Mutex. **Addressed** in AC 13: mutex released across `fn`
- `RR-7SLP6` — memoize silently discards extra return values from `fn`.
**Addressed** in AC 11: `PCall` with `lua.MultRet` and stack-delta capture of
all returns

Significant (7):

- `RR-ZAOMQ` — io.LimitReader at 10 MiB wrong place. **Resolved by
scope change** (no disk)
- `RR-CN3QA` — No TOCTOU protection on disk read. **Resolved by scope
change** (no disk)
- `RR-YELGX` — `get(key, opts?)` has undocumented options. **Addressed**:
`get(key)` has no opts; AC 12 enumerates per-function
- `RR-ZCL8D` — `refresh` on bare `set` meaningless. **Addressed**:
options scoped per-function; `refresh` collapsed into `bypass`
- `RR-ATWUG` — Coroutine interaction with memoize untested. **Deferred**:
correct by construction given mutex-release semantic; add test if a concrete bug
emerges
- `RR-M2GMP` — `.rela/lua-cache/` creation not in plan. **Resolved by
scope change** (no disk)
- `RR-5QNHW` — Logging `%v` on error paths risks leaking raw keys.
**Addressed**: AC 15 + test in AC 17

Minor (3):

- `RR-4J53G` — Time injection missing. **Addressed**: `Cache.now` field
- `RR-NQA0P` — `internal/lua` not gated. **Addressed**: AC 18 adds floor
- `RR-WT3DI` — Docs target. **Addressed**: AC 19 points at docs/

All 13 findings linked from TKT-V8UQC via `has-review-response`. No open
critical or significant findings remain.
