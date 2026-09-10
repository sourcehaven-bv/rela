---
id: AM-migration-delta-kinds-have-resolving-steps
type: automated-measure
title: Every TierMigration delta kind is resolvable by at least one migration step kind
description: Guard test asserting every TierMigration delta kind maps to at least one step kind that can resolve it, so detection can never ship without remediation.
kind: test
location: internal/datamigration (guard test over metamodel TierMigration delta kinds and the steps.go step-kind list)
status: proposed
---

A guard test over two lists: the delta kinds `CompareShapes` can raise at
`TierMigration`, and the step kinds `internal/datamigration` can execute. Every
delta kind classified as needing a migration must map to at least one step kind
capable of resolving it, with the mapping declared explicitly rather than
inferred.

This is the check that would have caught BUG-TMGWIN. `bare_face_introduced`
shipped as a `TierMigration` delta whose message instructs the operator to
"confirm with a migration", while no step kind could assign a row to a face.
Detection and remediation were maintained as separate lists, so the gap was
invisible: each side looked complete on its own.

The failure mode it prevents is a gate that demands an operator action the
system provides no primitive to perform. That leaves the operator with only bad
options, and the most natural one is to write a migration that declares the
right `to` hash while doing nothing, which silently satisfies the gate.

An explicit mapping also documents, for each delta kind, which step answers it.
A new delta kind must then either name its resolving step or be deliberately
exempted.
