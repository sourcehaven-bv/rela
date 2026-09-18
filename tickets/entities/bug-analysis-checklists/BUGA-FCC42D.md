---
id: BUGA-FCC42D
type: bug-analysis-checklist
title: 'Analysis: Relation edits silently dropped when a linked entity is outside the picker''s first-100 candidate window'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (unit-level, `RelationPicker.test.ts` — three failing assertions with a fixture where the linked id is outside the candidate page)
- [x] Minimal reproduction steps documented (see bug body)
- [x] Environment/conditions noted: requires >100 entities of the relation's target type; picker widget only, `cards` is immune

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined: resolve types for pre-existing values from `getEntityRelations` (which carries `type` per edge — the same source that makes `cards` immune), merge into the emitted map, and keep resolved entities in a lookup so `selectedEntities` can render out-of-page links
- [x] Regression test planned: `RelationPicker.test.ts` → "pre-existing value outside the candidate page (BUG-LSCDJK)", written before the fix and confirmed failing
- [x] Related areas checked for similar issues: `RelationCards` unaffected (types arrive with each edge); `reshapeLegacyToModern`'s null-on-unknown is the correct defence and stays; `emitIncomingDiff` has the same shape (RR-WY3CO0, previously deferred as latent) and is covered by the same fix direction
