import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { defaultWidgetFor, defaultRegistry, WIDGET_REGISTRATIONS } from '../registry'
import type { PropertyDef } from '@/types'

// TKT-3R7RF3 / RR-Z0GGTO: the other half of the cross-language drift guard.
// Go's sectionFieldWidgetTypes (internal/dataentryconfig/validate.go) validates
// a `widget:` override at config load; this registry decides what actually
// renders. Both assert against the SAME fixture, so neither can move alone.
//
// This asserts WIDGET_REGISTRATIONS — the array buildDefaultRegistry actually
// consumes. An earlier version re-declared the registrations inside the test
// and so asserted its own copy: mutating registry.ts left all 12 tests green,
// which is worse than no guard because the comment claimed otherwise. If you
// change this test, verify it still FAILS when you mutate a
// supportedPropertyTypes entry in registry.ts.
const FIXTURE = resolve(
  __dirname,
  '../../../../internal/dataentryconfig/testdata/widget_property_types.json',
)

function loadFixture(): Record<string, string[]> {
  const raw = JSON.parse(readFileSync(FIXTURE, 'utf-8')) as Record<string, string[]>
  return Object.fromEntries(Object.entries(raw).map(([k, v]) => [k, [...v].sort()]))
}

describe('widget/property-type table agrees with the Go validator', () => {
  it('the REAL registrations match the shared fixture exactly', () => {
    const actual = Object.fromEntries(
      WIDGET_REGISTRATIONS.map((e) => [e.name, [...e.supportedPropertyTypes].sort()]),
    )
    expect(actual).toEqual(loadFixture())
  })

  it('every fixture widget resolves to a real component in the default registry', () => {
    // The names are the contract between server and client: a name the server
    // accepts but the registry cannot resolve would silently fall back to the
    // type default. resolve() returns the fallback rather than throwing, so
    // compare against a KNOWN-unregistered name to prove these are distinct.
    const fallback = defaultRegistry.resolve('definitely-not-a-widget', {
      type: 'string',
    } as PropertyDef)
    for (const name of Object.keys(loadFixture())) {
      const component = defaultRegistry.resolve(name, undefined)
      expect(component, `widget "${name}" did not resolve`).toBeTruthy()
      if (name !== 'text') {
        expect(component, `widget "${name}" resolved to the fallback`).not.toBe(fallback)
      }
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
