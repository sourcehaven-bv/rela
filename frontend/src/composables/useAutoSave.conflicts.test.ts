// Per-field preconditions and conflict merging in useAutoSave (TKT-2VDVHF).

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { ref } from 'vue'
import { useEntitiesStore } from '@/stores/entities'
import {
  useAutoSave,
  conflictMessage,
  contentConflictMessage,
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
    target: { type: 'ticket', id: 'TKT-001' },
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
    expect(h.autoSave.fieldErrors.value.status).toBe(conflictMessage())
    expect(h.onError).toHaveBeenCalledWith(
      conflictMessage(),
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
    expect(h.autoSave.contentError.value).toBe(contentConflictMessage())
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
    expect(h.autoSave.contentError.value).toBe(contentConflictMessage())
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
    expect(h.autoSave.contentError.value).toBe(contentConflictMessage())
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

// Tokens are derived from the value. Another user writing back a value this
// client saved earlier gets the same token as that save, and must still be
// treated as their change (BUG-04V034W review): ticked on and off by us, then
// on again by them.
describe('useAutoSave after another user reverts to an older value of ours', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    vi.mocked(entitiesApi.getEntity).mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  const entity = (over: Partial<Entity>): Entity => ({
    id: 'TKT-001',
    type: 'ticket',
    properties: {},
    ...over,
  })
  const bodyAt = (content: string, token: string) =>
    entity({ content, _versions: versions({ content: token }) })
  const statusAt = (status: string, token: string) =>
    entity({ properties: { status }, _versions: versions({ properties: { status: token } }) })

  // Saves the body A, then B; the server answers each with its token.
  async function savedAThenB(h: ReturnType<typeof harness>) {
    h.updateMock.mockResolvedValueOnce(bodyAt('A', 'cA'))
    h.autoSave.scheduleContentSave('A')
    await vi.runAllTimersAsync()
    h.updateMock.mockResolvedValueOnce(bodyAt('B', 'cB'))
    h.autoSave.scheduleContentSave('B')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 1).preconditions).toEqual({ content: 'cA' })
  }

  it('merges a 412 on the body instead of resending over their revert', async () => {
    const h = harness({ content: 'v0', _versions: versions({ content: 'c0' }) })
    await savedAThenB(h)
    // They write A back. Our next save is refused, and the body they hold
    // carries the token of our first save.
    h.updateMock.mockRejectedValueOnce(conflict412({ content: { expected: 'cB', actual: 'cA' } }))
    vi.mocked(entitiesApi.getEntity).mockResolvedValue(bodyAt('A', 'cA'))
    h.autoSave.scheduleContentSave('C')
    await vi.runAllTimersAsync()

    // base B, ours C, theirs A: the same line changed on both sides.
    expect(h.updateMock).toHaveBeenCalledTimes(3)
    expect(h.onError).toHaveBeenCalledWith(contentConflictMessage(), expect.anything())
  })

  it('takes a snapshot of their revert to an older body of ours', async () => {
    const h = harness({ content: 'v0', _versions: versions({ content: 'c0' }) })
    await savedAThenB(h)
    h.autoSave.recordServerSnapshot(bodyAt('A', 'cA'))
    h.autoSave.scheduleContentSave('C')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 2)).toEqual({ content: 'C', preconditions: { content: 'cA' } })
  })

  it('merges a 412 on a property instead of resending over their revert', async () => {
    const h = harness({ properties: { status: 'off' }, _versions: versions() })
    h.updateMock.mockResolvedValueOnce(statusAt('on', 's-on'))
    h.autoSave.scheduleFieldSave('status', 'on')
    await vi.runAllTimersAsync()
    h.updateMock.mockResolvedValueOnce(statusAt('off', 's-off'))
    h.autoSave.scheduleFieldSave('status', 'off')
    await vi.runAllTimersAsync()

    // They tick it on again; we choose a third value meanwhile.
    h.updateMock.mockRejectedValueOnce(
      conflict412({ properties: { status: { expected: 's-off', actual: 's-on' } } })
    )
    vi.mocked(entitiesApi.getEntity).mockResolvedValue(statusAt('on', 's-on'))
    h.autoSave.scheduleFieldSave('status', 'maybe')
    await vi.runAllTimersAsync()

    expect(h.updateMock).toHaveBeenCalledTimes(3)
    expect(h.onError).toHaveBeenCalledWith(conflictMessage(), expect.anything())
    expect(h.autoSave.fieldErrors.value.status).toBe(conflictMessage())
  })
})

