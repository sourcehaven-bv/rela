import type { Component } from 'vue'
import type { PropertyDef, AttachmentInfo } from '@/types'

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
//
// `propertyDef` is typically present in edit mode (forms have the schema
// entry) and may be absent in display mode (cards/list resolve widgets
// via WidgetRoutingHint, not a real PropertyDef). Widgets that need
// schema info in display mode must tolerate its absence.
export interface WidgetProps<T = unknown> {
  modelValue: T
  // The widget's render mode. Required.
  mode: WidgetMode
  propertyDef?: PropertyDef
  // The field's wire-level property binding (the metamodel property
  // name). Required so Badge style lookup is deterministic across all
  // call sites; passing '' is acceptable for fields with no semantic
  // binding (RR-UD2D).
  propertyName: string
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
  // Attachment metadata for a `file`-type property, sourced from the
  // entity's `_attachments` map. The LIST of files currently on the
  // property (a property may hold several when `max` > 1); empty/absent
  // when none. The file widget renders download links / previews from it.
  // Ignored by non-file widgets.
  attachments?: AttachmentInfo[]
  // The property's attachment cap (metamodel `max`, default 1). Drives the
  // file widget's mode: replace at 1, add-up-to-max above.
  max?: number
  // Owning entity identity, supplied to the file widget so it can build
  // the upload/delete URL in edit mode. Optional — present only on the
  // file-property edit path; other widgets ignore it.
  entityType?: string
  entityId?: string
}

// WidgetRoutingHint is a lightweight description used by the view-side
// rendering path to pick a widget without inventing a fake PropertyDef
// (RR-UD2B). Forms still resolve via the real PropertyDef they own.
//
// `kind` maps to the same widget bucket defaultWidgetFor would have
// picked, but is explicit instead of being inferred from a synthetic
// shape that lies about being a schema entry.
export type WidgetHintKind =
  | 'text'
  | 'text-list'
  | 'enum'
  | 'enum-list'
  | 'boolean'
  | 'date'
  | 'integer'
  | 'rrule'

export interface WidgetRoutingHint {
  kind: WidgetHintKind
  // The field's wire-level property binding -- forwarded to widgets as
  // their `propertyName` prop.
  propertyName: string
}

export interface WidgetEntry {
  component: Component
  // Advisory only: a mismatch logs a console.warn but the widget still
  // renders. Tightening to a hard reject is a deliberate follow-up.
  supportedPropertyTypes?: PropertyType[]
}

export interface WidgetRegistry {
  register(name: string, entry: WidgetEntry): void
  // Form-side resolution: caller has a real schema entry. The historical
  // multi-axis fallback (list -> multi-select, values -> select, etc.)
  // applies via defaultWidgetFor.
  resolve(name: string | undefined, propertyDef?: PropertyDef): Component
  // View-side resolution: caller has wire-level field metadata, not a
  // schema entry. Hint is explicit; no schema lookup is involved
  // (RR-UD2B / RR-UD2A).
  resolveFromHint(hint: WidgetRoutingHint): Component
}
