---
id: BUGA-8ITGYM
type: bug-analysis-checklist
title: 'Analysis: rela migrate strips form labels the SPA cannot re-derive, permanently downgrading them to raw property/relation ids'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

**Both halves reproduced empirically, not inferred.**

*Go side* — temporary test against the real `migration.titleCase`:

```text
go titleCase("titel")             = "Titel"
go titleCase("laatste_contact")   = "Laatste Contact"
go titleCase("inschrijfdeadline") = "Inschrijfdeadline"
go titleCase("kebab-name")        = "Kebab Name"
```

So `label: 'Titel'` on property `titel` satisfies `isRedundantLabel` and is
deleted.

*SPA side* — temporary Vitest mount of `FieldRenderer` with the label omitted:

```text
FAIL  repro: label fallback > renders raw property when label omitted
AssertionError: expected 'laatste_contact' to be 'Laatste Contact'
Expected: "Laatste Contact"
Received: "laatste_contact"
```

The migration deletes `Laatste Contact`; the renderer produces
`laatste_contact`. Contract broken end-to-end.

*Kebab divergence* also confirmed — same input, two answers:

```text
Go: titleCase("kebab-name") = "Kebab Name"
JS: titleCase("kebab-name") = "Kebab-Name"
```

Both scratch files were removed; working tree verified clean.

**Steps:** set `label: 'Titel'` on a form field for property `titel` in
`data-entry.yaml` → run `rela migrate` → load the form → renders `titel`.

**Conditions:** built from 8cce2733; reported against a live customer project.
Not avoidable by declining the migration — `internal/dataentry/app.go:631-638`
runs `migration.DetectBytes` in `NewApp` and returns a `migration.Error` instead
of starting, so the server refuses to boot while the label is present.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded as `why1`–`why5` on BUG-8N2WT2. Summary of the chain: a raw fallback in
the renderer (why1); a cross-language contract with no shared definition and no
test (why2); the fallback branch never executed in CI (why3); a category error
that applied a metamodel-redundancy predicate to a convention-redundancy case
(why4); and eleven duplicated `titleCase` implementations that make the contract
unenforceable by construction (why5).

The why4 distinction is the useful one. Every *other* check in this migration is
grounded in the metamodel and re-derived **server-side**, where it is
verifiable:

| Check | Grounded in | Re-derived by |
|---|---|---|
| `isRedundantWidget` | `ResolveWidgetFromType` | server |
| `isRedundantRequired` | `IsPropertyRequired` | server |
| `isRedundantDefault` | property/custom-type default | server |
| `isRedundantDirection` | `from`/`to` membership | server |
| `isRedundantTargetType` | resolved target set | server |
| **`isRedundantLabel`** | **a naming convention** | **client — and it doesn't** |

`isRedundantRelationLabel` straddles both: its metamodel arm strips a value the
SPA *could* recover (`schemaStore.getRelationType(rel).label`, already plumbed
end-to-end per `types/schema.ts:37`) but never reads; its titleCase arm strips a
value the SPA cannot recover at all.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Approach — fix the consumer, keep the migration.** Stripping redundant config
is the right design; the renderer is the side that is wrong, and it is the side
`sections.go:184` already gets right server-side. Relaxing `isRedundantLabel`
instead would leave `data-entry.yaml` carrying noise forever and would not fix
the relation-label metamodel arm, which is a genuine missing feature.

1. Extract one `titleCase` into `frontend/src/utils/format.ts` (the established
home — sibling to `formatValue`/`formatCellValue`, with a colocated
`format.test.ts`). Replace `-` as well as `_` so it matches Go.
2. `FieldRenderer.vue:42` → `field.label || titleCase(field.property)`.
3. `RelationPicker.vue:97` → `field.label || relationType?.label || titleCase(field.relation)`.
`relationType` is already resolved at line 85 for `from`/`to` and `description`;
the metamodel label just needs to be consulted. This simultaneously makes
`label:` in `metamodel.yaml` live config for forms.
4. `RelationCards.vue:469/575/677` — same three-step fallback; these are the
same defect in the sibling widget and must not be left behind.
5. Route the other frontend copies (`FilterBar`, `AdHocFilterMenu`,
`SearchView`, `RelationCards.formatLabel`, `InlineCreateModal.formatLabel`, and
the inlined ones in `DocumentsPanel.vue:145` / `DocumentView.vue:54`) through
the shared util.

**Regression test:** `form-label-fallback-round-trip-test` — deliberately
two-sided. Frontend tests pin the fallback; a Go table test pins that everything
`isRedundantLabel`/`isRedundantRelationLabel` strips equals the documented
fallback. Without the Go half the contract is only enforceable in one direction
and a future widening of the predicate would silently degrade live forms again.

**Related areas checked.** The `.label ||` fallback audit across `frontend/src`
found the same raw-identifier pattern in list/table headers —
`EntityList.vue:815,885`, `EntityDetail.vue:1065,1128`, `DashboardView.vue:98`,
`KanbanView.vue:458`. These matter because `cleanupLists`
(`dataentry_cleanup.go:414`) applies `isRedundantLabel` to **list columns** too,
so column headers are stripped by the same predicate. They are in scope for the
fix.

Out of scope, worth a follow-up ticket: consolidating the four Go `titleCase`
copies (`dataentry/helpers.go:312`, `migration/dataentry_cleanup.go:443`,
`lua/flow.go:670`, `cmd/rela-desktop/main.go:895`). Not required to fix this bug
and touches unrelated packages.
