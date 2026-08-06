---
id: TKT-8CL0PO
type: ticket
title: Help-icon affordance for view descriptions (lists + kanbans)
kind: enhancement
priority: low
status: backlog
---

## Description

Give `description` an independent job on data-entry views: a short plain-text
help blurb surfaced via a help icon (ⓘ / ?) next to the view title, distinct
from the `header`/`footer` markdown banner regions.

## Background

Split out of TKT-6S331G (kanban header/footer). While scoping that ticket the
question came up of whether kanban should copy the list `description` → `header`
fallback. Today on lists, `description` is a *silent fallback*: when both
`header` and `description` are set, `header` wins and `description` is ignored —
a field that only does something when another field is absent.

The better model is for the two to stop competing for one slot: `header` is the
prose banner, `description` is a compact tooltip. That also matches how
`description` is used everywhere else in the SPA — field help
(`FieldRenderer.vue:44`, `RelationPicker.vue:98`), wizard step descriptions
(`DynamicForm.vue:1426`), the dashboard subtitle (`DashboardView.vue:17`) — all
short plain text, never a rendered markdown region. The list fallback is the
outlier.

## Scope

- A small shared help-icon/tooltip component. **None exists today** — the SPA
only uses bare `:title` attributes (`InaccessibleField.vue`,
`AutoSaveIndicator.vue`), so this needs building, including keyboard and
screen-reader access (a `:title` alone is not reachable by keyboard).
- Wire it to `description` on both `ListConfig` and `KanbanConfig`, so the
semantics are decided once and applied uniformly.
- Deprecate the list `description` → `header` fallback. **This is a behavior
change**: any config relying on `description` rendering as a header banner would
move to a tooltip instead. Needs a migration note, and possibly a migration, in
`docs/data-entry.md` (see TKT-SI02JV, which reworded that fallback note).

## Open questions

- Does the fallback get removed outright, or kept for a deprecation period?
- Should `description` stay plain text (consistent with other SPA uses), or
render inline markdown?
