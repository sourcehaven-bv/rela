---
id: RR-KC1UI
type: review-response
title: 410-line handler is too long; extract diff classifier
finding: |-
    handleV1UpdateEntity runs 484-785, doing five things: clone+apply, diff, propagate, validate, stage. Extracting `applyRelationDiff(tx, req, entityID, meta) (relationDiff, error)` would let the diff classifier be unit-testable without HTTP harness. Currently every diff edge case requires the full handler harness.

    Fix: extract the diff/propagate/validate inner loop to a helper. Bonus: the helpers C1, S5, S7 all become trivial when the diff is a first-class value.
severity: minor
resolution: Diff classifier extracted to computeDiff(tx, meta, entityID, fromType, relations, diff). Propagation extracted to propagateRelations(tx, meta, diff). Validation of staged edges extracted to validateStagedEdges(tx, meta, edges). Handler is now ~150 lines instead of 410. Each helper is independently testable.
status: addressed
---
