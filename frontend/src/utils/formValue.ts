import type { PropertyDef } from '@/types'

// isClearedForType: does the widget's emitted value mean "user cleared
// this property" (route to `properties_unset` on PATCH)? Booleans
// stay a legitimate value at any state; arrays clear by emptying;
// scalars clear via empty string / null / undefined.
//
// Lives here so DynamicForm and SectionEditForm route clears
// identically.
export function isClearedForType(value: unknown, def: PropertyDef | undefined): boolean {
  if (def?.type === 'boolean') return false
  if (Array.isArray(value)) return value.length === 0
  return value === '' || value === null || value === undefined
}
