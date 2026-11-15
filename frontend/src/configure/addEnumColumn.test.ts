import { describe, it, expect } from 'vitest'
import type { ConfigSnapshot, TreeValue } from '@/api/configure'
import { addEnumColumn, type AddColumnTarget } from './addEnumColumn'
import { getIn, newMap } from './tree'

const target: AddColumnTarget = { kanbanId: 'board', entityType: 'risk', property: 'stage' }

function snapshot(
  opts: { columns?: TreeValue[]; type?: string; labels?: Record<string, string> } = {}
): ConfigSnapshot {
  const stage = newMap({
    type: opts.type ?? 'enum',
    values: ['open', 'decided'],
    labels: opts.labels ? newMap(opts.labels) : undefined,
  })
  const board = newMap({ entity_type: 'risk', column_property: 'stage', columns: opts.columns })
  return {
    version: 'v1',
    schema: newMap({ entities: newMap({ risk: newMap({ properties: newMap({ stage }) }) }) }),
    data_entry: newMap({ kanbans: newMap({ board }) }),
    editable: {},
  }
}

const stagePath = ['entities', 'risk', 'properties', 'stage']

describe('addEnumColumn', () => {
  it('appends the value and its label to the inline enum', () => {
    const snap = snapshot()
    const edit = addEnumColumn(snap, target, '  In review ')
    if (!('draft' in edit)) throw new Error(`refused: ${edit.reason}`)
    expect(edit.value).toBe('in_review')
    expect(edit.draft.base_version).toBe(snap.version)
    expect(getIn(edit.draft.schema, [...stagePath, 'values'])).toEqual([
      'open',
      'decided',
      'in_review',
    ])
    expect(getIn(edit.draft.schema, [...stagePath, 'labels', 'in_review'])).toBe('In review')
    // The board lists no columns, so data-entry.yaml is left alone.
    expect(edit.draft.data_entry).toBeUndefined()
    // The snapshot itself is untouched.
    expect(getIn(snap.schema, [...stagePath, 'values'])).toEqual(['open', 'decided'])
  })

  it('writes no label when the label is the value', () => {
    const edit = addEnumColumn(snapshot(), target, 'parked')
    if (!('draft' in edit)) throw new Error(`refused: ${edit.reason}`)
    expect(getIn(edit.draft.schema, [...stagePath, 'labels'])).toBeUndefined()
  })

  it('appends to the board columns when it lists them', () => {
    const columns = [
      newMap({ value: 'open', label: 'Open' }),
      newMap({ value: 'decided', label: 'Decided' }),
    ]
    const edit = addEnumColumn(snapshot({ columns }), target, 'Parked')
    if (!('draft' in edit)) throw new Error(`refused: ${edit.reason}`)
    const listed = getIn(edit.draft.data_entry, ['kanbans', 'board', 'columns']) as TreeValue[]
    expect(listed).toHaveLength(3)
    expect(listed[2]).toEqual(newMap({ value: 'parked', label: 'Parked' }))
  })

  it.each([
    { name: 'an empty name', label: '  ', want: { reason: 'no_name' } },
    { name: 'symbols only', label: '!!!', want: { reason: 'no_name' } },
    { name: 'an existing value', label: 'Open', want: { reason: 'exists', name: 'open' } },
    { name: 'an existing label', label: 'on hold', want: { reason: 'exists', name: 'On hold' } },
  ])('refuses $name', ({ label, want }) => {
    const snap = snapshot({ labels: { decided: 'On hold' } })
    expect(addEnumColumn(snap, target, label)).toEqual(want)
  })

  it('shows a value the board leaves out, without a schema change', () => {
    const columns = [newMap({ value: 'open', label: 'Open' })]
    const snap = snapshot({ columns, labels: { decided: 'Decided' } })
    const edit = addEnumColumn(snap, target, 'DECIDED')
    if (!('draft' in edit)) throw new Error(`refused: ${edit.reason}`)
    expect(edit.value).toBe('decided')
    expect(edit.draft.schema).toBeUndefined()
    const listed = getIn(edit.draft.data_entry, ['kanbans', 'board', 'columns']) as TreeValue[]
    expect(listed[1]).toEqual(newMap({ value: 'decided', label: 'Decided' }))
  })

  it('refuses a board column label in another case', () => {
    const columns = [newMap({ value: 'open', label: 'To do' })]
    expect(addEnumColumn(snapshot({ columns }), target, 'TO DO')).toEqual({
      reason: 'exists',
      name: 'To do',
    })
  })

  it('refuses a property that is not an inline enum', () => {
    expect(addEnumColumn(snapshot({ type: 'risk_stage' }), target, 'Parked')).toEqual({
      reason: 'unsupported',
    })
    expect(addEnumColumn(snapshot(), { ...target, property: 'missing' }, 'Parked')).toEqual({
      reason: 'unsupported',
    })
  })
})
