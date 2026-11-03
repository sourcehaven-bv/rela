import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, RouterLinkStub, enableAutoUnmount } from '@vue/test-utils'
import { ref } from 'vue'

import MentionedByPanel from './MentionedByPanel.vue'
import { ApiError } from '@/api/errors'
import type { MentionedByPage } from '@/api/mentions'

const world = ref<string | undefined>(undefined)
vi.mock('@/composables/useWorld', () => ({
  useWorld: () => ({ worldParam: world }),
}))

const getMentionedByMock = vi.fn<(...args: unknown[]) => Promise<MentionedByPage>>()
vi.mock('@/api/mentions', () => ({
  getMentionedBy: (...args: unknown[]) => getMentionedByMock(...args),
}))

function mountPanel(entityId = 'FEAT-1') {
  return mount(MentionedByPanel, {
    props: { entityType: 'feature', entityId },
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
}

// The world ref is shared; a panel left mounted would refetch when a later
// test resets it.
enableAutoUnmount(afterEach)

describe('MentionedByPanel', () => {
  beforeEach(() => {
    getMentionedByMock.mockReset()
    world.value = undefined
  })

  it('renders nothing while nothing mentions the entity', async () => {
    getMentionedByMock.mockResolvedValue({ items: [] })
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('.mentioned-by-panel').exists()).toBe(false)
  })

  it('lists each source with its title and links to its served face', async () => {
    world.value = 'editorial'
    getMentionedByMock.mockResolvedValue({
      items: [
        { id: 'POL-1', face: 'draft', type: 'policy', title: 'Draft policy' },
        { id: 'TKT-1', type: 'ticket', title: 'A ticket' },
      ],
    })
    const wrapper = mountPanel()
    await flushPromises()

    expect(getMentionedByMock).toHaveBeenCalledWith(
      'feature',
      'FEAT-1',
      { cursor: undefined, world: 'editorial' },
      expect.any(AbortSignal)
    )
    const links = wrapper.findAllComponents(RouterLinkStub)
    expect(links.map((l) => l.props('to'))).toEqual([
      { path: '/entity/policy/POL-1@draft', query: { world: 'editorial' } },
      { path: '/entity/ticket/TKT-1', query: { world: 'editorial' } },
    ])
    expect(wrapper.text()).toContain('Draft policy')
    expect(wrapper.text()).toContain('POL-1@draft')
  })

  it('appends the next page on Show more', async () => {
    getMentionedByMock
      .mockResolvedValueOnce({
        items: [{ id: 'TKT-1', type: 'ticket', title: 'one' }],
        next_cursor: 'TKT-1',
      })
      .mockResolvedValueOnce({ items: [{ id: 'TKT-2', type: 'ticket', title: 'two' }] })
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.find('.panel-more button').trigger('click')
    await flushPromises()

    expect(getMentionedByMock.mock.calls[1][2]).toEqual({ cursor: 'TKT-1', world: undefined })
    expect(wrapper.text()).toContain('one')
    expect(wrapper.text()).toContain('two')
    expect(wrapper.find('.panel-more').exists()).toBe(false)
  })

  it('says so when the server stopped before every source', async () => {
    getMentionedByMock
      .mockResolvedValueOnce({
        items: [{ id: 'TKT-1', type: 'ticket', title: 'one' }],
        next_cursor: 'TKT-1',
      })
      .mockResolvedValueOnce({ items: [], truncated: true })
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('.panel-note').exists()).toBe(false)

    await wrapper.find('.panel-more button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('one')
    expect(wrapper.find('.panel-note').text()).toContain('could not be listed')
    expect(wrapper.find('.panel-more').exists()).toBe(false)
  })

  it('drops the held list when a refetch is refused', async () => {
    getMentionedByMock.mockResolvedValueOnce({
      items: [{ id: 'TKT-1', type: 'ticket', title: 'held' }],
    })
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('held')

    getMentionedByMock.mockRejectedValueOnce(
      new ApiError('Request failed (404)', {
        kind: 'http',
        status: 404,
        original: new Error('denied'),
      })
    )
    await wrapper.setProps({ entityId: 'FEAT-2' })
    await flushPromises()
    expect(wrapper.find('.mentioned-by-panel').exists()).toBe(false)
  })
})
