---
id: IMPL-YHS2D
type: implementation-checklist
title: 'Implementation: PATCH entity endpoint silently drops relations payload'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Scope widened during implementation.** Writing the e2e test surfaced that
`handleV1CreateEntity` (POST /api/v1/{plural}) has the identical bug: it decoded
only `id/properties/content` and silently dropped the `relations` payload. The
frontend's create-form path sends relations the same way, so this was producing
the same silent failure on every ticket create that used a required relation.
Fixed both handlers with the shared `reconcileOutgoingRelations` helper.

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

### Go tests (`go test ./internal/dataentry/`)

- `TestV1UpdateEntity_SavesRelations` — verified **red** before fix (`after add: outgoing implements edges = map[], want only FEAT-001`), **green** after. Covers add, multi-add, shrink-remove, empty-list-removes-all, and omitting the relations key (must leave edges untouched).
- `TestV1CreateEntity_SavesRelations` — POST with `relations` now creates the edges. Was red before the sibling fix to `handleV1CreateEntity`.
- Full package: `ok  github.com/Sourcehaven-BV/rela/internal/dataentry`.
- Full repo: `go test ./...` — all packages green.

### Playwright e2e (`npx playwright test forms.spec.ts`)

- New test `Edit Form - Default Relation Picker Save › adding a target in the picker persists after Save` drives the RelationPicker (`input[placeholder^="Search category"]` → `.dropdown-item`) and asserts persistence via the API. Green.
- All 10 `forms.spec.ts` tests pass.

### Manual repro (Puppeteer against `/Users/jeroen/Work/VWS/clean-arch-repo`)

Pre-fix: adding `PRS-ED-001` to `PRS-BF-001.afhankelijkVan` and clicking Save
returned 200 but the relation file never appeared; API readback showed the
original edges only.

Post-fix (rebuilt `bin/rela-server`): same flow persists the new edge. Relation
file appears on disk; API readback reflects the change.

### Side fixes

Three pre-existing breakages in the e2e infrastructure were blocking the new
test from running at all:

1. `fixtures.ts` imported `GraphPage` which was removed by PR #397. Removed the dangling imports/usages.
2. `isServerRunning()` treated 403 as "not running", so the Origin-missing rejection returned by the security middleware on the readiness probe caused a 30s timeout. Accepting 403 as "up".
3. The `api` fixture and the `apiPage` route interceptor made requests with no `Origin` header, which the security middleware rejected. Injected a matching Origin in both paths and added `-allowed-origin http://localhost:5173` to the server spawn.

These are infrastructure fixes, not feature work, but the ticket needed them to
land a real end-to-end test. Called out in the commit so they don't slip in
review.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