// BUG-NSGD6OE: an instance holds bases and edits for ONE entity. State about
// any other entity is refused, so the 412 merge can never fold another
// entity's values into a write.
describe('useAutoSave target identity', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    vi.mocked(entitiesApi.getEntity).mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('writes to the target it was built with, even if the caller mutates it later', async () => {
    const target = { type: 'ticket', id: 'TKT-001' }
    const h = harness({ properties: { status: 'open' } }, { target })
    target.id = 'TKT-002'
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(h.updateMock.mock.calls.map((c) => c[1])).toEqual(['TKT-001'])
  })

  it('refuses a snapshot of another entity', () => {
    const h = harness()
    expect(() =>
      h.autoSave.recordServerSnapshot({ id: 'TKT-002', type: 'ticket', properties: {} } as Entity)
    ).toThrow(/TKT-002/)
  })

  it('accepts a snapshot of its own row named by a faced address', () => {
    const h = harness({}, { target: { type: 'ticket', id: 'TKT-001@published' } })
    expect(() =>
      h.autoSave.recordServerSnapshot({ id: 'TKT-001', type: 'ticket', properties: {} } as Entity)
    ).not.toThrow()
  })

  it('accepts its own faced row and refuses another face of the same entity', () => {
    const h = harness({}, { target: { type: 'ticket', id: 'TKT-001@published' } })
    const row = (face: string) =>
      ({
        id: 'TKT-001',
        type: 'ticket',
        properties: {},
        _self: `/api/v1/tickets/TKT-001@${face}`,
      }) as Entity
    expect(() => h.autoSave.recordServerSnapshot(row('published'))).not.toThrow()
    expect(() => h.autoSave.recordServerSnapshot(row('draft'))).toThrow(/TKT-001/)
  })

  it('fails the save, without a merged resend, when the 412 refetch names another entity', async () => {
    const h = harness({ properties: { status: 'open' } })
    h.updateMock.mockRejectedValueOnce(
      conflict412({ properties: { status: { expected: 't-status-1', actual: 't-status-9' } } })
    )
    vi.mocked(entitiesApi.getEntity).mockResolvedValue({
      id: 'TKT-002',
      type: 'ticket',
      properties: { status: 'open' },
      _versions: versions({ properties: { status: 't-status-9' } }),
    } as Entity)
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.onError).toHaveBeenCalled()
    expect(h.autoSave.status.value).toBe('error')
  })
})

