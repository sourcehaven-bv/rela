---
id: PLAN-DDWJ
type: planning-checklist
title: 'Planning: Lua read bindings still use context.Background() — partial cancellation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- Replace `context.Background()` with `r.callerCtx()` at every Lua read-binding site in `internal/lua/runtime.go` (8 sites across 7 binding functions).
- Add a regression test verifying ctx propagation.

OUT of scope:
- Making stores honor ctx cancellation mid-iteration (memstore ignores ctx; that's a separate store-layer concern).
- Touching `applyTimeout` (line 552) — its `context.Background()` is a fallback inside auxiliary timeout-derivation code, not a binding.
- Changing the Lua-script-visible API (no new args, no behavior changes scripts can observe).
- Documentation updates (`docs/`, CLAUDE.md) — no user-visible behavior change.

**Acceptance Criteria:**

1. **No `context.Background()` in Lua bindings.** `grep "context.Background" internal/lua/runtime.go` returns only the `applyTimeout` fallback (line 552), nothing else. → Verified by a one-line grep in CI / manual check.
2. **Parent ctx flows through read bindings.** A test constructs a Runtime with `WithContext(parent)` and asserts that the ctx received by the store/tracer/searcher inside `luaGetEntity`, `luaListEntities`, `luaGetRelations`, `luaTraceFrom`, `luaTraceTo`, `luaSearch`, `luaFindPath` is `parent` (not `context.Background()`). → Verified by a spy store/tracer/searcher that records the ctx.
3. **No regression.** All existing tests in `internal/lua/...` pass with `just test`.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- The fix mechanism already exists in the codebase: `r.callerCtx()` at `internal/lua/runtime.go:138` is the very helper used by all write bindings (`luaCreateEntity`, `luaUpdateEntity`, etc.). This ticket extends its use to read bindings — no new abstraction.
- Closely related prior art: `RR-JWDHH` ("outgoingRelations uses context.Background() instead of request context") was the same shape of fix in a different package and is already addressed.
- Prior cancellation tests exist at `internal/lua/runtime_test.go:2214` (`TestWithContext_CancellationInterruptsBusyLoop`) and `:2248` (`TestWithContext_NoTimeoutStillCancels`). These test the gopher-lua VM-level cancellation, not the Go-side binding ctx — so the new test fills a different gap.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Mechanical edit: replace `context.Background()` with `r.callerCtx()` at the 8
binding sites listed below. The helper already returns the parent ctx (or
`context.Background()` if none was set), so behavior is identical when no parent
ctx is configured.

Sites in `internal/lua/runtime.go` (verified line numbers):
- L740 — `luaGetEntity` → `r.deps.Store.GetEntity(...)`
- L760 — `luaListEntities` → `r.deps.Store.ListEntities(...)`
- L827 — `luaGetRelations` → `r.deps.Store.ListRelations(...)`
- L847 — `luaTraceFrom` → `r.deps.Tracer.TraceFrom(...)`
- L865 — `luaTraceTo` → `r.deps.Tracer.TraceTo(...)`
- L1264 — `luaSearch` → `r.deps.Searcher.Search(...)`
- L1270 — `luaSearch` (per-hit) → `r.deps.Store.GetEntity(...)`
- L1474 — `luaFindPath` → `r.deps.Tracer.FindPath(...)`

For the test: a spy `store.Store` / `tracer.Tracer` / `search.Searcher` that
records `ctx` per call. Drive each binding from Lua via `RunString`, then assert
the recorded ctx is the one passed via `WithContext`.

**Alternatives considered:**

1. *Cancel the parent ctx and assert a `context.Canceled` error from the binding.* Rejected because memstore (and most fakes) ignore ctx — the test would not actually exercise propagation. The spy approach proves propagation regardless of whether the store honors it.
2. *Replace `context.Background()` calls everywhere in the file including `applyTimeout`.* Rejected — `applyTimeout`'s `context.Background()` is a `nil`-parent fallback, semantically correct.

**Files to modify:**

- `internal/lua/runtime.go` — 8 line edits.
- `internal/lua/runtime_test.go` — add one test `TestReadBindings_PropagateCallerContext` (table-driven over the 7 binding functions).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

No new inputs. The ctx flowing through is the runtime's own parent ctx, set at
construction by trusted host code (cobra command, MCP server, etc.). Lua scripts
cannot influence the ctx.

**Security-Sensitive Operations:**

None affected. Read bindings already gate access through the same store the host
owns. Passing the right ctx tightens cancellation, never widens access.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 (no `context.Background()` in bindings): verified by reading the file post-edit + a `grep`.
- AC2 (ctx propagation): single new table-driven test exercising each of the 7 read bindings with a spy that captures the ctx argument. The spy wraps memstore so non-ctx semantics still work for the binding to produce a result.
- AC3 (no regression): full `just test` run in the review phase.

**Edge Cases:**

- `WithContext` *not* set: `r.callerCtx()` returns `context.Background()`, so behavior is unchanged. Existing tests cover this implicitly.
- Iterator bindings (`ListEntities`, `ListRelations`, `Search`): the test asserts ctx was passed in; whether the iterator polls ctx mid-stream is out of scope.

**Negative Tests:**

- N/A — this is a propagation fix, not new functionality. The "negative" case (no parent ctx) degrades to the previous behavior, which is acceptable.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Risk:* A binding might be invoked from a context where `r.parentCtx` was never set (e.g., some test path). *Mitigation:* `callerCtx()` already falls back to `context.Background()` — same as today's behavior. No new failure mode introduced.
- *Risk:* Coverage floor regressions. *Mitigation:* the new test adds coverage; nothing is being removed.

**Effort:** xs (eight one-line edits + one focused test).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed

Lua script authors don't see ctx behavior; the binding API is unchanged. No
`docs-checklist` will be created.

## Design Review

- [x] Run `/design-review` before starting implementation — skipped: the fix is a mechanical, narrowly-scoped propagation, mirroring write-binding behavior already in place. Design surface is too small to benefit from a review pass.
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
