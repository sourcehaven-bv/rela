---
id: IMPL-3GTDBS
type: implementation-checklist
title: 'Implementation: Cut CI wall clock: de-serialize the build tail and fix Go cache thrash'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: CI workflow configuration, no application code)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the CI run itself is the integration test; verified on run 35272630661)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no test data)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no assertions)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no test code)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no test code)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no test code)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Run 35272630661 on branch `ci-speedup-tier1`, all jobs green except the ticket
gate (which this ticket exists to satisfy).

- **Wall clock**: 875s and 841s on the two baseline runs → **530s**. A 39% cut,
measured on a *cold* cache, so this is the floor rather than the steady state.
- **Tail de-serialized**: `Docs` completed 75s into the run. On the baseline it
started at 08:34 into a 14:35 run, because it waited on `Build`, which waited on
all ten other jobs. `Demos` likewise now runs concurrently.
- **Cache thrash fixed**: seven distinct caches written with zero `Unable to
reserve` failures. Sizes track each job's real build graph — 294-299MB for the
three cross-compile variants, 156MB arch-lint, 10MB comment-lint. Under the old
single key those same jobs overwrote each other, which is exactly the defect: a
10MB entry restored into a 294MB job is a cache miss with extra steps.
- **Job count**: 24 → 23, confirming `Build` is gone.
- **Cache steps**: present and succeeding in every Go job; the tool `Install`
steps still ran this time, correct on a cold cache since they are gated on
`cache-hit != 'true'`.
- **No new lint findings**: `actionlint` reports 20 findings on both the base
commit and this branch, all pre-existing (a stale runner-label list for
`ubuntu-26.04`, and SC2251 info on a pre-existing script).

Not yet observable: the warm-cache steady state, which needs a second run on the
same `go.sum`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — the 15 cache blocks are deliberately
per-job rather than factored into a composite action, since the whole point is
that each key differs; a shared action would need the job name passed in anyway
- [x] No security issues introduced — no new third-party actions, no untrusted
input interpolated into any `run:` block. The deferred apt-caching item was left
undone specifically because it would have required adding one.
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
