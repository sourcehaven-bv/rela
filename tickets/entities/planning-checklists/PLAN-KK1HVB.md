---
id: PLAN-KK1HVB
type: planning-checklist
title: 'Planning: Extract HTTP/cache/AI binding clusters off lua.Runtime'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: `internal/lua/{http.go, cache.go, ai.go, runtime.go}` — move the
HTTP (8), cache (4) and AI (3) binding methods onto focused types; `register*`
methods stay on Runtime as wiring seams; ratchet `//plimsoll:max-methods` 60 →
45. Stacked on TKT-4WBLG6. OUT: elevation/output/lifecycle clusters, any
behavior or capability-gating change.

**Acceptance Criteria:**
1. `just plimsoll` passes with the Runtime directive at 45.
2. Full `go test ./...` + `-race` on internal/lua green — http_test.go (856
lines) and the cache/ai suites drive every binding through Lua unchanged.
3. arch-lint/comment-lint/golangci-lint clean.

## Research

- [x] ~~Run `/research`~~ (N/A: continuation of the TKT-4WBLG6 extraction pattern)
- [x] ~~Searched for existing libraries~~ (N/A: no new functionality)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — structural survey of Runtime's 105 methods performed;
per-cluster field usage verified by grep (HTTP touches only L/L.Context; cache
touches cache+scriptPath+L; AI touches aiProvider+L).

**Existing Solutions:** `urlHelpers` (urls.go) and the fresh
mdHelpers/mdASTConverter/mdEntityRefs (TKT-4WBLG6) are the in-package templates;
`cacheStore` in runtime.go is already the narrow consumer-side interface the
cache type needs.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Three types — `httpBindings` (Lua state only;
`httpContext` signature narrowed to `*lua.LState`), `cacheBindings`
(`cacheStore` + `scriptPath func() string` closure, because
`Runtime.SetScriptPath` mutates the field after registration), `aiBindings`
(provider + state). The `caps.HTTP` gate stays in `registerBindings` — gating
location is a security property and does not move. Alternative rejected: one
combined "ioBindings" type — the three clusters have disjoint deps and lumping
them recreates the grab-bag this arc removes.

**Files to modify:** internal/lua/{http.go, cache.go, ai.go, runtime.go}

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Unchanged — Lua-supplied request params
validated by the same moved-verbatim code.

**Security-Sensitive Operations:** The HTTP capability gate (`caps.HTTP` in
`registerBindings`) is the one security-relevant seam near this change and is
explicitly out of scope: registration stays on Runtime, gate unmoved.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: no new behavior — existing Lua-level suites are the integration tests)

**Test Scenarios:** http_test.go, cache tests, ai tests drive all 15 moved
bindings through actual Lua scripts; pass unchanged.

**Edge Cases:** scriptPath mutation after registration (SetScriptPath) must keep
propagating — hence closure, pinned by existing cache-memoize tests that set a
script path.

**Negative Tests:** Existing capability-denied (caps.HTTP off) and error-path
tests pass unchanged.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Low — unexported-only moves; the one stateful wrinkle (scriptPath) is
called out with its mitigation. Stacked-branch conflict risk handled by basing
on TKT-4WBLG6's branch. Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor kind)

**Documentation Impact:** N/A — internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: repeats the TKT-4WBLG6 extraction design one cluster over; no new interfaces or behavior)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no review run)

**Design Review Findings:** N/A
