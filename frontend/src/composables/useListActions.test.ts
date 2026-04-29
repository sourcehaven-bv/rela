import { setActivePinia, createPinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, ref } from 'vue'

import type { ActionConfig, Entity } from '@/types'
import type { ScriptError } from '@/types/scriptError'

import { useListActions } from './useListActions'
import { useScriptErrorStore } from '@/stores/scriptError'
import { useSchemaStore, useUIStore } from '@/stores'
import { runAction } from '@/api/actions'
import { updateEntity } from '@/api/entities'

vi.mock('@/api/actions', () => ({ runAction: vi.fn() }))
vi.mock('@/api/entities', () => ({ updateEntity: vi.fn() }))

function makeScriptError(overrides: Partial<ScriptError> = {}): ScriptError {
  return {
    error: 'script_error',
    correlation_id: 'abc123',
    script: { surface: 'action', path: 'actions/x.lua' },
    lua: { message: 'boom', line: 1 },
    ...overrides,
  }
}

interface HarnessOptions {
  selectedIds?: Set<string>
  entities?: Entity[]
  action?: ActionConfig
  onClearSelection?: () => void
  onComplete?: () => void
  onRequestConfirm?: (action: ActionConfig, actionId: string) => void
}

function mountHarness(opts: HarnessOptions = {}) {
  const selectedIds = ref(opts.selectedIds ?? new Set<string>())
  const entities = ref<Entity[]>(opts.entities ?? [])
  const listId = ref('list-1')

  const Harness = defineComponent({
    setup() {
      return useListActions({
        listId,
        selectedIds,
        entities,
        onClearSelection: opts.onClearSelection ?? (() => {}),
        onRequestConfirm: opts.onRequestConfirm ?? (() => {}),
        onComplete: opts.onComplete ?? (() => {}),
      })
    },
    template: '<div/>',
  })

  const wrapper = mount(Harness)
  return { wrapper, selectedIds, entities }
}

function configureSchema(action: ActionConfig, actionId = 'act-1') {
  const schemaStore = useSchemaStore()
  // Cast through unknown: the test only uses `entity` and `actions`; the
  // production ListConfig type pulls in many required column fields we
  // don't need to populate for these unit tests.
  schemaStore.lists.set('list-1', {
    id: 'list-1',
    entity: 'task',
    actions: [actionId],
    columns: [],
  } as unknown as Parameters<typeof schemaStore.lists.set>[1])
  schemaStore.actions.set(actionId, action)
}

function ent(id: string): Entity {
  return { id, type: 'task', properties: {} }
}

