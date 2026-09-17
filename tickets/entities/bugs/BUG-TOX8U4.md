---
id: BUG-TOX8U4
type: bug
title: Face migration is not atomic and rename does not validate face sets
description: Two related gaps found while reviewing TKT-Z4L0IU. migrateFaceStep.Run moves rows in an untransacted loop, so a crash mid-run leaves a family split across the bare coordinate and named faces. renameEntityTypeStep.Validate does not compare the two types' face sets, so a rename into a type declaring different faces can strand rows. Both produce the mixed state that other checks then fail to report.
priority: medium
status: backlog
---

## Description

Two gaps in `internal/datamigration/steps.go`, surfaced by a review of
TKT-Z4L0IU. Filed together because they produce the same end state and share a
fix strategy.

**1. `migrateFaceStep.Run` moves rows in an untransacted loop.** It collects
`moves`, then applies them one at a time. A crash, cancellation or store error
partway through leaves the family split: some rows on the target face, the rest
still at the bare coordinate. The step is documented as idempotent and
re-runnable (`!e.Face.IsDefault()` skips rows already moved), so a re-run does
recover — but only if the operator knows to re-run, and `rela analyze` currently
cannot tell them (BUG-UA3BK3).

**2. `renameEntityTypeStep.Validate` does not compare face sets.** Renaming a
type into one that declares different faces can strand every row whose face the
target type does not declare. Validate is the right place to refuse this, since
the step's own precondition check is what the operator is relying on.

## Why together

Both end at the same place: a family holding rows at coordinates the type no
longer declares. That is the mixed state BUG-UA3BK3 makes invisible, and the
state the `otherwise:` follow-up would change behaviour for. Fixing detection
without fixing production means the operator sees a real finding they cannot
explain.

## Suspected fix

For (1), wrap the move loop in `store.Store.Tx` — this is exactly the sanctioned
transaction seam (DEC-8UIL0), and the moves are local writes with no external
I/O, so the "don't do slow I/O inside Tx" constraint is satisfied. Note fs/mem
give mutual exclusion only, not rollback, so on those tiers the fix is partial
and idempotent re-run remains the recovery path; sqlite and postgres get real
atomicity.

For (2), compare `def.Faces` between source and target in `Validate` and refuse
a rename that would strand rows, naming the faces that would be orphaned.

## Verification note

Not independently reproduced — filed from a code review of an adjacent change.
Confirm both before fixing; (1) in particular should be checked against what the
step already guarantees on re-run, since the idempotency may make the practical
impact smaller than the shape suggests.
