---
id: TKT-28FRME
type: ticket
title: Add color property type (#RRGGBB) with data-entry picker widget
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Goal

A new built-in property type `color` whose value is a CSS hex string `#RRGGBB`,
usable anywhere a scalar property is usable, with a real color picker in
data-entry instead of a text box.

**Depends on TKT-S9C14S** (widget-based display rendering in list cells and
kanban cards). Landing that first means the swatch is a single widget
registration instead of a per-surface special case in four render sites.

## Value representation

- Canonical form: `#RRGGBB`, seven characters, leading `#`.
- Validation regex: `^#[0-9A-Fa-f]{6}$`.
- Stored verbatim as a YAML string. No new Go representation, so
`internal/canonical` needs no change.
- No alpha, no named colors, no `rgb()`. One representation keeps CSS, the
`<input type="color">` element, and CalDAV's `calendar-color` in agreement.

Case is **preserved, not normalized**. Normalizing on write would rewrite the
operator's file on an unrelated save; validation is case-insensitive so both
`#3b82f6` and `#3B82F6` are accepted.

## Backend touch points

| File | Change |
|---|---|
| `internal/metamodel/types.go:404-413` | add `PropertyTypeColor = "color"` |
| `internal/metamodel/types.go:475-481` | add to `IsBuiltinType` |
| `internal/metamodel/validation.go:225` | new `case` in `validatePropertyValue` + `ParseColorValue` helper |
| `internal/metamodel/loader.go:409` | add to the display_property **deny** list (decided: a hex string is a poor entity label) |
| `internal/metamodel/schema_output.go:117` | `ResolveWidgetFromType` -> `color` |
| `internal/dataentryconfig/config.go:23` | `WidgetColor` constant |
| `internal/openapi/schemas.go:81` | JSON Schema: `string` + pattern |
| `internal/predicatefns/env.go:28` | `ScalarType` -> `StringType` |
| `internal/affordances/bindings.go:188` | `coerceScalar` — must stay in lockstep with `ScalarType` (RR-4189H) |
| `internal/filter/match.go:71,153` | match dispatch + allowed operators (`=`/`!=` only) |
| `internal/filter/sort.go:38,332` | string comparator |
| `internal/templating/fsloader.go:283` | default value |
| `docs/metamodel.md:588` | property-type table + dedicated section |

Confirmed **non**-touch points: `internal/markdown`, `internal/search`,
`internal/store` (all backends), `internal/validator` — none carry
per-property-type logic.

## Frontend touch points

With TKT-S9C14S landed, this is four files:

| File | Change |
|---|---|
| `frontend/src/types/schema.ts:22` | add `'color'` to the `PropertyDef['type']` union — the tripwire that surfaces everything else |
| `frontend/src/widgets/ColorWidget.vue` | new; implements both `mode: 'display'` and `mode: 'edit'` |
| `frontend/src/widgets/registry.ts:18-28` | `defaultWidgetFor` — add before the `text` fallback |
| `frontend/src/widgets/registry.ts:103-119` | register in `buildDefaultRegistry` |
| `frontend/src/widgets/types.ts:76-85` | add a `'color'` member to `WidgetHintKind` |
| `frontend/src/widgets/registry.ts:33-43` | add the `color` entry to `hintKindToWidgetName` |
| `frontend/src/widgets/viewRouting.ts` | route `type: 'color'` in the schema-driven hint builder added by TKT-S9C14S |

`utils/format.ts`, `EntityList.vue` and `KanbanView.vue` need **no change** —
that was only true after the refactor; before it, each needed its own swatch
branch.

## Widget behaviour

- **Edit mode**: `<input type="color">` (native picker) paired with a text input
accepting a typed hex, kept in sync. The native picker alone is unusable for
someone pasting a brand hex.
- **Display mode**: a swatch plus the hex value as text.
- **Empty value**: a neutral outlined chip reading `—`, not a black square
(`background: ''` collapses to transparent).
- The swatch needs a `1px` low-alpha `currentColor` border, or `#FFFFFF` is
invisible on light backgrounds and `#111827` vanishes in dark mode.

## Accessibility

A swatch must never be color-alone: render the hex value as text beside it, and
mark the swatch `aria-hidden="true"` (decorative once the text is present). A
color-blind or screen-reader user gets the same information.

## Testing

- `internal/metamodel` table test for `validatePropertyValue`: valid, missing
`#`, wrong length, non-hex chars, empty, non-string; both cases accepted.
- Loader test: `type: color` accepted as a property; **rejected** as a
`display_property`.
- `frontend/src/widgets/registry.test.ts:19-36` — add the `it.each` mapping row
(mandatory, else the dispatch is untested).
- `frontend/src/widgets/widgets.test.ts` — mount `ColorWidget` in both modes,
assert `update:modelValue`, assert the display branch does not render in edit
mode (RR-UD2D).
- Hint-builder test: `type: 'color'` routes to the color widget.
- E2E: set a color on an entity via the picker, reload, value persists.

## Validation error convention

Per `internal/dataentry/CLAUDE.md` lines 14-30 the data-entry write path is
write-then-warn: an invalid color returns 200 with a `{code, path, detail}`
warning, not a 400.

## Follow-on

The CalDAV/calendar-feed ticket consumes this property; not in scope here.
