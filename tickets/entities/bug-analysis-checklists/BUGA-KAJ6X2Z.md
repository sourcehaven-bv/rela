---
id: BUGA-KAJ6X2Z
type: bug-analysis-checklist
title: 'Analysis: List title link colour without link:, nested and card sections on phones'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a nested section added to a copy of the feature view: values clipped at 375px; a list without link renders the title in the text colour)
- [x] Minimal reproduction steps documented (open an entity with a nested section with parent_columns at phone width; open a list whose columns set no link)
- [x] Environment/conditions noted (any theme; phone width or a narrow detail panel)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (the title link class on every title link; a container query wraps nested rows; min() on the cards grid)
- [x] Regression test planned (EntityList.titleLink.test.ts; nested and cards checked in a browser)
- [x] Related areas checked for similar issues (related, list and properties sections already wrap; the only entity page tables are table sections, fixed in BUG-6WQ1U8S)
