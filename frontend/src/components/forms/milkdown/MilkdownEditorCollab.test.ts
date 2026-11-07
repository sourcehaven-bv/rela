/**
 * MilkdownEditor in a co-editing room (TKT-CJL604).
 *
 * Two editors share a room through two linked Y documents, the way the relay
 * links two browsers. The room is a stand-in for `openCollabRoom` with the
 * network taken out; the room's own behaviour is pinned in `collabRoom.test.ts`.
 */
import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { ref, shallowRef } from 'vue'
import * as Y from 'yjs'
import { Awareness } from 'y-protocols/awareness'
import type { EditorView } from '@milkdown/kit/prose/view'
import MilkdownEditor from './MilkdownEditor.vue'
import type { CollabRoom, CollabStatus, RoomBase } from '@/composables/collab/collabRoom'
import type { EntityRefResolver } from '@/utils/markdown'

vi.mock('@/api', () => ({
  searchEntities: vi.fn().mockResolvedValue({ data: [] }),
  listEntities: vi.fn(),
}))

interface FakeRoom extends CollabRoom {
  sync(): void
}

/** A room whose document is linked to `peer`'s, the way the relay links them. */
function fakeRoom(name: string, peer?: FakeRoom): FakeRoom {
  const doc = new Y.Doc()
  const awareness = new Awareness(doc)
  awareness.setLocalStateField('user', { name, color: '#123456' })
  const base = shallowRef<RoomBase | null>(peer?.base.value ?? null)
  let synced!: (ok: boolean) => void
  const whenSynced = new Promise<boolean>((resolve) => (synced = resolve))
  if (peer) {
    Y.applyUpdate(doc, Y.encodeStateAsUpdate(peer.doc), 'remote')
    doc.on('update', (u: Uint8Array, origin: unknown) => {
      if (origin !== 'remote') Y.applyUpdate(peer.doc, u, 'remote')
    })
    peer.doc.on('update', (u: Uint8Array, origin: unknown) => {
      if (origin !== 'remote') Y.applyUpdate(doc, u, 'remote')
    })
  }
  return {
    doc,
    awareness,
    status: ref<CollabStatus>('connecting'),
    isSaver: ref(!peer),
    online: ref(true),
    base,
    whenSynced: () => whenSynced,
    sync: () => synced(true),
    seed(b, build) {
      const fragment = doc.getXmlFragment('prosemirror')
      if (fragment.length > 0 || base.value) return
      doc.transact(() => build(fragment))
      base.value = b
    },
    recordSave: vi.fn(),
    isCurrentBase: () => false,
    destroy: () => awareness.destroy(),
  }
}

interface Exposed {
  flush: () => void
  setBaseline: (content: string) => void
  applyChange: (from: string, to: string) => void
  editorViewForTest: EditorView | null
}

const mounted: VueWrapper[] = []

async function mountIn(room: FakeRoom, base: RoomBase, refResolver?: EntityRefResolver) {
  const w = mount(MilkdownEditor, {
    props: { modelValue: base.content, collab: { room, base }, refResolver },
    attachTo: document.body,
  })
  mounted.push(w)
  await flushPromises()
  await flushPromises()
  return w
}

async function join(room: FakeRoom, base: RoomBase, refResolver?: EntityRefResolver) {
  const w = await mountIn(room, base, refResolver)
  room.sync()
  await flushPromises()
  return w
}

function api(w: VueWrapper): Exposed {
  return w.vm as unknown as Exposed
}

function view(w: VueWrapper): EditorView {
  const v = api(w).editorViewForTest
  if (!v) throw new Error('no view')
  return v
}

/** Types `text` at the end of the first block, as the user would. */
function typeAtEndOfFirstBlock(w: VueWrapper, text: string): void {
  const v = view(w)
  const first = v.state.doc.firstChild
  if (!first) throw new Error('empty doc')
  v.dispatch(v.state.tr.insertText(text, first.nodeSize - 1))
}

/** Waits out the editor's 200ms emit debounce. */
async function settle(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 250))
  await flushPromises()
}

function lastEmit(w: VueWrapper): string | undefined {
  const all = w.emitted('update:modelValue') as string[][] | undefined
  return all?.[all.length - 1]?.[0]
}

function undoButton(w: VueWrapper) {
  return w.find('button[aria-label="Undo"]')
}

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount()
  document.body.innerHTML = ''
})

