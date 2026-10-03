// Per-field preconditions and conflict merging in useAutoSave (TKT-2VDVHF).

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { ref } from 'vue'
import { useEntitiesStore } from '@/stores/entities'
import {
  useAutoSave,
  CONFLICT_MESSAGE,
  CONTENT_CONFLICT_MESSAGE,
  type AutoSaveOptions,
} from './useAutoSave'
import { ApiError } from '@/api/errors'
import type { Entity, FieldConflicts, FieldVersions } from '@/types'
import * as entitiesApi from '@/api/entities'

vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  getEntity: vi.fn(),
}))

const versions = (over: Partial<FieldVersions> = {}): FieldVersions => ({
  properties: { title: 't-title-1', status: 't-status-1' },
  content: 't-content-1',
  relations: 't-rel-1',
  ...over,
})

function conflict412(conflicts: FieldConflicts): ApiError {
  return new ApiError('Entity has been modified', {
    kind: 'http',
    status: 412,
    problem: { type: 'x', title: 'Entity has been modified', status: 412, conflicts },
    original: null,
  })
}

function harness(snapshot: Partial<Entity> = {}, overrides: Partial<AutoSaveOptions> = {}) {
  const formData = ref<Record<string, unknown>>({ ...(snapshot.properties ?? {}) })
  const contentRef = ref(snapshot.content ?? '')
  const applyServerProperty = vi.fn((k: string, v: unknown) => {
    if (v === undefined) delete formData.value[k]
    else formData.value[k] = v
  })
  const applyServerContent = vi.fn((c: string) => {
    contentRef.value = c
  })
  const onError = vi.fn()
  const buildRelationsBody = vi.fn(() => null)
  const store = useEntitiesStore()
  const updateMock = vi.spyOn(store, 'update') as unknown as Mock
  updateMock.mockImplementation(async (_t: string, _i: string, body: Record<string, unknown>) => ({
    id: 'TKT-001',
    type: 'ticket',
    properties: { ...formData.value, ...((body.properties as object) ?? {}) },
    _versions: versions({ properties: { title: 't-title-2', status: 't-status-2' } }),
  }))
  const autoSave = useAutoSave({
    getEntityType: () => 'ticket',
    getEntityId: () => 'TKT-001',
    debounceMs: 100,
    dirtyWindowMs: 200,
    formData,
    contentRef,
    inverseToCanonical: new Map(),
    buildRelationsBody,
    applyServerProperty,
    applyServerContent,
    onError,
    ...overrides,
  })
  autoSave.recordServerSnapshot({
    id: 'TKT-001',
    type: 'ticket',
    properties: {},
    _versions: versions(),
    ...snapshot,
  } as Entity)
  return {
    formData,
    contentRef,
    applyServerProperty,
    applyServerContent,
    onError,
    updateMock,
    autoSave,
  }
}

const bodyOf = (m: Mock, call: number) => m.mock.calls[call][2] as Record<string, unknown>

