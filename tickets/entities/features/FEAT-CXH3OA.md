---
id: FEAT-CXH3OA
type: feature
title: Canonical entityDisplayTitle helper + lint guard
summary: A single frontend helper for an entity's display name plus an ESLint guard banning .properties.title, so no render site can diverge from the backend's metamodel-aware _title.
description: 'Entity display-name resolution was duplicated across ~10 frontend components (pickers, cards, lists, search rows, detail heading), each re-deriving "how to show an entity name". Some used the backend''s metamodel-aware _title (correct); some read properties.title directly, which is empty whenever a project''s display_property is not literally "title" and so rendered bare IDs (BUG-1P88YM, and two latent instances in SearchView.vue and EntityDetail.vue). This feature introduces src/utils/entityDisplay.ts (entityDisplayTitle / entityDisplayTitleWithId), migrates every render site onto it, and adds an ESLint no-restricted-syntax rule that bans reading .properties.title (and properties[''title'']) in display code outside the helper. A non-"title" test fixture guards the contract so any component ignoring _title regresses in CI.'
priority: medium
status: in-progress
---

## Problem

`title` is re-derived per component. The literal `properties.title` is only
correct when a type's `display_property` is `title`; for any other (e.g.
`naam`) it is empty and the UI shows bare IDs. That divergence shipped as
BUG-1P88YM and sat latent in two more components.

## Approach

1. `src/utils/entityDisplay.ts` — `entityDisplayTitle(entity)` returns
   `_title || id`; `entityDisplayTitleWithId(entity)` returns `Title (ID)`.
2. Migrate all render sites: RelationCards, RelationPicker, EntityPickerModal,
   CommandPaletteModal, BacktickAutocompletePopup, EntityList, SearchView,
   EntityDetail.
3. ESLint `no-restricted-syntax` rule bans `.properties.title` /
   `.properties['title']` outside the helper and tests, with a message pointing
   at the helper.
4. Tests: helper unit tests; a RelationCards search fixture with only `_title`
   (no `properties.title`); corrected the CommandPalette test that previously
   asserted the buggy `properties.title` fallback.

## Outcome

Reading `properties.title` for display is now impossible without tripping the
lint guard, and the metamodel display contract is enforced on the client.
