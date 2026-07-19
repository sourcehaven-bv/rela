---
id: IMPL-RAIF9B
type: implementation-checklist
title: 'Implementation: Badge colors never resolve when a property''s name differs from its custom-type name'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Changes (frontend-only, as planned):

- `frontend/src/stores/schema.ts` — added `customTypeNameForProperty` (mirrors `labelsForProperty`'s resolution order and first-wins tie-break) and the exported `stylesForProperty(property, entityType?)` getter: type-keyed lookup first, property-name fallback.
- `frontend/src/components/common/Badge.vue` — `badgeClass` resolves via `schemaStore.stylesForProperty(props.property, props.entityType)`; value normalization and gray fallback unchanged.
- Tests: `schema.test.ts` +6 cases (type-key resolution, type-over-property precedence, property fallback, entityType disambiguation + documented first-wins tie-break, relation-type properties, unstyled → undefined); `Badge.test.ts` +4 cases (property ≠ type renders configured class, type key wins, entityType disambiguation, value normalization through the type key). The Badge cases are component-level integration: realistic type-keyed store state → rendered CSS class.

Edge cases: no `property` prop → gray (unchanged); property without custom type
(inline enum) → property-name fallback; same property name with different types
across entity types → `entityType` disambiguates, else documented first-wins. No
error paths added (pure lookup; miss → gray is the designed answer).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Followed the file's existing conventions (`withCustomType()` helper mirroring
`withInlineLabel()`; minimal store state per case).

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built the SPA (`npm run build`) into the embedded bundle, built `rela-server`,
ran it against the in-tree `tickets/` project (property `status` / type
`ticket_status` — the repro case). `/api/v1/_config` confirmed `styles` keyed
only by type names. Browser (puppeteer) on `/list/all_tickets`: 94 badges —
badge--blue 17, badge--orange 23, badge--yellow 15, badge--purple 12,
badge--green 22, badge--gray 5 (the grays are the *configured* `backlog: gray` /
`wont-fix: gray` values). Screenshot verified visually: kind=Enhancement blue,
status Done green, priority High orange, effort M yellow — matching
`tickets/data-entry.yaml` `styles:`. Pre-fix these all rendered gray.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — resolution deliberately mirrors `labelsForProperty` rather than merging with it: labels resolve from per-def data (def.labels / ct.labels), styles from the flat server map — the two mechanisms the schema.ts comment already documents as separate. Shared `fromDef` shape kept, predicate differs.
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`npm run test:run` 1339/1339 pass, `npm run typecheck` clean, `npm run lint` 0
errors (89 pre-existing warnings).
