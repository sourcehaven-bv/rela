import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import BackButton from './BackButton.vue'
import { usePageStore } from '@/stores/pages'
import type { BackTarget } from '@/composables/useBackTarget'

// Mock schemaStore.getList. Each test sets the return value for its case.
const mockGetList = vi.fn<(id: string) => { title?: string } | undefined>()
vi.mock('@/stores', () => ({
  useSchemaStore: () => ({
    getList: mockGetList,
  }),
}))

/*
 * A real router rather than a link stub: RlBackButton renders an anchor from
 * a resolved href, so the query and fragment assertions below only mean
 * something if something actually resolves them.
 */
function mountWith(target: BackTarget) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
  })
  return mount(BackButton, { props: { target }, global: { plugins: [router] } })
}

describe('BackButton', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockGetList.mockReset()
  })

  describe('label resolution', () => {
    it('renders "← Back" when labelHint is null', () => {
      const w = mountWith({ to: '/doc/x', labelHint: null })
      expect(w.text()).toBe('Back')
    })

    it('renders "← <list title>" when the list is known', () => {
      mockGetList.mockReturnValue({ title: 'All Tickets' })
      const w = mountWith({ to: '/list/all_tickets', labelHint: { kind: 'list', id: 'all_tickets' } })
      expect(w.text()).toBe('All Tickets')
      expect(mockGetList).toHaveBeenCalledWith('all_tickets')
    })

    it('falls back to "← Back" when list is unknown', () => {
      mockGetList.mockReturnValue(undefined)
      const w = mountWith({ to: '/list/nope', labelHint: { kind: 'list', id: 'nope' } })
      expect(w.text()).toBe('Back')
    })

    it('falls back to "← Back" when list has no title', () => {
      mockGetList.mockReturnValue({})
      const w = mountWith({ to: '/list/untitled', labelHint: { kind: 'list', id: 'untitled' } })
      expect(w.text()).toBe('Back')
    })
  })

  describe('page label', () => {
    it('names the page a page-tab target returns to', () => {
      usePageStore().set({ tickets: { label: 'Tickets', tabs: [] } })
      const w = mountWith({ to: '/p/tickets/table', labelHint: { kind: 'page', id: 'tickets' } })
      expect(w.text()).toBe('Tickets')
    })

    it('falls back to "← Back" when the page is unknown', () => {
      const w = mountWith({ to: '/p/nope/x', labelHint: { kind: 'page', id: 'nope' } })
      expect(w.text()).toBe('Back')
    })
  })

  describe('navigation target', () => {
    it('renders the target path as a real href', () => {
      const w = mountWith({ to: '/entity/ticket/TKT-001?doc=overview', labelHint: null })
      expect(w.get('a').attributes('href')).toBe('/entity/ticket/TKT-001?doc=overview')
    })

    it('preserves fragment in the target path', () => {
      const w = mountWith({
        to: '/entity/category/backend?doc=overview#edit-tkt-1-0',
        labelHint: null,
      })
      expect(w.get('a').attributes('href')).toBe(
        '/entity/category/backend?doc=overview#edit-tkt-1-0'
      )
    })
  })

  describe('styling', () => {
    it('renders the library back button', () => {
      const w = mountWith({ to: '/x', labelHint: null })
      expect(w.find('.rl-back-button').exists()).toBe(true)
    })
  })
})