describe('MilkdownEditor collab mode', () => {
  it('stays read-only until the room has synced', async () => {
    const room = fakeRoom('a')
    const w = await mountIn(room, { content: 'Hello\n', token: 't1' })
    expect(view(w).editable).toBe(false)
    room.sync()
    await flushPromises()
    expect(view(w).editable).toBe(true)
    expect(w.text()).toContain('Hello')
  })

  it('converges two editors, and either can emit the shared document', async () => {
    const a = fakeRoom('a')
    const wa = await join(a, { content: 'Hello\n', token: 't1' })
    const b = fakeRoom('b', a)
    await join(b, { content: 'Hello\n', token: 't1' })
    const wb = mounted[1]!

    typeAtEndOfFirstBlock(wb, ' world')
    await flushPromises()
    expect(view(wa).state.doc.textContent).toBe('Hello world')

    api(wa).flush()
    expect(lastEmit(wa)).toBe('Hello world\n')
  })

  it('keeps the room document over a stale body this client loaded', async () => {
    const a = fakeRoom('a')
    await join(a, { content: 'Room body\n', token: 't2' })
    const b = fakeRoom('b', a)
    const wb = await join(b, { content: 'Stale body\n', token: 't1' })

    expect(view(wb).state.doc.textContent).toBe('Room body')
    // The room's base is the baseline, so an unchanged room document is not a
    // change to save.
    api(wb).flush()
    expect(wb.emitted('update:modelValue')).toBeUndefined()
  })

  it("starts dirty when the room holds a peer's unsaved edit", async () => {
    const a = fakeRoom('a')
    const wa = await join(a, { content: 'Hello\n', token: 't1' })
    typeAtEndOfFirstBlock(wa, ' there')
    const b = fakeRoom('b', a)
    const wb = await join(b, { content: 'Hello\n', token: 't1' })

    api(wb).flush()
    expect(lastEmit(wb)).toBe('Hello there\n')
  })

  it("undoes this client's own change and leaves the peer's", async () => {
    const a = fakeRoom('a')
    const wa = await join(a, { content: 'Hello\n', token: 't1' })
    const b = fakeRoom('b', a)
    const wb = await join(b, { content: 'Hello\n', token: 't1' })

    typeAtEndOfFirstBlock(wa, ' A')
    typeAtEndOfFirstBlock(wb, ' B')
    await flushPromises()
    expect(view(wa).state.doc.textContent).toBe('Hello A B')

    expect(undoButton(wa).attributes('aria-disabled')).toBe('false')
    await undoButton(wa).trigger('click')
    await flushPromises()
    expect(view(wa).state.doc.textContent).toBe('Hello B')
    expect(view(wb).state.doc.textContent).toBe('Hello B')
  })

  // y-prosemirror marks a change from the room `addToHistory: false`, and
  // Milkdown's markdown listener skips those. Without its own emit, the saver
  // saved only up to its own last keystroke and lost a peer's typing after it.
  it("emits a peer's change and its own undo, which the markdown listener skips", async () => {
    const a = fakeRoom('a')
    const wa = await join(a, { content: 'Hello\n', token: 't1' })
    const b = fakeRoom('b', a)
    const wb = await join(b, { content: 'Hello\n', token: 't1' })

    typeAtEndOfFirstBlock(wb, ' B')
    await settle()
    expect(lastEmit(wa)).toBe('Hello B\n')

    typeAtEndOfFirstBlock(wa, ' A')
    await settle()
    expect(lastEmit(wa)).toBe('Hello B A\n')
    expect(undoButton(wa).attributes('aria-disabled')).toBe('false')
    await undoButton(wa).trigger('click')
    await settle()
    expect(view(wa).state.doc.textContent).toBe('Hello B')
    expect(lastEmit(wa)).toBe('Hello B\n')
  })

  // The room's undo stack grows after the transaction that refreshes the
  // toolbar, so without a subscription to the stack Undo stayed disabled after
  // the first edit until some later transaction arrived.
  it('enables undo after the first edit with no further transaction', async () => {
    const wa = await join(fakeRoom('a'), { content: 'Hello\n', token: 't1' })
    typeAtEndOfFirstBlock(wa, ' A')
    await flushPromises()
    expect(undoButton(wa).attributes('aria-disabled')).toBe('false')
  })

  it('disables undo before this client has changed anything', async () => {
    const a = fakeRoom('a')
    await join(a, { content: 'Hello\n', token: 't1' })
    const b = fakeRoom('b', a)
    const wb = await join(b, { content: 'Hello\n', token: 't1' })
    typeAtEndOfFirstBlock(mounted[0]!, ' A')
    await flushPromises()
    // The peer's change is not this client's to undo.
    expect(undoButton(wb).attributes('aria-disabled')).toBe('true')
  })

  it("never shows a peer the title this client's resolver gave a reference", async () => {
    const secret: EntityRefResolver = (id) =>
      id === 'TKT-ABC' ? { type: 'ticket', title: 'Secret title' } : null
    const a = fakeRoom('a')
    const wa = await join(a, { content: 'See `TKT-ABC`.\n', token: 't1' }, secret)
    const b = fakeRoom('b', a)
    const wb = await join(b, { content: 'See `TKT-ABC`.\n', token: 't1' })

    expect(wa.text()).toContain('Secret title')
    expect(wb.text()).not.toContain('Secret title')
    expect(wb.text()).toContain('TKT-ABC')
  })

  it('moves the baseline on setBaseline, so a saved document is clean', async () => {
    const a = fakeRoom('a')
    const wa = await join(a, { content: 'Hello\n', token: 't1' })
    typeAtEndOfFirstBlock(wa, ' world')
    api(wa).setBaseline('Hello world\n')
    api(wa).flush()
    expect(wa.emitted('update:modelValue')).toBeUndefined()
  })

  it('applies an outside change as an edit every peer sees', async () => {
    const a = fakeRoom('a')
    const wa = await join(a, { content: 'Hello\n', token: 't1' })
    const b = fakeRoom('b', a)
    const wb = await join(b, { content: 'Hello\n', token: 't1' })

    api(wa).applyChange('Hello\n', 'Merged\n')
    await flushPromises()
    expect(view(wb).state.doc.textContent).toBe('Merged')
    api(wa).flush()
    expect(lastEmit(wa)).toBe('Merged\n')
  })

  it('keeps what was typed since the text an outside change was made from', async () => {
    const a = fakeRoom('a')
    const wa = await join(a, { content: 'One\n\nTwo\n', token: 't1' })
    typeAtEndOfFirstBlock(wa, ' typed')
    api(wa).applyChange('One\n\nTwo\n', 'One\n\nTwo merged\n')
    await flushPromises()
    api(wa).flush()
    expect(lastEmit(wa)).toBe('One typed\n\nTwo merged\n')
  })

  it('ignores a new modelValue in a room', async () => {
    const a = fakeRoom('a')
    const w = await join(a, { content: 'Hello\n', token: 't1' })
    await w.setProps({ modelValue: 'Something else\n' })
    await flushPromises()
    expect(view(w).state.doc.textContent).toBe('Hello')
  })
})
