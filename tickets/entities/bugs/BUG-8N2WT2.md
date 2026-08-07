---
id: BUG-8N2WT2
type: bug
title: rela migrate strips form labels the SPA cannot re-derive, permanently downgrading them to raw property/relation ids
description: |-
    The data-entry cleanup migration and the SPA form renderer disagree about what a field's default label is. The migration strips any label it considers derivable; the renderer then does not derive it, so the field renders its raw snake_case identifier. Because the server REFUSES TO START while the redundant label is present, removing the label is mandatory and the degradation is unavoidable — there is no way to get 'Titel' rendered short of choosing a deliberately different string.

    Three independent gaps, same root cause (a strip/re-derive contract with no test):

    1. Property labels. internal/migration/dataentry_cleanup.go:159 strips label when `label == titleCase(prop)`. frontend/src/components/forms/FieldRenderer.vue:42 falls back to `props.field.property` RAW — no titleCase. So `laatste_contact` renders as `laatste_contact`, never `Laatste Contact`.

    2. Relation labels, titleCase arm. dataentry_cleanup.go:254 strips a relation label when it equals `titleCase(rel)`. RelationPicker.vue:97 (and RelationCards.vue:469/575/677) fall back to `props.field.relation` RAW.

    3. Relation labels, metamodel arm. dataentry_cleanup.go:247 ALSO strips a relation label when it equals the metamodel's `RelationDef.Label`. The SPA never consults it — RelationPicker.vue:85 already resolves `relationType` and uses it for from/to and description, but the label computed ignores `relationType.label`. A perfectly good `label:` in metamodel.yaml is dead config for forms.

    The backend does have the correct behavior elsewhere: internal/dataentry/sections.go:184,226 apply `titleCase(f.Property)` when the label is empty. Only the SPA form path is missing it.

    Secondary: titleCase is implemented 4x in Go and 7x in the frontend (FilterBar.vue:160, AdHocFilterMenu.vue:160, SearchView.vue:58, RelationCards.vue:363 formatLabel, InlineCreateModal.vue:97 formatLabel, plus inlined copies in DocumentsPanel.vue:145 and DocumentView.vue:54), with a behavioural divergence: the JS copies only replace `_`, the Go copies also replace `-`. So a kebab-case property is stripped by Go as `Kebab Name` but would be re-derived by JS as `Kebab-Name`.

    Observed in a live form — correctly-labelled fields sitting next to raw ones:

        titel *              <- stripped, raw
        soort *              <- stripped, raw
        Fase *               <- kept ('Fase' != titleCase('stage'))
        status *             <- stripped, raw
        Waarde (EUR/jr)      <- kept
        inschrijfdeadline    <- stripped, raw

    Version: built from 8cce2733. Reported against a live customer project.
