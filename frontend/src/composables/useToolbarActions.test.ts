import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, ref } from 'vue'

import type { ActionConfig, ListConfig } from '@/types'
import { useToolbarActions } from './useToolbarActions'
import { useSchemaStore, useUIStore } from '@/stores'
import { runAction } from '@/api/actions'

vi.mock('@/api/actions', () => ({ runAction: vi.fn() }))
const routerPush = vi.fn()
vi.mock('vue-router', async (orig) => ({
  ...(await orig<typeof import('vue-router')>()),
  useRouter: () => ({ push: routerPush }),
}))
type ConfirmOpts = { title: string; message: string; confirmLabel?: string }
const confirmMock = vi.fn<(o: ConfirmOpts) => Promise<boolean>>(async () => true)
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: (o: ConfirmOpts) => confirmMock(o) }),
}))

const mockRunAction = vi.mocked(runAction)

function setup(
  actions: Record<string, boolean> | undefined,
  config: ActionConfig = { label: 'New VvT', script: 'vvt.lua' }
) {
  const schema = useSchemaStore()
  const list: ListConfig = {
    entity: 'document',
    title: 'Docs',
    columns: [],
    toolbar_actions: ['vvt-nieuw', 'missing'],
  }
  schema.lists.set('all_documenten', list)
  schema.actions.set('vvt-nieuw', config)
  const offered = ref(actions)
  const onDone = vi.fn()
  let api!: ReturnType<typeof useToolbarActions>
  mount(
    defineComponent({
      setup() {
        api = useToolbarActions('all_documenten', offered, onDone)
        return () => null
      },
    })
  )
  return { api, onDone, offered }
}

describe('useToolbarActions', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockRunAction.mockReset()
    routerPush.mockReset()
    confirmMock.mockClear()
  })

  it('shows a button only when the list response offers action:<id>', () => {
    const { api, offered } = setup({ create: true, 'action:vvt-nieuw': true })
    expect(api.buttons.value.map((b) => b.id)).toEqual(['vvt-nieuw'])

    offered.value = { create: true }
    expect(api.buttons.value).toEqual([])

    offered.value = undefined
    expect(api.buttons.value).toEqual([])
  })

  it('runs without an entity and follows a redirect', async () => {
    mockRunAction.mockResolvedValue({ redirect: '/entity/document/DOC-7' })
    const { api, onDone } = setup({ 'action:vvt-nieuw': true })
    await api.run(api.buttons.value[0])
    expect(mockRunAction).toHaveBeenCalledWith('vvt-nieuw')
    expect(routerPush).toHaveBeenCalledWith('/entity/document/DOC-7')
    expect(onDone).not.toHaveBeenCalled()
  })

  it('toasts the message and refreshes the list without a redirect', async () => {
    mockRunAction.mockResolvedValue({ message: 'Created' })
    const { api, onDone } = setup({ 'action:vvt-nieuw': true })
    const ui = useUIStore()
    const success = vi.spyOn(ui, 'success')
    await api.run(api.buttons.value[0])
    expect(success).toHaveBeenCalledWith('Created')
    expect(onDone).toHaveBeenCalledOnce()
  })

  it('asks first when the action has confirm, and stops on no', async () => {
    const { api } = setup(
      { 'action:vvt-nieuw': true },
      { label: 'New VvT', script: 'v.lua', confirm: 'Start a new VvT?' }
    )
    confirmMock.mockResolvedValueOnce(false)
    await api.run(api.buttons.value[0])
    expect(confirmMock).toHaveBeenCalledWith(
      expect.objectContaining({ message: 'Start a new VvT?' })
    )
    expect(mockRunAction).not.toHaveBeenCalled()

    mockRunAction.mockResolvedValue(null)
    await api.run(api.buttons.value[0])
    expect(mockRunAction).toHaveBeenCalledOnce()
  })

  it('reports a failure and clears busy', async () => {
    mockRunAction.mockRejectedValue(new Error('nope'))
    const { api } = setup({ 'action:vvt-nieuw': true })
    const ui = useUIStore()
    const error = vi.spyOn(ui, 'error')
    await api.run(api.buttons.value[0])
    await flushPromises()
    expect(error).toHaveBeenCalledWith(expect.stringContaining('New VvT'))
    expect(api.busy.value).toBe(false)
  })
})
