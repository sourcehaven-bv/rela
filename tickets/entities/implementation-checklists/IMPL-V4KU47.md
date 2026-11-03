---
id: IMPL-V4KU47
type: implementation-checklist
title: 'Implementation: Comments over stdio MCP: add_comment and watch on list_comments'
started: "2026-10-28"
completed: "2026-10-28"
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

**Verification Evidence:** Built `rela` and ran it as `rela mcp` over stdio
JSON-RPC on a copy of the rela-mission project:
- Without `--run-as`: `add_comment` not listed. With
`--run-as integration:runner`: listed.
- A `watch` on `list_comments` for TKT-002; a second process
(`--run-as integration:other`) added a comment; the watch returned `changed
[0]`.
- `add_comment` stored a comment with author `integration:runner`;
`list_comments` showed both authors.
- Unit and wiring tests cover the face, not-found, type-not-enabled, text
anchor and empty-body cases (`internal/cli/mcp_comments_test.go`), the author
rule (`internal/comments/service_test.go`) and the watch poll
(`internal/mcp/tools_watch_comments_test.go`).
- `just arch-lint`, `just comment-lint`, golangci-lint: clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
