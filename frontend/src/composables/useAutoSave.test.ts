// Unit tests for useAutoSave (TKT-E6094).
//
// Mocks `entitiesStore.update` at the store level (Pinia) so we drive
// PATCH timing with fake timers. The composable's relations channel,
// commitImmediately, and warning categorization paths are all covered.

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { ref } from 'vue'
import { useEntitiesStore } from '@/stores/entities'
import { useAutoSave, type AutoSaveOptions, type AutoSaveWarning } from './useAutoSave'
import { ApiError } from '@/api/errors'
import type { Entity } from '@/types'

interface Harness {
  formData: ReturnType<typeof ref<Record<string, unknown>>>
  contentRef: ReturnType<typeof ref<string>>
  applyServerProperty: Mock
  applyServerContent: Mock
  onError: Mock
  buildRelationsBody: Mock
  updateMock: Mock
  autoSave: ReturnType<typeof useAutoSave>
}

function makeHarness(
  initial: Record<string, unknown> = {},
  overrides: Partial<AutoSaveOptions> = {},
): Harness {
  const formData = ref<Record<string, unknown>>({ ...initial })
  const contentRef = ref('')
  const applyServerProperty = vi.fn((prop: string, value: unknown) => {
    if (value === undefined) {
      delete (formData.value as Record<string, unknown>)[prop]
    } else {
      ;(formData.value as Record<string, unknown>)[prop] = value
    }
  })
  const applyServerContent = vi.fn((c: string) => {
    contentRef.value = c
  })
  const onError = vi.fn()
  const buildRelationsBody = vi.fn(() => null)
  const opts: AutoSaveOptions = {
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
  }
  const store = useEntitiesStore()
  const updateMock = vi.spyOn(store, 'update').mockResolvedValue({
    id: 'TKT-001',
    type: 'ticket',
    properties: {},
  } as Entity) as unknown as Mock

  const autoSave = useAutoSave(opts)
  autoSave.recordServerSnapshot({
    id: 'TKT-001',
    type: 'ticket',
    properties: { ...initial },
  } as Entity)

  return {
    formData,
    contentRef,
    applyServerProperty,
    applyServerContent,
    onError,
    buildRelationsBody,
    updateMock,
    autoSave,
  }
}

