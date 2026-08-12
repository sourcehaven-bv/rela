---
id: TKT-S9C14S
type: ticket
title: Render list cells and kanban card fields through the widget registry
kind: enhancement
priority: high
effort: m
status: review
---

## Goal

Finish the display-side widget migration. `PropertyDisplay` and `EntityDetail`
already resolve read-only rendering through the widget registry; list/table
cells and kanban card fields were left behind on a string-only path. Move them
onto the same seam, with `text` as the universal fallback.

This is the missing display-side increment of FEAT-72NR1. The three existing
tickets under that feature (TKT-IHCY7, TKT-HOIX1, TKT-GUPMK) are all inline-edit
work and none covers these two surfaces.

## Why now

`formatCellValue` returns a `string`, so a cell can only ever be text. Any
property type whose useful rendering is not text — a color swatch, a progress
bar, a rating, an image thumbnail — is unrepresentable. Adding a per-type branch
per surface is what the widget registry already exists to avoid; TKT-28FRME
(color property type) is blocked behind this and is the immediate motivating
consumer.

## Approach

Follow the `PropertyDisplay.vue:52-107` model exactly:

1. A precomputed `computed` resolving the widget **per column / per card
field**, not per cell (`PropertyDisplay`'s `rows`, cited RR-UD2A).
2. `<component :is>` with an explicit `:mode="'display'"` literal — `mode` is
required and deliberately has no default (RR-UD1I).
3. The ACL branch stays a **sibling** `v-if` outside the widget; no widget
models lock state.

### Widget resolution: a new schema-driven hint builder

Add a `propertyDef -> WidgetHintKind` builder in `widgets/viewRouting.ts`
alongside the existing `viewFieldRoutingHint`. Do **not** reuse either existing
path:

- `viewFieldRoutingHint` maps any truthy `propType` to `enum-list`, which is
documented as bug-compatibility with pre-refactor Badge behaviour, not a correct
type mapping. Adopting it in tables would badge every typed column.
- Bare `registry.resolve(propertyDef)` routes `file` to `FileWidget`, whose
display branch renders `<img>` previews — one network request per image per
cell. It also lets a list-valued enum reach `SelectWidget`, whose
`safeStringValue` `console.warn`s on multi-element arrays: one warning per row
per render.

The new builder is schema-driven (list views have the real `PropertyDef` via
`entityType.properties[column.property]`), routes list-valued enums to
`enum-list`, and inherits the safety property that `WidgetHintKind` has no
`'file'` member — a file column cannot reach `FileWidget` through a hint.

### Per-type routing decisions

| Type | Cell/card routing | Rationale |
|---|---|---|
| boolean | `text` (`Yes`/`No`) | **Not** `checkbox`. Preserves today's appearance and keeps cells text-searchable and copy-pasteable. The checkbox stays the detail-view rendering. |
| enum (scalar) | `enum` | single Badge, as today |
| enum (list) | `enum-list` | multi-select's Badge loop; also fixes kanban, which today renders one Badge of a comma-joined string |
| file | `text` | never `FileWidget` in a dense context |
| date / datetime / integer / rrule / string | matching widget | |
| relation | unchanged string path | no `PropertyDef`, no relation widget in the registry |

## Behaviour changes (intentional, not silent)

**Kanban gains formatting it never had.** `getCardFieldValue`
(`KanbanView.vue:336-355`) is bare `String(v || '')` and never called
`formatCellValue`, so cards today show datetimes as raw ISO strings, booleans as
`"true"`, rrules as `"FREQ=DAILY"`. The `|| ''` also collapses `0` and `false`
to empty, which makes `visibleCardFields` **drop the field from the card
entirely** — a `false` boolean silently vanishes. The migration fixes all of
this; it is a bug fix riding along, and both the ticket and the PR should say
so.

List cells are otherwise visually unchanged.

## Must not break

- **String-emptiness predicates.** `visibleMobileColumns`
(`EntityList.vue:365-368`) and `visibleCardFields` (`KanbanView.vue:360-362`)
use `formatted !== ''` to decide whether a field appears at all. These are
*filters*, not render paths. Keep them on a string source or convert to a
value-level emptiness check. If they break, mobile cards show dangling labels
with no values.
- `formatValue` stays — `RruleWidget:20` depends on it.
- `formatCellValue` stays for the emptiness predicate. Note that after this
change it is no longer a rendering path.

## Four render sites

No shared cell component exists, so the same change lands in four places; miss
one and they drift:

- `EntityList.vue:913` — desktop `<td>`
- `EntityList.vue:812` — mobile card (structural duplicate)
- `KanbanView.vue:530` — simple board
- `KanbanView.vue:610` — swimlane board

Extracting a shared cell component is optional but would prevent the next drift.

## Performance

Widgets are safe to mount densely: none of `TextWidget`, `NumberWidget`,
`DateWidget`, `CheckboxWidget`, `SelectWidget`, `MultiSelectWidget`,
`RruleWidget`, `FileWidget` has a lifecycle hook, event listener, observer,
dynamic import, or fetch. The only dependencies are `useStringValue` (one
computed) and `useSchemaStore()` (a Pinia singleton lookup).

Two real notes:

- **`FileWidget` is the exception** — its display branch renders `<img>`
previews, i.e. a network request per image per cell. Handled by routing `file`
to `text` above.
- **No rrule cache.** Vue's `computed` already is the cache: `RruleWidget`'s
`displayValue` depends only on `stringValue`, so it re-parses on dependency
change, not per render. Stable `:key`s are required so instances are reused. The
residual cost is identical RRULE strings across rows each parsing once on first
render — sub-millisecond `toText()` calls. Profile before optimising; if it
shows up, a bounded (~200-entry, oldest-out) cache belongs in the widget's
module scope, **not** in `format.ts` — a pure leaf formatter should not carry
hidden mutable state.

Fix while here: the enum branch calls `getCellValue` twice per cell per render
(`EntityList.vue:922,924`); precomputing rows removes it.

## Config surface (optional, can defer)

Neither `ListColumn` (`types/config.ts:256-264`) nor `KanbanCardField`
(`:330-335`) has `widget?: string`, so authored per-column widget overrides are
not possible. `FormField.widget` (`:99`) is the naming precedent. Adding it
needs a backend config-validator counterpart — reasonable to split into a
follow-up.

Unrelated latent bug found nearby: `ListColumn.width` is declared but never read
anywhere, so authored column widths are silently ignored.

## Testing

- Unit-test the new hint builder per property type, including the two
deliberate routings (boolean → text, file → text) so a future "simplification"
to `resolve(propertyDef)` fails loudly.
- `EntityList` mount test: enum column still badges, boolean still shows
`Yes`/`No`, relation column still shows joined titles, inaccessible cell still
shows the lock.
- `KanbanView` test pinning the fixed behaviour: `false` renders as `No` and the
field is **not** dropped; a datetime renders formatted, not as raw ISO.
- Mobile-card test that an empty column is still hidden (the emptiness
predicate).
- `npm run test:run`, `npm run typecheck`, `npm run lint`.
