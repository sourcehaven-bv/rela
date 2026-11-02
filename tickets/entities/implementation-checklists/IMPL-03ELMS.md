---
id: IMPL-03ELMS
type: implementation-checklist
title: 'Implementation: Serve the effective in: on message type views'
started: "2026-10-31"
completed: "2026-11-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestDefaultIn_Derive`, `TestDefaultIn_AbsentKeepsToday`, `TestDescribe_In`, `TestOpenAPI_ChannelMessageTypeSchema`
- [x] Integration tests written (test full flow, not just units): `TestChannelSchema_In` through the real HTTP handler, `TestReadChannel_In` through MCP, wire, OpenAPI and MCP goldens, `wireFields.test.ts`
- [x] Happy path implemented: each message type view serves `in` and `in_derived` next to `decision_button` in REST `_schema/channels`, OpenAPI and MCP `read_channel`
- [x] Edge cases from planning handled: no `in:` omits both fields; a written `in:` gives `in` only; a derived `in:` adds `in_derived: true`; unkeyed types under `default_in: keyed` carry neither; `in` is a copy so callers cannot change loaded config; same answer for every principal
- [x] ~~Error handling in place~~ (N/A: read-only view of loaded config; no new error paths)

## Test Quality

- [x] Using fixture builders or factories for test data: realtime config fixtures, `defaultInYAML` helper with exact-match edits
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: additive read fields, covered by HTTP handler and MCP tests)
- [x] Each acceptance criterion verified with test scenario from planning: AC table in the notes maps 1 to 8 to tests
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build on four tags; vet; golangci-lint on touched
packages for all four tag sets (0 issues; the full `just lint` could not finish
under load average 60 to 77, so CI covers the whole tree); arch-lint, plimsoll,
comment-lint; `go test -race` on realtimecfg, realtime, apiwire, openapi, mcp
and dataentry; 5527 frontend tests and type check; `just docs`; markdownlint;
`just build-frontend`; archplan ci; coverage check.

**Deviations:** the MCP test reads `room` under the default fixture before
loading the `default_in` fixture; `TestDescribe_In` creates its channel first.

## Quality

- [x] Code follows project patterns (check similar code): mirrors `decision_button` on the same views; config is served to every principal alike
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): the derived flag has one reader,
`MessageType.InDerived`
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
