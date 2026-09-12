import { describe, it, expect } from 'vitest'
import { Schema } from '@milkdown/kit/prose/model'
import { EditorState, TextSelection } from '@milkdown/kit/prose/state'
import {
  isInTable,
  isInHeaderRow,
  isOnlyBodyRow,
  canAddRowBefore,
  TABLE_COMMANDS,
  DIRECT_TABLE_COMMANDS,
  GUARDED_TABLE_COMMANDS,
} from './tableCommands'

// Mirrors the GFM table shape: a header row of `table_header` plus body rows
// of `table_cell`, which is what the header guard keys off.
const schema = new Schema({
  nodes: {
    doc: { content: 'block+' },
    paragraph: { group: 'block', content: 'inline*' },
    table: { group: 'block', content: 'table_header_row table_row*' },
    table_header_row: { content: 'table_header+' },
    table_row: { content: 'table_cell+' },
    table_header: { content: 'paragraph+' },
    table_cell: { content: 'paragraph+' },
    text: { group: 'inline' },
  },
  marks: {},
})

const para = (t: string) => schema.node('paragraph', null, t ? [schema.text(t)] : [])

function tableDoc(bodyRows = 1) {
  const header = schema.node('table_header_row', null, [
    schema.node('table_header', null, [para('a')]),
    schema.node('table_header', null, [para('b')]),
  ])
  const rows = Array.from({ length: bodyRows }, (_, i) =>
    schema.node('table_row', null, [
      schema.node('table_cell', null, [para(i === 0 ? 'c' : `c${i}`)]),
      schema.node('table_cell', null, [para(i === 0 ? 'd' : `d${i}`)]),
    ])
  )
  return schema.node('doc', null, [schema.node('table', null, [header, ...rows])])
}

/** A state with the cursor inside the cell whose text is `text`. */
function cursorInCell(text: string, bodyRows = 1): EditorState {
  const doc = tableDoc(bodyRows)
  let pos = -1
  doc.descendants((node, p) => {
    const n = node.type.name
    if ((n === 'table_cell' || n === 'table_header') && node.textContent === text && pos < 0) {
      pos = p
    }
  })
  const state = EditorState.create({ schema, doc })
  return state.apply(state.tr.setSelection(TextSelection.create(state.doc, pos + 2)))
}

describe('isInTable', () => {
  it('is false in a plain paragraph', () => {
    const doc = schema.node('doc', null, [para('plain')])
    const state = EditorState.create({ schema, doc })
    expect(isInTable(state)).toBe(false)
  })

  it('is true from a body cell', () => {
    expect(isInTable(cursorInCell('c'))).toBe(true)
  })

  it('is true from a header cell', () => {
    expect(isInTable(cursorInCell('a'))).toBe(true)
  })
})

describe('isInHeaderRow', () => {
  // A GFM table cannot lose its header row. ProseMirror's deleteRow does not
  // know that: asked to remove it, it reports success and empties the cells,
  // leaving a header of literal `<br />`. This guard is what refuses.
  it('is true in the header row', () => {
    expect(isInHeaderRow(cursorInCell('a'))).toBe(true)
  })

  it('is false in a body row', () => {
    expect(isInHeaderRow(cursorInCell('c'))).toBe(false)
  })

  it('is false outside a table', () => {
    const doc = schema.node('doc', null, [para('plain')])
    expect(isInHeaderRow(EditorState.create({ schema, doc }))).toBe(false)
  })
})

describe('TABLE_COMMANDS', () => {
  it('gives every command a unique id', () => {
    const ids = TABLE_COMMANDS.map((c) => c.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('labels every command for its tooltip and screen-reader name', () => {
    for (const cmd of TABLE_COMMANDS) expect(cmd.label, cmd.id).toBeTruthy()
  })

  it('lists exactly the deletes as directly dispatched', () => {
    const direct = TABLE_COMMANDS.filter((c) => DIRECT_TABLE_COMMANDS.has(c.id)).map((c) => c.id)
    expect(direct.sort()).toEqual(['deleteColumn', 'deleteRow', 'deleteTable'])
  })

  // The adds go through the command manager by slice name, so a typo would
  // only surface when the button is pressed.
  it('names the GFM slices for the add operations', () => {
    const adds = Object.fromEntries(
      TABLE_COMMANDS.filter((c) => !DIRECT_TABLE_COMMANDS.has(c.id)).map((c) => [c.id, c.command])
    )
    expect(adds).toEqual({
      addRowBefore: 'AddRowBefore',
      addRowAfter: 'AddRowAfter',
      addColBefore: 'AddColBefore',
      addColAfter: 'AddColAfter',
    })
  })
})

describe('isOnlyBodyRow', () => {
  // The schema is `table_header_row table_row+`, so the last body row cannot
  // be removed. deleteRow reports success and blanks its cells instead.
  it('is true when the table has a single body row', () => {
    expect(isOnlyBodyRow(cursorInCell('c'))).toBe(true)
  })

  it('is false when another body row remains', () => {
    expect(isOnlyBodyRow(cursorInCell('c', 2))).toBe(false)
  })

  it('is false in the header row, which the header guard owns', () => {
    expect(isOnlyBodyRow(cursorInCell('a'))).toBe(false)
  })

  it('is false outside a table', () => {
    const doc = schema.node('doc', null, [para('plain')])
    expect(isOnlyBodyRow(EditorState.create({ schema, doc }))).toBe(false)
  })
})

describe('canAddRowBefore', () => {
  // A GFM table opens with its header, so there is no position above it.
  // AddRowBefore does not refuse: it inserts a body row at index 0 and leaves
  // the header empty, splitting the output into two tables.
  it('is false in the header row', () => {
    expect(canAddRowBefore(cursorInCell('a'))).toBe(false)
  })

  it('is true in a body row', () => {
    expect(canAddRowBefore(cursorInCell('c'))).toBe(true)
  })

  it('is true in a later body row', () => {
    expect(canAddRowBefore(cursorInCell('c1', 2))).toBe(true)
  })

  it('is false outside a table', () => {
    const doc = schema.node('doc', null, [para('plain')])
    expect(canAddRowBefore(EditorState.create({ schema, doc }))).toBe(false)
  })

  // The guarded set must stay in step with the predicate: an id listed here is
  // excluded from the generic dry run, so a stale entry would leave a button
  // with no availability check at all.
  it('is the only guarded command, matching GUARDED_TABLE_COMMANDS', () => {
    expect([...GUARDED_TABLE_COMMANDS]).toEqual(['addRowBefore'])
    expect(TABLE_COMMANDS.some((c) => c.id === 'addRowBefore')).toBe(true)
  })
})
