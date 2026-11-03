/**
 * The convert button on task items, and applying a conversion's outcome
 * (TKT-3IBLIP). The server side is the host's job; these tests drive the
 * editor through its props, its `taskItemConvert` event and its exposed
 * `applyTaskItemResult` / `markTaskItemBusy` / `clearTaskItemBusy`.
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import MilkdownEditor from './MilkdownEditor.vue'
import type { TaskItemTarget } from './taskItemConvert'
import type { EntityRefInsert } from './insertEntityRef'
import type { EditorView } from '@milkdown/kit/prose/view'

vi.mock('@/api', () => ({
  searchEntities: vi.fn().mockResolvedValue({ data: [] }),
  listEntities: vi.fn(),
}))

interface Exposed {
  guardedValue: () => { value: string }
  applyTaskItemResult: (
    token: number,
    text: string,
    mode: 'link' | 'remove',
    ref: EntityRefInsert
  ) => boolean
  markTaskItemBusy: (t: TaskItemTarget) => number | null
  clearTaskItemBusy: (token: number) => void
}

const ref: EntityRefInsert = { id: 'TASK-7', title: 'Write it', entityType: 'task' }

async function mountEditor(modelValue: string, label = 'Convert to task') {
  const wrapper = mount(MilkdownEditor, {
    props: { modelValue, taskItemConvertLabel: label },
    attachTo: document.body,
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

function exposed(w: ReturnType<typeof mount>): Exposed {
  return w.vm as unknown as Exposed
}

function markdown(w: ReturnType<typeof mount>): string {
  return exposed(w).guardedValue().value.trim()
}

function buttons(w: ReturnType<typeof mount>): HTMLButtonElement[] {
  return Array.from((w.element as HTMLElement).querySelectorAll('.task-item-convert'))
}

async function clickButton(w: ReturnType<typeof mount>, index: number): Promise<TaskItemTarget> {
  const button = buttons(w)[index]
  button.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }))
  button.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }))
  await flushPromises()
  const events = w.emitted('taskItemConvert') ?? []
  return events[events.length - 1]?.[0] as TaskItemTarget
}

/** Marks the item busy, as the host does on a press, and applies `mode`. */
function convert(
  w: ReturnType<typeof mount>,
  target: TaskItemTarget,
  mode: 'link' | 'remove',
  insert: EntityRefInsert = ref
): boolean {
  const token = exposed(w).markTaskItemBusy(target)
  expect(token).not.toBeNull()
  return exposed(w).applyTaskItemResult(token!, target.text, mode, insert)
}

function view(w: ReturnType<typeof mount>): EditorView {
  return (w.vm as unknown as { editorViewForTest: EditorView }).editorViewForTest
}

