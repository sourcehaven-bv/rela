---
id: AM-migration-delta-kinds-have-resolving-steps
type: automated-measure
title: Every TierMigration delta kind is resolvable by at least one migration step kind
description: Guard test asserting every TierMigration delta kind maps to at least one step kind that can resolve it, so detection can never ship without remediation.
kind: test
location: internal/datamigration/file.go (resolvingSteps + validateDeltasResolved), TestResolvingSteps_CoversEveryMigrationDeltaKind; internal/metamodel/shapecompare_kinds_test.go (TestMigrationDeltaKinds_MatchesTheClassifier)
status: active
---

A guard over two lists: the delta kinds `CompareShapes` can raise at
`TierMigration`, and the step kinds `internal/datamigration` can execute. Every
delta kind classified as needing a migration must appear in `resolvingSteps`,
either naming the steps that resolve it or carrying an empty value and a written
reason.

This is the check that would have caught BUG-TMGWIN. `bare_face_introduced`
shipped as a `TierMigration` delta whose message instructs the operator to
"confirm with a migration", while no step kind could assign a row to a face.
Detection and remediation were maintained as separate lists, so the gap was
invisible: each side looked complete on its own.

## As implemented

Three parts, because a single table would have been trusted more than it
deserved:

1. **`resolvingSteps`** (`internal/datamigration/file.go`) maps each kind to the
step kinds that answer it. An empty value is a reviewed exemption with the
reason in a comment, not an omission.
2. **`validateDeltasResolved`** enforces the non-empty entries at parse time,
recomputing the deltas from the file's own embedded projections. This is what
turns "a step exists" into "the file must use it" — the distinction the original
fix missed.
3. **`TestMigrationDeltaKinds_MatchesTheClassifier`** scans `shapecompare.go`
for the kinds it actually raises and fails when `metamodel.MigrationDeltaKinds`
disagrees. Without it the table could silently fall behind the classifier, which
is the same failure one level up. The test fails loudly if its own scan pattern
stops matching, so it cannot go quietly vacuous.

Enforced today: `bare_face_introduced` → `confirm_face`. Exempted with reasons:
`bare_face_changed` (TKT-L3P8I6), `bare_face_removed` (TKT-1YBNQJ), and the
property/relation kinds, which are answered by a step the operator chooses
between or by hand — requiring "any step at all" there would be a check a
well-formed no-op passes.
