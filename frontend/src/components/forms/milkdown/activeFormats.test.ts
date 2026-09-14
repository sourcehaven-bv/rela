import { describe, it, expect } from 'vitest'
import { Schema } from '@milkdown/kit/prose/model'
import { EditorState, TextSelection } from '@milkdown/kit/prose/state'
import { activeCommandIds } from './activeFormats'
import { INLINE_COMMANDS, BLOCK_COMMANDS } from './editorCommands'

// A schema shaped like Milkdown's, carrying only the node and mark names the
// probes name. Building it here rather than booting an Editor keeps the test
// synchronous and makes a rename in `editorCommands` fail loudly.
const schema = new Schema({
  nodes: {
    doc: { content: 'block+' },
    paragraph: { group: 'block', content: 'inline*' },
    heading: {
      group: 'block',
      content: 'inline*',
      attrs: { level: { default: 1 } },
    },
    blockquote: { group: 'block', content: 'block+' },
    code_block: { group: 'block', content: 'text*', marks: '' },
    bullet_list: { group: 'block', content: 'list_item+' },
    ordered_list: { group: 'block', content: 'list_item+' },
    list_item: { content: 'paragraph block*' },
    text: { group: 'inline' },
  },
  marks: {
    strong: {},
    emphasis: {},
    strike_through: {},
    inlineCode: {},
  },
})

const ALL = [...INLINE_COMMANDS, ...BLOCK_COMMANDS]

/** A state whose cursor sits inside the single paragraph of `doc`. */
function stateWithCursorIn(node: ReturnType<typeof schema.node>, pos: number) {
  const doc = schema.node('doc', null, [node])
  const state = EditorState.create({ schema, doc })
  return state.apply(state.tr.setSelection(TextSelection.create(state.doc, pos)))
}

describe('activeCommandIds', () => {
  it('reports nothing in a plain paragraph', () => {
    const para = schema.node('paragraph', null, [schema.text('hello')])
    const active = activeCommandIds(stateWithCursorIn(para, 2), ALL)
    expect([...active]).toEqual([])
  })

  it('detects a mark spanning the selection', () => {
    const bold = schema.text('hello', [schema.mark('strong')])
    const para = schema.node('paragraph', null, [bold])
    const state = EditorState.create({ schema, doc: schema.node('doc', null, [para]) })
    const selected = state.apply(state.tr.setSelection(TextSelection.create(state.doc, 1, 6)))
    expect(activeCommandIds(selected, ALL).has('strong')).toBe(true)
  })

  it('detects a mark at a collapsed cursor inside the marked text', () => {
    const bold = schema.text('hello', [schema.mark('emphasis')])
    const para = schema.node('paragraph', null, [bold])
    expect(activeCommandIds(stateWithCursorIn(para, 3), ALL).has('emphasis')).toBe(true)
  })

  // Pressing bold with nothing selected arms the mark for the next keystroke.
  // It lives in storedMarks, not the document, so a toolbar that ignored it
  // would unlight itself the moment the user armed it.
  it('detects a stored mark armed for the next keystroke', () => {
    const para = schema.node('paragraph', null, [schema.text('hello')])
    const base = stateWithCursorIn(para, 3)
    const armed = base.apply(base.tr.addStoredMark(schema.mark('strong')))
    expect(activeCommandIds(armed, ALL).has('strong')).toBe(true)
  })

  it('distinguishes heading levels rather than lighting every heading button', () => {
    const h2 = schema.node('heading', { level: 2 }, [schema.text('Title')])
    const active = activeCommandIds(stateWithCursorIn(h2, 2), ALL)
    expect(active.has('h2')).toBe(true)
    expect(active.has('h1')).toBe(false)
    expect(active.has('h3')).toBe(false)
  })

  it('reports the enclosing list from a cursor nested in its item paragraph', () => {
    const para = schema.node('paragraph', null, [schema.text('item')])
    const item = schema.node('list_item', null, [para])
    const list = schema.node('bullet_list', null, [item])
    const active = activeCommandIds(stateWithCursorIn(list, 4), ALL)
    expect(active.has('bulletList')).toBe(true)
    expect(active.has('orderedList')).toBe(false)
  })

  it('reports a blockquote wrapping the cursor', () => {
    const para = schema.node('paragraph', null, [schema.text('quoted')])
    const quote = schema.node('blockquote', null, [para])
    expect(activeCommandIds(stateWithCursorIn(quote, 3), ALL).has('blockquote')).toBe(true)
  })

  it('reports a code block', () => {
    const code = schema.node('code_block', null, [schema.text('x := 1')])
    expect(activeCommandIds(stateWithCursorIn(code, 3), ALL).has('codeBlock')).toBe(true)
  })

  // The probes name schema types by string. If a preset ever renames one, the
  // lookup silently returns false rather than throwing, so this asserts the
  // names resolve against a real schema instead.
  it('names only marks and nodes that exist in the schema', () => {
    for (const cmd of ALL) {
      if (cmd.probe.kind === 'mark') {
        expect(schema.marks[cmd.probe.mark], `mark ${cmd.probe.mark}`).toBeDefined()
      }
      if (cmd.probe.kind === 'node') {
        expect(schema.nodes[cmd.probe.node], `node ${cmd.probe.node}`).toBeDefined()
      }
    }
  })
})
