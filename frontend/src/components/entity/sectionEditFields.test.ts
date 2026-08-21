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

// Fields opt in to inline edit (TKT-HOIX1): `render` defaults to 'display', so
// without it these suites would exercise the display path rather than the
// ACL-driven routing they exist to pin. The default itself is covered by the
// dedicated `render` describe block below.
function makeFields(): ViewSectionField[] {
  return [
    { property: 'title', label: 'Title', render: 'input' },
    { property: 'status', label: 'Status', render: 'input' },
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

  it('attaches per-field transitions from entry._transitions (TKT-3G93B8)', () => {
    const entry = makeEntity({
      _transitions: { status: [{ to: 'closed', label: 'Close', allowed: true }] },
    })
    const out = buildSectionEditFields(makeFields(), entry, schemaResolver)
    const status = out.find((f) => f.property === 'status')
    // A machine field carries its transitions → routes to StatusControl.
    expect(status?.transitions).toEqual([{ to: 'closed', label: 'Close', allowed: true }])
    // A non-machine field has no transitions → renders its ordinary widget.
    const title = out.find((f) => f.property === 'title')
    expect(title?.transitions).toBeUndefined()
  })

  it('passes an empty transitions list through (terminal machine field)', () => {
    // Key present with [] must survive so the detail view still routes to the
    // StatusControl (not a full enum select) at a terminal state.
    const entry = makeEntity({ _transitions: { status: [] } })
    const out = buildSectionEditFields(makeFields(), entry, schemaResolver)
    const status = out.find((f) => f.property === 'status')
    expect(status?.transitions).toEqual([])
  })

  it('leaves transitions undefined for a source without _transitions (list row)', () => {
    // A ViewEntity row carries no `_transitions`, so its status field keeps the
    // ordinary widget rather than a status control.
    const out = buildSectionEditFields(makeFields(), makeEntity(), schemaResolver)
    expect(out.find((f) => f.property === 'status')?.transitions).toBeUndefined()
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

  // ── render: input | display (TKT-HOIX1) ──────────────────────────────
  //
  // AC 9 / RR-8EISWO: an all-display section must NOT mount a
  // SectionEditForm. Mounting one would give an autosave host that can
  // never save, and would flip heading ownership via
  // sectionRendersOwnHeading for essentially every properties section.

  it('returns false when fields omit render, even with writable verdicts (AC 1)', () => {
    const section = makeSection([
      { property: 'title', label: 'Title' },
      { property: 'status', label: 'Status' },
    ])
    // No `_fields` at all → every verdict defaults writable. Pre-TKT-HOIX1
    // this returned true; display-by-default is the breaking change.
    expect(sectionShouldRouteToInlineEdit(section.fields, makeEntity(), schemaResolver)).toBe(false)
  })

  it('returns false when every field is explicitly render: display (AC 9)', () => {
    const section = makeSection([
      { property: 'title', label: 'Title', render: 'display' },
      { property: 'status', label: 'Status', render: 'display' },
    ])
    expect(sectionShouldRouteToInlineEdit(section.fields, makeEntity(), schemaResolver)).toBe(false)
  })

  it('returns true when at least one field opts in with render: input', () => {
    const section = makeSection([
      { property: 'title', label: 'Title', render: 'display' },
      { property: 'status', label: 'Status', render: 'input' },
    ])
    expect(sectionShouldRouteToInlineEdit(section.fields, makeEntity(), schemaResolver)).toBe(true)
  })

  it('returns false for render: input on an ACL-read-only field (AC 3)', () => {
    // Security-critical: config downgrades editability, never upgrades it.
    const section = makeSection([{ property: 'status', label: 'Status', render: 'input' }])
    const entry = makeEntity({ _fields: { status: { writable: false } } })
    expect(sectionShouldRouteToInlineEdit(section.fields, entry, schemaResolver)).toBe(false)
  })

  it('keeps the inaccessible guard ahead of render (AC 9)', () => {
    // An inaccessible field routes to PropertyDisplay for its lock UI even
    // when a sibling opts in to input.
    const section = makeSection([
      { property: 'title', label: 'Title', inaccessible: true },
      { property: 'status', label: 'Status', render: 'input' },
    ])
    expect(sectionShouldRouteToInlineEdit(section.fields, makeEntity(), schemaResolver)).toBe(false)
  })
})

describe('buildSectionEditFields — render passthrough (TKT-HOIX1)', () => {
  it('carries the server-resolved render onto each field', () => {
    const fields: ViewSectionField[] = [
      { property: 'title', label: 'Title', render: 'input' },
      { property: 'status', label: 'Status', render: 'display' },
    ]
    const out = buildSectionEditFields(fields, makeEntity(), schemaResolver)
    expect(out.map((f) => f.render)).toEqual(['input', 'display'])
  })

  it('leaves render undefined when the server omitted it', () => {
    // Undefined is not 'input', so the writable conjunct renders display —
    // the safe direction for a legacy/shape-drift payload.
    const out = buildSectionEditFields(
      [{ property: 'title', label: 'Title' }],
      makeEntity(),
      schemaResolver,
    )
    expect(out[0].render).toBeUndefined()
  })

  it('carries render through the kind:"hint" branch too', () => {
    const out = buildSectionEditFields(
      [{ property: 'unknown_prop', label: 'Unknown', render: 'input' }],
      makeEntity(),
      schemaResolver,
    )
    expect(out[0].kind).toBe('hint')
    expect(out[0].render).toBe('input')
  })

  it('does not fold render into verdict (RR-PGGRBD)', () => {
    // The verdict-flip watcher compares isFieldWritable(verdict) across prop
    // updates; a display field must not read as a revoked permission.
    const out = buildSectionEditFields(
      [{ property: 'title', label: 'Title', render: 'display' }],
      makeEntity(),
      schemaResolver,
    )
    expect(out[0].verdict).toBeUndefined()
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
    // render: 'input' for the same reason as makeFields() above (TKT-HOIX1).
    fields: [
      { property: 'title', label: 'Title', values: ['Some Row'], render: 'input' },
      { property: 'status', label: 'Status', values: ['open'], render: 'input' },
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

// TKT-3R7RF3: the `widget:` override is carried from the wire onto
// SectionEditField, on BOTH union arms. Which arm honours it is
// SectionEditForm's decision (widgetRows), not this builder's — carrying it
// uniformly is what keeps that decision in one place.
describe('widget override plumbing (TKT-3R7RF3)', () => {
  it('carries widget onto a schema-arm field', () => {
    const fields = buildSectionEditFields(
      [{ property: 'status', label: 'Status', widget: 'textarea' }],
      { type: 'ticket' },
      () => ({ type: 'enum', values: ['a'] }) as PropertyDef,
    )
    expect(fields[0].kind).toBe('schema')
    expect(fields[0].widget).toBe('textarea')
  })

  it('carries widget onto a hint-arm field too (dropped later, not here)', () => {
    const fields = buildSectionEditFields(
      [{ property: 'mystery', label: 'Mystery', widget: 'textarea' }],
      { type: 'ticket' },
      () => undefined,
    )
    expect(fields[0].kind).toBe('hint')
    // Carried, so the drop stays a single decision in widgetRows rather than
    // being spread across the builder as well (RR-2GBB0V).
    expect(fields[0].widget).toBe('textarea')
  })

  it('leaves widget undefined when the config omits it', () => {
    const fields = buildSectionEditFields(
      [{ property: 'status', label: 'Status' }],
      { type: 'ticket' },
      () => ({ type: 'enum', values: ['a'] }) as PropertyDef,
    )
    expect(fields[0].widget).toBeUndefined()
  })

  // The widget axis must not disturb the render/ACL routing decision.
  it('does not affect inline-edit routing', () => {
    const withWidget = sectionShouldRouteToInlineEdit(
      [{ property: 'status', label: 'Status', render: 'display', widget: 'textarea' }],
      { type: 'ticket', _fields: { status: { writable: true } } },
      () => ({ type: 'enum', values: ['a'] }) as PropertyDef,
    )
    // render: display still wins — a widget override is presentation, not
    // permission, and cannot promote a display field into an edit host.
    expect(withWidget).toBe(false)
  })
})