describe('useAutoSave', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('AC1: per-property PATCH carries only the changed property', async () => {
    const h = makeHarness({ title: 'Original', status: 'open' })
    h.autoSave.scheduleFieldSave('status', 'closed')
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.updateMock).toHaveBeenCalledWith(
      'ticket', 'TKT-001',
      { properties: { status: 'closed' } },
      undefined,
      expect.any(AbortSignal),
    )
  })

  it('AC2: properties_unset wire shape for clear', async () => {
    const h = makeHarness({ title: 'Original' })
    h.autoSave.scheduleUnset('title')
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.updateMock).toHaveBeenCalledWith(
      'ticket', 'TKT-001',
      { properties_unset: ['title'] },
      undefined,
      expect.any(AbortSignal),
    )
  })

  it('AC3: rapid edits coalesce to one PATCH with the latest value', async () => {
    const h = makeHarness({ title: 'Original' })
    h.autoSave.scheduleFieldSave('title', 'a')
    await vi.advanceTimersByTimeAsync(20)
    h.autoSave.scheduleFieldSave('title', 'ab')
    await vi.advanceTimersByTimeAsync(20)
    h.autoSave.scheduleFieldSave('title', 'abc')
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.updateMock.mock.calls[0][2]).toEqual({ properties: { title: 'abc' } })
  })

  it('AC5: content edit fires content patch after debounce', async () => {
    const h = makeHarness({})
    h.autoSave.scheduleContentSave('new body')
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.updateMock.mock.calls[0][2]).toEqual({ content: 'new body' })
  })

  it('AC4: two edits to different fields serialize through the FIFO queue', async () => {
    const h = makeHarness({ a: 'x', b: 'y' })
    h.autoSave.scheduleFieldSave('a', 'A')
    await vi.advanceTimersByTimeAsync(50)
    h.autoSave.scheduleFieldSave('b', 'B')
    await vi.advanceTimersByTimeAsync(250)
    expect(h.updateMock).toHaveBeenCalledTimes(2)
    const order = h.updateMock.mock.calls.map((c) => Object.keys((c[2] as { properties: object }).properties)[0])
    expect(order).toEqual(['a', 'b'])
  })

  it('AC-R1: scheduleRelationsChange fires a relations-only PATCH', async () => {
    const h = makeHarness({})
    h.buildRelationsBody.mockReturnValue({
      tagged: { data: [{ type: 'label', id: 'L-1' }] },
    })
    h.autoSave.scheduleRelationsChange()
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.updateMock.mock.calls[0][2]).toEqual({
      relations: { tagged: { data: [{ type: 'label', id: 'L-1' }] } },
    })
  })

  it('AC-R2: pristine relations Map produces no PATCH', async () => {
    const h = makeHarness({})
    h.buildRelationsBody.mockReturnValue(null)
    h.autoSave.scheduleRelationsChange()
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).not.toHaveBeenCalled()
  })

  it('AC-R3: property + relation edit bundle into ONE PATCH', async () => {
    const h = makeHarness({ title: 'x' })
    h.buildRelationsBody.mockReturnValue({
      tagged: { data: [{ type: 'label', id: 'L-1' }] },
    })
    h.autoSave.scheduleFieldSave('title', 'updated')
    h.autoSave.scheduleRelationsChange()
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.updateMock.mock.calls[0][2]).toEqual({
      properties: { title: 'updated' },
      relations: { tagged: { data: [{ type: 'label', id: 'L-1' }] } },
    })
  })

  it('warnings under /properties/<field> route to fieldWarnings', async () => {
    const warning: AutoSaveWarning = {
      code: 'property_value_invalid',
      path: '/properties/status',
      detail: 'not in enum',
    }
    const h = makeHarness({ title: 'x' })
    h.updateMock.mockResolvedValue({
      id: 'TKT-001', type: 'ticket', properties: {},
      warnings: [warning],
    } as Entity)
    h.autoSave.scheduleFieldSave('status', 'bogus')
    await vi.advanceTimersByTimeAsync(150)
    expect(h.autoSave.fieldWarnings.value.status).toMatchObject({ code: 'property_value_invalid' })
  })

  it('warnings under /relations/<inverse-key> with direction:incoming map to canonical widget id', async () => {
    const warning: AutoSaveWarning = {
      code: 'unknown_meta_key',
      path: '/relations/blockedBy/data/0/meta/severity',
      detail: 'unknown meta',
      direction: 'incoming',
    }
    const inverseToCanonical = new Map([['blockedBy', 'blocks']])
    const h = makeHarness({}, { inverseToCanonical })
    h.updateMock.mockResolvedValue({
      id: 'TKT-001', type: 'ticket', properties: {},
      warnings: [warning],
    } as Entity)
    h.buildRelationsBody.mockReturnValue({
      blockedBy: { data: [{ type: 'ticket', id: 'T-1', meta: { severity: 'x' } }] },
    })
    h.autoSave.scheduleRelationsChange()
    await vi.advanceTimersByTimeAsync(150)
    expect(h.autoSave.relationWarnings.value['blocks-incoming']).toMatchObject({
      code: 'unknown_meta_key',
    })
  })

  it('422 on a property surfaces fieldError; other fields keep working', async () => {
    const h = makeHarness({ title: 'x' })
    h.updateMock.mockRejectedValueOnce(
      new ApiError('invalid value', { kind: 'http', status: 422, original: null })
    )
    h.autoSave.scheduleFieldSave('title', 'bad')
    await vi.advanceTimersByTimeAsync(150)
    await vi.runOnlyPendingTimersAsync()
    expect(h.autoSave.fieldErrors.value.title).toBe('invalid value')
    // Second save resets the mock to default (no error).
    h.updateMock.mockResolvedValueOnce({ id: 'TKT-001', type: 'ticket', properties: {} } as Entity)
    h.autoSave.scheduleFieldSave('title', 'better')
    await vi.advanceTimersByTimeAsync(150)
    // First save's error should be cleared after the successful retry.
    expect(h.autoSave.fieldErrors.value.title).toBeUndefined()
  })

  it('commitImmediately resolves with settled:true on a quiet queue', async () => {
    const h = makeHarness({})
    const p = h.autoSave.commitImmediately()
    await vi.advanceTimersByTimeAsync(0)
    await expect(p).resolves.toEqual({ settled: true })
  })

  it('commitImmediately flushes pending timers and waits for the chain', async () => {
    const h = makeHarness({ title: 'x' })
    h.autoSave.scheduleFieldSave('title', 'changed')
    const p = h.autoSave.commitImmediately()
    await vi.advanceTimersByTimeAsync(150)
    const result = await p
    expect(result.settled).toBe(true)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
  })

  it('no-op suppression: setting a property back to the last-seen value emits no PATCH', async () => {
    const h = makeHarness({ status: 'open' })
    h.autoSave.scheduleFieldSave('status', 'open')
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).not.toHaveBeenCalled()
  })

  it('mergeServerResponse skips dirty fields and updates lastSeenServer', async () => {
    const h = makeHarness({ title: 'old' })
    h.autoSave.scheduleFieldSave('title', 'user-typed')
    // SSE-style merge arrives while user is still typing.
    h.autoSave.mergeServerResponse({
      id: 'TKT-001', type: 'ticket',
      properties: { title: 'server-changed', status: 'new' },
    } as Entity)
    // Dirty field is preserved.
    expect(h.applyServerProperty).not.toHaveBeenCalledWith('title', 'server-changed')
    // Non-dirty new field applied.
    expect(h.applyServerProperty).toHaveBeenCalledWith('status', 'new')
  })

  // ---- TKT-IHC7A ----------------------------------------------------------

  it('IHC7A AC1: fieldDebounceMs and contentDebounceMs fire independently', async () => {
    const h = makeHarness(
      { title: 'orig' },
      { fieldDebounceMs: 800, contentDebounceMs: 100 },
    )
    h.autoSave.scheduleFieldSave('title', 'field-edit')
    h.autoSave.scheduleContentSave('content-edit')
    // Content fires first (100ms), field still pending.
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.updateMock.mock.calls[0][2]).toEqual({ content: 'content-edit' })
    // Field fires later (800ms total).
    await vi.advanceTimersByTimeAsync(800)
    expect(h.updateMock).toHaveBeenCalledTimes(2)
    expect(h.updateMock.mock.calls[1][2]).toEqual({ properties: { title: 'field-edit' } })
  })

  it('IHC7A AC1: per-channel debounce overrides legacy debounceMs', async () => {
    const h = makeHarness(
      { title: 'orig' },
      { debounceMs: 800, contentDebounceMs: 100 },
    )
    h.autoSave.scheduleFieldSave('title', 'A')
    h.autoSave.scheduleContentSave('B')
    // Only content fires at 150ms; field still bound to legacy 800ms.
    await vi.advanceTimersByTimeAsync(150)
    expect(h.updateMock).toHaveBeenCalledTimes(1)
    expect(h.updateMock.mock.calls[0][2]).toEqual({ content: 'B' })
  })

  it('IHC7A AC2: initialServerSnapshot seeds the no-op baseline', async () => {
    // Override the makeHarness default snapshot by passing
    // initialServerSnapshot AND skipping the post-construct recordSnapshot.
    const formData = ref<Record<string, unknown>>({ title: 'seeded' })
    const contentRef = ref('')
    const apply = vi.fn()
    const seed = {
      id: 'TKT-001', type: 'ticket',
      properties: { title: 'seeded' },
    } satisfies Partial<Entity>
    const opts: AutoSaveOptions = {
      getEntityType: () => 'ticket',
      getEntityId: () => 'TKT-001',
      debounceMs: 100,
      dirtyWindowMs: 200,
      formData,
      contentRef,
      inverseToCanonical: new Map(),
      buildRelationsBody: () => null,
      applyServerProperty: apply,
      applyServerContent: vi.fn(),
      onError: vi.fn(),
      initialServerSnapshot: seed as Entity,
    }
    const store = useEntitiesStore()
    const updateMock = vi.spyOn(store, 'update').mockResolvedValue({
      id: 'TKT-001', type: 'ticket', properties: {},
    } as Entity)
    const auto = useAutoSave(opts)
    // Same value as the seed — should be suppressed as a no-op.
    auto.scheduleFieldSave('title', 'seeded')
    await vi.advanceTimersByTimeAsync(150)
    expect(updateMock).not.toHaveBeenCalled()
  })

  it('IHC7A AC2: later recordServerSnapshot fully replaces the initial seed', async () => {
    const formData = ref<Record<string, unknown>>({ title: 'seeded' })
    const seed = {
      id: 'TKT-001', type: 'ticket',
      properties: { title: 'seeded', stale: 'gone-after-replace' },
    } satisfies Partial<Entity>
    const opts: AutoSaveOptions = {
      getEntityType: () => 'ticket',
      getEntityId: () => 'TKT-001',
      debounceMs: 100,
      formData,
      contentRef: ref(''),
      inverseToCanonical: new Map(),
      buildRelationsBody: () => null,
      applyServerProperty: vi.fn(),
      applyServerContent: vi.fn(),
      onError: vi.fn(),
      initialServerSnapshot: seed as Entity,
    }
    const store = useEntitiesStore()
    const updateMock = vi.spyOn(store, 'update').mockResolvedValue({
      id: 'TKT-001', type: 'ticket', properties: {},
    } as Entity)
    const auto = useAutoSave(opts)
    // Replace the seed entirely — `stale` is no longer the baseline.
    auto.recordServerSnapshot({
      id: 'TKT-001', type: 'ticket',
      properties: { title: 'fresh' },
    } as Entity)
    // Writing 'gone-after-replace' to `stale` is NOT a no-op now that
    // the key was cleared from lastSeenServer.
    auto.scheduleFieldSave('stale', 'gone-after-replace')
    await vi.advanceTimersByTimeAsync(150)
    expect(updateMock).toHaveBeenCalledTimes(1)
  })

  it('IHC7A AC3: disabled channels throw on schedule*', () => {
    const formData = ref<Record<string, unknown>>({})
    const opts: AutoSaveOptions = {
      getEntityType: () => 'ticket',
      getEntityId: () => 'TKT-001',
      debounceMs: 100,
      formData,
      contentRef: ref(''),
      inverseToCanonical: new Map(),
      buildRelationsBody: () => null,
      applyServerProperty: vi.fn(),
      applyServerContent: vi.fn(),
      onError: vi.fn(),
      disablePropertyChannel: true,
      disableRelationsChannel: true,
    }
    const auto = useAutoSave(opts)
    expect(() => auto.scheduleFieldSave('title', 'x')).toThrow(/property channel is disabled/)
    expect(() => auto.scheduleUnset('title')).toThrow(/property channel is disabled/)
    expect(() => auto.scheduleRelationsChange()).toThrow(/relations channel is disabled/)
    // Content channel is enabled here — must not throw.
    expect(() => auto.scheduleContentSave('hi')).not.toThrow()
  })

  it('IHC7A AC3: mergeServerResponse skips apply* on disabled channels but updates baseline', async () => {
    const apply = vi.fn()
    const applyContent = vi.fn()
    const opts: AutoSaveOptions = {
      getEntityType: () => 'ticket',
      getEntityId: () => 'TKT-001',
      debounceMs: 100,
      formData: ref<Record<string, unknown>>({}),
      contentRef: ref(''),
      inverseToCanonical: new Map(),
      buildRelationsBody: () => null,
      applyServerProperty: apply,
      applyServerContent: applyContent,
      onError: vi.fn(),
      disablePropertyChannel: true,
    }
    const store = useEntitiesStore()
    vi.spyOn(store, 'update').mockResolvedValue({
      id: 'TKT-001', type: 'ticket', properties: {},
    } as Entity)
    const auto = useAutoSave(opts)
    auto.mergeServerResponse({
      id: 'TKT-001', type: 'ticket',
      properties: { title: 'server-set' },
      content: 'server-content',
    } as Entity)
    // Property apply must not have fired.
    expect(apply).not.toHaveBeenCalled()
    // Content apply must have fired (channel enabled).
    expect(applyContent).toHaveBeenCalledWith('server-content')
  })

  it('IHC7A AC3: commitImmediately is a no-op on disabled channels', async () => {
    const opts: AutoSaveOptions = {
      getEntityType: () => 'ticket',
      getEntityId: () => 'TKT-001',
      debounceMs: 100,
      formData: ref<Record<string, unknown>>({}),
      contentRef: ref(''),
      inverseToCanonical: new Map(),
      buildRelationsBody: () => null,
      applyServerProperty: vi.fn(),
      applyServerContent: vi.fn(),
      onError: vi.fn(),
      disablePropertyChannel: true,
      disableContentChannel: true,
      disableRelationsChannel: true,
    }
    const store = useEntitiesStore()
    const updateMock = vi.spyOn(store, 'update').mockResolvedValue({
      id: 'TKT-001', type: 'ticket', properties: {},
    } as Entity)
    const auto = useAutoSave(opts)
    const result = await auto.commitImmediately(1000)
    expect(result.settled).toBe(true)
    expect(updateMock).not.toHaveBeenCalled()
  })
})
