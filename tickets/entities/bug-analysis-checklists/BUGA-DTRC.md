---
id: BUGA-DTRC
type: bug-analysis-checklist
title: 'Analysis: v1 entity create bypasses field-affordance write gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally — a `fields:` policy denying a field (read-only/hidden) or filtering an enum option is enforced on PATCH but not on POST create; `handleV1CreateEntity` skips `validateFieldWrite`.
- [x] Minimal reproduction steps documented — POST `/api/v1/<type>` with a body setting a denied field; pre-fix returns 201 with the value persisted (PATCH of the same field returns 403).
- [x] Environment/conditions noted — any non-Nop field resolver (Demo or policy-backed) with a field denial; reproduced via unit test against a stub resolver.

## Root Cause

- [x] Immediate cause identified (why1) — create handler never calls the field-affordance gate.
- [x] Contributing factors found (why2-3) — gate was wired only into PATCH/serialize when added; no shared enforcement seam between create and update.
- [x] Systemic cause explored (why4-5) — no contract test enumerating write entry points; per-handler duplication lets a path silently skip the gate.

## Fix Planning

- [x] Fix approach determined — run `validateFieldWrite` against a candidate entity (type + proposed properties, no ID) before `CreateEntity`; reuse the existing gate so wire 403 + rule_id match PATCH. Relation/entity-scoped predicates fail closed for the unpersisted entity (safe direction).
- [x] Regression test planned — `TestHandleV1CreateEntity_FieldAffordances` (hidden/unknown/read-only/enum + allowed) and `_AffordanceDenial_EmitsAudit`; tracked as MEAS-create-field-affordance-test.
- [x] Related areas checked for similar issues — confirmed PATCH (`handleV1UpdateEntity`) and relation-write paths already gate; SPA create form lacks an affordance source (no `_fields` for an unsaved entity) → filed TKT-3I5U as a follow-up for the create-form UX gating (out of scope for the server bypass fix).
