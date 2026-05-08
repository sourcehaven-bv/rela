import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import BackButton from './BackButton.vue'
import type { BackTarget } from '@/composables/useBackTarget'

// Mock schemaStore.getList. Each test sets the return value for its case.
const mockGetList = vi.fn<(id: string) => { title?: string } | undefined>()
vi.mock('@/stores', () => ({
  useSchemaStore: () => ({
    getList: mockGetList,
  }),
}))

function mountWith(target: BackTarget) {
  return mount(BackButton, {
    props: { target },
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
}

describe('BackButton', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockGetList.mockReset()
  })

  describe('label resolution', () => {
    it('renders "← Back" when labelHint is null', () => {
      const w = mountWith({ to: '/doc/x', labelHint: null })
      expect(w.text()).toBe('← Back')
    })

    it('renders "← <list title>" when the list is known', () => {
      mockGetList.mockReturnValue({ title: 'All Tickets' })
      const w = mountWith({ to: '/list/all_tickets', labelHint: { kind: 'list', id: 'all_tickets' } })
      expect(w.text()).toBe('← All Tickets')
      expect(mockGetList).toHaveBeenCalledWith('all_tickets')
    })

    it('falls back to "← Back" when list is unknown', () => {
      mockGetList.mockReturnValue(undefined)
      const w = mountWith({ to: '/list/nope', labelHint: { kind: 'list', id: 'nope' } })
      expect(w.text()).toBe('← Back')
    })

    it('falls back to "← Back" when list has no title', () => {
      mockGetList.mockReturnValue({})
      const w = mountWith({ to: '/list/untitled', labelHint: { kind: 'list', id: 'untitled' } })
      expect(w.text()).toBe('← Back')
    })
  })

  describe('navigation target', () => {
    it('passes the target path to router-link', () => {
      const w = mountWith({ to: '/entity/ticket/TKT-001?doc=overview', labelHint: null })
      const link = w.findComponent(RouterLinkStub)
      expect(link.props('to')).toBe('/entity/ticket/TKT-001?doc=overview')
    })

    it('preserves fragment in the target path', () => {
      const w = mountWith({
        to: '/entity/category/backend?doc=overview#edit-tkt-1-0',
        labelHint: null,
      })
      const link = w.findComponent(RouterLinkStub)
      expect(link.props('to')).toBe('/entity/category/backend?doc=overview#edit-tkt-1-0')
    })
  })

  describe('styling', () => {
    it('uses the shared .scope-nav-btn class (visual parity with scope-nav bar)', () => {
      const w = mountWith({ to: '/x', labelHint: null })
      expect(w.classes()).toContain('scope-nav-btn')
    })
  })
})
