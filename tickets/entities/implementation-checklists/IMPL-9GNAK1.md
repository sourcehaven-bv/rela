---
id: IMPL-9GNAK1
type: implementation-checklist
title: 'Implementation: Sessions 4/3: interrupting timers'
started: "2026-10-22"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `realtimecfg/timers_test.go` (shape, limits, timer-only cycles); `realtime/timers_test.go` (due computation, restart anchors, reopen, restore, reload, stranded, degraded, rounds, budget); `channels/timers_test.go`; `TestDamageWarner`, `TestResumption`
- [x] Integration tests written (test full flow, not just units): `channelstest/timers.go` conformance on file, sqlite and postgres including `RunDamagedDueRow`; `sqlitedb/channeltimers_test.go` opens a real v24 file; sqlite due-index explain test; `TestRealtimeWorker_TimerWake`; dataentry, MCP, OpenAPI and CLI wire tests
- [x] Happy path implemented: a state's `after:` timer moves the session when due, posts a notice and writes an audit record, fired by the leader's timer loop
- [x] Edge cases from planning handled: closed channels freeze, repeated close/reopen, restore from tombstone, reload changing periods, stranded and final states, leader overlap (CAS on state and generation), failing rows isolated, timer chains without ceiling waits
- [x] Error handling in place (errors surfaced, not swallowed): failed writes logged once with the channel; damaged rows skipped with a rate-limited warning; refused configs name the cycle or limit

## Test Quality

- [x] Using fixture builders or factories for test data: conformance harness and an injected clock
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: backend feature without UI; the worker wake test runs the assembled service end to end)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-65ZH3Q
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build, vet and golangci-lint on all four tags; `go
test -race` on realtime, realtimecfg, appbuild, channels, dataentry, mcp, cli;
sqlite-tag tests on channels, sqlitedb, realtime, dataentry, mcp, appbuild;
postgres-tag tests on channels, pgstore and appbuild against a local database
with `RELA_TEST_DATABASE_REQUIRED=1`; arch-lint, plimsoll, comment-lint clean;
vitest, vue-tsc, eslint for the wire types; `just coverage-check` 84.9%; `just
docs` idempotent.

## Quality

- [x] Code follows project patterns (check similar code): the auto.after sweep and leader worker, store-side decider under the lock, conformance harness
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one schedule computation shared by moves,
lifecycle and restore; one DamageWarner for both backends
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
