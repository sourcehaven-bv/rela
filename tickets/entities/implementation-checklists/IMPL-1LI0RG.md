---
id: IMPL-1LI0RG
type: implementation-checklist
title: 'Implementation: Automation action that enqueues a Lua script as a background job'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (automationjob_test.go, updated_trigger_test.go, background_test.go, automationjobs_internal_test.go)
- [x] Integration tests written (test full flow, not just units) (automationjobs_test.go: foreground, queue mode, rename, rename on stored values, identity with and without grants)
- [x] Happy path implemented
- [x] Edge cases from planning handled (coalescing, saves during a run, collapsed save follow-up, self-trigger, hop limit, rename, stale payload, missing grants)
- [x] Error handling in place (errors surfaced, not swallowed) (enqueue and foreground failures become automation errors on the save; job errors go to retry)

## Test Quality

- [x] Using fixture builders or factories for test data (writeBackgroundSchema, createNoteWith, jobsMeta, newTestJobs)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (principal.UserAutomation, maxAutomationJobHops)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Temp project with `on: {created, updated}` and a background `push.lua` that
writes `pushed = principal:title`, run with a dev build of `rela`:

- `rela create note -P title=one`, then `rela update ... -P title=two`:
`pushed: system:automation:two`. The job ran as system:automation, in the
foreground, and its own write did not loop.
- Removed `pushed`, then `rela rename id NOTE-BYN2 NOTE-RNM`: the push ran
again for the new id.
- `background: false` under `on.updated`: load refused with "a script under
`on.updated` must be `background: true`".
- `retry: always`: load refused (earlier run).
- Queue mode, rename, raw trigger evaluation, grants and loops are covered
by the integration tests; three were mutation-checked.

## Quality

- [x] Code follows project patterns (check similar code) (consumer-side BackgroundScripts interface; jobs seam with Retry intent and IdempotencyKey; AliasRewriter fanout for rename)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (security review: two significant and two minor, all addressed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
