---
id: BUG-R0Q216O
type: bug
title: 'Mobile: hamburger shows content through it and an empty Piles heading shows'
description: On a phone the hamburger has no background so scrolled text shows through it; the drawer shows an empty Piles heading with + for a user without piles.
priority: medium
effort: s
why1: The fallback hamburger is a transparent icon button made sticky over the pane; the Piles group was built even with no piles.
why2: RlIconButton had no surface of its own because it was made for toolbars and headers that supply one; pilesNavGroup always returned a group so its + could make the first pile.
why3: The sticky fallback was added later for pages without a header slot and reused the toolbar button as it was; the + was kept as the only way to start a pile although Add to pile already offers one.
why4: The mobile e2e tests checked that the button stays reachable and clears the notch but not that it can be read over content; no test covered the drawer for a user without piles.
why5: Layout tests check position and size; nothing checks that a floating control keeps its own surface.
prevention: mobile-layout.spec.ts asserts the hamburger background is opaque after scrolling; Sidebar.piles.test.ts asserts there is no Piles group without piles, that it comes in with the first pile and that it stays when every pile is placed.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

Two defects in the mobile layout:

1. On a page without a sticky topbar (an entity page, for example), the
hamburger floats over the content as it scrolls. It has a transparent
background, so the text under it shows through the icon and both are hard to
read.
2. A user without piles sees a "Piles" heading with a "+" and nothing under
it in the drawer. The heading reads as broken.

## Expected

The hamburger has its own opaque surface. The Piles group shows only when the
user has a pile; the first pile is made from "Add to pile" in a list, in search
or on an entity page.
