---
id: TKT-00SFFP
type: ticket
title: Filter-bar controls are three different widget stacks that must be styled into agreement
kind: refactor
priority: low
effort: m
status: backlog
---

## Description

The list filter bar renders three unrelated control stacks side by side:

| Widget | Rendered by |
| --- | --- |
| multi-enum | `TagSelect` → SlimSelect (`div.ss-main`) |
| scalar enum | native `<select>` |
| relation | `EntityTargetSelect` (native `<select>` or typeahead) |
| free text | native `<input>` |

They share no styling source, so matching them is manual and re-breaks whenever
one side moves. BUG-AMK38R hit this: after swapping the multi-enum control to
`TagSelect`, it rendered taller, with a different corner radius and a font size
larger than the `<select>` beside it.

## The non-obvious part

The mismatch was **not** simply "nobody set the values." `TagSelect`'s `<style>`
block is global but lives in the **form** route chunk, which the list page never
loads — so on `/list/*` the `.ss-main` rules were absent entirely and the widget
fell back to browser defaults (16px text, no themed border). It looked
approximately right only by coincidence.

The fix duplicated the control styling into FilterBar's own chunk, nested under
`.filter-bar`. That works and is scoped safely, but it is a **second copy of the
same visual contract** — exactly the duplication pattern this repo keeps paying
for elsewhere (see the `TODO(TKT-HFEKVN)` note about three copies of the
temporal-layout list that "already disagree").

## Options

1. **Shared control-surface class** — one `.control-surface` (height, radius,
border, background, font, focus ring) in `styles/`, applied by every filter
control regardless of stack. Cheapest, keeps native selects native.
2. **Route TagSelect's base styles out of the form chunk** so any page using the
widget gets them. Fixes the root cause but needs care: the styles are global and
unlayered, so hoisting them changes cascade order for existing consumers.
3. **Make every filter control a SlimSelect** (the original suggestion when this
was spotted). Most visually uniform, but converts scalar enum filters away from
native `<select>` — losing native keyboard behavior and mobile pickers, and
widening the surface for a purely cosmetic gain. Weigh before choosing.

Option 1 or 2 is likely right; 3 is recorded so the tradeoff is not re-litigated
from scratch.

## Constraints

- `TagSelect.vue`'s `<style>` is **global, not scoped**, and shared with edit
forms and Settings. Anything moved there affects those surfaces.
- `:deep()` from FilterBar cannot reach `.ss-main` — TagSelect renders no
`data-v` attribute of the consuming component. This is why the current fix uses
a plain global block nested under `.filter-bar`.
- Style through **theme tokens**, never literals: the controls sit next to each
other in both light and dark mode, and `custom.css` must still be able to
restyle them (see the `@layer rela` rules in `frontend/CLAUDE.md`).
- Vitest does not apply scoped SFC styles, so a mounted-component assertion on
appearance passes against no CSS at all. Any regression guard has to read the
source (like `styles/focusRing.test.ts`) or measure in e2e.

## Acceptance criteria

1. The filter controls derive their shared surface styling from **one** source,
not a per-component copy.
2. A multi-enum, scalar-enum, relation and text filter in one row agree on
height, corner radius, border, background and font size.
3. Alignment holds when the tag picker wraps to a second row of chips, and on
the mobile breakpoint.
4. Correct in both light and dark mode, via tokens.
5. TagSelect's appearance in edit forms and Settings is unchanged.
6. A guard that fails if the surfaces diverge again — source-reading test or an
e2e measurement, not a mounted-component style assertion.
