---
id: IMPL-28A6B0
type: implementation-checklist
title: 'Implementation: Top-of-stack smoke tests: MCP dispatch, router walk, ServeHTTP test convention'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (the deliverable IS tests: dispatch_test.go, router_walk_test.go)
- [x] Integration tests written (test full flow, not just units) — dispatch tests run JSON-RPC → mcp-go dispatch → decode → handler; walk test runs request → middleware → mux → handler
- [x] Happy path implemented (one realistic call per registered tool; one probe per registered route)
- [x] Edge cases from planning handled (unknown tool → JSON-RPC error; missing required arg → error result; unregistered route → loud stdlib-404 oracle)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (makeTestFixture extracted from makeTestServer; newHandlerTestApp reused)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test (wantStatus pinned only where the fixture makes it deterministic; 0 = "handler answered")
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- AC1/AC2: `TestDispatch_ToolInventoryMatches` + `TestDispatch_EveryToolDecodesAndRuns` pass for all 25 tools. **Caught a real fixture booby trap:** `newTestDeps` passed `templating.NewFSTemplater(nil, nil)` — create_entity/create_relation panicked (nil *project.Context) the moment the full create path ran; production's WithRecovery masks this class as JSON-RPC internal errors. Fixed with a documented nopTemplater.
- AC3: `TestRouterWalk_AllAPIRoutesReachHandlers` covers 33 probes over every registered route. **Caught two more fixture gaps:** newHandlerTestApp bypassed NewApp and left fieldResolver nil (panic via _search serialization) and OpenAPIGen nil (panic via _openapi.json). Both fixed in the fixture with comments.
- AC3 negative: manually removed the `_sidebar` registration → test failed with "answered by the mux's stdlib 404 — route is not registered"; restored.
- AC4: 18 `TestAppRouter_*` functions renamed to `TestV1Affordance_*` (defs + doc comments); zero references remain.
- AC5: `doRequest` helper added to test_helpers_test.go with the convention comment; registration sites in router.go / api_v1.go carry pointer comments.
- `go test -race ./internal/dataentry/ ./internal/mcp/` green; `golangci-lint run` on both packages: 0 issues. One collateral fix: my fixture change initially set `UserPalette: &PaletteConfig{}`, which broke `TestThemeImport_RoundTrip`'s nil-as-not-saved assertion — reverted that field with a comment explaining why it stays nil.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — dispatch/callTool helpers shared across the three dispatch tests; fixture extraction instead of duplication
- [x] No security issues introduced (test-only + comments)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