describe('useAutoSave preconditions', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    vi.mocked(entitiesApi.getEntity).mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('sends a precondition for exactly the field it writes', async () => {
    const h = harness({ properties: { title: 'T', status: 'open' } })
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 0)).toEqual({
      properties: { status: 'done' },
      preconditions: { properties: { status: 't-status-1' } },
    })
  })

  it('sends no preconditions when the snapshot carried no tokens', async () => {
    const h = harness({ properties: { status: 'open' }, _versions: undefined })
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 0)).toEqual({ properties: { status: 'done' } })
  })

  it('uses the token from its own last save next time', async () => {
    const h = harness({ properties: { status: 'open' } })
    h.autoSave.scheduleFieldSave('status', 'doing')
    await vi.runAllTimersAsync()
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 1).preconditions).toEqual({ properties: { status: 't-status-2' } })
  })

  it('resends unchanged when the 412 names no field', async () => {
    const h = harness({ properties: { status: 'open' } })
    h.updateMock.mockRejectedValueOnce(conflict412({}))
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(h.updateMock).toHaveBeenCalledTimes(2)
    expect(bodyOf(h.updateMock, 1)).toEqual(bodyOf(h.updateMock, 0))
    expect(entitiesApi.getEntity).not.toHaveBeenCalled()
    expect(h.autoSave.status.value).not.toBe('error')
  })

  it('gives up after three attempts and reports the error', async () => {
    const h = harness({ properties: { status: 'open' } })
    h.updateMock.mockRejectedValue(conflict412({}))
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(h.updateMock).toHaveBeenCalledTimes(3)
    expect(h.onError).toHaveBeenCalled()
    expect(h.autoSave.status.value).toBe('error')
  })

  it('writes nothing when the other side already holds our value', async () => {
    const h = harness({ properties: { status: 'open' } })
    h.updateMock.mockRejectedValueOnce(
      conflict412({ properties: { status: { expected: 'a', actual: 'b' } } })
    )
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: { status: 'done' },
      _versions: versions({ properties: { status: 't-status-9' } }),
    } as Entity)
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.onError).not.toHaveBeenCalled()
    expect(h.formData.value.status).toBe('done')
  })

  it('reports a real conflict, writes nothing, and keeps the user value on screen', async () => {
    const h = harness({ properties: { status: 'open' } })
    h.updateMock.mockRejectedValueOnce(
      conflict412({ properties: { status: { expected: 'a', actual: 'b' } } })
    )
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: { status: 'blocked' },
      _versions: versions({ properties: { status: 't-status-9' } }),
    } as Entity)
    h.formData.value.status = 'done'
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()

    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.autoSave.fieldErrors.value.status).toBe(CONFLICT_MESSAGE)
    expect(h.onError).toHaveBeenCalledWith(
      CONFLICT_MESSAGE,
      expect.objectContaining({ status: 412 })
    )
    expect(h.formData.value.status).toBe('done')

    // Editing again overwrites deliberately: the base is now their value.
    h.autoSave.scheduleFieldSave('status', 'done!')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 1).preconditions).toEqual({ properties: { status: 't-status-9' } })
  })

  it('merges a body edited on different lines and resends with the fresh token', async () => {
    const h = harness({ properties: {}, content: 'a\nb\nc\n' })
    h.updateMock.mockRejectedValueOnce(conflict412({ content: { expected: 'x', actual: 'y' } }))
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: {},
      content: 'a\nb\nC\n',
      _versions: versions({ content: 't-content-9' }),
    } as Entity)
    h.autoSave.scheduleContentSave('A\nb\nc\n')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 1)).toEqual({
      content: 'A\nb\nC\n',
      preconditions: { content: 't-content-9' },
    })
  })

  it('refuses a body edited on the same line', async () => {
    const h = harness({ properties: {}, content: 'a\n' })
    h.updateMock.mockRejectedValueOnce(conflict412({ content: { expected: 'x', actual: 'y' } }))
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: {},
      content: 'theirs\n',
      _versions: versions({ content: 't-content-9' }),
    } as Entity)
    h.contentRef.value = 'mine\n'
    h.autoSave.scheduleContentSave('mine\n')
    await vi.runAllTimersAsync()
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.autoSave.contentError.value).toBe(CONTENT_CONFLICT_MESSAGE)
    expect(h.contentRef.value).toBe('mine\n')
  })

  it('merges relation sets and types their additions from the include map', async () => {
    const buildRelationsBody = vi.fn(() => ({
      implements: {
        data: [
          { type: 'feature', id: 'F1' },
          { type: 'feature', id: 'F2' },
        ],
      },
    }))
    const h = harness({ properties: {}, relations: { implements: ['F1'] } }, { buildRelationsBody })
    h.updateMock.mockRejectedValueOnce(conflict412({ relations: { expected: 'x', actual: 'y' } }))
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: {},
      relations: { implements: ['F1', 'F3'] },
      included: { F3: { id: 'F3', type: 'feature', properties: {} } },
      _versions: versions({ relations: 't-rel-9' }),
    } as Entity)
    h.autoSave.scheduleRelationsChange()
    await vi.runAllTimersAsync()

    expect(entitiesApi.getEntity).toHaveBeenCalledWith(
      'ticket',
      'TKT-001',
      expect.objectContaining({ include: 'implements' }),
      expect.anything()
    )
    const second = bodyOf(h.updateMock, 1) as {
      relations: { implements: { data: { id: string }[] } }
    }
    expect(second.relations.implements.data.map((e) => e.id).sort()).toEqual(['F1', 'F2', 'F3'])
    expect((second as Record<string, unknown>).preconditions).toEqual({ relations: 't-rel-9' })
  })

  // The base must not run ahead of the form. A response to another save
  // carries someone else's new status while the user has an unsent edit to
  // it; that edit must still be checked against the value the user saw.
  it('keeps the base of a dirty field when another save returns a newer value', async () => {
    const h = harness({ properties: { title: 'T', status: 'open' } })
    h.updateMock.mockResolvedValueOnce({
      id: 'TKT-001',
      type: 'ticket',
      properties: { title: 'T2', status: 'blocked' },
      _versions: versions({ properties: { title: 't-title-2', status: 't-status-theirs' } }),
    })
    h.autoSave.scheduleFieldSave('title', 'T2')
    await vi.advanceTimersByTimeAsync(50)
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 1).preconditions).toEqual({ properties: { status: 't-status-1' } })
  })

  // A snapshot (a form reload) must not move the base of a field the user
  // is still editing: the edit derives from the old value.
  it('keeps the base of a pending field across a fresh snapshot', async () => {
    const h = harness({ properties: { title: 'T' } })
    h.autoSave.scheduleFieldSave('title', 'mine')
    h.autoSave.recordServerSnapshot({
      id: 'TKT-001',
      type: 'ticket',
      properties: { title: 'theirs' },
      _versions: versions({ properties: { title: 't-title-9' } }),
    } as Entity)
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 0).preconditions).toEqual({ properties: { title: 't-title-1' } })
  })

  // A body save queued behind a property save is unsaved state too. The
  // property response must not overwrite the editor or move its base.
  it('treats a queued body save as unsaved when an earlier response lands', async () => {
    const h = harness({ properties: { status: 'open' }, content: 'a\n' })
    let answer!: (e: Entity) => void
    h.updateMock.mockImplementationOnce(() => new Promise<Entity>((r) => (answer = r)))
    h.autoSave.scheduleFieldSave('status', 'done')
    h.autoSave.scheduleContentSave('mine\n')
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    answer({
      id: 'TKT-001',
      type: 'ticket',
      properties: { status: 'done' },
      content: 'theirs\n',
      _versions: versions({ content: 't-content-9' }),
    } as Entity)
    await vi.runAllTimersAsync()
    expect(h.applyServerContent).not.toHaveBeenCalledWith('theirs\n')
    expect(bodyOf(h.updateMock, 1)).toEqual({
      content: 'mine\n',
      preconditions: { content: 't-content-1' },
    })
  })

  // After a body conflict the editor keeps the other side's clean hunks, so
  // the next save overwrites only the region that conflicted.
  it('shows their clean hunks after a body conflict', async () => {
    const h = harness({ properties: {}, content: 'a\nb\nc\n' })
    h.updateMock.mockRejectedValueOnce(conflict412({ content: { expected: 'x', actual: 'y' } }))
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: {},
      content: 'A2\nb\nC\n',
      _versions: versions({ content: 't-content-9' }),
    } as Entity)
    h.contentRef.value = 'A1\nb\nc\n'
    h.autoSave.scheduleContentSave('A1\nb\nc\n')
    await vi.runAllTimersAsync()
    expect(h.autoSave.contentError.value).toBe(CONTENT_CONFLICT_MESSAGE)
    expect(h.contentRef.value).toBe('A1\nb\nC\n')

    h.autoSave.scheduleContentSave('A1\nb\nC\nd\n')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 1)).toEqual({
      content: 'A1\nb\nC\nd\n',
      preconditions: { content: 't-content-9' },
    })
  })

  // A save that runs out of attempts reports a generic error. The conflict
  // it found on the way was never shown, so no base may have moved.
  it('moves no base when the attempts run out', async () => {
    const h = harness({ properties: { title: 'T', status: 'open' } })
    h.updateMock
      .mockRejectedValueOnce(
        conflict412({ properties: { status: { expected: 'a', actual: 'b' } } })
      )
      .mockRejectedValueOnce(conflict412({}))
      .mockRejectedValueOnce(conflict412({}))
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: { title: 'T', status: 'blocked' },
      _versions: versions({ properties: { title: 't-title-1', status: 't-status-9' } }),
    } as Entity)
    h.autoSave.scheduleFieldSave('title', 'T2')
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(h.updateMock).toHaveBeenCalledTimes(3)
    expect(h.autoSave.status.value).toBe('error')

    h.autoSave.scheduleFieldSave('status', 'done!')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 3).preconditions).toEqual({ properties: { status: 't-status-1' } })
  })

  it('reports a clean body merge it cannot guard rather than drop it', async () => {
    const h = harness({ properties: {}, content: 'a\nb\nc\n' })
    h.updateMock.mockRejectedValueOnce(conflict412({ content: { expected: 'x', actual: 'y' } }))
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: {},
      content: 'a\nb\nC\n',
    } as Entity)
    h.autoSave.scheduleContentSave('A\nb\nc\n')
    await vi.runAllTimersAsync()
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.autoSave.contentError.value).toBe(CONTENT_CONFLICT_MESSAGE)
  })

  // AC10 (RR-R2A2T5): a property the refetch does not carry (redacted for
  // this user) is never turned into an unset by the merge.
  it('never unsets a property the refetch omits', async () => {
    const h = harness({ properties: { status: 'open', salary: 100 } })
    h.updateMock.mockRejectedValueOnce(
      conflict412({ properties: { status: { expected: 'a', actual: 'b' } } })
    )
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: { status: 'open' },
      _versions: versions({ properties: { status: 't-status-9' } }),
    } as Entity)
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    for (const call of h.updateMock.mock.calls) {
      expect((call[2] as Record<string, unknown>).properties_unset).toBeUndefined()
    }
    expect(bodyOf(h.updateMock, 1)).toEqual({
      properties: { status: 'done' },
      preconditions: { properties: { status: 't-status-9' } },
    })
  })
})
