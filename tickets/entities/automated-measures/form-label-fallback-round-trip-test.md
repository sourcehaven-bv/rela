---
id: form-label-fallback-round-trip-test
type: automated-measure
title: 'No label is derived from an identifier: raw-fallback pinned, titleCase absent'
description: 'Guards DEC-6C1NAA. Three parts. (a) Behavioural: server-rendered view sections emit the RAW property when no label: is configured (sections.go); a flow field''s label stays empty; InverseDef.GetLabel returns the raw inverse id. (b) Migration: the cleanup migration no longer strips a label matching titleCase(property) — Detect() is false for such a config, so the server starts and the label survives; inverse_simplify likewise. The one KEPT strip (relation label == metamodel RelationDef.Label) is pinned together with the RelationPicker test proving the SPA re-derives it from relationType.label. (c) Structural: a lint test asserting no titleCase-style helper is reintroduced, so the heuristic cannot creep back in one component at a time. Replaces the earlier round-trip design, which would have welded 11 duplicate titleCase implementations into a pinned cross-language invariant instead of removing them.'
kind: test
location: internal/migration/label_derivation_lint_test.go (structural guard), internal/migration/dataentry_cleanup_test.go, internal/migration/inverse_simplify_test.go, internal/dataentry/sections_ihc7d_test.go, internal/lua/flow_test.go, internal/metamodel/types_test.go, cmd/rela-desktop/main_test.go, frontend/src/components/forms/FieldRenderer.test.ts, frontend/src/components/forms/RelationPicker.test.ts
status: active
---

## Purpose

Guard [[DEC-6C1NAA]] — *a label is authored, never derived*. The risk after this
change is not that the code is wrong today; it is that the heuristic creeps back
one component at a time, because deriving `Due Date` from `due_date` always
looks like a small helpful improvement in isolation.

## Why this replaced the original design

The first version of this measure was a two-sided round-trip test pinning the
migration's strip predicate against the SPA's fallback. That was the right guard
for the *rejected* fix. Under DEC-6C1NAA it would have been actively harmful: it
would have welded eleven duplicate `titleCase` implementations into a pinned
cross-language invariant, making the heuristic permanent instead of removing it.

## What it covers

### (a) Behavioural — the raw identifier is the default

1. `sections.go` — a view-section field with no `label:` yields the raw
`Property` in `SectionFieldData.Label`. Note this path has **no** existing
`Label` assertions at all (`sections_test.go`, `sections_ihc7d_test.go`), so
this is new coverage of a previously untested derivation.
2. `internal/lua/flow.go` — a flow field with no `label` keeps an empty label,
matching the sibling `parseMarkdownField` path that never derived. Replaces
`TestFlowRuntime_LabelDefaultsToTitleCase`, whose premise is now inverted.
3. `InverseDef.GetLabel()` — returns the raw inverse id, not
`camelCaseToSpaced(id)`.

### (b) The migration no longer strips what it cannot prove

4. `Detect()` is **false** for a config carrying `label: 'Due Date'` on property
`due_date`. This is the bug's core symptom: the server must start, and the label
must survive `rela migrate`.
5. Same for `inverse_simplify` and an inverse label matching the old derivation.
6. **The one kept strip**, pinned deliberately: a relation label equal to the
metamodel's `RelationDef.Label` is still removed — and a companion
`RelationPicker.test.ts` case asserts the SPA re-derives it from
`relationType.label`. Both halves must exist or neither is safe. This is the
only sanctioned derivation, and it derives from an *authored label*, never from
an identifier.

### (c) Structural — the heuristic cannot return

7. A lint-style test asserting no `titleCase`/`formatLabel`-shaped helper is
reintroduced in the label paths. Modelled on the existing grep test
`internal/dataentry/lint_test.go:26` (`TestNoStrayWriteRequestConstruction`),
which is the in-tree precedent for enforcing a rule structurally rather than by
review.

Part (c) is the one that actually holds the decision over time. Parts (a) and
(b) prove the change landed; (c) prevents it from being quietly undone.

## Explicitly NOT covered

Enum **value** display labels (FEAT-JIBWQP's `labels:` map,
`schemaStore.getEnumLabel`) are a separate explicit mechanism that never used
`titleCase`. Unaffected, and their existing tests must keep passing untouched —
`stores/schema.test.ts:317,322`, `Badge.test.ts:150,285,299,317`,
`AdHocFilterMenu.test.ts:84,106`.
