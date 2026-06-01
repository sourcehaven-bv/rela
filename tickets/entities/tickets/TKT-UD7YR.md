---
id: TKT-UD7YR
type: ticket
title: Route view-side per-field rendering through widget registry
kind: enhancement
priority: high
effort: m
status: backlog
---

## Goal\n\nMake the `properties`, `cards`, and `list` view display modes render per-field cells via the widget registry (introduced in the previous ticket) in display mode.\n\n## Scope\n\n- `EntityDetail.vue` per-display-mode renderers delegate to `WidgetRegistry` for each field.\n- Widgets gain a `display` mode rendering (e.g. `CheckboxWidget` in display mode renders a static ✓ or ☐; `DateWidget` renders the formatted date).\n- `PropertyDisplay` and ad-hoc badge logic in `EntityDetail.vue` are removed in favour of registry dispatch.\n- Rendered output is visually identical to today.\n\n## Non-goals\n\n- No inline-edit on views yet (next ticket).\n- `table` and `content` display modes deferred — they have more structure and ship later.\n- No config changes — defaults handle which widget to use per property type.\n\n## Why\n\nSets up the surface that lets the next ticket add inline-edit by simply enabling a different mode on the same widgets.