// Found by useAutoSave.twoClients.prop.test.ts.
describe('useAutoSave across saves of one tab', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    vi.mocked(entitiesApi.getEntity).mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  const entity = (over: Partial<Entity>) => ({ id: 'TKT-001', type: 'ticket', ...over }) as Entity

  it('merges a second 412 in one save against the body the first merge used', async () => {
    const h = harness({ properties: {}, content: 'a\n-\nb\n-\nc' })
    const refused = conflict412({ content: { expected: 'x', actual: 'y' } })
    h.updateMock.mockRejectedValueOnce(refused).mockRejectedValueOnce(refused)
    vi.mocked(entitiesApi.getEntity)
      .mockResolvedValueOnce(
        entity({ content: 'A1\n-\nb\n-\nc', _versions: versions({ content: 't-9' }) })
      )
      .mockResolvedValueOnce(
        entity({ content: 'A2\n-\nb\n-\nC', _versions: versions({ content: 't-10' }) })
      )
    h.autoSave.scheduleContentSave('a\n-\nB\n-\nc')
    await vi.runAllTimersAsync()
    expect(h.autoSave.contentError.value).toBeNull()
    expect(bodyOf(h.updateMock, 2)).toEqual({
      content: 'A2\n-\nB\n-\nC',
      preconditions: { content: 't-10' },
    })
  })

  it('merges a second 412 in one save against the relations the first merge used', async () => {
    const buildRelationsBody = vi.fn(() => ({
      implements: {
        data: [
          { type: 'feature', id: 'F1' },
          { type: 'feature', id: 'F2' },
        ],
      },
    }))
    const h = harness({ properties: {}, relations: { implements: ['F1'] } }, { buildRelationsBody })
    const refused = conflict412({ relations: { expected: 'x', actual: 'y' } })
    h.updateMock.mockRejectedValueOnce(refused).mockRejectedValueOnce(refused)
    const included = { F3: { id: 'F3', type: 'feature', properties: {} } }
    vi.mocked(entitiesApi.getEntity)
      .mockResolvedValueOnce(
        entity({
          relations: { implements: ['F1', 'F3'] },
          included,
          _versions: versions({ relations: 't-9' }),
        })
      )
      // They removed F3 again before the merged resend landed.
      .mockResolvedValueOnce(
        entity({ relations: { implements: ['F1'] }, _versions: versions({ relations: 't-10' }) })
      )
    h.autoSave.scheduleRelationsChange()
    await vi.runAllTimersAsync()

    const third = bodyOf(h.updateMock, 2) as {
      relations: { implements: { data: { id: string }[] } }
      preconditions: unknown
    }
    expect(third.relations.implements.data.map((e) => e.id).sort()).toEqual(['F1', 'F2'])
    expect(third.preconditions).toEqual({ relations: 't-10' })
  })

  it('rebases typing onto its own merged save instead of reporting a conflict', async () => {
    const h = harness({ properties: {}, content: 'a\n-\nb' })
    h.updateMock.mockRejectedValueOnce(conflict412({ content: { expected: 'x', actual: 'y' } }))
    vi.mocked(entitiesApi.getEntity).mockResolvedValueOnce(
      entity({ content: 'a\n-\nB', _versions: versions({ content: 't-9' }) })
    )
    let answer!: (e: Entity) => void
    h.updateMock.mockImplementationOnce(() => new Promise<Entity>((r) => (answer = r)))
    h.autoSave.scheduleContentSave('A\n-\nb')
    await vi.advanceTimersByTimeAsync(1000)
    expect(bodyOf(h.updateMock, 1).content).toBe('A\n-\nB')

    // The user keeps typing while the merged save is on its way.
    h.contentRef.value = 'A2\n-\nb'
    h.autoSave.scheduleContentSave('A2\n-\nb')
    answer(entity({ content: 'A\n-\nB', _versions: versions({ content: 't-10' }) }))
    await vi.runAllTimersAsync()

    expect(bodyOf(h.updateMock, 2)).toEqual({
      content: 'A2\n-\nB',
      preconditions: { content: 't-10' },
    })
    expect(h.autoSave.contentError.value).toBeNull()
  })

  it('keeps a conflicting value on screen when a later save answers', async () => {
    const h = harness({ properties: { status: 'open', title: 'T' } })
    h.updateMock.mockRejectedValueOnce(
      conflict412({ properties: { status: { expected: 'a', actual: 'b' } } })
    )
    vi.mocked(entitiesApi.getEntity).mockResolvedValueOnce(
      entity({ properties: { status: 'blocked', title: 'T' }, _versions: versions() })
    )
    h.formData.value.status = 'done'
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()

    h.updateMock.mockResolvedValueOnce(
      entity({ properties: { status: 'blocked', title: 'T2' }, _versions: versions() })
    )
    h.formData.value.title = 'T2'
    h.autoSave.scheduleFieldSave('title', 'T2')
    await vi.runAllTimersAsync()

    expect(h.formData.value).toEqual({ status: 'done', title: 'T2' })
    expect(h.autoSave.fieldErrors.value.status).toBe(conflictMessage())
  })

  it('keeps a value whose save failed on screen when a later save answers', async () => {
    const h = harness({ properties: { status: 'open', title: 'T' } })
    h.updateMock.mockRejectedValueOnce(
      new ApiError('unavailable', { kind: 'http', status: 503, original: null })
    )
    h.formData.value.status = 'done'
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()

    h.updateMock.mockResolvedValueOnce(
      entity({ properties: { status: 'open', title: 'T2' }, _versions: versions() })
    )
    h.formData.value.title = 'T2'
    h.autoSave.scheduleFieldSave('title', 'T2')
    await vi.runAllTimersAsync()

    expect(h.formData.value).toEqual({ status: 'done', title: 'T2' })
    expect(h.autoSave.fieldErrors.value.status).toBeTruthy()

    // Editing it again sends it, and the error clears once it is saved.
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()
    expect(bodyOf(h.updateMock, 2).properties).toEqual({ status: 'done' })
  })

  it('releases a field once a later save of it goes through', async () => {
    const h = harness({ properties: { status: 'open', title: 'T' } })
    let fail!: (e: unknown) => void
    h.updateMock.mockImplementationOnce(() => new Promise<Entity>((_, r) => (fail = r)))
    h.formData.value.status = 'v1'
    h.autoSave.scheduleFieldSave('status', 'v1')
    await vi.advanceTimersByTimeAsync(1000)

    // A second save of the field queues behind the first, which then fails.
    h.updateMock.mockResolvedValueOnce(
      entity({ properties: { status: 'v2', title: 'T' }, _versions: versions() })
    )
    h.formData.value.status = 'v2'
    h.autoSave.scheduleFieldSave('status', 'v2')
    await vi.advanceTimersByTimeAsync(1000)
    fail(new ApiError('unavailable', { kind: 'http', status: 503, original: null }))
    await vi.runAllTimersAsync()
    expect(h.autoSave.fieldErrors.value.status).toBeUndefined()
    expect(h.autoSave.unsavedFields().props).toEqual([])

    // So a later answer shows the field's server value.
    h.updateMock.mockResolvedValueOnce(
      entity({ properties: { status: 'theirs', title: 'T2' }, _versions: versions() })
    )
    h.formData.value.title = 'T2'
    h.autoSave.scheduleFieldSave('title', 'T2')
    await vi.runAllTimersAsync()
    expect(h.formData.value.status).toBe('theirs')
  })

  it('rebases typing onto their body when it already held the save', async () => {
    const h = harness({ properties: {}, content: 'a\n-\nb' })
    let refuse!: (e: unknown) => void
    h.updateMock.mockImplementationOnce(() => new Promise<Entity>((_, r) => (refuse = r)))
    // Their body already holds our edit, so nothing is written for it.
    vi.mocked(entitiesApi.getEntity).mockResolvedValueOnce(
      entity({ content: 'A\n-\nB', _versions: versions({ content: 't-9' }) })
    )
    h.autoSave.scheduleContentSave('A\n-\nb')
    await vi.advanceTimersByTimeAsync(1000)

    // The user keeps typing before the 412 is answered.
    h.contentRef.value = 'A2\n-\nb'
    h.autoSave.scheduleContentSave('A2\n-\nb')
    h.updateMock.mockResolvedValueOnce(
      entity({ content: 'A2\n-\nB', _versions: versions({ content: 't-10' }) })
    )
    refuse(conflict412({ content: { expected: 'x', actual: 'y' } }))
    await vi.runAllTimersAsync()

    expect(bodyOf(h.updateMock, 1)).toEqual({
      content: 'A2\n-\nB',
      preconditions: { content: 't-9' },
    })
    expect(h.autoSave.contentError.value).toBeNull()
  })

  it('reports a held field to commitImmediately', async () => {
    const h = harness({ properties: { status: 'open' } })
    h.updateMock.mockRejectedValueOnce(
      new ApiError('unavailable', { kind: 'http', status: 503, original: null })
    )
    h.formData.value.status = 'done'
    h.autoSave.scheduleFieldSave('status', 'done')
    await vi.runAllTimersAsync()

    const result = await h.autoSave.commitImmediately()
    expect(result.settled).toBe(true)
    expect(result.error).toBeTruthy()
  })

  it('reverts the body to the one its merged save wrote', async () => {
    const h = harness({ properties: {}, content: 'a\n-\nb' })
    h.updateMock.mockRejectedValueOnce(conflict412({ content: { expected: 'x', actual: 'y' } }))
    vi.mocked(entitiesApi.getEntity).mockResolvedValueOnce(
      entity({ content: 'a\n-\nB', _versions: versions({ content: 't-9' }) })
    )
    let answer!: (e: Entity) => void
    h.updateMock.mockImplementationOnce(() => new Promise<Entity>((r) => (answer = r)))
    h.autoSave.scheduleContentSave('A\n-\nb')
    await vi.advanceTimersByTimeAsync(1000)
    h.contentRef.value = 'A2\n-\nb'
    h.autoSave.scheduleContentSave('A2\n-\nb')
    answer(entity({ content: 'A\n-\nB', _versions: versions({ content: 't-10' }) }))
    await vi.advanceTimersByTimeAsync(10)

    // Reverted before the typing is sent.
    h.autoSave.revertContent()
    expect(h.contentRef.value).toBe('A\n-\nB')
  })
})
