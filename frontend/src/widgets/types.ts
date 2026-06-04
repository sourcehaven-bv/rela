import type { Component } from 'vue'
import type { PropertyDef } from '@/types'

// PropertyType mirrors PropertyDef['type'] from the metamodel schema.
// Kept as a named alias so widget entries can declare which property
// types they support; if PropertyDef['type'] gains a member, widgets
// that need updating surface as type errors here.
export type PropertyType = PropertyDef['type']

// Widget render mode. Strict union: `'inline-edit'` is reserved for
// TKT-IHCY7 and is not part of this contract yet — widening the union
// when it lands forces every widget consumer to be re-examined.
export type WidgetMode = 'display' | 'edit'

// WidgetProps is the contract every property widget accepts. Cross-cutting
// concerns (disabled, error, etc.) are first-class so widgets never reach
// into an untyped options blob for them; options carries only genuinely
// widget-specific config.
//
// `mode` is REQUIRED and has no default. Every consumer must pass it
// explicitly; that includes test mounts. A default would be silently
// load-bearing on every form (RR-UD1I).
export interface WidgetProps<T = unknown> {
  modelValue: T
  // The widget's render mode. Required.
  mode: WidgetMode
  propertyDef?: PropertyDef
  // The field's wire-level property binding (separate from
  // `propertyDef`, which is the schema entry). Forwarded to Badge in
  // display mode so badge styling survives view-config property
  // aliases (RR-UD1E). Falls back to `propertyDef.name` when absent.
  propertyName?: string
  disabled?: boolean
  required?: boolean
  error?: string
  id?: string
  placeholder?: string
  help?: string
  // Sparse per-option allow map (select / multi-select). Only `false`
  // entries appear; absent keys default to allowed.
  optionVerdicts?: Record<string, boolean>
  // Allowed status transitions keyed by current value (select only).
  transitions?: Record<string, string[]>
}

export interface WidgetEntry {
  component: Component
  // Advisory only: a mismatch logs a console.warn but the widget still
  // renders. Tightening to a hard reject is a deliberate follow-up.
  supportedPropertyTypes?: PropertyType[]
}

export interface WidgetRegistry {
  register(name: string, entry: WidgetEntry): void
  resolve(name: string | undefined, propertyDef?: PropertyDef): Component
}