priority: high
effort: s
why1: FieldRenderer.vue:42 falls back to `props.field.property` raw, while the cleanup migration deleted the label on the assumption that titleCase(property) would be re-derived. RelationPicker.vue:97 has the same defect against two arms of isRedundantRelationLabel (titleCase(relation) AND the metamodel RelationDef.Label, which the SPA never reads at all).
why2: The migration's strip predicate and the consumer's fallback are two independent expressions of one contract, written in two languages, in two packages, with no shared definition and no test asserting they agree. `isRedundantLabel` encodes 'the consumer will produce titleCase(prop)' as a claim about code it cannot see.
why3: No test exercised the fallback branch. FieldRenderer.test.ts passes an explicit `label` in all ten of its cases, so the only branch that runs after `rela migrate` had never been executed in CI. The migration's own tests assert what gets removed, never that anything re-derives it.
why4: The migration was built to remove config that is redundant *with respect to the metamodel* (widget, required, default, direction, target_type all consult m.meta and are genuinely re-derived server-side). Labels were added to the same batch, but a label is redundant with respect to a *rendering convention in the SPA* — a different and unverifiable kind of redundancy. The category error let a metamodel-grounded predicate be applied to a non-metamodel-grounded default.
why5: There is no single source of truth for 'the default presentation of an identifier'. titleCase is implemented four times in Go (dataentry/helpers.go:312, migration/dataentry_cleanup.go:443, lua/flow.go:670, cmd/rela-desktop/main.go:895) and seven times in the frontend, and the two families have already diverged on kebab-case (Go 'Kebab Name' vs JS 'Kebab-Name'). With the transform duplicated eleven ways, no strip/re-derive contract can be enforced anywhere; the migration and the renderer were never able to agree by construction, only by coincidence.
prevention: |-
    Structural, and applied platform-wide rather than patching one renderer (DEC-6C1NAA: a label is authored, never derived).

    1. Removed all 12 identifier->label derivations rather than teaching the SPA the missing one. Fixing the consumer would have welded 11 duplicate titleCase copies into a pinned cross-language invariant, making the heuristic permanent. Deleted — Go: dataentry/helpers.go, migration/dataentry_cleanup.go, lua/flow.go, cmd/rela-desktop/main.go, metamodel/types.go (camelCaseToSpaced, the same bug one layer down), cli/graph.go (DOT cluster heading; found during review BY the new lint test, not by the survey). Frontend: FilterBar, AdHocFilterMenu, SearchView, RelationCards.formatLabel, InlineCreateModal.formatLabel, HistoryView.propertyLabel, RelationHistoryView.propertyLabel, DocumentsPanel, DocumentView.

    2. The migration no longer strips any label it cannot prove is re-derived. isRedundantLabel is gone entirely, along with the titleCase arm of isRedundantRelationLabel and the now-empty list-column cleanup. The only surviving label strip is metamodel-grounded (relation label == RelationDef.Label), and RelationPicker/RelationCards were fixed to read relationType.label — direction-aware, so an incoming picker uses inverse.label — so that value is genuinely recovered.

    3. STRUCTURAL GUARD (the part that lasts): internal/migration/label_derivation_lint_test.go walks every .go/.ts/.vue file and fails on the SHAPE of an identifier->prose transform — four regexes covering JS per-word capitalization (replace(/\b\w/), JS first-char (charAt(0).toUpperCase()), Go first-byte (ToUpper(x[:1])), Go first-rune (runes[0] = unicode.ToUpper). The first version banned four helper NAMES and code review proved it worthless: an arrow function passed, and so did restoring HistoryView.propertyLabel — one of the derivations this change deleted. Matching behaviour instead of names is what makes it hold; verified with three planted probes (arrow function, restored propertyLabel, renamed Go helper) plus a clean baseline. Two allowlist entries with stated reasons (OpenAPI schema names; DynamicForm template file names, follow-up).

    4. General rule for future migrations, documented in the dataentry_cleanup doc comment: strip only metamodel-grounded redundancy, where the server re-derives the value and the contract is verifiable. Never strip convention-grounded redundancy that depends on a client re-implementing a transform. A migration may only delete config it can prove is re-derived, and that proof belongs in a test.

    5. Docs must be edited at SOURCE (docs-project/entities/guides/), not in generated docs/. Review caught two of three doc files edited on the generated side, which the next `just docs` run would have silently reverted — the same drift class this bug is about.
status: done
---

## Description

`rela migrate` removes data-entry.yaml config it deems redundant, on the
assumption that the consumer re-derives the same value. For form labels that
assumption is false: the SPA's fallback is the raw identifier. The migration is
not optional — `rela-server` refuses to start while the "redundant" label is
present — so the user is forced into the degraded state.

```text
failed to initialize: data-entry.yaml uses deprecated syntax:
  - Remove redundant labels and default widgets from data-entry.yaml
Run 'rela migrate' to update your project files.
```

## Reproduce

1. In `data-entry.yaml`, on a form field for property `titel`, set `label: 'Titel'`.
2. Run `rela migrate` — the label is removed (`isRedundantLabel` matches, since
`titleCase("titel") == "Titel"`).
3. Load the form — the field renders as `titel`, lowercase.

Same for a relation field whose `label:` matches either `titleCase(relation)` or
the metamodel's `RelationDef.Label`.

## The mismatch, in two lines

```go
// internal/migration/dataentry_cleanup.go:153
func (m *DataEntryCleanupMigration) isRedundantLabel(node *yaml.Node) bool {
    ...
    return label == titleCase(prop)     // strips 'Titel' for property `titel`
}
```

```ts
// frontend/src/components/forms/FieldRenderer.vue:42
const label = computed(() => props.field.label || props.field.property || '')
//                                                 ^^^^^^^^^^^^^^^^^^^^ raw, not titleCase'd
```

`titleCase()` (`dataentry_cleanup.go:443`) replaces `_`/`-` with spaces and
upper-cases each word's first rune. The renderer's fallback does none of that.

## Wire-level confirmation

`internal/dataentryconfig/config.go:178,197,212` all declare
`json:"label,omitempty"`, and `handleV1Config`
(`internal/dataentry/api_v1.go:1300`) copies `Forms` verbatim — its only
derivation is `resolveRelationWidgets`. So a stripped label reaches the SPA as
*absent*, and the SPA is solely responsible for re-deriving it. It doesn't.

## Prior art in the same area

`internal/dataentry/sections.go:184` and `:226` already do the right thing for
server-rendered view sections:

```go
label := f.Label
if label == "" {
    label = titleCase(f.Property)
}
```

Only the SPA form path lacks the equivalent.

## Missing invariant

No test asserts the round-trip contract the migration depends on: *config the
migration strips must be re-derived identically by the consumer*. That contract
is currently false for `FieldRenderer` and `RelationPicker`.
`FieldRenderer.test.ts` passes an explicit `label` in all 10 cases and never
exercises the fallback path.
