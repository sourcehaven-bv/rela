import { describe, expect, it } from 'vitest'
import { changeCount, describeChanges, diffList } from '../changes'
import {
  clone,
  getIn,
  isMap,
  listAt,
  moveItem,
  remove,
  renameKey,
  set,
  type TreeMap,
} from '../tree'
import { snapshot, toTree } from './fixture'

function at(tree: unknown, path: (string | number)[]): TreeMap {
  const node = getIn(tree as never, path)
  if (!isMap(node)) throw new Error(`no mapping at ${path.join('.')}`)
  return node
}

describe('describeChanges', () => {
  const base = snapshot()

  it('reports nothing for an unchanged draft', () => {
    const groups = describeChanges(
      { schema: base.schema, dataEntry: base.data_entry },
      { schema: clone(base.schema), dataEntry: clone(base.data_entry) },
      []
    )
    expect(groups).toEqual([])
    expect(changeCount(groups)).toBe(0)
  })

  it('words each kind of change, grouped by what it belongs to', () => {
    const schema = clone(base.schema)
    const dataEntry = clone(base.data_entry)
    renameKey(at(schema, ['entities', 'ticket', 'properties']), 'effort', 'estimate')
    set(at(schema, ['types', 'status']), 'values', ['open', 'done', 'blocked'])
    set(at(schema, ['entities', 'feature']), 'label', 'Epic')
    const fields = listAt(dataEntry, ['forms', 'new_ticket', 'fields'])!
    moveItem(fields, fields[2], 0)
    const styles = at(dataEntry, ['styles', 'status'])
    set(styles, 'blocked', 'red')
    remove(styles, 'open')

    const groups = describeChanges(
      { schema: base.schema, dataEntry: base.data_entry },
      { schema, dataEntry },
      [{ entity_type: 'ticket', from: 'effort', to: 'estimate' }]
    )

    const flat = groups.flatMap((g) => g.items.map((i) => `${g.title}: ${i.kind} ${i.label}`))
    expect(flat).toEqual([
      'Ticket: changed Property estimate',
      'Epic: changed Name',
      'status: added Option blocked',
      'status: removed Option doing',
      'status: changed Colour of open',
      'New ticket: changed Order of fields',
    ])
    const rename = groups[0].items[0]
    expect(rename).toMatchObject({ detail: 'renamed', before: 'effort', after: 'estimate' })
    expect(rename.to).toBe('/configure/entity-types/ticket?property=estimate')
    expect(changeCount(groups)).toBe(6)
  })

  it('never shows the configuration format', () => {
    const schema = clone(base.schema)
    set(at(schema, ['entities', 'ticket', 'properties', 'effort']), 'required', true)
    const groups = describeChanges(
      { schema: base.schema, dataEntry: base.data_entry },
      { schema, dataEntry: base.data_entry },
      []
    )
    const text = JSON.stringify(groups)
    expect(text).not.toMatch(/\$m|\$i|yaml|properties\./)
  })
})

describe('diffList', () => {
  it('matches items by their original index and spots a reorder', () => {
    const before = toTree([{ a: 1 }, { a: 2 }, { a: 3 }])
    const after = clone(before) as unknown[]
    moveItem(after, after[2], 0)
    const diff = diffList(before, after as never)
    expect(diff.reordered).toBe(true)
    expect(diff.added).toEqual([])
    expect(diff.removed).toEqual([])
  })
})
