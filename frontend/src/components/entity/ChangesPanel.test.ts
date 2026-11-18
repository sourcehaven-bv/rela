import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'

import ChangesPanel from './ChangesPanel.vue'
import { useSchemaStore } from '@/stores/schema'
import { ApiError } from '@/api/errors'
import type { ChangesPage } from '@/api/audit'

const world = ref<string | undefined>(undefined)
vi.mock('@/composables/useWorld', () => ({
  useWorld: () => ({ worldParam: world }),
}))

const getChangesMock = vi.fn<(...args: unknown[]) => Promise<ChangesPage>>()
vi.mock('@/api/audit', async (orig) => ({
  ...(await orig<typeof import('@/api/audit')>()),
  getChanges: (...args: unknown[]) => getChangesMock(...args),
}))

function mountPanel(entityId = 'TKT-1') {
  return mount(ChangesPanel, { props: { entityType: 'ticket', entityId } })
}

enableAutoUnmount(afterEach)

describe('ChangesPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getChangesMock.mockReset()
    world.value = undefined
  })

  it('renders nothing and fetches nothing when audit.changes is false', async () => {
    useSchemaStore().setAudit({ changes: false, global: false })
    const wrapper = mountPanel()
    await flushPromises()
    expect(getChangesMock).not.toHaveBeenCalled()
    expect(wrapper.find('.changes-panel').exists()).toBe(false)
  })

  it('stays hidden for a server that sends no audit flags', async () => {
    useSchemaStore().setAudit(undefined)
    const wrapper = mountPanel()
    await flushPromises()
    expect(getChangesMock).not.toHaveBeenCalled()
    expect(wrapper.find('.changes-panel').exists()).toBe(false)
  })

  it('lists changes and names the other end of a relation', async () => {
    useSchemaStore().setAudit({ changes: true, global: false })
    getChangesMock.mockResolvedValue({
      changes: [
        {
          seq: 9,
          time: '2026-01-02T10:00:00Z',
          op: 'relation-create',
          subject: { kind: 'relation', relation_type: 'affects', from_id: 'TKT-1', to_id: 'CON-7' },
          user: 'alice',
          trigger: 'flow:triage',
        },
        {
          seq: 8,
          time: '2026-01-01T10:00:00Z',
          op: 'update',
          subject: { kind: 'entity', type: 'ticket', id: 'TKT-1' },
          user: '<b>bob</b>',
          summary: 'status: ready → done',
        },
      ],
    })
    const wrapper = mountPanel()
    await flushPromises()

    const rows = wrapper.findAll('[data-testid="change-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('alice')
    expect(rows[0].text()).toContain('affects → CON-7')
    expect(rows[0].text()).toContain('flow:triage')
    expect(rows[1].text()).toContain('status: ready → done')
    // Text interpolation, never markup.
    expect(rows[1].text()).toContain('<b>bob</b>')
    expect(rows[1].find('b').exists()).toBe(false)
  })

  it('passes next_before as the before cursor on Load more', async () => {
    useSchemaStore().setAudit({ changes: true, global: false })
    getChangesMock
      .mockResolvedValueOnce({
        changes: [{ seq: 50, time: '2026-01-02T10:00:00Z', op: 'update', user: 'a' }],
        next_before: 50,
      })
      .mockResolvedValueOnce({
        changes: [{ seq: 49, time: '2026-01-01T10:00:00Z', op: 'create', user: 'b' }],
      })
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.find('.panel-more button').trigger('click')
    await flushPromises()

    expect(getChangesMock).toHaveBeenLastCalledWith(
      'ticket',
      'TKT-1',
      { before: 50, world: undefined },
      expect.any(AbortSignal)
    )
    expect(wrapper.findAll('[data-testid="change-row"]')).toHaveLength(2)
    expect(wrapper.find('.panel-more').exists()).toBe(false)
  })

  it('shows nothing on a 404', async () => {
    useSchemaStore().setAudit({ changes: true, global: false })
    getChangesMock.mockRejectedValue(
      new ApiError('Not found', { kind: 'http', status: 404, original: null })
    )
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('.changes-panel').exists()).toBe(false)
  })
})
