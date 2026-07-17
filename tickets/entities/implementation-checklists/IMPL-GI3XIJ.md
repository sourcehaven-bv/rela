---
id: IMPL-GI3XIJ
type: implementation-checklist
title: 'Implementation: Selection relation not saved when creating an entity in data-entry (edit works)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`RelationPicker.test.ts` — incoming-on-create emits)
- [x] Integration tests written (e2e `reverse-relations.spec.ts` — create form incoming picker persists the edge)
- [x] Happy path implemented (`RelationPicker.loadIncomingValue` establishes an empty loaded baseline on create)
- [x] Edge cases handled (add then remove on create emits an empty desired set; covered by unit test)
- [x] ~~Error handling in place~~ (N/A: fix removes a silent no-op; no new error surface. Pre-existing inverse pre-flight in DynamicForm still guards missing-inverse relations)

## Test Quality

- [x] Using fixtures/factories (unit uses the existing `entity()`/`seedCandidates` helpers; e2e uses the seeded `feature`/`task` fixtures + `SEED` constants)
- [x] No hardcoded values where an object is in scope (e2e asserts against `created.id` from the create response)
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects
- [x] Property comparisons use the original object (`created.id`, `SEED.tasks.refactorAuth`)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:** Built `rela-server` with the fix, ran the real SPA
(Puppeteer) against a live project. On the create form with both an outgoing and
an incoming `blocks` picker:
- Captured POST body BEFORE fix: `{"relations":{"blocks":{"data":[{"type":"thing","id":"THG-100"}]}}}` — incoming selection dropped.
- Captured POST body AFTER fix: `{"relations":{"blocks":{...THG-100...},"blockedBy":{"data":[{"type":"thing","id":"THG-101"}]}}}` — incoming selection sent under the inverse key.
- On-disk edges after create: both `THG-103--blocks--THG-100.md` (outgoing) and `THG-101--blocks--THG-103.md` (incoming) present; API `?direction=incoming` lists THG-101.
Both the unit test and the e2e test were confirmed to FAIL without the fix (git
stash) and PASS with it.

## Quality

- [x] Code follows project patterns (mirrors the existing early-return/guard style in `loadIncomingValue`; comment cites BUG-10IPBP + TKT-GFQK)
- [x] ~~DRY opportunities~~ (N/A: single localized guard; no repetition introduced)
- [x] No security issues introduced
- [x] No silent failures (the fix specifically eliminates a silent no-op)
- [x] No debug code left behind
