---
id: BUG-7EDVI4C
type: bug
title: 'E2E flakes: contract test SSE windows and form page helpers'
description: The fake-vs-server contract test compares entity:changed events between quiet points but takes its marks by timing instead of by a known server event so events from the per-run reset or from a late batch land in the wrong window and fail the property in CI.
priority: high
effort: s
why1: 'The server sent an entity:changed in the first window although no write in it succeeded: the batch of the per-run reset reached the event stream opened after it.'
why2: The server counts store changes in a goroutine and flushes them in 200ms batches, so a write can reach a stream after its response returned; the test assumed the reset was done once its requests answered.
why3: The test synchronised with the server by timing (sleep one window, then wait until the server had as many events as the fake), and that check passes at once when the fake has none.
why4: The server offers no event that marks 'everything so far has been sent', and the test did not create one, so a delay stood in for it.
why5: 'Tests that compare an asynchronous stream use time as a proxy for causality. Playwright helpers in e2e/pages do the same: count() and a 100ms spinner check stand in for waiting on the element the assertion needs.'
prevention: Synchronise on an event the server owes (the marker flush) instead of a delay, and in page objects wait for the element before reading or counting it. The contract and Playwright tests keep running in the E2E job.
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

## Description

The E2E job has flaky tests. The contract test fails the step outright and
ejects unrelated PRs from the merge queue. The Playwright flakes pass on retry
but show up as `flaky` in most runs.

## Flakes fixed here

### Contract test, `entity:changed between quiet points 0 and 1`

Runs 41823250597 (job 126551413581, seed 516079759) and 41598355667 (develop
push, job 125790953297, seed -504315250). In both counterexamples no write
before the first quiet point succeeds (a create with NUL in `summary`, a delete
of a missing id), yet the server sent an `entity:changed` for `note`.

Cause: the server counts store changes in a goroutine (changefeed pump) and
reports them in 200ms batches. The per-run reset writes to the server just
before the test opens its event stream; the reset's batch can reach the new
stream and was counted in the first window. The quiet points had the same race:
they slept one window and then accepted the server as caught up whenever it had
as many events as the fake, which holds trivially when the fake has none.

Fix: a `marker` entity type in the contract fixture. `sync()` writes a marker
and waits for the end of the flush that reports it. The store emits changes in
write order, so that flush reports or follows every earlier write and no later
write can join it. The first mark, each quiet point and the final mark use it.
No sleeps remain. `CONTRACT_SEED`/`CONTRACT_PATH` replay a reported run.

Reproduction: opening the stream before the reset (what a late pump does) fails
the old test deterministically with seed 516079759; the fixed test passes the
same forced run.

### forms.spec.ts:45 "form blocks POST when required fields are missing"

The create form POSTs `/api/v1/features?dry_run=true` on mount to learn field
affordances. The test counted every POST to `/api/v1/features` as a create, so a
dry-run answered after the listener was attached failed it. Forced by delaying
dry-run responses 1.5s: old test fails 3/3 with the CI message, the fixed one
passes. Fix: ignore `dry_run` requests.

### forms.spec.ts:119 and wizard.spec.ts:80

`waitForSpinnerToDisappear` returns at once when the spinner never showed, and
the edit form and wizard show a placeholder until they load. `getSelectOptions`
and `activeStepIndex` then counted elements that did not exist yet (0 options,
step -1). Fix: getSelectOptions waits for the select and its first option; the
wizard specs assert the active step with FormPage.expectActiveStep, which
retries.

## Not a flake: separate bug

presence.spec.ts:60 loses a lease change on the live view. Filed as BUG-33Q36BI.

## Triage of the other runs

- 41718755255 (TKT-VXBE16H-channel-placement): view sections lacked
`source` in the fake. Deterministic on that branch. Not a flake.
- 41720361762 (same branch): e2e lint errors. Not a flake.
- 41782926104, 41781881280 (claude/next-actions-completion): worlds manual
assertion at manual:851 failed in both. Real bug on that branch.
- 41720627106 (claude/payload-cms-rela-gaps): view-section-widget-override
failed on every retry. Real bug on that branch.
