---
id: IMPL-W6T22R9
type: implementation-checklist
title: 'Implementation: Flows M4a: parallel branches'
started: "2026-11-11"
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

- Unit: `internal/flowcfg/forks_test.go` (fork and join parsing, every
  load refusal), `internal/flowengine/branches_test.go` (start, join all,
  any, when, final branch, timers, pending outputs, per-branch claims,
  cancel, backoff, loop, one run read per page), flowruntest conformance
  on kv, sql, sqlite and postgres with a reflection check that the
  fixture covers every field.
- Integration: `internal/appbuild/flowsteps_fork_test.go` drives a forked
  run through FlowControl on disk: per-branch places, diagram marks both
  states, tasks per branch, a step refused outside the branch's writes
  and at the run's own generation, the join firing after the last step.
- Frontend: vitest for the branch cards (step form posts the branch
  generation, arrived branch has no form) and the timeline (current entry
  per live branch, join entry).
- Manual: built `rela` and, in a scratch project with the fork flow,
  ran `rela flow validate` (valid), `rela flow show hire` (both fork
  edges labelled "(parallel)"), `rela flow start hire PER-001` and
  `rela flow runs` (run at `joined`, `input_required`).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
