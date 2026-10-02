import { describe, it, expect } from 'vitest'
import type { ListColumn } from '@/types/config'
import {
  columnHeader,
  columnKey,
  listColumnOf,
  nameColumn,
  toTableColumns,
} from './tableColumns'

/**
 * The column key is the one thing here that fails silently when it is wrong.
 *
 * It names the `cell-<key>` slot, and Vue resolves slot names at runtime: a
 * slot matching no column renders empty, with nothing from vue-tsc, eslint or
 * this suite to say so. These tests therefore assert the KEYS themselves and
 * the properties that make them safe — distinctness and stability — rather
 * than only that a conversion happened.
 */
describe('columnKey', () => {
  it('distinguishes a property from a relation of the same name', () => {
    expect(columnKey({ property: 'blocks' })).not.toBe(columnKey({ relation: 'blocks' }))
  })

  it('cannot collide with the placeholder a malformed column gets', () => {
    // Asserted because the kind prefix is otherwise unpinned: the relation
    // key carries its own prefix, so dropping the property one still leaves
    // property and relation keys distinct and the test above passes anyway.
    // A bare property name is one `col:unnamed` away from colliding with
    // malformed config, and that collision is silent.
    expect(columnKey({ property: 'unnamed' })).not.toBe(columnKey({}))
    expect(columnKey({ property: 'col:unnamed' })).not.toBe(columnKey({}))
  })

  it('distinguishes the two directions of one relation type', () => {
    // A list may show both halves of a relation at once, which is the case a
    // bare relation name would merge into a single slot.
    const outgoing = columnKey({ relation: 'blocks', direction: 'outgoing' })
    const incoming = columnKey({ relation: 'blocks', direction: 'incoming' })
    expect(outgoing).not.toBe(incoming)
  })

  it('treats an unspecified direction as outgoing', () => {
    expect(columnKey({ relation: 'blocks' })).toBe(
      columnKey({ relation: 'blocks', direction: 'outgoing' }),
    )
  })

  it('ignores the label, so renaming a column keeps its slot', () => {
    // The label is operator-facing prose. If it fed the key, an operator
    // retitling a column in data-entry.yaml would silently detach its cell
    // slot, and the column would go blank rather than error.
    expect(columnKey({ property: 'due', label: 'Due date' })).toBe(
      columnKey({ property: 'due', label: 'Deadline' }),
    )
  })
})

describe('face column', () => {
  it('has its own key and a header of its own', () => {
    expect(columnKey({ face: true })).toBe('face')
    expect(columnHeader({ face: true })).toBe('Face')
    expect(columnHeader({ face: true, label: 'Status' })).toBe('Status')
  })
})

describe('toTableColumns', () => {
  /** The first column is the title, which the library's table owns. */
  const columns: ListColumn[] = [
    { property: 'title', label: 'Title' },
    { property: 'status', label: 'Status' },
    { relation: 'blocks', direction: 'incoming', label: 'Blocked by' },
  ]

  it('drops the title column, which the table renders itself', () => {
    const result = toTableColumns(columns)
    expect(result).toHaveLength(2)
    expect(result.map((c) => c.header)).toEqual(['Status', 'Blocked by'])
  })

  it('gives every column a distinct key', () => {
    const keys = toTableColumns(columns).map((c) => c.key)
    expect(new Set(keys).size).toBe(keys.length)
  })

  it('keeps keys distinct even when config is malformed', () => {
    // Two columns naming neither a property nor a relation both reach the
    // placeholder. Left alone they would share one `cell-<key>` slot and the
    // second would render the first's contents.
    const malformed: ListColumn[] = [{ property: 'title' }, {}, {}, {}]
    const keys = toTableColumns(malformed).map((c) => c.key)
    expect(new Set(keys).size).toBe(3)
  })

  it('falls back to the property name when no label is given', () => {
    expect(columnHeader({ property: 'status' })).toBe('status')
    expect(columnHeader({ relation: 'blocks' })).toBe('blocks')
  })

  it('marks a property column sortable unless the config opts out', () => {
    const [status] = toTableColumns([{ property: 'title' }, { property: 'status' }])
    expect(status.sortable).toBe(true)

    const [optedOut] = toTableColumns([
      { property: 'title' },
      { property: 'status', sortable: false },
    ])
    expect(optedOut.sortable).toBe(false)
  })

  it('never marks a relation column sortable', () => {
    // Sorting is pushed to the server as a property name; a relation column
    // has none, so a sort control on it would emit a key the API cannot use.
    const [relation] = toTableColumns([{ property: 'title' }, { relation: 'blocks' }])
    expect(relation.sortable).toBe(false)
  })

  it('sets no field, so every cell renders through its slot', () => {
    // `field` is the library's plain-text path. rela has no plain-text cells:
    // each one may be a property widget, a relation chip, a lock or a badge.
    // A field set here would render raw text underneath a slot that failed to
    // match, which is the failure this whole module exists to avoid.
    for (const column of toTableColumns(columns)) {
      expect(column.field).toBeUndefined()
    }
  })

  it('carries the originating config through meta', () => {
    const [status] = toTableColumns(columns)
    expect(listColumnOf(status)).toBe(columns[1])
  })
})

describe('nameColumn', () => {
  it('is the first column, by the convention the card layout already uses', () => {
    const columns: ListColumn[] = [{ property: 'title' }, { property: 'status' }]
    expect(nameColumn(columns)).toBe(columns[0])
  })

  it('is undefined when the list declares no columns', () => {
    expect(nameColumn([])).toBeUndefined()
  })
})
