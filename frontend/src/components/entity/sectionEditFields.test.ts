import { describe, it, expect } from 'vitest'
import {
  buildSectionEditFields,
  sectionShouldRouteToInlineEdit,
  applyPropertyToEntry,
  applyPropertyToRow,
  rowShouldRouteToInlineEdit,
} from './sectionEditFields'
import type { ViewEntity, ViewSection, ViewSectionField } from '@/api'
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

describe('sectionShouldRouteToInlineEdit', () => {
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
    expect(sectionShouldRouteToInlineEdit(section.fields, makeEntity(), schemaResolver)).toBe(true)
  })

  it('returns true when entry._fields is {} (evaluated, no deviations)', () => {
    const section = makeSection(makeFields())
    const entry = makeEntity({ _fields: {} })
    expect(sectionShouldRouteToInlineEdit(section.fields, entry, schemaResolver)).toBe(true)
  })

  it('returns false when all listed fields are explicitly non-writable', () => {
    const section = makeSection([{ property: 'status', label: 'Status' }])
    const entry = makeEntity({ _fields: { status: { writable: false } } })
    expect(sectionShouldRouteToInlineEdit(section.fields, entry, schemaResolver)).toBe(false)
  })

  it('returns true when at least one field is writable', () => {
    const section = makeSection(makeFields())
    const entry = makeEntity({ _fields: { status: { writable: false } } })
    // title has no verdict → defaults writable
    expect(sectionShouldRouteToInlineEdit(section.fields, entry, schemaResolver)).toBe(true)
  })

  it('returns false when any field is inaccessible (git-crypt etc.)', () => {
    // Even though the field is otherwise writable per `_fields`, the
    // inaccessible affordance (lock placeholder) is only rendered by
    // PropertyDisplay; route there so the lock UI is preserved.
    const section = makeSection([
      { property: 'title', label: 'Title', inaccessible: true },
      { property: 'status', label: 'Status' },
    ])
    expect(sectionShouldRouteToInlineEdit(section.fields, makeEntity(), schemaResolver)).toBe(false)
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

// TKT-IHC7C — parameterized helpers + applyPropertyToRow ------------------

function makeRow(overrides: Partial<ViewEntity> = {}): ViewEntity {
  return {
    id: 'TKT-002',
    title: 'Some Row',
    type: 'ticket',
    hasContent: false,
    fields: [
      { property: 'title', label: 'Title', values: ['Some Row'] },
      { property: 'status', label: 'Status', values: ['open'] },
    ],
    _props: { title: 'Some Row', status: 'open' },
    ...overrides,
  } as ViewEntity
}

describe('buildSectionEditFields — parameterized over FieldVerdictSource', () => {
  it('accepts a ViewEntity (cards/list row) as the source', () => {
    const row = makeRow({ _fields: { status: { writable: false } } })
    const fields = makeFields()
    const out = buildSectionEditFields(fields, row, schemaResolver)
    expect(out).toHaveLength(2)
    const status = out.find((f) => f.property === 'status')
    expect(status?.verdict?.writable).toBe(false)
  })

  it('falls back to hint kind for a ViewEntity row whose type is not in the schema', () => {
    const row = makeRow({ type: 'unknown_type' })
    const fields: ViewSectionField[] = [{ property: 'title', label: 'Title' }]
    const out = buildSectionEditFields(fields, row, schemaResolver)
    expect(out[0].kind).toBe('hint')
  })
})

describe('sectionShouldRouteToInlineEdit — parameterized', () => {
  it('returns true for a ViewEntity row with at least one writable field', () => {
    const row = makeRow({ _fields: { status: { writable: false } } })
    expect(sectionShouldRouteToInlineEdit(makeFields(), row, schemaResolver)).toBe(true)
  })

  it('returns false for a ViewEntity row with all fields non-writable', () => {
    const row = makeRow({
      _fields: { title: { writable: false }, status: { writable: false } },
    })
    expect(sectionShouldRouteToInlineEdit(makeFields(), row, schemaResolver)).toBe(false)
  })

  it('returns false for a ViewEntity row with any inaccessible field', () => {
    const row = makeRow()
    const fields: ViewSectionField[] = [
      { property: 'title', label: 'Title', inaccessible: true },
      { property: 'status', label: 'Status' },
    ]
    expect(sectionShouldRouteToInlineEdit(fields, row, schemaResolver)).toBe(false)
  })
})

describe('applyPropertyToRow', () => {
  it('returns null for null/undefined input', () => {
    expect(applyPropertyToRow(null, 'title', 'x', { type: 'ticket', id: 'TKT-002' })).toBeNull()
    expect(applyPropertyToRow(undefined, 'title', 'x', { type: 'ticket', id: 'TKT-002' })).toBeNull()
  })

  it('rejects stale owner (different id)', () => {
    const row = makeRow({ id: 'TKT-003' })
    expect(applyPropertyToRow(row, 'title', 'leaked', { type: 'ticket', id: 'TKT-002' })).toBeNull()
  })

  it('rejects stale owner (different type)', () => {
    const row = makeRow()
    expect(applyPropertyToRow(row, 'title', 'leaked', { type: 'feature', id: 'TKT-002' })).toBeNull()
  })

  it('produces a new row with patched _props when owner matches', () => {
    const row = makeRow()
    const result = applyPropertyToRow(row, 'title', 'New', { type: 'ticket', id: 'TKT-002' })
    expect(result?._props?.title).toBe('New')
    expect(result?._props?.status).toBe('open') // unchanged
    expect(result).not.toBe(row) // new reference
    expect(result?._props).not.toBe(row._props) // new _props reference
  })

  it('deletes the key when value is undefined', () => {
    const row = makeRow()
    const result = applyPropertyToRow(row, 'title', undefined, { type: 'ticket', id: 'TKT-002' })
    expect(result?._props).not.toHaveProperty('title')
    expect(result?._props?.status).toBe('open')
  })

  it('does NOT touch fields[i].values (string mirror; RR-FC1C)', () => {
    // Display-mode reads _props first, so the string mirror is left
    // intentionally stale. Verifying this guarantees the verdict-flip
    // race condition stays closed.
    const row = makeRow()
    const result = applyPropertyToRow(row, 'title', 'New', { type: 'ticket', id: 'TKT-002' })
    expect(result?.fields).toBe(row.fields) // same reference
  })

  it('handles a row with no _props (legacy server / shape drift)', () => {
    const row = makeRow()
    delete row._props
    const result = applyPropertyToRow(row, 'title', 'New', { type: 'ticket', id: 'TKT-002' })
    expect(result?._props).toEqual({ title: 'New' })
  })
})

describe('rowShouldRouteToInlineEdit (TKT-IHC7C cap behaviour)', () => {
  const CAP = 100

  it('returns false for a row without _props (legacy fallback)', () => {
    const row = makeRow()
    delete row._props
    expect(rowShouldRouteToInlineEdit(row, 10, CAP, schemaResolver)).toBe(false)
  })

  it('returns true under the cap with a writable row', () => {
    const row = makeRow({ _fields: {} })
    expect(rowShouldRouteToInlineEdit(row, 50, CAP, schemaResolver)).toBe(true)
  })

  it('returns true at exactly the cap', () => {
    const row = makeRow({ _fields: {} })
    expect(rowShouldRouteToInlineEdit(row, 100, CAP, schemaResolver)).toBe(true)
  })

  it('returns false when rowCount exceeds the cap (RR-FC1D + RR-FC2C)', () => {
    const row = makeRow({ _fields: {} })
    expect(rowShouldRouteToInlineEdit(row, 101, CAP, schemaResolver)).toBe(false)
  })

  it('returns false for an inaccessible field even under the cap', () => {
    const row = makeRow({
      fields: [{ property: 'title', label: 'Title', inaccessible: true }],
    })
    expect(rowShouldRouteToInlineEdit(row, 1, CAP, schemaResolver)).toBe(false)
  })

  it('returns false when every field is non-writable', () => {
    const row = makeRow({
      _fields: { title: { writable: false }, status: { writable: false } },
    })
    expect(rowShouldRouteToInlineEdit(row, 1, CAP, schemaResolver)).toBe(false)
  })
})
