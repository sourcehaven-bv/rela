---
id: BUG-1P88YM
type: bug
title: Rel picker in data-entry shows IDs instead of titles when display_property is not 'title'
description: 'The RelationCards relation widget (used for relations with editable per-edge metadata) rendered entity.properties.title in its search dropdown and "Linking to:" label. For any project whose metamodel display_property is not literally "title" (e.g. the generiekefuncties-architectuur project, which uses "naam"), properties.title is empty, so every search result and the link label fell back to the bare entity ID. The sibling RelationPicker.vue and this file''s own getEntityTitle() already use the backend''s metamodel-aware _title; only these two render sites in RelationCards.vue bypassed it. Reported against /form/applicatiefunctie/... where the data-objecten picker showed "PRS-DO-DKSN PRS-DO-DKSN dataobject" instead of the entity name.'
priority: medium
effort: s
why1: 'RelationCards.vue rendered entity.properties.title, which is empty for any project whose display_property is not literally "title" (this project uses "naam"), so the search dropdown fell back to the entity ID.'
why2: 'The component hardcoded the conventional property name "title" instead of using the metamodel-aware _title the backend already computes from display_property (via entityToV1 -> Meta.DisplayTitle, which is total and never empty).'
why3: 'There is no single shared "display an entity''s title" helper on the frontend — each picker/card/list/search row re-derives the display name, so they drift. RelationPicker.vue used _title; RelationCards.vue''s two render sites did not.'
why4: 'Tests and CI did not catch it because the test fixtures and the in-repo example projects all use a property literally named "title" (the RelationCards test fixture seeds properties title: id). The _title-vs-properties.title divergence is only observable when display_property differs from "title", which no fixture exercised.'
why5: 'Systemic: entity display-name resolution is duplicated across the frontend with no canonical helper, AND the test fixtures never exercise a non-"title" display_property, so the metamodel display contract is not enforced on the client. The two together let a render site silently disagree with the backend.'
prevention: 'This fix adds a regression test that seeds a search result with only _title set (no properties.title), so any RelationCards render site that bypasses _title fails CI. The durable fix for the class is to extract a single entityDisplayTitle(entity) helper and route every picker/card/list/search row through it, plus add at least one shared test fixture whose display_property is not "title" so client components that ignore _title regress visibly. Filed as follow-up.'
status: ready
---

## Symptom

In the data-entry SPA, the relation **rel picker** showed entity **IDs instead
of titles**. Reproduced on `/form/applicatiefunctie/...` in the
`generiekefuncties-architectuur` project: the "Gebruikt data-objecten" picker's
search dropdown rendered rows like `PRS-DO-DKSN  PRS-DO-DKSN  dataobject`
instead of the entity name (`Persoonsnummer`, `Audit log`, …).

Confusingly, **most** pickers on the same form worked, and even within the
data-objecten widget the **already-selected cards** showed correct titles —
only the search dropdown and the "Linking to:" label were affected.

## Root cause

The widget for the affected relation is `RelationCards.vue` (relations with
editable per-edge metadata), distinct from the plain `RelationPicker.vue`. Two
render sites used `entity.properties.title || entity.id`:

- the search dropdown row (`.result-title`)
- the "Linking to:" selected-target label

`properties.title` is empty whenever the metamodel `display_property` is not
literally `title`. This project uses `naam`, so both sites fell back to the ID.

The backend already serializes the correct display name into `_title`
(`entityToV1` → `Meta.DisplayTitle`, which resolves `display_property` and is
total — it returns the resolved value or the entity ID, never empty).
`RelationPicker.vue` and `RelationCards.vue`'s own `getEntityTitle()` use
`_title`; only these two sites diverged.

## Fix

Render `entity._title` directly at both sites. Since `_title` is always
backend-populated and non-empty, no `properties.title`/`id` fallback is needed.

Added a regression test in `RelationCards.test.ts` that seeds a search result
with only `_title` set (no `properties.title`) — it fails on the old code
(renders the ID) and passes with the fix.

## Fixed by

PR sourcehaven-bv/rela#1002 — `fix(data-entry): use _title in RelationCards
picker`.
