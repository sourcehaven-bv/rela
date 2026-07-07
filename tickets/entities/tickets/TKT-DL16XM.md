---
id: TKT-DL16XM
type: ticket
title: Relation filter_controls render as target selector (select → typeahead), not free text
kind: enhancement
priority: medium
effort: s
status: review
---

## Problem

When a list config declares a relation filter:

```yaml
filter_controls:
  - relation: verantwoordelijk_voor
    direction: incoming
    label: Verantwoordelijke
```

the SPA renders a **free-text input**. `FilterBar.vue` explicitly falls back to
`widget: 'text'` for every relation filter (`resolveFilter`, ~line 44):

```ts
// Relation filters: use text input for now (could be enhanced to select with targets)
return { key, label, widget: 'text', options: [], isRelation: true }
```

Users must *type* the target's display title exactly. For a small closed set of
relation targets this is strictly worse than a picker.

Property enum filters already collapse to `select` / `multi-select` via
`resolveWidgetType` and render through the existing `<select>` template in the
same component. Relation filters were left as the TODO.

## Goal

Relation filter controls render as a **target selector**:

- **≤ N targets (N ≈ 10): plain `<select>`** — the machinery already exists in
the template.
- **> N targets: typeahead combobox** — reuse the search+dropdown UX already
shipped in `RelationPicker.vue` (type to filter, first 10 shown, `+N more`).

## Key correctness fact (corrects the input request)

The backend relation filter matches by **display title**, NOT by entity ID.
`App.matchRelationFilter` (`internal/dataentry/api_v1.go:476`):

```go
if svc.Meta.DisplayTitle(e.ID, e.Type, e.Properties) == want {
```

So the widget MUST send the entity's resolved display title (the `_title` field
the API already returns per entity) as the filter value — NOT the ID. Sending an
ID would match zero rows and return an empty list. This is the opposite of what
the original feature-request draft assumed ("send the ID, not the title").

## Where the option set comes from (no new backend)

`RelationPicker.vue` already resolves candidates entirely client-side:

- `loadCandidates()` walks `relationType.from` (incoming) / `relationType.to`
(outgoing) from the schema store and calls `entitiesStore.fetchList(targetType,
{ per_page: 100 })` per source type.
- It filters in-memory by `id` / `_title` as the user types.

The same generic `GET /api/v1/{plural}` list endpoint the picker already uses
supplies the filter's options. **No `_filter_options` endpoint and no backend
change is required for the atlas case.** The direction-aware relation-filter
query pass (`applyRelationFilters`, TKT-5U7QBR) is already merged and unchanged.

## Concrete need

`atlas/data-entry.yaml` already ships `all_taken.filter_controls` with the
`verantwoordelijk_voor` / `incoming` relation entry. Merging this upgrades the
widget from text to selector with no atlas config change.

## Scope

**In scope**

- Relation filter → `<select>` (small) or typeahead (large), value = `_title`.
- Count-gate threshold (N ≈ 10; final value TBD in planning).
- Extract/reuse the picker's search dropdown so FilterBar doesn't drag in the
form's edit/save/affordance/inline-create machinery.
- Frontend unit test + e2e.

**Out of scope**

- Multi-select relation filter (`filter_controls` is single-value today;
`CurrentValue` returns one string).
- Server-side "only-actually-populated" `_filter_options` endpoint (offer every
target of the source type, mirroring how property enum filters offer every
declared value). Revisit only if a source type exceeds the `per_page: 100` fetch
ceiling and truncation becomes visible.
- Pre-populating "my taken" from the current user (distinct UX / scope, not a
filter widget).

## Known limitations to carry forward

- **Title collisions**: because matching is by title, two source entities with
the same `DisplayTitle` collapse to one option and the filter matches both.
Pre-existing backend behavior; the widget inherits it.
- **100-per-type fetch ceiling**: `RelationPicker` fetches only the first 100
candidates per source type. Fine for the atlas (dozens of persoons); a source
type with hundreds of entities would silently truncate options — that's the
trigger for the deferred `_filter_options` endpoint.

## Acceptance criteria (draft — refine in planning)

1. A `filter_controls` entry with `relation:` renders as a `<select>` (≤ N
options) populated with the display titles of the relation's source-type
entities.
2. Above N options, the control renders as a typeahead combobox reusing the
RelationPicker search UX.
3. Selecting an option filters the list; the value SENT is the display title
(`_title`), and the list narrows correctly (backend title-match).
4. Empty selection = no filter.
5. Property filters render exactly as today (no regression).
6. `direction: outgoing` (or omitted) pulls options from the relation's `to[*]`
types; `incoming` from `from[*]`.

## Files (expected)

- `frontend/src/components/lists/FilterBar.vue` — swap the text fallback; add the
relation-select / typeahead branch.
- `frontend/src/components/forms/RelationPicker.vue` — extract the reusable
search-dropdown piece (or a shared child component) without the edit machinery.
- Frontend unit test(s) + `e2e/` coverage.

Backend: no change expected.
