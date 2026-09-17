---
id: RR-27MW0U
type: review-response
title: Non-.md-body markdown surfaces got a tab stop without the cell sizing
finding: wrapTablesForScroll runs inside renderMarkdown, the chokepoint every markdown surface uses, so view headers (.view-info in EntityList/Kanban/Gantt/Calendar) and the about box (StatusBar) received a focusable role=region wrapper while the cell min/max bounds were .md-body-scoped and did not apply. That is the accessibility cost of a tab stop with none of the sizing benefit.
severity: significant
resolution: 'The table and cell rules are now keyed off .md-table-scroll as well as .md-body, so any surface that receives a wrapper gets the matching sizing. Chose this over gating the wrapper to .md-body because the wrapper genuinely is universal: every surface rendering a markdown table wants it.'
status: addressed
---

# Finding

`wrapTablesForScroll` was placed inside `renderMarkdown` — the shared chokepoint
every markdown surface funnels through. Three call sites render into containers
that are **not** `.md-body`:

- `StatusBar.vue` → `.about-body`
- `EntityList.vue` (plus Kanban, Gantt, Calendar view headers) → `.view-info`,
whose stylesheet has no table rules at all

Those surfaces received a `<div class="md-table-scroll" role="region"
tabindex="0">` where `overflow-x: auto` applied (the class is global) but the
cell bounds did not (they were `.md-body`-scoped). Net effect on a view header:
an unconditional tab stop and an announced region around a table that still
squeezed — the accessibility cost without the benefit.

# Resolution

Made the sizing follow the wrapper rather than the other way round. The table
and cell rules are now keyed off `.md-table-scroll` in addition to `.md-body`:

```css
.md-body:not(.milkdown-prose) table,
.md-table-scroll > table { … }

.md-body:not(.milkdown-prose) th,
.md-body:not(.milkdown-prose) td,
.md-table-scroll > table > * > tr > th,
.md-table-scroll > table > * > tr > td { … }
```

The alternative was gating the wrapper so only `.md-body` consumers get one.
Rejected: the wrapper genuinely **is** universal — a markdown table in a view
header has the same overflow problem as one in an entity body, so the fix
belongs everywhere the renderer runs. The reviewer's point stands that the
placement decision should be explicit rather than incidental; this records it.
