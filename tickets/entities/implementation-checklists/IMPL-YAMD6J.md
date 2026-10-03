---
id: IMPL-YAMD6J
type: implementation-checklist
title: 'Implementation: neoq Shutdown stops every listener on the database'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Fork tests: `TestShutdownDoesNotStopOtherBackends`,
`TestListenerIgnoresShutdownBroadcast`, `TestShutdownClosesListenerConnection`.
rela: `TestPostgresQueue_SurvivesAnotherProcessClosing`.

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

- The goroutine dump from Atlas showed no listener goroutine and 10 idle
  workers after a `rela-postgres` command had run.
- `TestPostgresQueue_SurvivesAnotherProcessClosing` fails on the previous neoq
  pin and passes on the fork.
- `TestShutdownDoesNotStopOtherBackends` fails on upstream c4c1564.
- `TestShutdownClosesListenerConnection` fails on the fork before the listener
  close fix.
- Fork suite passes with `-race`; rela `internal/jobs` passes with `-race`
  against Postgres.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
