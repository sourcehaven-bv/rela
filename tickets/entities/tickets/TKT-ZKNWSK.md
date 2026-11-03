---
id: TKT-ZKNWSK
type: ticket
title: 'Palette: UI kit shades, teal, distinct orange/yellow and operator colours'
kind: enhancement
priority: medium
effort: m
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

## Description

Extend the data-entry colour vocabulary so a risk heat map and other graded
displays can use the full Atlas UI kit.

- Colour names follow the kit. Hues: `grey` (alias `gray`), `red`, `orange`,
`yellow`, `green`, `teal`, `blue`, plus `purple`. Shades `50` to `900`, written
`red/700`. A bare hue keeps today's look.
- `orange` and `yellow` become distinct colours.
- Accepted wherever a style colour is accepted: `styles:` (badges, kanban
dots, relation columns) and the matrix card of the dashboard ticket.
- Operators define their own colours under `palette.colors` with a `light`
and optional `dark` hex. Names must not collide with kit names.
- In dark mode kit shades mirror (`900` to `50`, `800` to `100`, ...),
unless `palette.dark.colors` overrides a shade.
- The text colour on a filled colour is chosen automatically to meet WCAG AA.
- Config validation rejects unknown colour names and bad hex.
- Docs in the data-entry guide.

Source: `.ignored/gaps/G3-dashboards.md`, ticket 1.
