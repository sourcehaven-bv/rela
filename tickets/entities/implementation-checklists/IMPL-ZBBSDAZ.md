---
id: IMPL-ZBBSDAZ
type: implementation-checklist
title: 'Implementation: Flows M0: extract the session engine into internal/flow'
started: "2026-11-05"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (internal/flow/flow_test.go, flow_unset_test.go; 93% coverage)
- [x] Integration tests written (existing realtime, realtimecfg and channels suites, all four channel backends, pass unchanged; realtimecfg/flow_test.go pins the definition against the compiled session)
- [x] Happy path implemented
- [x] Edge cases from planning handled (unset session keeps stored reminders and restarts, nil versus empty schedule maps, stranded states, paused reminders)
- [x] Error handling in place (guard errors surface as flow.GuardFailed and keep the old guardFailure path)

## Test Quality

- [x] Using fixture builders or factories for test data (review(t) definition builder in flow tests)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: pure refactor with no behavior change; covered by the unchanged realtime and appbuild suites)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: pure refactor; edge cases pinned by unit tests)

**Verification Evidence:** go test -race ./... (realtime, realtimecfg, channels
and backends, flow all ok); just arch-lint clean; golangci-lint on changed
packages 0 issues; comment-lint clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities: the engine logic now lives once in internal/flow; realtime keeps thin wrappers
- [x] No security issues introduced (no read or write path, ACL or identity change)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
