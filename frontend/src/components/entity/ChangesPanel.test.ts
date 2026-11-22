import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'

import ChangesPanel from './ChangesPanel.vue'
import { useSchemaStore } from '@/stores/schema'
import { ApiError } from '@/api/errors'
import type { ChangesPage } from '@/api/audit'
import type { EntityType } from '@/types'

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

  it('stays hidden and fetches nothing where header_buttons hides the history', async () => {
    const store = useSchemaStore()
    store.setAudit({ changes: true, global: false })
    store.setHeaderButtons({ history: false, add_to_pile: true })
    const wrapper = mountPanel()
    await flushPromises()
    expect(getChangesMock).not.toHaveBeenCalled()
    expect(wrapper.find('.changes-panel').exists()).toBe(false)
  })

  it('names relations, fields and entities by label and title', async () => {
    const store = useSchemaStore()
    store.setAudit({ changes: true, global: false })
    store.entityTypes = new Map([
      [
        'ticket',
        {
          label: 'Ticket',
          properties: { status: { type: 'string', label: 'Status' } },
        } as unknown as EntityType,
      ],
    ])
    store.relationTypes = new Map([
      [
        'affects',
        {
          label: 'Affects',
          from: [],
          to: [],
          inverse: { id: 'affected-by', label: 'Affected by' },
        },
      ],
    ])
    getChangesMock.mockResolvedValue({
      changes: [
        {
          seq: 9,
          time: '2026-01-02T10:00:00Z',
          op: 'create-relation',
          subject: { kind: 'relation', relation_type: 'affects', from_id: 'TKT-1', to_id: 'CON-7' },
          user: 'alice',
          trigger: 'automation:close-on-done',
        },
        {
          seq: 8,
          time: '2026-01-01T10:00:00Z',
          op: 'delete-relation',
          subject: { kind: 'relation', relation_type: 'affects', from_id: 'BUG-3', to_id: 'TKT-1' },
          user: 'system:scheduler',
        },
        {
          seq: 7,
          time: '2026-01-01T09:00:00Z',
          op: 'update-entity',
          subject: { kind: 'entity', type: 'ticket', id: 'TKT-1' },
          user: '<b>bob</b>',
          summary: 'updated: status',
        },
        {
          seq: 6,
          time: '2026-01-01T08:00:00Z',
          op: 'create-entity',
          subject: { kind: 'entity', type: 'ticket', id: 'TKT-1' },
          user: 'PSN-19',
          summary: 'created',
        },
      ],
      titles: { 'CON-7': 'Login', 'PSN-19': 'Carol' },
    })
    const wrapper = mountPanel()
    await flushPromises()

    const rows = wrapper.findAll('[data-testid="change-row"]').map((r) => r.text())
    expect(rows).toHaveLength(4)
    expect(rows[0]).toContain('alice')
    expect(rows[0]).toContain('Linked')
    expect(rows[0]).toContain('Affects → Login')
    expect(rows[0]).toContain('By an automation')
    expect(rows[0]).not.toContain('close-on-done')
    // An incoming relation reads by its inverse label; an id without a
    // title stays the id.
    expect(rows[1]).toContain('Unlinked')
    expect(rows[1]).toContain('Affected by → BUG-3')
    expect(rows[1]).toContain('System')
    expect(rows[1]).not.toContain('system:scheduler')
    expect(rows[2]).toContain('Changed: Status')
    expect(rows[2]).not.toContain('updated')
    expect(rows[3]).toContain('Created')
    expect(rows[3]).not.toContain('created')
    // A person who made a change reads by their title (BUG-9WCZYOX).
    expect(rows[3]).toContain('Carol')
    expect(rows[3]).not.toContain('PSN-19')
    // Text interpolation, never markup.
    expect(rows[2]).toContain('<b>bob</b>')
    expect(wrapper.find('b').exists()).toBe(false)
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

  it('offers Load more, not "no changes", for an empty page with a cursor', async () => {
    useSchemaStore().setAudit({ changes: true, global: false })
    getChangesMock.mockResolvedValue({ changes: [], next_before: 40 })
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('.panel-note').exists()).toBe(false)
    expect(wrapper.find('.panel-more button').exists()).toBe(true)
  })

  describe('a failed Load more', () => {
    async function mountWithFirstPage() {
      useSchemaStore().setAudit({ changes: true, global: false })
      getChangesMock.mockResolvedValueOnce({
        changes: [{ seq: 50, time: '2026-01-02T10:00:00Z', op: 'update', user: 'a' }],
        next_before: 50,
      })
      const wrapper = mountPanel()
      await flushPromises()
      expect(wrapper.findAll('[data-testid="change-row"]')).toHaveLength(1)
      return wrapper
    }

    it('drops the held list when the read is refused', async () => {
      const wrapper = await mountWithFirstPage()
      getChangesMock.mockRejectedValueOnce(
        new ApiError('Not found', { kind: 'http', status: 404, original: null })
      )
      await wrapper.find('.panel-more button').trigger('click')
      await flushPromises()
      expect(wrapper.findAll('[data-testid="change-row"]')).toHaveLength(0)
    })

    it('keeps the held list and shows the error on a transient failure', async () => {
      const wrapper = await mountWithFirstPage()
      getChangesMock.mockRejectedValueOnce(
        new ApiError('Unavailable', { kind: 'http', status: 503, original: null })
      )
      await wrapper.find('.panel-more button').trigger('click')
      await flushPromises()
      expect(wrapper.findAll('[data-testid="change-row"]')).toHaveLength(1)
      expect(wrapper.find('.panel-error').exists()).toBe(true)
    })
  })
  // BUG-UZGMHIK: an action on the page bumps `revision`, and the list is read
  // again in place, with the rows on screen kept until the answer lands.
  it('reads the list again in place when the revision changes', async () => {
    useSchemaStore().setAudit({ changes: true, global: false })
    getChangesMock.mockResolvedValue({
      changes: [{ seq: 1, time: '2026-01-01T10:00:00Z', op: 'create-entity', user: 'a' }],
    })
    const wrapper = mount(ChangesPanel, {
      props: { entityType: 'ticket', entityId: 'TKT-1', revision: 0 },
    })
    await flushPromises()
    expect(wrapper.findAll('[data-testid="change-row"]')).toHaveLength(1)

    let answer: (p: ChangesPage) => void = () => {}
    getChangesMock.mockReturnValueOnce(new Promise((r) => (answer = r)))
    await wrapper.setProps({ revision: 1 })
    expect(getChangesMock).toHaveBeenCalledTimes(2)
    expect(wrapper.findAll('[data-testid="change-row"]')).toHaveLength(1)

    answer({
      changes: [
        { seq: 2, time: '2026-01-01T11:00:00Z', op: 'update-entity', user: 'a' },
        { seq: 1, time: '2026-01-01T10:00:00Z', op: 'create-entity', user: 'a' },
      ],
    })
    await flushPromises()
    expect(wrapper.findAll('[data-testid="change-row"]')).toHaveLength(2)
  })
})
