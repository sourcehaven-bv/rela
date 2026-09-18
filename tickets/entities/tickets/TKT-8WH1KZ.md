---
id: TKT-8WH1KZ
type: ticket
title: 'Markdown tables in the detail body render cramped: full body width + horizontal overflow'
kind: enhancement
priority: medium
effort: s
status: done
---

# Markdown tables in the detail body render cramped

## Problem

A markdown table in an entity detail page body renders with columns squeezed far
narrower than the available space. Header cells break mid-word ("Nive au", "vold
oen de"), link text wraps character-by-character, and the result is unreadable —
on a screen with plenty of horizontal room.

TKT-YYZRGW added `overflow-x: auto` on `.md-body` plus `width: 100%` on the
table, and claimed "wide tables scroll horizontally". Measured: that was never
true. `scrollWidth == clientWidth` always, so the rule never fired once.

## Root cause

`.md-body` sets `overflow-wrap: anywhere` for prose, and table cells inherit it.
Per CSS Text 3 §5.5, `anywhere` — unlike `break-word` — **counts toward
min-content width**, so every column's minimum collapsed to a single character.
With `table-layout: auto` the table then always fit its container and squeezed
instead of overflowing.

Measured at a 1200px column: the "Niveau" column rendered at 58px with a 68px
(3-line) header.

## Scope

In scope:

- `.md-body` table sizing and cell wrapping in `frontend/src/styles/markdown-content.css`.
- A per-table horizontal scroll container, so a wide table scrolls without
dragging the surrounding prose sideways.

Out of scope:

- Exports/transforms and mail (`internal/mailrender`).
- App layout tables (kanban, analyze, dashboard) that are not `.md-body`.
- The `.entity-detail` 1200px width cap. Widening it is a separate preference
change shared by four files (EntityDetail, EntityList, ConflictsView,
DashboardView); widening one alone makes the page jump width between list and
detail. Split out rather than ridden along on a bug fix.
- The Milkdown WYSIWYG surface, which wears `.md-body` deliberately but sets its
own per-column widths via ProseMirror `columnResizing`. Explicitly excluded via
`:not(.milkdown-prose)`.
- The EasyMDE preview keeps body-level overflow: it renders through its own
marked instance and never sees rela's wrapper. Documented, not silently
divergent.

## Solution

Three CSS declarations plus one renderer helper.

```css
.md-table-scroll { overflow-x: auto; margin: 0 0 14px; }
.md-body:not(.milkdown-prose) table { width: auto; min-width: 100%; }
.md-body:not(.milkdown-prose) th,
.md-body:not(.milkdown-prose) td {
  min-width: 12ch; max-width: 40ch; overflow-wrap: break-word;
}
```

Each bound is load-bearing:

- `overflow-wrap: break-word` re-wins the inherited `anywhere`, so a column's
minimum is its longest **word**, not its longest character.
- `min-width: 12ch` is the floor that makes overflow possible at all. Without
it the table still shrinks to fit and never scrolls.
- `max-width: 40ch` is the ceiling. `break-word` also does not contribute to
min-content, so without this cap a single 300-char URL blew one table out to
2111px.

`wrapTablesForScroll` (`frontend/src/utils/markdown.ts`) wraps each `<table>` in
a focusable scroll region. It runs on **already-sanitized** HTML and builds the
wrapper with DOM APIs inside an inert `<template>` — never string-splicing
markup around DOMPurify's output. Wired into `renderMarkdown` and into both
document views, which render server-side goldmark HTML and would otherwise be
missed.

The table keeps `display: table`. `display: block` also produces scrolling but
drops table roles from the accessibility tree (RR-5SURSS) and re-breaks
narrow-table fill, which RR-5ZVPC5 rejected once already.

## Acceptance criteria

1. A table that fits the body column sizes columns to content — no mid-word
breaking of short header labels. **Measured: Niveau 58px → 103px, header 68px →
43px.**
2. A table wider than the column scrolls **in its own wrapper**; the surrounding
prose does not move and the page gains no horizontal scrollbar. **Measured:
wrapper 1236 > 1200, paragraph fixed at 264px while scrolled.**
3. A long unbroken token (300-char URL) wraps rather than forcing unbounded
table width. **Measured: stays at 1200px, no scroll.**
4. A narrow 2-column table still fills the width (the RR-5ZVPC5 case).
5. The scroll region is keyboard-reachable: `tabindex="0"`, `role="region"`, and
an accessible name from the caption or first columns (RR-B22351).
6. Light and dark themes both correct — styling stays token-driven, no new
colour literals.

## Verification

- 2740 frontend tests pass across 168 files; typecheck clean; lint 0 errors.
- 13 tests for `wrapTablesForScroll`, mutation-checked: deleting the
`tabindex` line fails the suite rather than passing silently.
- Live browser verification at 1800px viewport against `rela-server` +
the demo entity, for every case in the table above.