describe('useListActions — script error dispatch', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    // resetAllMocks (not clearAllMocks) drops queued mockReturn-/Resolved-/RejectedValue
    // setups too, so a default added by a future test can't bleed across cases.
    vi.resetAllMocks()
  })

  it('opens the script-error dialog when one rejection is a ScriptError', async () => {
    const action: ActionConfig = { label: 'Run', key: 'r' }
    configureSchema(action)

    const err = makeScriptError({ correlation_id: 'first' })
    vi.mocked(runAction).mockRejectedValueOnce(err).mockResolvedValueOnce(null)

    const { wrapper } = mountHarness({
      selectedIds: new Set(['t1', 't2']),
      entities: [
        ent('t1'),
        ent('t2'),
      ],
    })

    const showSpy = vi.spyOn(useScriptErrorStore(), 'show')
    const errorSpy = vi.spyOn(useUIStore(), 'error')

    const { executeAction } = wrapper.vm as unknown as ReturnType<typeof useListActions>
    await executeAction('act-1', action)

    expect(showSpy).toHaveBeenCalledTimes(1)
    expect(showSpy.mock.calls[0]?.[0]).toBe(err)
    // No triggerEl supplied → store is called with explicit null, never
    // undefined. The store guards against undefined too, but the wire
    // contract is null and tests should pin it down.
    expect(showSpy.mock.calls[0]?.[1]).toBeNull()
    expect(errorSpy).toHaveBeenCalledWith('Run: 1 failed, 1 succeeded')
  })

  it('shows only the first ScriptError when multiple rejections are script errors', async () => {
    const action: ActionConfig = { label: 'Run', key: 'r' }
    configureSchema(action)

    const first = makeScriptError({ correlation_id: 'first' })
    const second = makeScriptError({ correlation_id: 'second' })
    vi.mocked(runAction).mockRejectedValueOnce(first).mockRejectedValueOnce(second)

    const { wrapper } = mountHarness({
      selectedIds: new Set(['t1', 't2']),
      entities: [
        ent('t1'),
        ent('t2'),
      ],
    })

    const showSpy = vi.spyOn(useScriptErrorStore(), 'show')

    const { executeAction } = wrapper.vm as unknown as ReturnType<typeof useListActions>
    await executeAction('act-1', action)

    expect(showSpy).toHaveBeenCalledTimes(1)
    expect(showSpy.mock.calls[0]?.[0]).toBe(first)
  })

  it('does not open the dialog when rejections are not ScriptErrors', async () => {
    const action: ActionConfig = { label: 'Run', key: 'r' }
    configureSchema(action)

    vi.mocked(runAction).mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce(null)

    const { wrapper } = mountHarness({
      selectedIds: new Set(['t1', 't2']),
      entities: [
        ent('t1'),
        ent('t2'),
      ],
    })

    const showSpy = vi.spyOn(useScriptErrorStore(), 'show')
    const errorSpy = vi.spyOn(useUIStore(), 'error')

    const { executeAction } = wrapper.vm as unknown as ReturnType<typeof useListActions>
    await executeAction('act-1', action)

    expect(showSpy).not.toHaveBeenCalled()
    expect(errorSpy).toHaveBeenCalledWith('Run: 1 failed, 1 succeeded')
  })

  it('passes triggerEl through to the dialog store for focus restore', async () => {
    const action: ActionConfig = { label: 'Run', key: 'r' }
    configureSchema(action)

    const err = makeScriptError()
    vi.mocked(runAction).mockRejectedValueOnce(err)

    const trigger = document.createElement('button')

    const { wrapper } = mountHarness({
      selectedIds: new Set(['t1']),
      entities: [ent('t1')],
    })

    const showSpy = vi.spyOn(useScriptErrorStore(), 'show')

    const { executeAction } = wrapper.vm as unknown as ReturnType<typeof useListActions>
    await executeAction('act-1', action, trigger)

    expect(showSpy).toHaveBeenCalledTimes(1)
    expect(showSpy.mock.calls[0]?.[1]).toBe(trigger)
  })

  it('skips ScriptError dispatch for set-only actions that fail via updateEntity', async () => {
    const action: ActionConfig = { label: 'Done', key: 'd', set: { status: 'done' } }
    configureSchema(action)

    vi.mocked(updateEntity).mockRejectedValueOnce(new Error('500'))

    const { wrapper } = mountHarness({
      selectedIds: new Set(['t1']),
      entities: [ent('t1')],
    })

    const showSpy = vi.spyOn(useScriptErrorStore(), 'show')
    const errorSpy = vi.spyOn(useUIStore(), 'error')

    const { executeAction } = wrapper.vm as unknown as ReturnType<typeof useListActions>
    await executeAction('act-1', action)

    expect(showSpy).not.toHaveBeenCalled()
    expect(errorSpy).toHaveBeenCalledWith('Done: 1 failed, 0 succeeded')
  })

  it('does not open the dialog on the all-success path', async () => {
    const action: ActionConfig = { label: 'Run', key: 'r' }
    configureSchema(action)

    vi.mocked(runAction).mockResolvedValueOnce(null).mockResolvedValueOnce(null)

    const { wrapper } = mountHarness({
      selectedIds: new Set(['t1', 't2']),
      entities: [
        ent('t1'),
        ent('t2'),
      ],
    })

    const showSpy = vi.spyOn(useScriptErrorStore(), 'show')
    const successSpy = vi.spyOn(useUIStore(), 'success')

    const { executeAction } = wrapper.vm as unknown as ReturnType<typeof useListActions>
    await executeAction('act-1', action)

    expect(showSpy).not.toHaveBeenCalled()
    expect(successSpy).toHaveBeenCalledWith('Run: 2 updated')
  })

  it('forwards triggerEl through onRequestConfirm for confirm-required actions', () => {
    const action: ActionConfig = { label: 'Run', key: 'r', confirm: true }
    configureSchema(action)

    const onRequestConfirm = vi.fn()
    const trigger = document.createElement('button')

    const selectedIds = ref(new Set(['t1']))
    const entities = ref<Entity[]>([ent('t1')])
    const listId = ref('list-1')

    const Harness = defineComponent({
      setup() {
        return useListActions({
          listId,
          selectedIds,
          entities,
          onClearSelection: () => {},
          onRequestConfirm,
          onComplete: () => {},
        })
      },
      template: '<div/>',
    })
    const wrapper = mount(Harness)

    const { triggerAction } = wrapper.vm as unknown as ReturnType<typeof useListActions>
    triggerAction('act-1', action, trigger)

    expect(onRequestConfirm).toHaveBeenCalledTimes(1)
    expect(onRequestConfirm.mock.calls[0]?.[0]).toBe(action)
    expect(onRequestConfirm.mock.calls[0]?.[1]).toBe('act-1')
    expect(onRequestConfirm.mock.calls[0]?.[2]).toBe(trigger)
  })
})
