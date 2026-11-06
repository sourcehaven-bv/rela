import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick, ref, shallowRef } from 'vue'
import * as Y from 'yjs'
import { Awareness } from 'y-protocols/awareness'
import type { CollabRoom, CollabRoomOptions, CollabStatus, RoomBase } from './collabRoom'
import {
  useBodyCollab,
  FALLBACK_MS,
  SAVE_DEBOUNCE_MS,
  STALE_RELOAD_KEY,
  SYNC_TIMEOUT_MS,
  type BodyCollabDeps,
} from './useBodyCollab'

function fakeRoom() {
  let synced!: (ok: boolean) => void
  const whenSynced = new Promise<boolean>((resolve) => (synced = resolve))
  const doc = new Y.Doc()
  const roomBase = shallowRef<RoomBase | null>(null)
  const room = {
    doc,
    awareness: new Awareness(doc),
    status: ref<CollabStatus>('connecting'),
    isSaver: ref(false),
    online: ref(true),
    base: roomBase,
    whenSynced: () => whenSynced,
    seed: vi.fn(),
    recordSave: vi.fn((stored: string, token: string | undefined, sent?: string) => {
      roomBase.value = { content: stored, token, sent }
    }),
    isCurrentBase: (c: string) => roomBase.value?.content === c || roomBase.value?.sent === c,
    destroy: vi.fn(),
  } satisfies CollabRoom
  return {
    room,
    /** Makes `content` the room's latest save, as a peer's save does. */
    async saved(content: string, token = 'tp') {
      room.base.value = { content, token }
      await flush()
    },
    /** Syncs the room, as the relay's first answer does. */
    async sync(base: RoomBase | null = null) {
      room.base.value = base
      room.status.value = 'synced'
      synced(true)
      await flush()
    },
  }
}

async function flush() {
  await Promise.resolve()
  await nextTick()
  await Promise.resolve()
}

function fakeSave() {
  return {
    scheduleContentSave: vi.fn(),
    setContentBase: vi.fn(),
    commitImmediately: vi.fn().mockResolvedValue(undefined),
  }
}

function fakeEditor() {
  return { flush: vi.fn(), setBaseline: vi.fn(), applyChange: vi.fn() }
}

const target = { type: 'ticket', id: 'TKT-1' }
const base: RoomBase = { content: 'Hello\n', token: 't1' }

