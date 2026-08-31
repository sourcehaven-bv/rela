---
id: IMPL-0NYT04
type: implementation-checklist
title: 'Implementation: testIdempotencyFreed races the queue completion bookkeeping'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Test-only. No production code changed — the queue behaviour is correct; the
test's synchronisation was not.

The second `Enqueue` now runs inside `require.Eventually` until it succeeds,
instead of once immediately after the handler has started. The comment records
why, because the previous shape looked careful: `require.Eventually` on the
counter reads as proper async handling, so nobody asks whether it waits for the
RIGHT event.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Same helpers, same timeouts, same shape. The change is one call site.

Two alternatives were explicitly rejected and the comment says so, since both
are the obvious next edit:

- `time.Sleep` before the re-enqueue — trades a visible flake for a slow
invisible one, and encodes a guess about a machine.
- ignoring `ErrDuplicateJob` — that error IS the failure this test exists to
catch, so swallowing it would delete the test while leaving it green.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| step | expected | observed |
| --- | --- | --- |
| `GOMAXPROCS=1 ... -count=20` BEFORE the fix | fails | FAIL, `an identical job is already pending` |
| same command AFTER the fix | passes | ok (2.1s) |
| block the handler so the job never completes | still FAILS | FAIL after the full 10s settleTimeout |
| `./internal/jobs/... -race` | green | ok (69s) |

The third row is the one that matters. A polling fix is only worth having if it
cannot pass when the property genuinely does not hold — otherwise it silences
the flake by deleting the assertion. Holding the handler open so the key is
never freed still fails, so the test retained its teeth.

The first two rows use the reproduction that made the diagnosis possible.
`GOMAXPROCS=1` turns an intermittent CI failure into a deterministic local one,
which is what moved this from "flake, rerun it" to a fixable defect.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Uses the file's existing `settleTimeout` / `pollInterval` constants rather than
introducing a bespoke timeout, so the whole suite still tunes from one place.

DRY: nothing to extract — one call site changed.

Worth recording how this was reached, because the first conclusion was wrong. I
initially called it a flake and re-ran CI, on the strength of: the diff not
touching `internal/jobs`, both sibling branches passing, and 5 consecutive local
`-race` runs plus CI's exact `-shuffle` seed all green. The rerun failed
identically, which disproved it.

What the flake hypothesis never explained was why it failed TWICE on one branch
and zero times elsewhere. "Flaky" is a hypothesis that predicts randomness; two
for two is not random. Reaching for load rather than ordering — `GOMAXPROCS=1`
instead of another seed — made it deterministic in one run.
