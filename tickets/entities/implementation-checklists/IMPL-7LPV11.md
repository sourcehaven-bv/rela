---
id: IMPL-7LPV11
type: implementation-checklist
title: 'Implementation: Remote MCP: lua_eval/lua_run bypass read ACL; search_entities returns unreadable hits'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestGatedReads_Searcher, TestGatedReads_SearcherHitShape, TestNewServer_LuaToolsAreOptIn, TestACL_SearchEntities_OmitsHidden)
- [x] Integration tests written (test full flow, not just units) (cmd/rela-server/mcp_remote_test.go: real MCP client over HTTP against newRemoteMCPServer)
- [x] Happy path implemented
- [x] Edge cases from planning handled (NopACL parity, missing principal, gate construction failure, hidden-field-only match, hidden title, denied type filter)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (appbuildtest.New)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: a live -mcp run needs a JWT IdP; the HTTP round-trip tests drive the production server constructor with a real MCP client instead)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: covered by the table-driven tests above)

**Verification Evidence:**
New remote tests fail against the pre-fix wiring (Lua tools listed and callable; hidden feature, hidden title and hidden-field matches returned) and pass after it. `just test` passes.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors dataentry searchScopedHits / readGate.SearchScope and lua rela.search hydration)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
