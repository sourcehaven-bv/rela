---
id: BUGA-59L4L1I
type: bug-analysis-checklist
title: 'Analysis: Mobile: hamburger shows content through it and an empty Piles heading shows'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (e2e "the hamburger keeps its own surface over scrolled content" fails on develop with a background alpha of 0; Sidebar.piles.test.ts "has no empty Piles heading when the user has no piles" fails on develop)
- [x] Minimal reproduction steps documented (on a 375px viewport open an entity page and scroll: the text shows through the hamburger; open the drawer as a user without piles: a "Piles" heading with "+" and no rows shows)
- [x] Environment/conditions noted (any backend; a mobile viewport, or up to 1079px with a detail panel open, on a page without a sticky topbar; piles available)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (RlIconButton gets a `raised` option with an opaque raised surface, border and shadow, and the hamburger uses it; Sidebar.vue leaves the Piles group out until the user has a pile)
- [x] Regression test planned (mobile-layout.spec.ts background alpha test; Sidebar.piles.test.ts for the absent group)
- [x] Related areas checked for similar issues (RlPageHeader's own toggle sits in the header band, which has its own background; the declared piles group already returns null when empty; "Add to pile" offers a new pile on lists, search and entity pages)
