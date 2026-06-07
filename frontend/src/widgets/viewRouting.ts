import type { ViewSectionField } from '@/api'
import type { WidgetRoutingHint } from './types'

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
