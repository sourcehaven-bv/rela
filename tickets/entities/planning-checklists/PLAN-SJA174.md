---
id: PLAN-SJA174
type: planning-checklist
title: 'Planning: Extract typeResolver + trace/export handlers off mcp.Server'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: `internal/mcp` — `typeResolver` (3 shared helpers off
tools_helpers.go), `traceHandler` (4 methods, tools_trace.go), `exportHandler`
(4 methods, tools_export.go); registerTools line re-points; directive 49 → 38;
the `mcp.Services` → `Deps` doc-drift fix in consumer-side-interfaces.md. OUT:
entity/relation/analysis/schema/prompt/resource clusters (later arc steps), any
behavior/wire/identity change.

**Acceptance Criteria:**
1. `just plimsoll` with directive at 38.
2. Full suite green — dispatch_test.go + golden_test.go pin tool behavior end
to end.
3. arch-lint/comment-lint/golangci-lint clean.

## Research

- [x] ~~Run `/research`~~ (N/A: mechanical extraction; full package survey done instead)
- [x] ~~Searched for existing libraries~~ (N/A: no new functionality)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — Explore survey of all 49 Server methods: clusters,
per-cluster deps footprint, ranked candidates with risk notes (recorded in the
arc roadmap).

**Existing Solutions:** `urlHelpers`/`mdHelpers` (internal/lua) for the
focused-type shape; `GraphReader`/`GraphCounter`/`Watcher` in server.go are the
existing consumer-side interfaces to reuse; the 25 `tool*()` builders are
already receiver-free.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** typeResolver first (the cross-cluster substrate:
resolveType/resolveEntityType/validatePropertyNames over deps.Meta only), then
the two cleanest read-only clusters: traceHandler {store GraphReader, tracer}
and exportHandler {store} — export touches exactly one dep. Server keeps
principalMiddleware/HTTPHandler/Serve/register*; handlers read identity from ctx
(middleware wraps at SDK level). Alternative rejected: starting with the
analysis cluster (−9) — bigger win but 5 deps wide; trace+export deliver −8 at
near-zero risk and install the injection seam first.

**Files to modify:** internal/mcp/{tools_helpers.go, tools_trace.go,
tools_export.go, server.go, tools.go},
docs/architecture/consumer-side-interfaces.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Unchanged — MCP tool args validated by the same
moved-verbatim code.

**Security-Sensitive Operations:** Identity attribution: principalMiddleware
stays on Server above all handlers; NO principal is threaded into handler
structs (pinned by principal_test.go/acl_test.go). Read gating unchanged —
handlers keep the same GraphReader they reached via deps.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: no new behavior — dispatch/golden suites are the integration tests)

**Test Scenarios:** dispatch_test.go exercises registration→handler routing;
golden_test.go pins tool output. Both must pass unchanged.

**Edge Cases:** The toolGetSchema/toolGetMetamodel aliasing (both register
handleGetSchema) is out of scope but must not be disturbed by tools.go edits.

**Negative Tests:** Existing invalid-type/unknown-entity error tests pass
unchanged through typeResolver.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Low — all moved methods unexported, read-only clusters, no watcher
involvement (that's the entity cluster, a later step). Effort: m.

## Documentation Planning

- [x] User-facing docs identified: the consumer-side-interfaces.md drift fix is in scope
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor kind; the one doc edit is named in scope)

**Documentation Impact:** docs/architecture/consumer-side-interfaces.md — stale
`mcp.Services` reference updated to the `Deps` bundle.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: repeats the established extraction design; plan derived from a dedicated structural survey with risk ranking)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no review run)

**Design Review Findings:** N/A
