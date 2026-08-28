import { describe, it, expect, vi } from 'vitest'
import type { ViewSectionField } from '@/api'
import type { PropertyDef } from '@/types'
import { viewFieldRoutingHint, densePropertyRoutingHint } from './viewRouting'
import { defaultRegistry } from './registry'
import TextWidget from './TextWidget.vue'
import MultiSelectWidget from './MultiSelectWidget.vue'

describe('viewFieldRoutingHint', () => {
  it('routes propType-having fields to enum-list', () => {
    const field: ViewSectionField = {
      label: 'Status',
      property: 'status',
      propType: 'concept_status',
      values: ['stable'],
    }
    expect(viewFieldRoutingHint(field)).toEqual({
      kind: 'enum-list',
      propertyName: 'concept_status',
    })
  })

  it('falls back to text for single-value text fields with no propType', () => {
    const field: ViewSectionField = { label: 'Note', property: 'note', values: ['hi'] }
    expect(viewFieldRoutingHint(field)).toEqual({ kind: 'text', propertyName: 'note' })
  })

  it('routes multi-value text fields with no propType to text-list', () => {
    const field: ViewSectionField = { label: 'Tags', property: 'tags', values: ['a', 'b'] }
    expect(viewFieldRoutingHint(field)).toEqual({ kind: 'text-list', propertyName: 'tags' })
  })

  it('emits empty propertyName when both propType and property are absent', () => {
    const field: ViewSectionField = { label: 'Untagged' }
    expect(viewFieldRoutingHint(field)).toEqual({ kind: 'text', propertyName: '' })
  })

  it('is referentially stable across repeated calls (no schema lookup, no allocation in hot path) (RR-UD2L)', () => {
    // Hint-routing is pure: no schemaStore subscription, no Map walk.
    // If a future change pulls schemaStore into the hint path, this
    // test should be updated DELIBERATELY (a behaviour change), not
    // silently.
    const field: ViewSectionField = { label: 'Status', propType: 'status', values: ['open'] }
    const a = viewFieldRoutingHint(field)
    const b = viewFieldRoutingHint(field)
    expect(a).toEqual(b)
    expect(a.kind).toBe('enum-list')
  })
})