/** The position after the last character of the `index`th task item's line. */
function lineEnd(w: ReturnType<typeof mount>, index: number): number {
  const ends: number[] = []
  view(w).state.doc.descendants((node, pos) => {
    if (node.type.name === 'list_item' && node.firstChild?.type.name === 'paragraph') {
      ends.push(pos + 2 + node.firstChild.content.size)
    }
  })
  return ends[index]
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('task item convert button', () => {
  it('draws no button without a label', async () => {
    const w = await mountEditor('- [ ] todo\n', '')
    expect(buttons(w)).toHaveLength(0)
  })

  it('draws one button per task item with text of its own', async () => {
    const w = await mountEditor('- [ ] one\n- [x] two\n- [ ] `TASK-1`\n\n* plain\n')
    const all = buttons(w)
    expect(all).toHaveLength(2)
    expect(all[0].getAttribute('aria-label')).toBe('Convert to task: one')
    expect(all[0].contentEditable).toBe('false')
    expect(all[0].type).toBe('button')
  })

  it('appears when the label arrives after mount', async () => {
    const w = await mountEditor('- [ ] todo\n', '')
    await w.setProps({ taskItemConvertLabel: 'Convert' })
    await flushPromises()
    expect(buttons(w)).toHaveLength(1)
  })

  it('reports the item line only, not its nested items', async () => {
    const w = await mountEditor('- [ ] parent  item\n  - [ ] child\n')
    const target = await clickButton(w, 0)
    expect(target.text).toBe('parent item')
  })

  it('does nothing while the item is busy', async () => {
    const w = await mountEditor('- [ ] todo\n')
    const target = await clickButton(w, 0)
    const token = exposed(w).markTaskItemBusy(target)
    expect(token).not.toBeNull()
    await flushPromises()
    expect(buttons(w)[0].getAttribute('aria-disabled')).toBe('true')
    await clickButton(w, 0)
    expect(w.emitted('taskItemConvert')).toHaveLength(1)
    exposed(w).clearTaskItemBusy(token!)
    await flushPromises()
    expect(buttons(w)[0].hasAttribute('aria-disabled')).toBe(false)
  })

  it('does not pass a busy mark on to the item that takes a removed one’s place', async () => {
    const w = await mountEditor('- [ ] parent\n  - [ ] child\n- [ ] next\n')
    const target = await clickButton(w, 0)
    const token = exposed(w).markTaskItemBusy(target)
    await flushPromises()
    expect(exposed(w).applyTaskItemResult(token!, target.text, 'remove', ref)).toBe(true)
    await flushPromises()
    // The lifted child now starts where the parent did.
    expect(buttons(w).map((b) => b.hasAttribute('aria-disabled'))).toEqual([false, false])
    exposed(w).clearTaskItemBusy(token!)
    await flushPromises()
    expect(buttons(w)).toHaveLength(2)
  })

  it('clears a linked item’s mark by its token, not by its text', async () => {
    const w = await mountEditor('- [ ] write it\n')
    const target = await clickButton(w, 0)
    const token = exposed(w).markTaskItemBusy(target)
    expect(exposed(w).applyTaskItemResult(token!, target.text, 'link', ref)).toBe(true)
    exposed(w).clearTaskItemBusy(token!)
    await flushPromises()
    // Text typed after the reference gives the item a button again, and it
    // must not come back disabled.
    view(w).dispatch(view(w).state.tr.insertText(' more', lineEnd(w, 0)))
    await flushPromises()
    expect(buttons(w)).toHaveLength(1)
    expect(buttons(w)[0].hasAttribute('aria-disabled')).toBe(false)
  })

  it('never reaches the saved markdown', async () => {
    const source = '- [ ] one\n- [ ] two\n'
    const w = await mountEditor(source)
    expect(buttons(w)).toHaveLength(2)
    expect(exposed(w).guardedValue().value).toBe(source)
  })
})

describe('applying a conversion', () => {
  it('link replaces the line with a reference and emits it', async () => {
    const w = await mountEditor('- [ ] write it\n- [ ] other\n')
    const target = await clickButton(w, 0)
    expect(convert(w, target, 'link')).toBe(true)
    expect(markdown(w)).toBe('- [ ] `TASK-7`\n- [ ] other')
    const updates = w.emitted('update:modelValue') ?? []
    expect(String(updates[updates.length - 1]?.[0]).trim()).toBe('- [ ] `TASK-7`\n- [ ] other')
  })

  it('remove deletes the item and lifts its nested items', async () => {
    const w = await mountEditor(
      '- [ ] before\n- [ ] parent\n  - [ ] child a\n  - [x] child b\n- [ ] after\n'
    )
    const target = await clickButton(w, 1)
    expect(convert(w, target, 'remove')).toBe(true)
    expect(markdown(w)).toBe('- [ ] before\n- [ ] child a\n- [x] child b\n- [ ] after')
  })

  it('remove of the only item removes the list', async () => {
    const w = await mountEditor('Intro\n\n- [ ] only\n\nOutro\n')
    const target = await clickButton(w, 0)
    expect(convert(w, target, 'remove')).toBe(true)
    expect(markdown(w)).toBe('Intro\n\nOutro')
  })

  it('remove links instead when the item holds more than its line', async () => {
    const w = await mountEditor('- [ ] keep me\n\n  A second paragraph.\n')
    const target = await clickButton(w, 0)
    expect(convert(w, target, 'remove')).toBe(true)
    expect(markdown(w)).toContain('`TASK-7`')
    expect(markdown(w)).toContain('A second paragraph.')
  })

  it.each([
    [
      'a link',
      '- [ ] read [the spec](https://example.com/spec) first\n',
      '[the spec](https://example.com/spec)',
    ],
    [
      'an image',
      '- [ ] fix ![shot](https://example.com/a.png) here\n',
      '![shot](https://example.com/a.png)',
    ],
  ])('remove links instead when the line holds %s', async (_name, source, kept) => {
    const w = await mountEditor(source)
    const target = await clickButton(w, 0)
    expect(convert(w, target, 'remove')).toBe(true)
    expect(markdown(w)).toBe(`- [ ] \`TASK-7\` ${kept}`)
  })

  it('remove still removes a line whose references became ids in its text', async () => {
    const w = await mountEditor('- [ ] follow up on `TKT-1`\n- [ ] other\n')
    const target = await clickButton(w, 0)
    expect(convert(w, target, 'remove')).toBe(true)
    expect(markdown(w)).toBe('- [ ] other')
  })

  it('link keeps links and references, after the new one', async () => {
    const w = await mountEditor(
      '- [ ] see [docs](https://example.com/d) and `TKT-1` then [more](https://example.com/m)\n'
    )
    const target = await clickButton(w, 0)
    expect(convert(w, target, 'link')).toBe(true)
    expect(markdown(w)).toBe(
      '- [ ] `TASK-7` [docs](https://example.com/d) `TKT-1` [more](https://example.com/m)'
    )
  })

  it('follows the converted item through edits before it', async () => {
    const w = await mountEditor('- [ ] same\n- [ ] same\n')
    const target = await clickButton(w, 1)
    const token = exposed(w).markTaskItemBusy(target)
    // The other item changes while the request runs, moving this one.
    view(w).dispatch(view(w).state.tr.insertText(' and more', lineEnd(w, 0)))
    expect(exposed(w).applyTaskItemResult(token!, target.text, 'link', ref)).toBe(true)
    expect(markdown(w)).toBe('- [ ] same and more\n- [ ] `TASK-7`')
  })

  it('leaves the document alone when the converted item was edited', async () => {
    const w = await mountEditor('- [ ] same\n- [ ] same\n')
    const target = await clickButton(w, 0)
    const token = exposed(w).markTaskItemBusy(target)
    view(w).dispatch(view(w).state.tr.insertText(' edited', lineEnd(w, 0)))
    // The other item has the text the server converted, but it is not the
    // item the user converted.
    expect(exposed(w).applyTaskItemResult(token!, target.text, 'link', ref)).toBe(false)
    expect(markdown(w)).toBe('- [ ] same edited\n- [ ] same')
  })

  it('refuses without a mark, or after its item was deleted', async () => {
    const w = await mountEditor('- [ ] same\n- [ ] same\n')
    const target = await clickButton(w, 0)
    expect(exposed(w).applyTaskItemResult(999, target.text, 'link', ref)).toBe(false)
    const token = exposed(w).markTaskItemBusy(target)
    const item = view(w).state.doc.nodeAt(target.pos)!
    view(w).dispatch(view(w).state.tr.delete(target.pos, target.pos + item.nodeSize))
    expect(exposed(w).applyTaskItemResult(token!, target.text, 'link', ref)).toBe(false)
    expect(markdown(w)).toBe('- [ ] same')
  })

  it('refuses an invalid reference id', async () => {
    const w = await mountEditor('- [ ] write it\n')
    const target = await clickButton(w, 0)
    expect(convert(w, target, 'link', { ...ref, id: 'not an id' })).toBe(false)
    expect(markdown(w)).toBe('- [ ] write it')
  })
})
