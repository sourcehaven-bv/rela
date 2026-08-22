---
id: TKT-CBSTYLE
type: ticket
title: CheckboxWidget is unstyled — the only widget with no design tokens
kind: enhancement
priority: low
effort: xs
status: done
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

## Outcome

Both arms are now drawn from tokens: an 18px box with `--radius-sm`, an
accent fill and a CSS checkmark, ported from the `.inline-edit-checkbox`
prior art in `RelationCards.vue`. The display arm stays a real disabled
checkbox, as required.

Three things the change had to solve that the original scope did not
anticipate, all of which came out of code review:

1. **`appearance: none` discards the browser's disabled greying**, which the
   display arm had been relying on. The muted state is now drawn explicitly —
   and at 0.6, not the old 0.85, which had been tuned as a supplement to the
   native rendering rather than as the whole signal ([[RR-CBM5OP]]).
2. **The first draft's read-only rule lost the cascade** and rendered
   `not-allowed` on every read-only boolean — the opposite of its own comment,
   and a regression. Fixed with `:not(.display-checkbox)` and pinned by a
   computed-style test that reproduces the bug when the guard is removed
   ([[RR-CBC1XZ]]).
3. **`appearance: none` + `outline: none` erases the control in Windows High
   Contrast**, where box-shadow is dropped and `::after` is not guaranteed. A
   `forced-colors` block hands the control back to the OS ([[RR-CBS3AC]]).

The focus ring is derived from `--accent-color` via `color-mix` rather than
copying the sibling widgets' hardcoded indigo, which is a literal that
actually renders and does not follow the theme ([[RR-CBS2QW]]).

## Follow-ups

- **Extract a shared `.rela-checkbox`** ([[RR-CBLEV8]], deferred). The visual
  now exists in two files — this widget and `RelationCards.vue` — which have
  already diverged, and `RelationCards:998` still carries the hardcoded indigo
  ring this ticket fixed here. `frontend/CLAUDE.md` documents the same
  trajectory for `properties-list.css`. The widget is the better copy, so the
  follow-up is to lift its version into `src/styles/` and adopt it in
  RelationCards — not to merge equals.
- **`forced-colors` is unaddressed everywhere else in the SPA.** This ticket
  fixed one control; `forced-colors` still appears nowhere else in `src/`.
  Worth a dedicated accessibility pass rather than fixing it one widget at a
  time as tickets happen to touch them.
