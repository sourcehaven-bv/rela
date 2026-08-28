---
id: IMPL-FD681W
type: implementation-checklist
title: 'Implementation: rela migrate strips form labels the SPA cannot re-derive, permanently downgrading them to raw property/relation ids'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: this change only removes derivation logic and reads config values; no new error paths introduced)

Removed all 12 identifier→label derivations. Go: `dataentry/helpers.go`,
`migration/dataentry_cleanup.go`, `lua/flow.go`, `cmd/rela-desktop/main.go`,
`metamodel/types.go` (`camelCaseToSpaced`), `cli/graph.go` (found by the lint
test during review). Frontend: `FilterBar`, `AdHocFilterMenu`, `SearchView`,
`RelationCards.formatLabel`, `InlineCreateModal.formatLabel`,
`HistoryView.propertyLabel`, `RelationHistoryView.propertyLabel`,
`DocumentsPanel`, `DocumentView`.

The one kept strip (relation label == metamodel `RelationDef.Label`) is now
genuinely recovered by `RelationPicker`/`RelationCards` via
`relationType.label`, with direction-awareness added in review (RR-GW9711).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

New Go tests reuse the existing `testViewApp()` / `mockMetamodel` harnesses
rather than hand-rolling fixtures. Frontend tests reuse `seedCandidates` and the
existing mount helpers.

Test discrimination was verified, not assumed: the reviewer mutation-tested
`TestRelationLabelStrippedOnlyWhenMetamodelSupplied` by restoring the old
`titleCase` arm, and the subtest fails — so it is discriminating, not vacuous.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Reproduced the bug report's exact scenario against the fixed code (temporary
test, since removed):

```text
server refuses to start (Detect)? false
after rela migrate:
      - property: titel
        label: "Titel"              <- survives
      - property: inschrijfdeadline
        label: "Inschrijfdeadline"  <- survives
      - property: laatste_contact
        label: "Laatste Contact"    <- survives
```

Both halves of the original defect are gone: the server boots, and the Dutch
labels survive the migration.

Edge cases verified:

- **Empty `entDef.Label`** — `GetLabelPlural()` returns a bare `"s"`. Caught in
the scaffolder (a test fixture without a label produced an empty title), fixed
with an explicit raw-type-name fallback, and the same trap avoided in
`cli/graph.go` by guarding on `Label` rather than calling `GetLabelPlural()`.
- **Lua flow empty label** — `Field.Name` is validated non-empty and
length-capped before the fallback, so `makeRequiredValidator` can never emit `"
is required"` with a blank subject.
- **Kebab-case divergence** — confirmed empirically pre-fix (Go `Kebab Name` vs
JS `Kebab-Name`); now moot since neither side derives.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: the review flagged `repoRoot` duplicating `findRepoRoot`. Resolved by
moving the lint test into `package migration` so it reuses the existing helper
rather than carrying a second copy — the same "second copy is how the last
eleven started" reasoning that motivates this bug.

All scratch/probe files removed; `git status` verified clean of them.
