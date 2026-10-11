---
id: IMPL-6WB4AP
type: implementation-checklist
title: 'Implementation: MCP tools for comments'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Stdio, by hand: built `rela`, made a project with `comments:` on `item`, and
sent JSON-RPC over stdin to `rela mcp`. `tools/list` holds only `list_comments`;
`list_comments` on ITEM-001 returns `{"comments": []}`; on an unknown id it
answers `Entity not found [not_found]`; `add_comment` is an unknown tool.
- Remote: `TestMCPComments_ThroughTheRemoteMount` sends a JWT-authenticated
request through `App.NewRouter` to `/api/v1/_mcp` and runs the host's comment
operations on that request's ctx: the verified user is the author, and a user
without `comment:add` is refused. The MCP protocol layer is covered by
`internal/mcp` tests over the real SDK dispatch.
- AC1-AC9: `TestMCPComments_*` (roundtrip, refusals, read-only, accept,
authorization with four principals, faces), `TestCommentTools_*`,
`TestStdioComments_*`, `TestRemoteMCPDeps_PassesTheHostComments`, and the
regenerated `tools_list` / `tool_calls` goldens.
- AC8: no existing dataentry test file was changed; `go test
./internal/dataentry` passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
