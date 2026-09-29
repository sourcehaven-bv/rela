---
id: IMPL-N1T8VY
type: implementation-checklist
title: 'Implementation: Remote MCP: faced entities are invisible to every read tool'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`worldreader/bound_test.go`, `dataentry/mcp_world_test.go`, `mcp/convert_test.go`)
- [x] Integration tests written (`cmd/rela-server/mcp_world_test.go`: real services, ACL policy, faced schema, every remote MCP read handle)
- [x] Happy path implemented
- [x] Edge cases from planning handled (explicit ID@face, denied world, face-restricted reader ranks readable faces only, unreadable type stays hidden, config hot reload)
- [x] Error handling in place (a world-source error is returned, never rendered as an empty result)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: remote MCP needs a JWT issuer; covered by the wiring integration test over real services. Re-check on atlas after deploy.)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: verified by table-driven tests; see above)

**Verification Evidence:** Reverting `WorldBound` to a no-op fails 7 subtests in
`TestRemoteMCPDeps_*` (bare id, list, search for both tools and Lua; denied
world). With the fix all pass. `go test` on worldreader, appbuild, mcp,
dataentry, lua, visibility and cmd/rela-server passes; golangci-lint, arch-lint
and comment-lint are clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (`resolveNamedWorld` shared by the web API and MCP) — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (reads stay on the gated reader; world grant checked as on the web; denied world reads nothing)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
