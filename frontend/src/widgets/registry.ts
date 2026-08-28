import type { Component } from 'vue'
import type { PropertyDef } from '@/types'
import type { WidgetEntry, WidgetRegistry, WidgetRoutingHint, WidgetHintKind } from './types'
import TextWidget from './TextWidget.vue'
import TextareaWidget from './TextareaWidget.vue'
import NumberWidget from './NumberWidget.vue'
import CheckboxWidget from './CheckboxWidget.vue'
import DateWidget from './DateWidget.vue'
import DatetimeWidget from './DatetimeWidget.vue'
import SelectWidget from './SelectWidget.vue'
import MultiSelectWidget from './MultiSelectWidget.vue'
import RruleWidget from './RruleWidget.vue'
import FileWidget from './FileWidget.vue'

// defaultWidgetFor reproduces FieldRenderer's historical dispatch order
// exactly (RR-0Z1P6). Order matters: `list` wins over `values`, which
// wins over scalar type. Changing this order is a behaviour change and a
// separate ticket.
export function defaultWidgetFor(propertyDef?: PropertyDef): string {
  if (propertyDef?.list === true) return 'multi-select'
  if ((propertyDef?.values?.length ?? 0) > 0) return 'select'
  if (propertyDef?.type === 'boolean') return 'checkbox'
  if (propertyDef?.type === 'date') return 'date'
  if (propertyDef?.type === 'datetime') return 'datetime'
  if (propertyDef?.type === 'integer') return 'number'
  if (propertyDef?.type === 'rrule') return 'rrule'
  if (propertyDef?.type === 'file') return 'file'
  return 'text'
}

// hintKindToWidgetName maps a WidgetRoutingHint kind to the registered
// widget name. View-side callers use this via resolveFromHint instead of
// inventing a synthetic PropertyDef (RR-UD2B).
const hintKindToWidgetName: Record<WidgetHintKind, string> = {
  text: 'text',
  'text-list': 'multi-select',
  enum: 'select',
  'enum-list': 'multi-select',
  boolean: 'checkbox',
  date: 'date',
  datetime: 'datetime',
  integer: 'number',
  rrule: 'rrule',
}

export function defineWidgetRegistry(): WidgetRegistry {
  const entries = new Map<string, WidgetEntry>()

  return {
    register(name, entry) {
      if (entries.has(name)) {
        console.warn(`[widget-registry] re-registering widget "${name}"`)
      }
      entries.set(name, entry)
    },

    resolve(name, propertyDef) {
      // An explicit widget name wins; falls back to type-based defaulting.
      const requested = name && name.trim() !== '' ? name : undefined
      const resolvedName = requested ?? defaultWidgetFor(propertyDef)

      let entry = entries.get(resolvedName)
      if (!entry) {
        if (requested) {
          console.warn(
            `[widget-registry] unknown widget "${requested}"; falling back to type default`
          )
        }
        entry = entries.get(defaultWidgetFor(propertyDef))
      }
      if (!entry) {
        // text is the universal fallback and is always registered.
        entry = entries.get('text')
      }
      if (!entry) {
        throw new Error('[widget-registry] no widget could be resolved (text widget missing)')
      }

      const ptype = propertyDef?.type
      if (
        ptype &&
        entry.supportedPropertyTypes &&
        !entry.supportedPropertyTypes.includes(ptype)
      ) {
        console.warn(
          `[widget-registry] widget "${resolvedName}" does not declare support for property type "${ptype}"`
        )
      }

      return entry.component
    },

    resolveFromHint(hint: WidgetRoutingHint) {
      const name = hintKindToWidgetName[hint.kind]
      const entry = entries.get(name) ?? entries.get('text')
      if (!entry) {
        throw new Error('[widget-registry] no widget could be resolved (text widget missing)')
      }
      return entry.component
    },
  }
}

// WIDGET_REGISTRATIONS is the single list buildDefaultRegistry consumes, and
// the one the cross-language drift guard asserts against (TKT-3R7RF3).
//
// It is exported DATA rather than a sequence of register() calls inside the
// builder because a test cannot observe those calls: the registry exposes no
// enumeration, so the guard had to re-declare them — which meant it asserted
// its own copy and stayed green while the real registrations drifted. Keeping
// the list addressable makes the guard structurally unable to miss a change.
export const WIDGET_REGISTRATIONS: ReadonlyArray<{
  name: string
  component: Component
  supportedPropertyTypes: NonNullable<WidgetEntry['supportedPropertyTypes']>
}> = [
  { name: 'text', component: TextWidget, supportedPropertyTypes: ['string'] },
  { name: 'textarea', component: TextareaWidget, supportedPropertyTypes: ['string'] },
  { name: 'number', component: NumberWidget, supportedPropertyTypes: ['integer'] },
  { name: 'checkbox', component: CheckboxWidget, supportedPropertyTypes: ['boolean'] },
  { name: 'date', component: DateWidget, supportedPropertyTypes: ['date'] },
  { name: 'datetime', component: DatetimeWidget, supportedPropertyTypes: ['datetime'] },
  { name: 'select', component: SelectWidget, supportedPropertyTypes: ['enum', 'string'] },
  {
    name: 'multi-select',
    component: MultiSelectWidget,
    supportedPropertyTypes: ['enum', 'string'],
  },
  { name: 'rrule', component: RruleWidget, supportedPropertyTypes: ['rrule'] },
  { name: 'file', component: FileWidget, supportedPropertyTypes: ['file'] },
]

function buildDefaultRegistry(): WidgetRegistry {
  const r = defineWidgetRegistry()
  for (const { name, component, supportedPropertyTypes } of WIDGET_REGISTRATIONS) {
    r.register(name, { component, supportedPropertyTypes })
  }
  return r
}

export const defaultRegistry: WidgetRegistry = buildDefaultRegistry()
