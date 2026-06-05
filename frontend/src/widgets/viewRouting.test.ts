import { describe, it, expect, vi } from 'vitest'
import type { ViewSectionField } from '@/api'
import { viewFieldRoutingHint } from './viewRouting'
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
