---
id: BUGA-UNS838A
type: bug-analysis-checklist
title: 'Analysis: Relation move skips the manager''s relation meta gate'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestMoveRelation_FieldGateJudgesOrderKey: a field-gated `UpdateRelation` with a `Position` succeeded while the gate refused the order key)
- [x] Minimal reproduction steps documented (field-gated manager, orderable relation, gate refusing `_order_out` for the source; `UpdateRelation(key, {Position: {Step: 1}})` writes the order)
- [x] Environment/conditions noted (any backend; the field-gated manager handle; the data-entry handler already checks the order key before it calls the manager, which TestRelationPosition_FieldsGrantRealPolicy pins against a compiled policy)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (`moveRelation` runs `checkRelationMetaWrite` on the order key for the moved edge's source tail and, on the incoming side, for each other source tail whose edge the move may rewrite)
- [x] Regression test planned (manager test with a source-scoped gate; data-entry test against a compiled `fields:` policy)
- [x] Related areas checked for similar issues (renumber after an explicit order write runs after `checkRelationMetaUpdate` gated the key; create assigns order on a new edge, which a create grant covers)
