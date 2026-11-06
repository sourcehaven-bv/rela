---
id: BUGA-TIBLNE4
type: bug-analysis-checklist
title: 'Analysis: Relation fields: grants leave unnamed meta fields writable'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestV1UpdateRelation_MetaFieldsGrantIsAnAllowlist returned 200 for `note` and `extra` before the fix)
- [x] Minimal reproduction steps documented (role with `relations: ticket: [{relation: blocks, fields: [weight]}]` PATCHes `blocks` meta `note`)
- [x] Environment/conditions noted (any backend; data-entry PATCH, unified PATCH, relation create, conflict resolve)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (relation meta deny universe = declared meta and order properties plus grant candidates; `MetaClosed` refuses undeclared keys; role union for roles without a meta list that hold update)
- [x] Regression test planned (compiled-policy tests in internal/affordances and internal/dataentry)
- [x] Related areas checked for similar issues (relation meta `visible:` already closed-world with role union; conflict resolve keeps ungoverned undeclared keys out, as for entities)
