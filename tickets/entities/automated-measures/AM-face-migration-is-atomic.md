---
id: AM-face-migration-is-atomic
type: automated-measure
title: A failed face migration leaves no split family
description: Asserts migrateFaceStep.Run leaves no family split across the bare coordinate and a named face when the run fails partway, and that renameEntityTypeStep.Validate refuses a rename whose target type declares a different face set.
kind: test
location: internal/datamigration/steps_test.go
status: proposed
---

## Measure

Two assertions over `internal/datamigration`:

1. A `migrateFaceStep.Run` interrupted partway (injected store error or
cancelled context) leaves no family holding rows at both the bare coordinate and
a named face — on the backends whose `Tx` tier gives rollback.
2. `renameEntityTypeStep.Validate` refuses a rename whose target type declares a
face set that would strand rows, naming the orphaned faces.

## Why automated

Both failures are silent and only observable later, as stranded rows that no
other check reports today. The first is also timing-dependent, so it needs a
deliberate injection point rather than a chance of being caught by hand.

On fs/mem `Tx` is mutual exclusion without rollback, so assertion 1 is
backend-conditional — the harness should skip rather than assert where the tier
cannot provide it, the same way the store conformance suite gates on
capabilities.
