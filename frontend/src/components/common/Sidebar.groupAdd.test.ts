import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import { defineComponent } from 'vue'
import type { SidebarData } from '@/types'

const sidebar: SidebarData = {
  app: { name: 'Atlas' },
  navigation: [
    {
      group: 'Topics',
      items: [],
      items_key: 'pm:0',
      items_page: 'topic',
      items_list: 'actieve_topics',
      items_create: { type: 'topic', label: 'Topic', form: 'create_topic' },
    },
  ],
} as SidebarData

vi.mock('@/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api')>()),
  getSidebar: vi.fn(async () => sidebar),
}))
vi.mock('@/composables/useNavItems', () => ({
  useNavItems: () => ({ itemsFor: () => ({ entries: [] }) }),
}))

const CreateStub = defineComponent({
  name: 'InlineCreateFormModal',
  props: ['show', 'formId', 'entityType'],
  emits: ['close', 'created'],
  template: '<div data-testid="create-stub" />',
})

import Sidebar from './Sidebar.vue'

/**
 * The "+" on a generated group's heading opens the create dialog for the
 * group's list, and the new entity then opens on the group's entity page.
 */
describe('Sidebar group add', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    router = createRouter({
      history: createWebHistory(),
      routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }],
    })
    await router.push('/')
    await router.isReady()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
  })

  it('opens the create dialog and then the new entity page', async () => {
    wrapper = mount(Sidebar, {
      global: { plugins: [router], stubs: { InlineCreateFormModal: CreateStub } },
    })
    await flushPromises()
    expect(wrapper.findComponent(CreateStub).exists()).toBe(false)

    await wrapper.get('button[aria-label="New Topic"]').trigger('click')
    const modal = wrapper.getComponent(CreateStub)
    expect(modal.props('formId')).toBe('create_topic')
    expect(modal.props('entityType')).toBe('topic')

    const push = vi.spyOn(router, 'push')
    modal.vm.$emit('created', { id: 'TOP-7', type: 'topic', properties: {} })
    await flushPromises()
    expect(push).toHaveBeenCalledWith('/p/topic/TOP-7')
    expect(wrapper.findComponent(CreateStub).exists()).toBe(false)
  })
})
