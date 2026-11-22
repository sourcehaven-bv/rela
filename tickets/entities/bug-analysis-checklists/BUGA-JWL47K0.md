---
id: BUGA-JWL47K0
type: bug-analysis-checklist
title: 'Analysis: Link style, phone layout, sidebar title and empty list text'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a copy of the harbourmaster project served at 375px width: no list search box, an empty band under the header, a shadow strip at the left edge, dashboard tables scrolling sideways, a boxed menu button, a cut-off workspace name and "No persons found.")
- [x] Minimal reproduction steps documented (open a list and a dashboard with table cards in a 375px wide viewport; open an empty list of a type with a label; open an entity with a display: link URL property that has link_text)
- [x] Environment/conditions noted (phone widths below 768px for the layout; any width for the link style, sidebar name and empty text)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (theme link colour and no title when a link text is set; show the header tools on a phone as a full-width row; shadow only on the open drawer; less top padding when the header slot is filled; stacked label and value rows for dashboard tables; no raised menu button; wrapping workspace name; label_plural on the wire and in the empty and search text with a neutral fallback)
- [x] Regression test planned (vitest for UrlWidget, DashboardView, dashboardCards, SortMenu and EntityList empty text; Go test for label_plural on the wire; e2e mobile-layout tests for the search box, the band and the shadow)
- [x] Related areas checked for similar issues (the kanban search placeholder used the same type-name plural and is fixed too; the sort button label wrapped below its icon on a phone and is fixed)
