---
id: TKT-4Q2EIVZ
type: ticket
title: Edit owned and linked items inline on the parent page
kind: enhancement
priority: medium
effort: l
started: "2026-11-03"
completed: "2026-11-03"
status: done
description: Edit owned items (line-item tables and blocks) and linked entities in place on the parent entity page, following the rela-components mockup.
---

## Description

Let users edit owned and linked items in place on the parent entity page, as
shown in the rela-components mockup "Mockups/Owned and related items"
(`frontend/packages/rela-components/src/fixtures/RlOwnedItemsMockup.vue`).

Three patterns:

1. **Line items**: owned items of one type shown as an editable table. Cells
edit inline (numeric cells right-aligned with tabular figures). A totals block
(`RlTotals`) sums the numeric columns.
2. **Blocks**: owned items of several types shown as collapsible cards
(`RlAccordion`) with an actions menu. Prose fields use `RlMarkdownEditor`.
3. **Linked entities**: related, non-owned entities edited in place. The
action removes the link rather than deleting the entity, and says that the
entity is shared.

Edits save through rela's existing autosave (`useAutoSave` and
`AutoSaveIndicator`). Deleting an owned item offers Undo in a toast.
