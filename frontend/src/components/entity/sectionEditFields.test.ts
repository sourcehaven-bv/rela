import { describe, it, expect } from 'vitest'
import {
  buildSectionEditFields,
  sectionHasAnyWritable,
  applyPropertyToEntry,
} from './sectionEditFields'
import type { ViewSection, ViewSectionField } from '@/api'
import type { Entity, PropertyDef } from '@/types'

const TEXT_DEF: PropertyDef = { type: 'string' } as PropertyDef
const ENUM_DEF: PropertyDef = { type: 'enum', values: ['open', 'closed'] } as PropertyDef

function makeEntity(overrides: Partial<Entity> = {}): Entity {
  return {
    id: 'TKT-001',
    type: 'ticket',
    properties: { title: 'Original', status: 'open' },
    ...overrides,
  } as Entity
}

function makeFields(): ViewSectionField[] {
  return [
    { property: 'title', label: 'Title' },
    { property: 'status', label: 'Status' },
  ]
}

const schemaResolver = (entityType: string, prop: string): PropertyDef | undefined => {
  if (entityType !== 'ticket') return undefined
  if (prop === 'title') return TEXT_DEF
  if (prop === 'status') return ENUM_DEF
  return undefined
}

describe('buildSectionEditFields', () => {
  it('returns [] for undefined fields', () => {
    expect(buildSectionEditFields(undefined, makeEntity(), schemaResolver)).toEqual([])
  })

  it('filters out fields without a property name (RR-FB1J)', () => {
    const fields: ViewSectionField[] = [
      { property: 'title', label: 'Title' },
      { label: 'Detached Label' },
    ]
    const out = buildSectionEditFields(fields, makeEntity(), schemaResolver)
    expect(out).toHaveLength(1)
    expect(out[0].property).toBe('title')
  })

  it('resolves to kind:"schema" when PropertyDef is found', () => {
    const out = buildSectionEditFields(makeFields(), makeEntity(), schemaResolver)
    expect(out[0].kind).toBe('schema')
    if (out[0].kind === 'schema') {
      expect(out[0].propertyDef).toBe(TEXT_DEF)
    }
  })

  it('falls back to kind:"hint" when PropertyDef is not found', () => {
    const fields: ViewSectionField[] = [{ property: 'unknown_prop', label: 'Unknown' }]
    const out = buildSectionEditFields(fields, makeEntity(), schemaResolver)
    expect(out[0].kind).toBe('hint')
    if (out[0].kind === 'hint') {
      expect(out[0].routingHint.propertyName).toBe('unknown_prop')
    }
  })

  it('attaches per-field verdict from entry._fields', () => {
    const entry = makeEntity({ _fields: { status: { writable: false } } })
    const out = buildSectionEditFields(makeFields(), entry, schemaResolver)
    const status = out.find((f) => f.property === 'status')
    expect(status?.verdict?.writable).toBe(false)
    const title = out.find((f) => f.property === 'title')
    expect(title?.verdict).toBeUndefined()
  })
})

describe('sectionHasAnyWritable', () => {
  function makeSection(fields: ViewSectionField[]): ViewSection {
    return {
      heading: 'Test',
      sectionId: 'test',
      display: 'properties',
      isEmpty: false,
      fields,
      isGrouped: false,
      hasContent: false,
    } as ViewSection
  }

  it('returns true when entry._fields is undefined (default writable)', () => {
    const section = makeSection(makeFields())
    expect(sectionHasAnyWritable(section, makeEntity(), schemaResolver)).toBe(true)
  })

  it('returns true when entry._fields is {} (evaluated, no deviations)', () => {
    const section = makeSection(makeFields())
    const entry = makeEntity({ _fields: {} })
    expect(sectionHasAnyWritable(section, entry, schemaResolver)).toBe(true)
  })

  it('returns false when all listed fields are explicitly non-writable', () => {
    const section = makeSection([{ property: 'status', label: 'Status' }])
    const entry = makeEntity({ _fields: { status: { writable: false } } })
    expect(sectionHasAnyWritable(section, entry, schemaResolver)).toBe(false)
  })

  it('returns true when at least one field is writable', () => {
    const section = makeSection(makeFields())
    const entry = makeEntity({ _fields: { status: { writable: false } } })
    // title has no verdict → defaults writable
    expect(sectionHasAnyWritable(section, entry, schemaResolver)).toBe(true)
  })
})

describe('applyPropertyToEntry', () => {
  it('returns null when entry is null/undefined', () => {
    expect(applyPropertyToEntry(null, 'title', 'x', { type: 'ticket', id: 'TKT-001' })).toBeNull()
    expect(applyPropertyToEntry(undefined, 'title', 'x', { type: 'ticket', id: 'TKT-001' })).toBeNull()
  })

  it('returns null when owner identity mismatches (stale-response guard, RR-FB2A)', () => {
    const entry = makeEntity({ id: 'TKT-002' }) // current entity is B
    const result = applyPropertyToEntry(entry, 'title', 'leaked', { type: 'ticket', id: 'TKT-001' })
    expect(result).toBeNull()
  })

  it('returns null when owner type mismatches', () => {
    const entry = makeEntity()
    const result = applyPropertyToEntry(entry, 'title', 'leaked', { type: 'feature', id: 'TKT-001' })
    expect(result).toBeNull()
  })

  it('produces a new entry with the patched property when owner matches', () => {
    const entry = makeEntity()
    const result = applyPropertyToEntry(entry, 'title', 'New', { type: 'ticket', id: 'TKT-001' })
    expect(result?.properties.title).toBe('New')
    expect(result?.properties.status).toBe('open') // unchanged
    expect(result).not.toBe(entry) // new reference
    expect(result?.properties).not.toBe(entry.properties) // new properties reference
  })

  it('deletes the key when value is undefined (RR-FB2D NEW-5)', () => {
    const entry = makeEntity()
    const result = applyPropertyToEntry(entry, 'title', undefined, { type: 'ticket', id: 'TKT-001' })
    expect(result?.properties).not.toHaveProperty('title')
    expect(result?.properties.status).toBe('open')
  })
})
