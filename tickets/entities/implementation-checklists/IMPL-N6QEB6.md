---
id: IMPL-N6QEB6
type: implementation-checklist
title: 'Implementation: Realtime 3/6: channel MCP tools, watch and CLI'
started: "2026-10-17"
completed: "2026-10-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `Service.Read`, `UserText` and `LogOnly` (`internal/realtime`), per-type JSON schema (`internal/realtimecfg`), MCP channel tools and watch (`internal/mcp/tools_channels_test.go`, `tools_watch_channels_test.go`), run-as identity (`internal/cli/identity_test.go`), `rela channel` commands, rendering and watch (`internal/cli/channel_test.go`, `channel_render_test.go`, `channel_watch_test.go`), the reserved identity registry (`internal/principal`)
- [x] Integration tests written: `TestMCPStdio_ChannelToolsApplyACL` runs the stdio MCP server over a real project with acl.yaml; the channel CLI tests run against real projects on the default, postgres and sqlite builds; a subprocess test checks that rela-server refuses `RELA_RUN_AS`; an archguard test checks that every audit record sets `AssertedBy`
- [x] Happy path implemented: MCP channel tools and watch (PR #1928, merged), run-as identity and `rela channel` (T6b PR)
- [x] Edge cases from planning handled: hidden and missing entities answer like `rela show`, untrusted text is fenced, signals are refused on stdio, look-alike and reserved identity names are refused, `$USER` cannot bring a `system:` name, asserted_by comes from the uid and never from the environment, watch timeouts and interrupts have their own exit codes
- [x] Error handling in place: every realtime denial maps to a user text on both transports; identity errors name the fix (`$USER` or `RELA_RUN_AS`)

## Test Quality

- [x] Using fixture builders or factories for test data: the MCP channel fixture and the CLI project fixtures
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the CLI and stdio MCP flows run end to end in-process over real projects in the tests above; no UI ships in this ticket)
- [x] Each acceptance criterion verified with test scenario from planning: each AC in plan section 6 has a named test listed in plan section 7
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are covered by the identity, rendering, watch and ACL tests listed above)

**Verification Evidence:** Local checks ran on each slice: build and vet on the
default, `postgres` and `sqlite` tags; `go test -race` on the touched packages;
the sqlite-tagged tests and the postgres-tagged channel tests against a local
database; golangci-lint on all four tag sets; `just arch-lint`; `just
comment-lint`; `just plimsoll`; docs regenerated with `just docs`. PR #1928
merged with green CI. Four existing `TestLeaseCLI_*` tests fail under the
postgres tag locally because they open the project without database settings;
this predates the ticket.

## Quality

- [x] Code follows project patterns (check similar code): identity bound at the edge (#1907) with the archguard allowlist, consumer-side interfaces for the CLI and MCP channel seams, CLI channel commands go through acl.yaml like `rela lease`; arch-lint and plimsoll clean
- [x] Checked for DRY opportunities: one `UserText` mapping for both transports, one reserved-identity registry in `internal/principal`, one `audit.AssertedByFrom` helper
- [x] No security issues introduced: rela-security-reviewer ran on both slices with a re-review; all critical and significant findings are addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
