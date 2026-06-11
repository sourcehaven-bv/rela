---
id: TKT-JMIS
type: ticket
title: Analyze view shows ID-derived placeholder instead of entity title
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

The data-entry Analyze page at `/analyze` renders an `Entity` column intended to
show the entity's display title in addition to its ID (the styling in
`AnalyzeView.vue` already distinguishes `.entity-title` from `.entity-id`). For
entity-linked issues, the "title" line shows a cosmetic transform of the ID —
`getEntityTitle()` in `frontend/src/views/AnalyzeView.vue` just upper-cases the
first character of the ID and replaces hyphens with spaces. The entity's actual
title is never rendered.

```js
// frontend/src/views/AnalyzeView.vue (current placeholder)
function getEntityTitle(issue: AnalyzeIssue): string {
  // For now, capitalize first letter as title approximation
  // In v1, this comes from the entity properties
  const id = issue.entityId
  return id.charAt(0).toUpperCase() + id.slice(1).replace(/-/g, ' ')
}
```

This is a frontend-only bug: the backend already populates the title correctly.
`internal/dataentry/analyze.go` calls `s.Meta.DisplayTitle(e.ID, e.Type,
e.Properties)` for every entity-linked `AnalysisIssue` (Orphans, Duplicates,
Properties, Cardinality, Validations). `handleV1Analyze` in
`internal/dataentry/api_v1.go` copies `issue.Title` onto `APIIssue.Title`, which
serializes as the `title` field — and `AnalyzeIssue.title?: string` already
exists in `frontend/src/types/config.ts`. The wire data is present and ignored;
`getEntityTitle()` reformats `entityId` instead of reading `issue.title`.

## Proposal

Replace the placeholder body of `getEntityTitle()` with a real fallback:

```js
function getEntityTitle(issue: AnalyzeIssue): string {
  return issue.title || issue.entityId
}
```

That's it on the frontend. No backend changes required.

## Acceptance criteria

1. Analyze rows for entity-linked issues display the entity's actual title
(from `issue.title`) on the title line; the ID line continues to show the ID
verbatim.
2. Entities whose title resolves to empty fall back to showing only the
ID (no cosmetic transform of the ID).
3. Validation script-error / load-error rows (which already populate
`Title` with the rule name) continue to show the rule name — no regression in
the existing `title` use.
4. Frontend unit tests cover: row renders backend-provided title; row
falls back to ID when title is empty; rule-name rows still render the rule name.
5. Manual verification in the running dev server confirms titles render
for real entities across the entity-linked check types (Properties, Cardinality,
Validations, Orphans, Duplicates).

## Out of scope

- Backend changes (the backend already supplies the title).
- ID Gaps rows (no `entityId`, no entity to title — the existing
`entity-empty` `—` rendering applies).
- Lock affordances / inaccessibility handling on the analyze view
(covered separately by RR-MGJN — deferred).
- Changing the navigation behaviour or row click semantics.

## Related

- `frontend/src/views/AnalyzeView.vue` — `getEntityTitle()` placeholder.
- `frontend/src/types/config.ts` — `AnalyzeIssue.title` (already typed,
already populated by backend, currently unused by the renderer).
- `internal/dataentry/analyze.go` — already calls `DisplayTitle`.
- `internal/dataentry/api_v1.go` — `handleV1Analyze` already wires it
onto the wire envelope.
- TKT-YR7OW — relation pickers showing name + id (same UX pattern).
- TKT-NYJG — recent fix to the analyze view warning count (same file).
