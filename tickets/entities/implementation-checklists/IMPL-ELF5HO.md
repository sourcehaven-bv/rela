---
id: IMPL-ELF5HO
type: implementation-checklist
title: 'Implementation: Sessions 4/1: session config and message-driven transitions'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `realtimecfg/session_test.go`, `load_test.go`, `compiled_test.go` (config shape, every load check, merge keys, immutable session); `realtime/session_test.go` (transitions, roles, `by: trigger`, guards incl. absent optional property and failing guard, notices, locked causing message, stranded, entered_at); `predicate/TestEval_NilOperand`
- [x] Integration tests written (test full flow, not just units): channel store conformance `channelstest/session.go` on file, sqlite and postgres (moves, invalid move on a missing channel, reply that moves, revision order); migration tests `sqlitedb/channelsessions_test.go` and `TestLegacyRowHasNoSession`; HTTP golden and list budget tests in dataentry; MCP and CLI session tests; `TestOpenAPI_ChannelSession`; frontend `wireFields.test.ts`
- [x] Happy path implemented: a channel type declares `session:`; a posted message of a type in the current state's `on:` moves the session, posts a notice and writes an audit record, all in one store step
- [x] Edge cases from planning handled: closed channels never move, `when_closed: post` triggers, re-entry, replies, two channels of a many type, reload with changed `initial` or removed states (stranded), concurrent posts move once
- [x] Error handling in place (errors surfaced, not swallowed): load checks name the field and the fix; 409 `session_notice` and `session_message`; failing guards are logged once per guard per reload

## Test Quality

- [x] Using fixture builders or factories for test data: shared test schema with an `incident` session type and the conformance harness
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no UI in this slice; HTTP, MCP and CLI paths are covered by integration tests; the UI is slice 4/2)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-I4J0JC
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on the default, `postgres`, `sqlite`
and `memorybackend` tags; `go test -race ./internal/...` on the default tag;
sqlite-tag tests on the channel stores, sqlitedb, realtime, realtimecfg,
dataentry, mcp and cli; golangci-lint on all tags (0 issues); `just arch-lint`,
`just comment-lint`, `just plimsoll`; vitest and vue-tsc; `just docs`
idempotent. Postgres tests were skipped locally (no DSN); CI runs them.

## Quality

- [x] Code follows project patterns (check similar code): shared logic in package channels, backend helpers as free functions (no new store methods), compiled config immutable and shared, predicate for guards
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one `isNotice` helper; one decider shared
by all stores
- [x] No security issues introduced: rela-security-reviewer found one minor finding (causing message editable), addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
