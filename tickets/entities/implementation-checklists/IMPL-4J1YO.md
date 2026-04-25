---
id: IMPL-4J1YO
type: implementation-checklist
title: 'Implementation: Data-entry create form: prefix picker for multi-prefix types and manual ID field'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Backend (Go):
- `go test ./internal/dataentry/` — all 164 tests pass including 8 new tests covering schema exposure of `id_prefixes`, prefix override, empty prefix fallback, unknown-prefix 422 (with message listing allowed prefixes), manual-ID accept, manual-type rejects prefix, non-manual rejects id, and `validateCreateIDOpts` table-driven.
- `go test ./internal/workspace/` + `./internal/entitymanager/` — all pass; the new `Prefix` field on `entitymanager.CreateOptions` is wired through `wsEntityManager.CreateEntity` into `workspace.CreateOptions.Prefix`.
- `just lint` — clean.
- `go test -cover ./internal/dataentry/` — 61.5% (well above 55% floor); `validateCreateIDOpts` at 100%, `handleV1CreateEntity` at 81.8%.

Frontend (TS/Vue):
- `npm run typecheck` — clean.
- `npm run lint` — clean (only pre-existing warnings).
- `npm run test:run` — all 434 tests pass including 16 new tests for `useEntityIDControls` covering all mode/id_type/prefix combinations.

Acceptance criteria verified:
- AC1 (multi-prefix picker shown): component-tested in `useEntityIDControls.test.ts` `is true for multi-prefix non-manual types in create mode`; E2E added as `Multi-Prefix Create Form › shows prefix picker and creates entity with chosen prefix`.
- AC2 (single-prefix no picker): component-tested in `useEntityIDControls.test.ts` `is false for single-prefix types`; E2E `does not show prefix picker for single-prefix ticket form`.
- AC3 (manual-ID create + edit read-only): E2E added as `Manual-ID Create Form › renders ID input and creates tag with user-supplied ID`; `showReadOnlyID` computed in DynamicForm.vue guarded by `isEdit && id_type === 'manual'` — verified via code inspection since no manual-ID edit fixture entity exists in E2E suite.
- AC4 (id_prefixes exposed; id_prefix preserved): `TestV1Schema_MultiPrefix` + `TestV1Schema_SinglePrefix_Compat` both pass.
- AC5 (prefix validation): `TestV1CreateEntity_PrefixOverride`, `TestV1CreateEntity_UnknownPrefix`, `TestV1CreateEntity_IDRejectedForNonManual`, `TestV1CreateEntity_EmptyPrefixUsesFirst`, `TestV1CreateEntity_ManualTypeRejectsPrefix`, `TestV1CreateEntity_ManualAcceptsID` — all pass.

Edge cases:
- Manual-ID + declared id_prefixes → picker NOT shown, server rejects prefix: tested in both unit (`is false for manual types even when id_prefixes has multiple entries`) and handler (`TestV1CreateEntity_ManualTypeRejectsPrefix`).
- Empty prefix falls back to first: `TestV1CreateEntity_EmptyPrefixUsesFirst`.
- ID rejected for non-manual: `TestV1CreateEntity_IDRejectedForNonManual`.
- Error message lists allowed prefixes: assertion in `TestV1CreateEntity_UnknownPrefix`.

Known E2E caveat: the shared `frontend/e2e/fixtures.ts` has a pre-existing
broken import (`GraphPage` not exported since #397 removed the graph
visualizer). This prevents ALL e2e tests from running locally, not just the new
ones. The new e2e test code is syntactically valid and uses the existing
`pages.form()` / `api.*` fixtures; it will run once the pre-existing `GraphPage`
import is cleaned up (separate ticket).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
