---
id: relationcards-title-display-test
type: automated-measure
title: RelationCards renders _title not properties.title
description: 'Vitest unit test in RelationCards.test.ts that seeds a search result with only _title set and no properties.title (mirroring a metamodel whose display_property is not literally "title", e.g. "naam"), opens the add picker, types a query, and asserts the rendered .result-title equals _title and is not the entity ID. Guards the class of bug where a frontend render site reads properties.title directly and silently shows bare IDs for any project whose display_property differs from "title".'
kind: test
location: frontend/src/components/forms/RelationCards.test.ts
status: active
---

## Purpose

Close the test gap that let BUG-1P88YM ship: the existing RelationCards fixtures
always seeded `properties: { title: id }`, so a render site reading
`properties.title` looked correct in CI even though it shows bare IDs for any
real project whose `display_property` is not literally `title`.

## What it covers

- Seeds a `searchEntities` result with `_title: 'Pseudoniem API'` and
  `properties: { naam: ... }` — deliberately **no** `title` property.
- Drives the add-picker search and asserts the `.result-title` row renders
  `_title`, not the entity ID.
- Verified to fail on the pre-fix code (`properties.title || id` → renders the
  ID) and pass after the fix (`_title`).

## Limitation

This guards `RelationCards.vue` specifically. The durable class-wide guard is a
shared `entityDisplayTitle()` helper plus an ESLint rule banning
`.properties.title` in display code, and a shared non-`title` fixture — tracked
as follow-up.
