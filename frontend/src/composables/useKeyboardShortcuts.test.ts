import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import { paletteOpen, shortcutsModalOpen, useKeyboardShortcuts } from './useKeyboardShortcuts'
import { _resetModalStack, registerModal } from './modalStack'

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
    paletteOpen.value = false
    _resetModalStack()

    // Create router with routes
    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/', name: 'home', component: { template: '<div/>' } },
        { path: '/dashboard', name: 'dashboard', component: { template: '<div/>' } },
        { path: '/search', name: 'search', component: { template: '<div/>' } },
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

    it('ignores invalid g-sequence keys', async () => {
      await mountWithRouter()
      const pushSpy = vi.spyOn(router, 'push')

      document.dispatchEvent(createKeyEvent('g'))
      document.dispatchEvent(createKeyEvent('x')) // Invalid key

      // Should not navigate anywhere
      expect(pushSpy).not.toHaveBeenCalled()
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

      vi.spyOn(router, 'push')
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

  describe('command palette (Cmd/Ctrl+K)', () => {
    it('opens the palette on Cmd+K', async () => {
      await mountWithRouter()

      const event = createKeyEvent('k', { metaKey: true })
      const preventDefaultSpy = vi.spyOn(event, 'preventDefault')

      document.dispatchEvent(event)

      expect(preventDefaultSpy).toHaveBeenCalled()
      expect(paletteOpen.value).toBe(true)
    })

    it('opens the palette on Ctrl+K', async () => {
      await mountWithRouter()

      const event = createKeyEvent('k', { ctrlKey: true })
      const preventDefaultSpy = vi.spyOn(event, 'preventDefault')

      document.dispatchEvent(event)

      expect(preventDefaultSpy).toHaveBeenCalled()
      expect(paletteOpen.value).toBe(true)
    })

    it('opens even when an input is focused (bypasses isInputFocused)', async () => {
      await mountWithRouter()

      const input = document.createElement('input')
      document.body.appendChild(input)
      input.focus()

      document.dispatchEvent(createKeyEvent('k', { metaKey: true }))

      expect(paletteOpen.value).toBe(true)

      document.body.removeChild(input)
    })

    it('is idempotent when palette is already open', async () => {
      await mountWithRouter()

      document.dispatchEvent(createKeyEvent('k', { metaKey: true }))
      expect(paletteOpen.value).toBe(true)

      // Second press: still open, no flip-flop.
      document.dispatchEvent(createKeyEvent('k', { metaKey: true }))
      expect(paletteOpen.value).toBe(true)
    })

    it('opens even when another modal is registered (Cmd+K bypasses the modal-stack gate)', async () => {
      await mountWithRouter()
      registerModal(Symbol('confirm'))

      document.dispatchEvent(createKeyEvent('k', { metaKey: true }))
      expect(paletteOpen.value).toBe(true)
    })
  })

  describe('modal-stack gate', () => {
    it('global handlers stand down while any modal is registered', async () => {
      await mountWithRouter()
      await router.push({ name: 'form-create', params: { id: 'test' } })
      await nextTick()
      registerModal(Symbol('palette'))

      const backSpy = vi.spyOn(router, 'back')
      const pushSpy = vi.spyOn(router, 'push')

      // Escape on a form route — without the modal-stack gate this would
      // call router.back(). With the gate it must not.
      document.dispatchEvent(createKeyEvent('Escape'))
      expect(backSpy).not.toHaveBeenCalled()

      // ? would normally open the shortcuts modal. The gate suppresses it
      // because another modal is already on the stack.
      document.dispatchEvent(createKeyEvent('?'))
      expect(shortcutsModalOpen.value).toBe(false)

      // g-prefix nav also stands down.
      document.dispatchEvent(createKeyEvent('g'))
      document.dispatchEvent(createKeyEvent('d'))
      expect(pushSpy).not.toHaveBeenCalled()
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