function setup(overrides: Partial<BodyCollabDeps> = {}) {
  const fr = fakeRoom()
  const opened: CollabRoomOptions[] = []
  const store = new Map<string, string>()
  const deps = {
    enabled: () => true,
    user: () => 'alice',
    closeEditor: vi.fn(),
    notify: vi.fn(),
    build: 'b1',
    reload: vi.fn(),
    storage: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
    },
    openRoom: (opts: CollabRoomOptions) => {
      opened.push(opts)
      return fr.room
    },
    ...overrides,
  }
  const collab = useBodyCollab(deps)
  const save = fakeSave()
  const editor = fakeEditor()
  collab.bindEditor(editor)
  return { ...fr, deps, collab, save, editor, opened, store }
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('useBodyCollab', () => {
  it('stays single when co-editing is off or the user is unknown', () => {
    for (const o of [{ enabled: () => false }, { user: () => null }]) {
      const t = setup(o)
      expect(t.collab.start(target, base, t.save)).toBe(false)
      expect(t.collab.mode.value).toBe('single')
      expect(t.opened).toHaveLength(0)
      expect(t.collab.input('x')).toBe(false)
    }
  })

  it('joins as the user id, never a person title, with the build', () => {
    const t = setup()
    expect(t.collab.start(target, base, t.save)).toBe(true)
    expect(t.collab.mode.value).toBe('collab')
    expect(t.collab.editorCollab.value).toEqual({ room: t.room, base })
    expect(t.opened[0]).toMatchObject({ type: 'ticket', id: 'TKT-1', build: 'b1' })
    expect(t.opened[0]!.user.name).toBe('alice')
  })

  it('saves after the typing pause when this client is the saver', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    t.room.isSaver.value = true
    await t.sync(base)
    t.collab.input('Hello world\n')
    expect(t.save.scheduleContentSave).toHaveBeenLastCalledWith('Hello world\n', SAVE_DEBOUNCE_MS)
  })

  it('falls back to saving when the saver did not', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync(base)
    t.collab.input('Hello w\n')
    vi.advanceTimersByTime(FALLBACK_MS - 1)
    t.collab.input('Hello world\n')
    vi.advanceTimersByTime(FALLBACK_MS - 1)
    expect(t.save.scheduleContentSave).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(t.save.scheduleContentSave).toHaveBeenCalledWith('Hello world\n', 0)
  })

  it('does not fall back when the room saved the document', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync(base)
    t.collab.input('Hello world\n')
    await t.saved('Hello world\n')
    vi.advanceTimersByTime(FALLBACK_MS)
    expect(t.save.scheduleContentSave).not.toHaveBeenCalled()
  })

  it("lets the saver's save of the room's latest body replace a pending save", async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    t.room.isSaver.value = true
    await t.sync(base)
    await t.saved('Saved\n')
    t.save.scheduleContentSave.mockClear()
    expect(t.collab.input('Saved\n')).toBe(true)
    expect(t.save.scheduleContentSave).toHaveBeenCalledWith('Saved\n', SAVE_DEBOUNCE_MS)
  })

  it('saves an unsaved document on becoming the saver', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync(base)
    t.collab.input('Unsaved\n')
    t.room.isSaver.value = true
    await flush()
    expect(t.editor.flush).toHaveBeenCalled()
    expect(t.save.scheduleContentSave).toHaveBeenCalledWith('Unsaved\n', 0)
  })

  it('does not save on becoming the saver when the room saved it', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync(base)
    t.collab.input('Saved\n')
    await t.saved('Saved\n')
    t.room.isSaver.value = true
    await flush()
    expect(t.save.scheduleContentSave).not.toHaveBeenCalled()
  })

  it("records this client's saves in the room and follows the room's base", async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync(base)
    t.save.setContentBase.mockClear()
    t.collab.autoSaveHooks(target).onContentSaved('Next\n', 't2', 'Next\n')
    expect(t.room.recordSave).toHaveBeenCalledWith('Next\n', 't2', 'Next\n')
    await flush()
    expect(t.save.setContentBase).toHaveBeenCalledWith('Next\n', 't2')
    expect(t.editor.setBaseline).toHaveBeenCalledWith('Next\n')
  })

  it("resends the document only for the room's latest save, and only for the room's row", async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    const hooks = t.collab.autoSaveHooks(target)
    expect(hooks.resendContent('Hello\n')).toBeNull()
    await t.sync(base)
    t.collab.input('Typed\n')
    expect(hooks.resendContent('Hello\n')).toBe('Typed\n')
    expect(hooks.resendContent('Other\n')).toBeNull()
    await t.saved('Theirs\n')
    // An older room body, restored from history, is merged, not overwritten.
    expect(hooks.resendContent('Hello\n')).toBeNull()
    const other = t.collab.autoSaveHooks({ type: 'ticket', id: 'TKT-2' })
    expect(other.resendContent('Theirs\n')).toBeNull()
    other.onContentSaved('X\n', 't9', 'X\n')
    expect(t.room.recordSave).not.toHaveBeenCalled()
  })

  it("saves on the room's token when it differs from this client's load", async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync({ content: 'Room\n', token: 't0' })
    expect(t.save.setContentBase).toHaveBeenCalledWith('Room\n', 't0', true)
    expect(t.save.scheduleContentSave).toHaveBeenCalledWith('Room\n', 0)
  })

  it('edits alone when the room does not sync in time', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    vi.advanceTimersByTime(SYNC_TIMEOUT_MS)
    await flush()
    expect(t.collab.mode.value).toBe('single')
    expect(t.collab.editorCollab.value).toBeNull()
    expect(t.room.destroy).toHaveBeenCalled()
    expect(t.deps.notify).toHaveBeenCalledWith('unavailable')
    expect(t.collab.input('x')).toBe(false)
  })

  it('saves, closes the editor and says so when the room expires', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync(base)
    t.room.status.value = 'expired'
    await flush()
    expect(t.editor.flush).toHaveBeenCalled()
    expect(t.save.commitImmediately).toHaveBeenCalled()
    expect(t.deps.closeEditor).toHaveBeenCalled()
    expect(t.deps.notify).toHaveBeenCalledWith('expired')
    expect(t.collab.mode.value).toBe('single')
  })

  it('treats a refused connection as unavailable before sync and as expiry after', async () => {
    const before = setup()
    before.collab.start(target, base, before.save)
    before.room.status.value = 'failed'
    await flush()
    expect(before.deps.notify).toHaveBeenCalledWith('unavailable')
    expect(before.deps.closeEditor).not.toHaveBeenCalled()

    const after = setup()
    after.collab.start(target, base, after.save)
    await after.sync(base)
    after.room.status.value = 'failed'
    await flush()
    expect(after.deps.notify).toHaveBeenCalledWith('expired')
    expect(after.deps.closeEditor).toHaveBeenCalled()
  })

  it('reloads once for a stale build, then edits alone', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    t.room.status.value = 'stale-build'
    await flush()
    expect(t.deps.notify).toHaveBeenCalledWith('stale-build')
    expect(t.deps.reload).toHaveBeenCalledTimes(1)
    expect(t.store.get(STALE_RELOAD_KEY)).toBe('b1')

    const again = setup({ storage: t.deps.storage })
    again.collab.start(target, base, again.save)
    again.room.status.value = 'stale-build'
    await flush()
    expect(again.deps.reload).not.toHaveBeenCalled()
    expect(again.deps.notify).toHaveBeenCalledWith('unavailable')
    expect(again.collab.mode.value).toBe('single')
  })

  it("puts a merge into the room once synced and online, and only for the room's row", async () => {
    const t = setup()
    const hooks = t.collab.autoSaveHooks(target)
    expect(hooks.inRoom()).toBe(false)
    t.collab.start(target, base, t.save)
    expect(hooks.inRoom()).toBe(true)
    hooks.mergedContent('Hello\n', 'Merged\n')
    expect(t.editor.applyChange).not.toHaveBeenCalled()
    await t.sync(base)
    t.collab.autoSaveHooks({ type: 'ticket', id: 'TKT-2' }).mergedContent('Hello\n', 'Other\n')
    expect(t.editor.applyChange).not.toHaveBeenCalled()
    t.room.online.value = false
    hooks.mergedContent('Hello\n', 'Merged\n')
    expect(t.editor.applyChange).not.toHaveBeenCalled()
    t.room.online.value = true
    hooks.mergedContent('Hello\n', 'Merged\n')
    expect(t.editor.applyChange).toHaveBeenCalledWith('Hello\n', 'Merged\n')
  })

  it('saves and leaves the room on stop', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync(base)
    await t.collab.stop()
    expect(t.editor.flush).toHaveBeenCalled()
    expect(t.save.commitImmediately).toHaveBeenCalled()
    expect(t.room.destroy).toHaveBeenCalled()
    expect(t.collab.mode.value).toBe('single')
    // A late status change from the destroyed room is ignored.
    t.room.status.value = 'expired'
    await flush()
    expect(t.deps.closeEditor).not.toHaveBeenCalled()
  })

  it('saves an undo back to an earlier saved body', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    t.room.isSaver.value = true
    await t.sync(base)
    t.collab.autoSaveHooks(target).onContentSaved('Next\n', 't2', 'Next\n')
    await flush()
    t.save.scheduleContentSave.mockClear()
    t.collab.input('Hello\n')
    expect(t.save.scheduleContentSave).toHaveBeenCalledWith('Hello\n', SAVE_DEBOUNCE_MS)
  })

  it('does not fall back to saving while offline, and saves once back online', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync(base)
    t.room.online.value = false
    await flush()
    t.collab.input('Offline\n')
    vi.advanceTimersByTime(FALLBACK_MS)
    expect(t.save.scheduleContentSave).not.toHaveBeenCalled()
    t.room.online.value = true
    await flush()
    vi.advanceTimersByTime(FALLBACK_MS)
    expect(t.save.scheduleContentSave).toHaveBeenCalledWith('Offline\n', 0)
  })

  it('saves what a non-saver holds when it leaves', async () => {
    const t = setup()
    t.collab.start(target, base, t.save)
    await t.sync(base)
    t.collab.input('Mine\n')
    await t.collab.stop()
    expect(t.save.scheduleContentSave).toHaveBeenCalledWith('Mine\n', 0)
    expect(t.save.commitImmediately).toHaveBeenCalled()
  })

  it("passes the relay's epoch when it joins", () => {
    const t = setup({ epoch: () => 'e1' })
    t.collab.start(target, base, t.save)
    expect(t.opened[0]).toMatchObject({ epoch: 'e1' })
  })
})
