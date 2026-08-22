---
id: TKT-CBSTYLE
type: ticket
title: CheckboxWidget is unstyled — the only widget with no design tokens
kind: enhancement
priority: low
effort: xs
status: backlog
---

## Goal

Give `CheckboxWidget`'s **edit** mode the same visual treatment every other
widget in the registry already has, so a boolean field doesn't read as a
foreign control dropped into an otherwise consistent form grid.

## Evidence

Design-token references per widget (`grep -c 'var(--'`):

| Widget           | tokens |
| ---------------- | ------ |
| `SelectWidget`   | 13     |
| `TextWidget`     | 6      |
| `NumberWidget`   | 6      |
| `DateWidget`     | 6      |
| `CheckboxWidget` | **0**  |

`CheckboxWidget.vue`'s entire `<style>` block is three lines, and they apply
only to the DISPLAY arm (`.display-checkbox`: muted opacity, default cursor).
The edit arm is a bare native `<input type="checkbox">` — OS default size and
colour, no `--radius-*`, no `--accent-color`, no `--border-color`. Its
neighbours in the same 12-column grid all carry padding, a token border-radius,
and a focus ring, so it sits visibly outside the visual system.

## Why this surfaced now

Not a regression. The widget has always looked like this; TKT-3R7RF3 (`widget:`
override) simply made a checkbox easy to place on a detail page next to the
styled controls, where the mismatch is obvious. Booleans were previously
uncommon on those surfaces.

## Scope

- Style the edit-arm checkbox from `src/styles/scales.css` /
  `tokens.css` — size, radius, accent, focus ring, disabled state — matching
  the other widgets' conventions.
- Keep the display arm a REAL disabled checkbox. `RR-UD2I` chose that
  deliberately for native screen-reader semantics ("checkbox, checked,
  read-only") and cross-font consistency; do not replace it with a glyph or a
  styled `<span>`.
- Respect the frozen typography/token contract (`frontend/CLAUDE.md`): use
  existing token values rather than inventing new ones.

## Non-goals

- No behaviour change. Checked/unchecked semantics, the `@change` handler and
  the auto-save path stay exactly as they are.
- Not a new widget, and no change to which widget the registry selects.
- The `display: table` cell rendering of booleans, which deliberately routes to
  text so values stay Cmd-F searchable (`frontend/CLAUDE.md`).
