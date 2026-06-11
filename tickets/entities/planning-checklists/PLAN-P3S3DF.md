---
id: PLAN-P3S3DF
type: planning-checklist
title: 'Planning: Top-of-stack smoke tests (MCP dispatch, router walk)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-TLQ94B ticket body (MCP dispatch test, dataentry router walk
test, `TestAppRouter_*` rename, `doRequest` helper + convention). Out of scope:
rewriting existing handler-level tests, CLI argv smoke (covered by the Demos CI
job).

**Acceptance Criteria:**

1. An MCP test enumerates registered tools via a real JSON-RPC `tools/list` through `MCPServer.HandleMessage` and fails if the registered set diverges from the test table (loud failure on unlisted new tools).
2. Every registered tool is invoked via a real JSON-RPC `tools/call` with real JSON arguments (no hand-built `float64` maps); test asserts dispatch + argument decode + non-error handler result (or expected domain error).
3. A dataentry router walk test drives every registered API route through `app.NewRouter().ServeHTTP`, asserting not-404/405 and the no-cache middleware header; the route table fails loudly when the mux registers a route the table doesn't cover (where enumerable) or is documented as needing manual extension.
4. `TestAppRouter_*` tests renamed to reflect their actual altitude (handler-level affordance tests).
5. A `doRequest`-style helper goes through the real router; a comment establishes the convention that new endpoint tests use it.
6. New tests fail when: a route is removed from the mux, a tool is unregistered/renamed, an argument schema stops decoding.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small test-only addition)
- [x] Checked codebase for similar patterns
- [x] Reviewed relevant prior art

**Existing Solutions:**

- `mcp-go v0.54.1` exposes `MCPServer.HandleMessage(ctx, json.RawMessage)` — the in-process dispatch entry (used by mcp-go's own tests). `internal/mcp.Server` holds the `*server.MCPServer` in the unexported `s.mcp` field; tests are in-package and can reach it via the existing `newTestServer` helper.
- 25 tools registered in `internal/mcp/tools.go` `registerTools`.
- `internal/dataentry/router_test.go` already has a few `ServeHTTP` tests to extend alongside; `test_helpers_test.go` has the shared app fixture.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:**

1. `internal/mcp/dispatch_test.go`: build the server with the existing test fixture; send `tools/list` via `HandleMessage`, diff registered names against the test table; for each tool send `tools/call` with realistic JSON arguments and assert a well-formed, non-error result.
2. `internal/dataentry/router_walk_test.go`: table of registered routes (method, path, expected status class); drive through `NewRouter().ServeHTTP` on the shared fixture app.
3. Rename `TestAppRouter_*` in `api_v1_test.go` (mechanical).
4. Add `doRequest(t, app, method, path, body)` helper to `test_helpers_test.go` with convention comment; use it in the walk test.

**Alternatives considered:**

- *Stdio transport round-trip for MCP*: rejected — `HandleMessage` is the same dispatch path without process/pipe overhead.
- *Auto-deriving the dataentry route list from the mux*: `http.ServeMux` doesn't expose its route table; a literal table + the existing OpenAPI generator as cross-check is pragmatic. tools/list gives MCP the auto-derived property for free.

**Files to modify:** `internal/mcp/dispatch_test.go` (new),
`internal/dataentry/router_walk_test.go` (new),
`internal/dataentry/api_v1_test.go` (renames),
`internal/dataentry/test_helpers_test.go` (helper).

## Security Considerations

- [x] Input sources identified — test-only change, no production inputs
- [x] ~~Input validation approach defined~~ (N/A: no production code touched)
- [x] Security-sensitive operations identified — none; tests exercise existing surfaces
- [x] Error handling doesn't leak sensitive information — N/A

## Test Plan

- [x] Test scenarios documented for each acceptance criterion (the deliverable IS tests)
- [x] Edge cases identified: write-tool calls must use real store fixtures so they succeed (or assert the documented warning path); tools/call with unknown tool name asserted as JSON-RPC error
- [x] Negative test cases defined: removing a route/tool from registration must fail the new tests (verified manually during development)
- [x] Integration test approach defined — this ticket adds the integration layer

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed — none (test-only)
- [x] Effort estimated — `s`

**Risks:**

| Risk | Mitigation |
|---|---|
| mcp-go `HandleMessage` shape changes on upgrade | It's the library's stable public dispatch API; breakage is a compile error in one file |
| Route walk table goes stale (new route, no entry) | Pair with tools/list-style cross-check where possible; convention comment in `NewRouter` pointing at the walk test |
| Write tools mutate shared fixture state | Fresh fixture per subtest, same as existing tests |

## Documentation Planning

- [x] User-facing docs identified — N/A (test-only)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no user-facing docs affected)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A-with-substitute: approach discussed and revised with reviewer in working session 2026-06-10 — e2e/Demos coverage overlap surfaced, CLI argv smoke dropped, rewrite-vs-additive tradeoff explicitly decided; test-only change, no production design surface)
- [x] All critical/significant findings addressed in plan (session findings folded into scope)
