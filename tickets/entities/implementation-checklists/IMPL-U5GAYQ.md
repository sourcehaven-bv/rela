---
id: IMPL-U5GAYQ
type: implementation-checklist
title: 'Implementation: Sessions 4/5b: timer and trigger outputs (run_as)'
started: "2026-10-26"
completed: "2026-10-26"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `principal` `TestValidJobIdentity`; `acl` `TestJobIdentityGrants*`; `realtimecfg` run_as validation, job templates, trigger outputs, `TestRunAsGrantCheck*`, `TestJobLoops`; `realtime` timer/trigger mark, runner, run_as_changed and transition_removed tests
- [x] Integration tests written (test full flow, not just units): `SetSessionWithMark` and crash-recovery conformance on all four channel stores; `TestOutputs_TimerSetAsRunAs`, `TestOutputs_TriggerMoveWritesAsRunAs`, `TestOutputs_RunAsWriteReauthorized` against the real entitymanager; `TestRealtimeWorker_MarksWakeOutputs`; `rela validate` grant check through `ValidateCmd.Run`; `TestDocsExample_Realtime`
- [x] Happy path implemented: timer- and trigger-fired transitions run their outputs as the session type's `run_as`, which `acl.yaml` must grant at load
- [x] Edge cases from planning handled: faced and unfaced anchors, union-rule field grants, strict load everywhere, run_as changed or removed, transition no longer job-reachable, legacy system:realtime marks, final-state close-only marks, timer+trigger loops with outputs, file-tier crash window
- [x] Error handling in place (errors surfaced, not swallowed): load problems name the missing grant; runtime failures recorded as `forbidden`, `field_denied`, `run_as_changed`, `transition_removed`

## Test Quality

- [x] Using fixture builders or factories for test data: conformance harness, testdata policy with `system:breach-bot`, injected clock
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no UI; the worker and writer tests run the assembled flow)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-ZU5X29
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build, vet and golangci-lint on default,
memorybackend, postgres, sqlite and maildemo,mailmanual; `go test -race` on
touched packages; sqlite- and postgres-tag tests with
`RELA_TEST_DATABASE_REQUIRED=1` (only the pre-existing TestGoldenParity pile
goldens fail); arch-lint, plimsoll, comment-lint clean; markdownlint clean;
frontend vitest 4785, vue-tsc, eslint; `just coverage-check` 85.1%; `just docs`
idempotent.

## Quality

- [x] Code follows project patterns (check similar code): 5a outputs runner and writer, acl grant helpers, conformance harness, timer loop checks
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one `acl.JobGrants` type for all load
checks; shared `principal.ValidJobIdentity`
- [x] No security issues introduced: rela-security-reviewer findings (3 minor) addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
