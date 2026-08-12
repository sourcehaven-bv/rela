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

// densePropertyRoutingHint maps a REAL PropertyDef to a WidgetRoutingHint for
// the dense read-only surfaces -- list/table cells and kanban card fields.
//
// Deliberately NOT viewFieldRoutingHint: that one routes any typed field to
// 'enum-list' to preserve a pre-refactor Badge behaviour, which would badge
// every typed column in a table. Deliberately NOT registry.resolve(propertyDef)
// either -- see the two routing exceptions below, both of which resolve() would
// get wrong for a dense surface.
//
// Callers have the real schema entry (entityType.properties[column.property]),
// so routing is by declared type rather than by a wire-level heuristic.
export function densePropertyRoutingHint(
  propertyDef: PropertyDef | undefined,
  propertyName: string
): WidgetRoutingHint {
  return { kind: denseHintKind(propertyDef), propertyName }
}

function denseHintKind(propertyDef: PropertyDef | undefined): WidgetHintKind {
  if (!propertyDef) return 'text'

  // Enum-ness wins over the scalar type, and list-ness picks the multi-value
  // widget. Order mirrors defaultWidgetFor (registry.ts): list before values
  // before scalar type.
  const isEnum = propertyDef.type === 'enum' || (propertyDef.values?.length ?? 0) > 0
  if (isEnum) {
    // A list-valued enum MUST route to 'enum-list'. SelectWidget's
    // safeStringValue console.warns on a multi-element array, which in a table
    // is one warning per row per render.
    return propertyDef.list === true ? 'enum-list' : 'enum'
  }
  if (propertyDef.list === true) return 'text-list'

  switch (propertyDef.type) {
    case 'date':
      return 'date'
    case 'datetime':
      return 'datetime'
    case 'integer':
      return 'integer'
    case 'rrule':
      return 'rrule'

    // --- Two deliberate exceptions. Do not "simplify" these to the matching
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

    default:
      return 'text'
  }
}
