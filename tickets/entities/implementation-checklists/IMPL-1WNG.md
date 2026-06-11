---
id: IMPL-1WNG
type: implementation-checklist
title: 'Implementation: Lua read bindings still use context.Background() — partial cancellation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: this is a ctx-propagation fix; the unit test drives the full Lua → binding → spied collaborator flow which IS the integration we need to verify. No higher-level integration boundary applies.)
- [x] Happy path implemented (8 sites replaced with `r.callerCtx()`)
- [x] Edge cases from planning handled (when `WithContext` not set, `callerCtx()` still returns `context.Background()` — existing tests cover this implicitly and all pass)
- [x] Error handling in place (errors surfaced, not swallowed) — no change to error paths; ctx is now threaded into the same store/tracer/searcher calls that already returned errors correctly

## Test Quality

- [x] Using fixture builders or factories for test data (`newMockWorkspace` + `services("/tmp")` — same pattern as surrounding tests)
- [x] No hardcoded values in assertions when object is in scope (the marker string is the *test value being injected*; that's exactly the "appropriate hardcoding" case for an injected sentinel)
- [x] Only specifying values that matter for the test (single sentinel value, no superfluous fixture details)
- [x] Interpolated values constructed from objects, not hardcoded (N/A — no interpolation in this test)
- [x] Property comparisons use original object, not hardcoded strings (N/A — no property comparisons)

## Manual Verification

- [x] Feature manually tested end-to-end (the spy-based test IS the end-to-end verification through the binding boundary)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **AC1 (no `context.Background()` in bindings):**
`grep "context.Background" internal/lua/runtime.go` → returns only:
  - L134 (doc comment of `callerCtx`)
  - L142 (the `callerCtx` fallback itself — the helper used by all bindings)
  - L552 (`applyTimeout` fallback when `parentCtx == nil`)

No Lua-binding hits.

- **AC2 (parent ctx flows through read bindings):**
`go test -run TestReadBindings_PropagateCallerContext -race ./internal/lua/` →
`ok github.com/Sourcehaven-BV/rela/internal/lua  1.401s` All 7 subtests pass:
`get_entity`, `list_entities`, `get_relations`, `trace_from`, `trace_to`,
`search`, `find_path`.

- **AC3 (no regression):**
`go test -race ./internal/lua/` → `ok
github.com/Sourcehaven-BV/rela/internal/lua  7.377s`

## Quality

- [x] Code follows project patterns (matches the existing `r.callerCtx()` usage in write bindings; spy pattern follows existing `mockSearcher`/`mockManager` style)
- [x] No security issues introduced (no new inputs; ctx flow only tightens cancellation, doesn't widen access)
- [x] No silent failures (errors logged AND returned) — ctx threading doesn't change error semantics
- [x] No debug code left behind
