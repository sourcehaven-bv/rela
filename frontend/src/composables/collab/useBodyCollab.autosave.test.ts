// useBodyCollab wired to the real useAutoSave, as EntityDetail wires them
// (TKT-CJL604). The unit tests fake one side each; the save coordination
// between the two is only visible here.

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { nextTick, ref, shallowRef } from 'vue'
import * as Y from 'yjs'
import { Awareness } from 'y-protocols/awareness'
import { useEntitiesStore } from '@/stores/entities'
import { useAutoSave } from '@/composables/useAutoSave'
import { ApiError } from '@/api/errors'
import type { Entity, FieldVersions } from '@/types'
import * as entitiesApi from '@/api/entities'
import type { CollabRoom, CollabStatus, RoomBase } from './collabRoom'
import { useBodyCollab, SAVE_DEBOUNCE_MS } from './useBodyCollab'

vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  getEntity: vi.fn(),
}))

const versions = (content: string): FieldVersions => ({
  properties: {},
  content,
  relations: 'r1',
})

function bodyAt(content: string, token: string): Entity {
  const e: Entity = {
    id: 'TKT-1',
    type: 'ticket',
    properties: {},
    content,
    _versions: versions(token),
  }
  return e
}

function conflict412(expected: string, actual: string): ApiError {
  return new ApiError('Entity has been modified', {
    kind: 'http',
    status: 412,
    problem: {
      type: 'x',
      title: 'Entity has been modified',
      status: 412,
      conflicts: { content: { expected, actual } },
    },
    original: null,
  })
}

function fakeRoom() {
  const doc = new Y.Doc()
  const base = shallowRef<RoomBase | null>(null)
  let synced!: (ok: boolean) => void
  const whenSynced = new Promise<boolean>((resolve) => (synced = resolve))
  const room = {
    doc,
    awareness: new Awareness(doc),
    status: ref<CollabStatus>('connecting'),
    isSaver: ref(true),
    online: ref(true),
    base,
    whenSynced: () => whenSynced,
    seed: vi.fn(),
    recordSave: vi.fn((content: string, token: string | undefined, sent?: string) => {
      base.value = { content, token, sent }
    }),
    isCurrentBase: (c: string) => base.value?.content === c || base.value?.sent === c,
    destroy: vi.fn(),
  } satisfies CollabRoom
  return { room, synced: () => synced(true) }
}

async function flush() {
  for (let i = 0; i < 3; i++) {
    await Promise.resolve()
    await nextTick()
  }
}

const target = { type: 'ticket', id: 'TKT-1' }

async function setup() {
  const fr = fakeRoom()
  const collab = useBodyCollab({
    enabled: () => true,
    user: () => 'alice',
    closeEditor: vi.fn(),
    notify: vi.fn(),
    build: 'b1',
    reload: vi.fn(),
    storage: { getItem: () => null, setItem: () => {} },
    openRoom: () => fr.room,
  })
  const editor = { flush: vi.fn(), setBaseline: vi.fn(), applyChange: vi.fn() }
  collab.bindEditor(editor)

  const store = useEntitiesStore()
  const updateMock = vi.spyOn(store, 'update') as unknown as Mock
  let n = 1
  updateMock.mockImplementation(async (_t: string, _i: string, body: Record<string, unknown>) =>
    bodyAt(body.content as string, `c${++n}`)
  )
  const hooks = collab.autoSaveHooks(target)
  const contentRef = ref('Hello\n')
  const autoSave = useAutoSave({
    target,
    debounceMs: 100,
    dirtyWindowMs: 200,
    formData: ref({}),
    contentRef,
    inverseToCanonical: new Map(),
    buildRelationsBody: () => null,
    applyServerProperty: vi.fn(),
    applyServerContent: (next: string, cause: string, from?: string) => {
      if (hooks.inRoom()) {
        if (cause === 'merged' && from !== undefined) hooks.mergedContent(from, next)
        return
      }
      contentRef.value = next
    },
    onError: vi.fn(),
    resendContent: hooks.resendContent,
    onContentSaved: hooks.onContentSaved,
  })
  autoSave.recordServerSnapshot(bodyAt('Hello\n', 'c1'))
  const base = { content: 'Hello\n', token: 'c1' }
  collab.start(target, base, autoSave)
  fr.room.base.value = base
  fr.room.status.value = 'synced'
  fr.synced()
  await flush()

  /** The editor emits `body` and the saver's debounce runs out. */
  async function type(body: string) {
    collab.input(body)
    await vi.advanceTimersByTimeAsync(SAVE_DEBOUNCE_MS + 200)
    await flush()
  }
  const call = (i: number) => updateMock.mock.calls[i]![2] as Record<string, unknown>
  const sent = () => updateMock.mock.calls.map((c) => (c[2] as Record<string, unknown>).content)
  return { ...fr, collab, editor, updateMock, autoSave, type, call, sent }
}

describe('useBodyCollab with useAutoSave', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    vi.mocked(entitiesApi.getEntity).mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('saves an undo back to a body the room saved earlier', async () => {
    const t = await setup()
    await t.type('Two\n')
    await t.type('Three\n')
    await t.type('Two\n')
    expect(t.sent()).toEqual(['Two\n', 'Three\n', 'Two\n'])
  })

  it('saves an undo back to the loaded body after a peer saved', async () => {
    const t = await setup()
    // A peer's save moves the room's base.
    t.room.base.value = { content: 'Peer\n', token: 'c7' }
    await flush()
    await t.type('Hello\n')
    expect(t.sent()).toEqual(['Hello\n'])
    expect(t.call(0).preconditions).toEqual({ content: 'c7' })
  })

  it("resends the room's document over the room's latest save", async () => {
    const t = await setup()
    t.updateMock.mockImplementationOnce(async () => {
      // A peer's save lands while this save is in flight.
      t.room.base.value = { content: 'Peer\n', token: 'c7' }
      throw conflict412('c1', 'c7')
    })
    vi.mocked(entitiesApi.getEntity).mockResolvedValue(bodyAt('Peer\n', 'c7'))
    await t.type('Peer, then mine\n')
    await vi.runAllTimersAsync()
    expect(t.updateMock.mock.calls.map((c) => c[2])).toEqual([
      { content: 'Peer, then mine\n', preconditions: { content: 'c1' } },
      { content: 'Peer, then mine\n', preconditions: { content: 'c7' } },
    ])
    expect(t.editor.applyChange).not.toHaveBeenCalled()
  })

  it('puts a clean merge with an outside edit into the room as a change', async () => {
    const t = await setup()
    t.updateMock.mockRejectedValueOnce(conflict412('c1', 'c9'))
    vi.mocked(entitiesApi.getEntity).mockResolvedValue(bodyAt('Hello\n\nOutside\n', 'c9'))
    await t.type('Hello there\n')
    await vi.runAllTimersAsync()
    expect(t.editor.applyChange).toHaveBeenCalledWith('Hello there\n', 'Hello there\n\nOutside\n')
  })
})
