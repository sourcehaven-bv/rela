---
id: TKT-5U7QBR
type: ticket
title: Restore relation filter controls on the v1 list pipeline + add direction:incoming
kind: refactor
priority: medium
effort: m
tags:
    - regression
status: ready
---

## Problem

A `filter_control` on a relation returns zero rows. The incoming spec attributes
this to `filterByRelation` hard-coding outgoing — but the reality is worse:
**relation filtering is entirely absent from the live pipeline.**

## Why (verified against develop tip)

`FEAT-024` added `filterByRelation` and `resolveRelationFilterValues`
(`helpers.go:707,722`) and wired them into the **old `handleList`** (added in
#214). `handleList` was **removed in the Vue SPA migration `ebbcb5b6` (#230)**,
orphaning both helpers — they now have **no non-test callers** (only
`helpers_test.go`). The live path is:

- SPA: relation filter control renders as a **plain text input** (`FilterBar.vue:44`, "use text input for now") and sends `filter[<relation>]=value` (`filters.ts:161`).
- Server: `applyV1Filters` (`api_v1.go:1841`) filters **only** on `e.Properties[property]`. A `filter[verantwoordelijk_voor]` key reads `e.Properties["verantwoordelijk_voor"]`, finds nothing, and (eq/non-empty) returns zero rows. There is **no relation branch, outgoing or incoming.**

`resolveScope` → `scopedSortedEntities` → `applyV1Filters` (scope nav) has the
same gap.

## Scope

IN: relation filtering works on the live v1 pipeline for both outgoing and
`direction: incoming`; add `Direction` to `FilterControl`; wire it into
`applyV1Filters`/`scopedSortedEntities` so lists and scope nav agree. OUT:
**populated relation-filter dropdown** UX (the SPA has none today —
`FilterBar.vue` uses a text input). File separately if the "dropdown of all
persoons" UX is wanted. `resolveRelationFilterValues` belongs to that follow-up.

## Recommended approach

1. `dataentryconfig`: add `Direction Direction` to `FilterControl` (default outgoing), matching `ListColumn`.
2. Decide the wiring seam: `applyV1Filters` is a free function without ctx/svc. Either promote relation-filter handling into `scopedSortedEntities` (which has `a`/ctx) as a post-`applyV1Filters` pass, or give the relation branch access to `svc`. Reuse the direction-aware `resolveRelationColumnValues` + title match (retire the dead `filterByRelation`, or revive and call it with a direction param).
3. Load-time validation: warn when `direction: incoming` references a relation whose `to:` excludes the list's `entity_type`.

## Acceptance criteria

- `filter_control {relation: verantwoordelijk_voor, direction: incoming}`; `filter[verantwoordelijk_voor]=Jeroen Vloothuis` filters the taak list to tasks whose incoming source titles to Jeroen.
- Outgoing relation filter (default direction) also works (currently returns zero).
- Scope prev/next navigation over a relation-filtered list matches the list ordering.

## Test plan

- Handler test: `filter[<rel>]` with incoming direction returns only matching rows; outgoing default likewise.
- `_position` test: scope over a relation-filtered list yields consistent prev/next/total.
- Load-validation test for the incoming/`to`-mismatch warning.
