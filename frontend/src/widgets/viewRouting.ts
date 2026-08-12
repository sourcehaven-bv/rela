import type { ViewSectionField } from '@/api'
import type { PropertyDef } from '@/types'
import type { WidgetRoutingHint, WidgetHintKind } from './types'

// viewFieldRoutingHint maps a wire-level ViewSectionField (cards/list
// rendering input) to a WidgetRoutingHint, so the view-side rendering
// path doesn't synthesise fake PropertyDef objects (RR-UD2B).
//
// Heuristic (preserves the pre-refactor "propType truthy -> Badge per
// value" behaviour from EntityDetail's cards/list inline rendering):
//
//   propType set       -> 'enum-list'   (renders via MultiSelectWidget,
//                                        loops one Badge per value;
//                                        empty array shows em-dash)
//   propType empty +   -> 'text-list'   (renders via MultiSelectWidget;
//   multi-value           Badge styling falls back to no colour)
//   propType empty +   -> 'text'        (renders via TextWidget as a
//   single value          plain span)
//
// propertyName is forwarded into the widget as :propertyName so Badge
// looks up styles deterministically (RR-UD2D).
export function viewFieldRoutingHint(field: ViewSectionField): WidgetRoutingHint {
  const propertyName = field.propType ?? field.property ?? ''
  if (field.propType) {
    return { kind: 'enum-list', propertyName }
  }
  if ((field.values?.length ?? 0) > 1) {
    return { kind: 'text-list', propertyName }
  }
  return { kind: 'text', propertyName }
}

// DenseRoutingHint extends the widget-routing decision with the SHAPE the
// chosen widget expects, because on a dense surface those are one decision,
// not two.
//
// `preformatted: true` means the widget does no formatting of its own (it is
// a passthrough span), so the caller must hand it an already-formatted
// string. `false` means the widget has a display-mode formatter and wants the
// raw value.
//
// Keeping these together is load-bearing: an earlier version tracked the
// shape separately as `hint.kind === 'text'` at the two call sites, which
// silently mis-shaped every kind that is neither 'text' nor a real formatter
// (notably 'text-list') and would have mis-shaped each new widget by default.
export interface DenseRoutingHint extends WidgetRoutingHint {
  preformatted: boolean
}

// isDenseEmpty reports whether a value should render as nothing on a dense
// surface (a blank table cell, a dropped card field).
//
// Dense surfaces and detail views disagree here, deliberately. A widget's
// display mode may render a placeholder for "no value" -- MultiSelectWidget
// em-dashes an empty array (RR-UD2C) so it reads as distinct from a loading
// state. formatCellValue documents the opposite contract for cells: "blank
// table cells stay visually quiet". A sparsely-populated tags column would
// otherwise become a column of dashes.
//
// So the CELL decides emptiness before the widget ever sees the value, and
// the two surfaces share this one predicate rather than each inventing it.
export function isDenseEmpty(value: unknown): boolean {
  if (value === null || value === undefined || value === '') return true
  return Array.isArray(value) && value.length === 0
}

// densePropertyRoutingHint maps a REAL PropertyDef to a DenseRoutingHint for
// the dense read-only surfaces -- list/table cells and kanban card fields.
//
// Deliberately NOT viewFieldRoutingHint: that one routes any typed field to
// 'enum-list' to preserve a pre-refactor Badge behaviour, which would badge
// every typed column in a table. Deliberately NOT registry.resolve(propertyDef)
// either -- see the routing exceptions below, which resolve() would get wrong
// for a dense surface.
//
// Callers have the real schema entry (entityType.properties[column.property]),
// so routing is by declared type rather than by a wire-level heuristic.
export function densePropertyRoutingHint(
  propertyDef: PropertyDef | undefined,
  propertyName: string
): DenseRoutingHint {
  const kind = denseHintKind(propertyDef)
  return { kind, propertyName, preformatted: PASSTHROUGH_KINDS.has(kind) }
}

// Widget kinds with NO display-mode formatter of their own: they render
// String(value) and nothing else, so the cell must pre-format. Every other
// kind ('date', 'datetime', 'rrule', 'enum', 'enum-list') owns its display
// rendering and must receive the raw value.
//
// A new widget defaults to NOT being here, i.e. it receives the raw value --
// which is the right default, since the reason to add a widget at all is
// that it renders something text cannot.
const PASSTHROUGH_KINDS: ReadonlySet<WidgetHintKind> = new Set<WidgetHintKind>([
  'text',
  'integer',
])

function denseHintKind(propertyDef: PropertyDef | undefined): WidgetHintKind {
  if (!propertyDef) return 'text'

  // Enum-ness wins over the scalar type: an enum's display rendering is a
  // Badge regardless of the underlying storage type.
  const isEnum = propertyDef.type === 'enum' || (propertyDef.values?.length ?? 0) > 0
  if (isEnum) {
    // A list-valued enum MUST route to 'enum-list'. SelectWidget's
    // safeStringValue console.warns on a multi-element array, which in a table
    // is one warning per row per render.
    return propertyDef.list === true ? 'enum-list' : 'enum'
  }

  // A non-enum LIST routes to text and is pre-formatted, whatever its type.
  //
  // The scalar widgets (DateWidget, RruleWidget, NumberWidget) take a single
  // value and would mangle an array; MultiSelectWidget would badge it and
  // em-dash it when empty (a detail-view contract, RR-UD2C, that cells
  // explicitly do not share). Handing the whole array to formatCellValue
  // reproduces the pre-migration string byte for byte -- including its own
  // limitations (a list of dates joins unformatted; a list of rrules renders
  // only the first). Improving THAT is a behaviour change and a separate
  // ticket; this refactor must not silently alter it.
  //
  // Note this check sits AFTER the enum branch but BEFORE the type switch:
  // list-ness only overrides a scalar type's formatter, never enum badging.
  if (propertyDef.list === true) return 'text'

  switch (propertyDef.type) {
    case 'date':
      return 'date'
    case 'datetime':
      return 'datetime'
    case 'rrule':
      return 'rrule'

    // --- Deliberate exceptions. Do not "simplify" these to the matching
    // widget; each is load-bearing and pinned by a test. ---

    // boolean -> text, NOT 'boolean'/CheckboxWidget. Cells keep today's
    // Yes/No text so they stay searchable (Cmd-F) and copy-pasteable. The
    // disabled checkbox remains the DETAIL-view rendering, where those
    // concerns don't apply.
    case 'boolean':
      return 'text'

    // file -> text, NOT a file widget. FileWidget's display branch renders
    // <img> previews, i.e. one network request per image per cell; 200 rows
    // of a file column would issue 200 image fetches. (WidgetHintKind has no
    // 'file' member, so the hint path cannot reach FileWidget anyway -- this
    // case is here to make the intent explicit rather than incidental.)
    case 'file':
      return 'text'

    case 'integer':
      return 'integer'

    default:
      return 'text'
  }
}
