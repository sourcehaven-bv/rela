import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import { shortcutsModalOpen, useKeyboardShortcuts } from './useKeyboardShortcuts'

// Helper to create a keyboard event
function createKeyEvent(key: string, options: Partial<KeyboardEvent> = {}): KeyboardEvent {
  return new KeyboardEvent('keydown', { key, bubbles: true, ...options })
}

// Test component that uses the composable
const TestComponent = defineComponent({
  setup() {
    useKeyboardShortcuts()
    return {}
  },
  template: '<div>Test</div>',
})

describe('useKeyboardShortcuts', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    // Reset modal state
    shortcutsModalOpen.value = false

    // Create router with routes
    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/', name: 'home', component: { template: '<div/>' } },
        { path: '/dashboard', name: 'dashboard', component: { template: '<div/>' } },
        { path: '/search', name: 'search', component: { template: '<div/>' } },
        { path: '/graph', name: 'graph', component: { template: '<div/>' } },
        { path: '/analyze', name: 'analyze', component: { template: '<div/>' } },
        { path: '/form/create/:id', name: 'form-create', component: { template: '<div/>' } },
        { path: '/form/edit/:id/:entityId', name: 'form-edit', component: { template: '<div/>' } },
      ],
    })

    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  async function mountWithRouter() {
    const wrapper = mount(TestComponent, {
      global: {
        plugins: [router],
      },
    })
    await router.isReady()
    return wrapper
  }

  describe('shortcuts modal', () => {
    it('opens modal with ? key', async () => {
      await mountWithRouter()

      document.dispatchEvent(createKeyEvent('?'))
      expect(shortcutsModalOpen.value).toBe(true)
    })

    it('closes modal with Escape', async () => {
      await mountWithRouter()
      shortcutsModalOpen.value = true

      document.dispatchEvent(createKeyEvent('Escape'))
      expect(shortcutsModalOpen.value).toBe(false)
    })
  })

  describe('G-prefix navigation', () => {
    it('navigates to dashboard with g+d', async () => {
      await mountWithRouter()
      const pushSpy = vi.spyOn(router, 'push')

      document.dispatchEvent(createKeyEvent('g'))
      document.dispatchEvent(createKeyEvent('d'))

      expect(pushSpy).toHaveBeenCalledWith('/dashboard')
    })

    it('navigates to graph with g+g', async () => {
      await mountWithRouter()
      const pushSpy = vi.spyOn(router, 'push')

      document.dispatchEvent(createKeyEvent('g'))
      document.dispatchEvent(createKeyEvent('g'))

      expect(pushSpy).toHaveBeenCalledWith('/graph')
    })

    it('navigates to search with g+s', async () => {
      await mountWithRouter()
      const pushSpy = vi.spyOn(router, 'push')

      document.dispatchEvent(createKeyEvent('g'))
      document.dispatchEvent(createKeyEvent('s'))

      expect(pushSpy).toHaveBeenCalledWith('/search')
    })

    it('navigates to analyze with g+a', async () => {
      await mountWithRouter()
      const pushSpy = vi.spyOn(router, 'push')

      document.dispatchEvent(createKeyEvent('g'))
      document.dispatchEvent(createKeyEvent('a'))

      expect(pushSpy).toHaveBeenCalledWith('/analyze')
    })

    it('cancels g-sequence after timeout', async () => {
      await mountWithRouter()
      const pushSpy = vi.spyOn(router, 'push')

      document.dispatchEvent(createKeyEvent('g'))
      vi.advanceTimersByTime(1100) // Past 1000ms timeout
      document.dispatchEvent(createKeyEvent('d'))

      // Should not navigate because g-sequence expired
      expect(pushSpy).not.toHaveBeenCalledWith('/dashboard')
    })
  })

  describe('search shortcut', () => {
    it('navigates to search with /', async () => {
      await mountWithRouter()
      const pushSpy = vi.spyOn(router, 'push')

      document.dispatchEvent(createKeyEvent('/'))

      expect(pushSpy).toHaveBeenCalledWith('/search')
    })

    it('does not navigate when already on search page', async () => {
      await mountWithRouter()
      await router.push('/search')
      await nextTick()

      const pushSpy = vi.spyOn(router, 'push')
      document.dispatchEvent(createKeyEvent('/'))

      expect(pushSpy).not.toHaveBeenCalled()
    })
  })

  describe('input focus handling', () => {
    it('ignores shortcuts when input is focused', async () => {
      await mountWithRouter()

      const input = document.createElement('input')
      document.body.appendChild(input)
      input.focus()

      const pushSpy = vi.spyOn(router, 'push')
      document.dispatchEvent(createKeyEvent('?'))

      // Should not open modal
      expect(shortcutsModalOpen.value).toBe(false)

      document.body.removeChild(input)
    })

    it('ignores shortcuts when textarea is focused', async () => {
      await mountWithRouter()

      const textarea = document.createElement('textarea')
      document.body.appendChild(textarea)
      textarea.focus()

      document.dispatchEvent(createKeyEvent('?'))
      expect(shortcutsModalOpen.value).toBe(false)

      document.body.removeChild(textarea)
    })

    it('blurs input on Escape', async () => {
      await mountWithRouter()

      const input = document.createElement('input')
      document.body.appendChild(input)
      input.focus()
      const blurSpy = vi.spyOn(input, 'blur')

      document.dispatchEvent(createKeyEvent('Escape'))

      expect(blurSpy).toHaveBeenCalled()

      document.body.removeChild(input)
    })
  })

  describe('form page handling', () => {
    it('goes back on Escape from form page', async () => {
      await mountWithRouter()
      await router.push({ name: 'form-create', params: { id: 'test' } })
      await nextTick()

      const backSpy = vi.spyOn(router, 'back')
      document.dispatchEvent(createKeyEvent('Escape'))

      expect(backSpy).toHaveBeenCalled()
    })
  })

  describe('meta key shortcuts', () => {
    it('handles Cmd+K (reserved for command palette)', async () => {
      await mountWithRouter()

      const event = createKeyEvent('k', { metaKey: true })
      const preventDefaultSpy = vi.spyOn(event, 'preventDefault')

      document.dispatchEvent(event)

      expect(preventDefaultSpy).toHaveBeenCalled()
    })

    it('handles Ctrl+K (reserved for command palette)', async () => {
      await mountWithRouter()

      const event = createKeyEvent('k', { ctrlKey: true })
      const preventDefaultSpy = vi.spyOn(event, 'preventDefault')

      document.dispatchEvent(event)

      expect(preventDefaultSpy).toHaveBeenCalled()
    })
  })

  describe('cleanup', () => {
    it('removes event listener on unmount', async () => {
      const wrapper = await mountWithRouter()
      const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener')

      wrapper.unmount()

      expect(removeEventListenerSpy).toHaveBeenCalledWith('keydown', expect.any(Function))
    })
  })
})
