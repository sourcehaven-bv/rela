---
id: BUG-YKJSZM
type: bug
title: DocumentsPanel rendered as sibling of .sections missing 32px gap
description: 'DocumentsPanel rendered as a sibling of div.sections in EntityDetail.vue instead of one of its flex children; it never picked up .sections gap: 32px and nothing else supplied that spacing either; there was no gap between the last section and the Documents panel.'
priority: low
effort: xs
why1: DocumentsPanel was placed after the closing </div> of .sections rather than before it; it fell outside the flex container that provides the 32px gap between sections.
why2: EntityDetail.vue was structured with .sections as a flex container relying on gap for inter-child spacing; DocumentsPanel was appended as a template sibling rather than a child when it was added to the page.
why3: No margin or gap was added for DocumentsPanel at the point it was introduced since the author relied on visual review rather than checking which flex container the gap actually applied to.
why4: There is no test or lint rule asserting DOM nesting/spacing invariants for EntityDetail.vue so a layout regression like this is only caught by manual visual inspection.
why5: Layout spacing in this component is implicit (derived from DOM nesting under a flex gap) rather than an explicit named spacing token or utility applied uniformly; correctness depends on remembering which elements must nest inside which container.
prevention: Nest new entity-detail sections inside .sections so they share the flex gap by construction; consider a visual regression test on EntityDetail.vue if this class of spacing bug recurs.
status: done
---

## Problem

`DocumentsPanel` was rendered as a sibling of `div.sections` in
`EntityDetail.vue`, after that div's closing tag, instead of as one of its flex
children. `.sections` supplies `gap: 32px` between its children; nothing else in
the template supplied equivalent spacing for `DocumentsPanel`, so there was no
gap between the last relation section and the Documents panel on an entity
detail page with a configured document.

## Fix

Moved `<DocumentsPanel>` inside `div.sections`, right after the `v-for` rendered
`<section>` elements, so it is covered by the existing flex `gap` instead of
duplicating that spacing via a new margin.

Verified `entry` is `computed(() => viewData.value?.entry || null)`, so `entry`
truthy already implies `viewData` truthy — nesting inside `v-if="viewData"`
introduces no behavioral divergence from the previous unconditional-sibling
placement.

Manually verified in a local rela-server build: 32px gap now renders between the
last relation section and the Documents panel.

Fixed in PR #1242.
