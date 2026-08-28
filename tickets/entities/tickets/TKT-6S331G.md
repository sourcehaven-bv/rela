---
id: TKT-6S331G
type: ticket
title: Render admin-authored header/footer markdown on kanban boards
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Extend the list-view header/footer info regions (FEAT-RUGB28, TKT-H7E611) to
kanban board views so an admin can author contextual markdown above and below a
board from `data-entry.yaml` — sharing the implementation with lists rather than
duplicating it, and moving the board's horizontal scroll off the page wrapper so
the regions stay visible.

## Problem

`list:` views support admin-authored markdown info regions (`header`, `footer`,
with `description` as a legacy fallback for `header`) — added in TKT-H7E611.
Kanban boards have no equivalent: a board cannot carry a short blurb explaining
what the board is for, links to relevant guides, or footer notes about the
workflow the columns represent.

The asymmetry is arbitrary — a kanban board is just as much a landing surface as
a list, and the same "this register is ISO 27001 scope, see the scoring guide"
context applies.

Separately, `.kanban-view` carries `overflow-x: auto` on the whole page wrapper,
so on a wide board the page title, filter bar, and truncation banner scroll
sideways along with the columns. Info regions placed there would inherit the
same quirk.

## Scope

In scope:

- **Shared, not duplicated.** Extract the list header/footer implementation
into a form both view types use:
  - `viewHeaderMarkdown` / `viewFooterMarkdown` resolvers in
`frontend/src/types/config.ts`, taking a structural param type so `ListConfig`
(which has `description`) and `KanbanConfig` (which does not) both satisfy it.
`EntityList.vue` migrates to them; the existing `listHeaderMarkdown` /
`listFooterMarkdown` names are kept as thin re-exports or removed with their
call sites updated.
  - `.list-info` styles move to a shared `frontend/src/styles/view-info.css`
as `.view-info`, loaded in `main.ts` alongside the existing shared stylesheets.
`:deep()` wrappers drop (unnecessary once global).
- `dataentryconfig.Kanban` gains `Header`/`Footer` with the same yaml/json
tags as `List`.
- TS `KanbanConfig` gains `header?` / `footer?`.
- `KanbanView.vue` renders the two regions using the shared resolvers + class.
- **Scroll containment fix:** move `overflow-x` off `.kanban-view` onto the
board containers — `.kanban-board { overflow-x: auto }` and
`.kanban-swimlane-board { overflow: auto hidden }` (the swimlane grid currently
uses `overflow: hidden` to clip its cells to the rounded border, so it needs the
two-value form to keep that clipping while scrolling horizontally). Page title,
filter bar, truncation banner, and both info regions then stay put while only
the columns scroll.
- Document in `docs/data-entry.md`.

Out of scope:

- **No `description` field on kanban.** Unlike lists — where `description`
predated the feature and was adopted as a silent fallback for `header` — a
kanban has no existing `description` field, so there is no legacy config to
accommodate. The shared resolver gates the fallback behind an explicit
`allowDescriptionAlias` opt-in that only the list call site passes, so sharing
the helper does not leak the semantic. (Originally planned to rely on
`KanbanConfig` simply lacking the field — corrected per RR-GNWJFO, since TS
types erase at runtime and configs arrive from the `_config` response.)
- **Help-icon affordance.** Using `description` as a short tooltip/help blurb
next to the view title (distinct from the markdown banner) is a good idea but a
different feature, and it should be decided once and applied to lists and
kanbans uniformly rather than introduced asymmetrically here. Split to
TKT-8CL0PO.

## Acceptance criteria

1. A kanban config with `header` markdown renders sanitized HTML above the board.
2. A kanban config with `footer` markdown renders sanitized HTML below the board.
3. Markdown is sanitized through the same `renderMarkdown` path as lists (no raw HTML injection).
4. A kanban with no header/footer configured renders exactly as before — no empty region, no layout shift.
5. `_config` API serves `header`/`footer` for kanbans and omits the keys when unset.
6. On a board wider than the viewport, the page title, filter bar, truncation
banner, header, and footer remain visible while the columns scroll horizontally
— in both the simple-board and swimlane branches.
7. Existing list header/footer behavior is unchanged after the shared
extraction (including the `description` fallback and its precedence).
