import type { PropertyDef } from '@/types'
import type { WidgetEntry, WidgetRegistry, WidgetRoutingHint, WidgetHintKind } from './types'
import TextWidget from './TextWidget.vue'
import TextareaWidget from './TextareaWidget.vue'
import NumberWidget from './NumberWidget.vue'
import CheckboxWidget from './CheckboxWidget.vue'
import DateWidget from './DateWidget.vue'
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

function buildDefaultRegistry(): WidgetRegistry {
  const r = defineWidgetRegistry()
  r.register('text', { component: TextWidget, supportedPropertyTypes: ['string'] })
  r.register('textarea', { component: TextareaWidget, supportedPropertyTypes: ['string'] })
  r.register('number', { component: NumberWidget, supportedPropertyTypes: ['integer'] })
  r.register('checkbox', { component: CheckboxWidget, supportedPropertyTypes: ['boolean'] })
  r.register('date', { component: DateWidget, supportedPropertyTypes: ['date'] })
  r.register('select', { component: SelectWidget, supportedPropertyTypes: ['enum', 'string'] })
  r.register('multi-select', {
    component: MultiSelectWidget,
    supportedPropertyTypes: ['enum', 'string'],
  })
  r.register('rrule', { component: RruleWidget, supportedPropertyTypes: ['rrule'] })
  r.register('file', { component: FileWidget, supportedPropertyTypes: ['file'] })
  return r
}

export const defaultRegistry: WidgetRegistry = buildDefaultRegistry()