describe('densePropertyRoutingHint', () => {
  const def = (p: Partial<PropertyDef>): PropertyDef => {
    const base: PropertyDef = { type: 'string' }
    return { ...base, ...p }
  }

  it.each([
    ['string', def({ type: 'string' }), 'text'],
    ['date', def({ type: 'date' }), 'date'],
    ['datetime', def({ type: 'datetime' }), 'datetime'],
    ['integer', def({ type: 'integer' }), 'integer'],
    ['rrule', def({ type: 'rrule' }), 'rrule'],
    ['enum', def({ type: 'enum', values: ['a', 'b'] }), 'enum'],
    ['enum via values on a string', def({ type: 'string', values: ['a'] }), 'enum'],
    ['list-valued enum', def({ type: 'enum', values: ['a'], list: true }), 'enum-list'],
  ])('routes %s to %s', (_label, propertyDef, expected) => {
    expect(densePropertyRoutingHint(propertyDef, 'p').kind).toBe(expected)
  })

  // A non-enum list must NOT reach MultiSelectWidget on a dense surface: it
  // badges each element and em-dashes an empty array (RR-UD2C, a detail-view
  // contract), where cells must stay visually quiet. Letting list-ness win
  // over the type also erased the type's formatter -- a list-valued rrule
  // rendered an em-dash instead of "every day".
  it.each([
    ['string', def({ type: 'string', list: true })],
    ['date', def({ type: 'date', list: true })],
    ['datetime', def({ type: 'datetime', list: true })],
    ['integer', def({ type: 'integer', list: true })],
    ['rrule', def({ type: 'rrule', list: true })],
  ])('routes list-valued %s to preformatted text, never text-list', (_label, propertyDef) => {
    const hint = densePropertyRoutingHint(propertyDef, 'p')
    expect(hint.kind).toBe('text')
    expect(hint.preformatted).toBe(true)
  })

  it('marks passthrough widgets preformatted and formatter-owning widgets not', () => {
    // preformatted === "the widget renders String(value) and nothing else, so
    // the caller must pre-format". Getting this wrong renders a boolean as
    // "true" or double-formats a date.
    const cases: [PropertyDef, boolean][] = [
      [def({ type: 'string' }), true],
      [def({ type: 'boolean' }), true],
      [def({ type: 'file' }), true],
      [def({ type: 'integer' }), true],
      [def({ type: 'date' }), false],
      [def({ type: 'datetime' }), false],
      [def({ type: 'rrule' }), false],
      [def({ type: 'enum', values: ['a'] }), false],
      [def({ type: 'enum', values: ['a'], list: true }), false],
    ]
    for (const [propertyDef, expected] of cases) {
      expect(densePropertyRoutingHint(propertyDef, 'p').preformatted).toBe(expected)
    }
  })

  it('never routes a dense surface to text-list', () => {
    // text-list -> MultiSelectWidget, whose empty-array em-dash is wrong for
    // cells. Enum lists intentionally still use enum-list (badges are correct
    // for enums); this guards the NON-enum list types.
    const types: PropertyDef['type'][] = ['string', 'date', 'datetime', 'integer', 'file', 'rrule']
    for (const t of types) {
      expect(densePropertyRoutingHint(def({ type: t, list: true }), 'p').kind).not.toBe('text-list')
    }
  })

  it('forwards the property name onto the hint', () => {
    expect(densePropertyRoutingHint(def({ type: 'string' }), 'title')).toEqual({
      kind: 'text',
      propertyName: 'title',
      preformatted: true,
    })
  })

  it('falls back to text for an unknown property (no schema entry)', () => {
    expect(densePropertyRoutingHint(undefined, 'mystery').kind).toBe('text')
  })

  // --- The two deliberate exceptions. These are behaviour decisions from
  // TKT-S9C14S, not oversights; a change here must be intentional. ---

  it('routes boolean to text, NOT checkbox, so cells stay searchable and copy-pasteable', () => {
    expect(densePropertyRoutingHint(def({ type: 'boolean' }), 'done').kind).toBe('text')
  })

  it('routes file to text, NOT a file widget, to avoid one image request per cell', () => {
    expect(densePropertyRoutingHint(def({ type: 'file' }), 'attachment').kind).toBe('text')
  })

  it('never emits a hint kind that resolves to FileWidget', () => {
    // WidgetHintKind has no 'file' member, so the hint path structurally
    // cannot reach FileWidget. This pins that safety property against a
    // future kind being added without revisiting dense surfaces.
    const kinds = (['string', 'file', 'date', 'boolean', 'enum', 'rrule'] as const).map(
      (t) => densePropertyRoutingHint(def({ type: t as PropertyDef['type'] }), 'p').kind
    )
    expect(kinds).not.toContain('file')
  })

  it('does not warn for any built-in property type', () => {
    // resolveFromHint must not hit the supportedPropertyTypes mismatch path;
    // in a 200-row table that would be one warning per row per render.
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const types: PropertyDef['type'][] = [
      'string',
      'date',
      'datetime',
      'integer',
      'boolean',
      'enum',
      'file',
      'rrule',
    ]
    for (const t of types) {
      defaultRegistry.resolveFromHint(densePropertyRoutingHint(def({ type: t }), 'p'))
    }
    expect(warn).not.toHaveBeenCalled()
    warn.mockRestore()
  })
})

describe('defaultRegistry.resolveFromHint', () => {
  it('routes text hint to TextWidget', () => {
    expect(defaultRegistry.resolveFromHint({ kind: 'text', propertyName: '' })).toBe(TextWidget)
  })

  it('routes enum-list hint to MultiSelectWidget', () => {
    expect(defaultRegistry.resolveFromHint({ kind: 'enum-list', propertyName: 'status' })).toBe(
      MultiSelectWidget
    )
  })

  it('routes text-list hint to MultiSelectWidget (RR-UD2A: one schema lookup per kind, never per cell)', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    expect(defaultRegistry.resolveFromHint({ kind: 'text-list', propertyName: 'tags' })).toBe(
      MultiSelectWidget
    )
    // resolveFromHint does NOT walk supportedPropertyTypes (that's
    // resolve()'s job). No warnings should fire for plain hint lookups.
    expect(warn).not.toHaveBeenCalled()
    warn.mockRestore()
  })
})
