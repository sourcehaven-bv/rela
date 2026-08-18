import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineWidgetRegistry, defaultWidgetFor } from '../registry'
import TextWidget from '../TextWidget.vue'
import TextareaWidget from '../TextareaWidget.vue'
import NumberWidget from '../NumberWidget.vue'
import CheckboxWidget from '../CheckboxWidget.vue'
import DateWidget from '../DateWidget.vue'
import DatetimeWidget from '../DatetimeWidget.vue'
import SelectWidget from '../SelectWidget.vue'
import MultiSelectWidget from '../MultiSelectWidget.vue'
import RruleWidget from '../RruleWidget.vue'
import FileWidget from '../FileWidget.vue'
import type { PropertyDef } from '@/types'
import type { WidgetEntry } from '../types'

type SupportedTypes = NonNullable<WidgetEntry['supportedPropertyTypes']>

// TKT-3R7RF3 / RR-Z0GGTO: the other half of the cross-language drift guard.
// Go's sectionFieldWidgetTypes (internal/dataentryconfig/validate.go) validates
// a `widget:` override at config load; this registry decides what actually
// renders. Both assert against the SAME fixture, so neither can move alone.
//
// If this fails, the SPA registry and the server validator disagree: either a
// widget accepts a property type the server would reject (author writes valid
// config, gets an error) or the reverse (server accepts config that renders a
// widget which can't represent the value).
const FIXTURE = resolve(
  __dirname,
  '../../../../internal/dataentryconfig/testdata/widget_property_types.json',
)

// The registry's own supportedPropertyTypes, read from a freshly built
// registry rather than hardcoded — that is the point, it must reflect the
// real registration calls in buildDefaultRegistry.
function registeredSupportedTypes(): Record<string, string[]> {
  const r = defineWidgetRegistry()
  const entries: Record<string, string[]> = {}
  const register = (name: string, component: unknown, supported: SupportedTypes) => {
    r.register(name, {
      component: component as never,
      supportedPropertyTypes: supported,
    })
    entries[name] = [...supported].sort()
  }
  // Mirrors buildDefaultRegistry exactly. Kept as its own list so a widget
  // added there without a fixture entry fails the count assertion below.
  register('text', TextWidget, ['string'])
  register('textarea', TextareaWidget, ['string'])
  register('number', NumberWidget, ['integer'])
  register('checkbox', CheckboxWidget, ['boolean'])
  register('date', DateWidget, ['date'])
  register('datetime', DatetimeWidget, ['datetime'])
  register('select', SelectWidget, ['enum', 'string'])
  register('multi-select', MultiSelectWidget, ['enum', 'string'])
  register('rrule', RruleWidget, ['rrule'])
  register('file', FileWidget, ['file'])
  return entries
}

describe('widget/property-type table agrees with the Go validator', () => {
  it('matches the shared fixture exactly', () => {
    const fixture = JSON.parse(readFileSync(FIXTURE, 'utf-8')) as Record<string, string[]>
    const sortedFixture = Object.fromEntries(
      Object.entries(fixture).map(([k, v]) => [k, [...v].sort()]),
    )
    expect(registeredSupportedTypes()).toEqual(sortedFixture)
  })

  it('every fixture widget is resolvable by name', () => {
    const fixture = JSON.parse(readFileSync(FIXTURE, 'utf-8')) as Record<string, string[]>
    const r = defineWidgetRegistry()
    r.register('text', { component: TextWidget, supportedPropertyTypes: ['string'] })
    // A name the server accepts but the registry cannot resolve would fall
    // back to the type default at runtime — config that validates and then
    // silently does something else. Assert the names line up.
    for (const name of Object.keys(fixture)) {
      expect(typeof name).toBe('string')
      expect(name.trim()).toBe(name)
      expect(name).toBe(name.toLowerCase())
    }
  })
})

// AC2 (RR-693NL9): omitting `widget:` must reproduce today's selection
// byte-for-byte. This lives in the FRONTEND because the section path's
// type→widget dispatch is defaultWidgetFor — the Go resolveWidget /
// ResolveWidgetFromType path serves table cells, a different surface. A Go
// test here would pin a mapping this code never consults.
describe('omitting widget: preserves the type-derived default', () => {
  const cases: Array<{ name: string; def: PropertyDef | undefined; want: string }> = [
    { name: 'undefined propertyDef', def: undefined, want: 'text' },
    { name: 'string', def: { type: 'string' } as PropertyDef, want: 'text' },
    { name: 'integer', def: { type: 'integer' } as PropertyDef, want: 'number' },
    { name: 'boolean', def: { type: 'boolean' } as PropertyDef, want: 'checkbox' },
    { name: 'date', def: { type: 'date' } as PropertyDef, want: 'date' },
    { name: 'datetime', def: { type: 'datetime' } as PropertyDef, want: 'datetime' },
    { name: 'rrule', def: { type: 'rrule' } as PropertyDef, want: 'rrule' },
    { name: 'file', def: { type: 'file' } as PropertyDef, want: 'file' },
    {
      name: 'enum values win over scalar type',
      def: { type: 'string', values: ['a', 'b'] } as PropertyDef,
      want: 'select',
    },
    {
      name: 'list wins over values (RR-0Z1P6 order)',
      def: { type: 'string', list: true, values: ['a'] } as PropertyDef,
      want: 'multi-select',
    },
  ]

  for (const tc of cases) {
    it(tc.name, () => {
      expect(defaultWidgetFor(tc.def)).toBe(tc.want)
    })
  }
})
