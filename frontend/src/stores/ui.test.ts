import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { useUIStore } from './ui'
import { useToasts } from 'rela-components/components/feedback/useToasts'

describe('UI Store', () => {
  let store: ReturnType<typeof useUIStore>

  beforeEach(() => {
    store = useUIStore()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('dark mode', () => {
    it('toggles dark mode and sets explicit theme mode', () => {
      const initialDark = store.darkMode

      store.toggleDarkMode()

      expect(store.darkMode).toBe(!initialDark)
      expect(store.themeMode).toBe(store.darkMode ? 'dark' : 'light')
    })

    /**
     * rela's own palette keys off `:root.dark` alone, so an absent class means
     * light. The rela-components palette does NOT: as well as `.dark` it
     * honours the OS through
     * `@media (prefers-color-scheme: dark) { :root:not(.light) }`.
     *
     * So an explicit Light choice has to be written positively, or a user on a
     * dark-OS machine who picked Light gets rela's chrome light and every Rl
     * component dark. `system` must NOT write it: there the OS preference is
     * the answer, and the class would override what it is meant to follow.
     */
    it('states an explicit light choice positively, for the Rl palette', async () => {
      const root = document.documentElement

      store.setThemeMode('light')
      await nextTick()
      expect(root.classList.contains('light')).toBe(true)
      expect(root.classList.contains('dark')).toBe(false)

      store.setThemeMode('dark')
      await nextTick()
      expect(root.classList.contains('light')).toBe(false)
      expect(root.classList.contains('dark')).toBe(true)

      store.setThemeMode('system')
      await nextTick()
      expect(root.classList.contains('light')).toBe(false)
    })
  })

  describe('sidebar state', () => {
    it('starts with sidebar expanded', () => {
      expect(store.sidebarCollapsed).toBe(false)
      expect(store.sidebarMobileOpen).toBe(false)
    })

    it('toggles sidebar collapsed state', () => {
      store.toggleSidebar()
      expect(store.sidebarCollapsed).toBe(true)

      store.toggleSidebar()
      expect(store.sidebarCollapsed).toBe(false)
    })

    it('opens and closes mobile sidebar', () => {
      store.openMobileSidebar()
      expect(store.sidebarMobileOpen).toBe(true)

      store.closeMobileSidebar()
      expect(store.sidebarMobileOpen).toBe(false)
    })

    it('computes sidebar visibility correctly', () => {
      // Desktop: visible when not collapsed
      expect(store.isSidebarVisible).toBe(true)

      store.toggleSidebar()
      expect(store.isSidebarVisible).toBe(false)

      // Mobile: visible when mobile open is true
      store.openMobileSidebar()
      expect(store.isSidebarVisible).toBe(true)
    })
  })

  describe('command palette', () => {
    it('starts closed', () => {
      expect(store.commandPaletteOpen).toBe(false)
    })

    it('toggles command palette', () => {
      store.toggleCommandPalette()
      expect(store.commandPaletteOpen).toBe(true)

      store.toggleCommandPalette()
      expect(store.commandPaletteOpen).toBe(false)
    })
  })

  describe('modals', () => {
    it('opens modal with data', () => {
      const data = { id: 'test-123' }
      store.openModal('confirm-delete', data)

      expect(store.currentModal).toBe('confirm-delete')
      expect(store.modalData).toEqual(data)
    })

    it('closes modal and clears data', () => {
      store.openModal('some-modal', { data: true })
      store.closeModal()

      expect(store.currentModal).toBeNull()
      expect(store.modalData).toBeNull()
    })
  })

  describe('toast notifications', () => {
    it('shows toast and returns id', () => {
      const id = store.showToast('info', 'Test message')

      expect(id).toBeTruthy()
      expect(store.toasts).toHaveLength(1)
      expect(store.toasts[0]).toMatchObject({
        id,
        type: 'info',
        message: 'Test message',
      })
    })

    // The timer itself belongs to RlToast (it pauses on hover and focus), so
    // the store's job is to hand the timeout over as the toast's duration.
    it('forwards the timeout to the library queue as the duration', () => {
      store.showToast('info', 'Test message', 3000)
      expect(useToasts().toasts.value[0]).toMatchObject({
        title: 'Test message',
        tone: 'info',
        duration: 3000,
      })
    })

    it('maps error to the danger tone', () => {
      store.error('Broken')
      expect(useToasts().toasts.value[0].tone).toBe('danger')
      expect(store.toasts[0].type).toBe('error')
    })

    it('passes an action through to the library toast', () => {
      const onAction = vi.fn()
      store.showToast('success', 'Deleted 2 items', 10000, { label: 'Undo', onAction })
      const action = useToasts().toasts.value[0].action
      expect(action?.label).toBe('Undo')
      action?.onAction()
      expect(onAction).toHaveBeenCalledOnce()
    })

    it('shows toasts raised straight on the library queue', () => {
      useToasts().warning('From the library')
      expect(store.toasts[0]).toMatchObject({ type: 'warning', message: 'From the library' })
    })

    it('dismisses toast manually', () => {
      const id = store.showToast('info', 'Test', 0)
      expect(store.toasts).toHaveLength(1)

      store.dismissToast(id)
      expect(store.toasts).toHaveLength(0)
    })

    it('handles dismissing non-existent toast', () => {
      store.dismissToast('non-existent-id')
      expect(store.toasts).toHaveLength(0)
    })

    it('success() creates success toast with default timeout', () => {
      store.success('Operation completed')

      expect(store.toasts[0].type).toBe('success')
      expect(store.toasts[0].message).toBe('Operation completed')
    })

    it('error() creates error toast with longer timeout', () => {
      store.error('Something went wrong')

      expect(store.toasts[0].type).toBe('error')
      expect(store.toasts[0].timeout).toBe(10000)
    })

    it('warning() creates warning toast', () => {
      store.warning('Be careful')

      expect(store.toasts[0].type).toBe('warning')
    })

    it('info() creates info toast', () => {
      store.info('Just so you know')

      expect(store.toasts[0].type).toBe('info')
    })

    it('handles multiple toasts', () => {
      store.success('First')
      store.error('Second')
      store.warning('Third')

      expect(store.toasts).toHaveLength(3)
      expect(store.toasts.map((t) => t.type)).toEqual(['success', 'error', 'warning'])
    })
  })

  describe('datetime timezone', () => {
    beforeEach(() => {
      // localStorage is a shared mock that isn't reset between tests; clear it
      // and re-create the store (fresh Pinia) so each case starts from a clean
      // override initialized from the now-empty storage.
      localStorage.clear()
      setActivePinia(createPinia())
      store = useUIStore()
    })

    it('defaults to browser zone (empty override)', () => {
      expect(store.datetimeTimezone).toBe('')
      // effectiveTimezone falls back to the browser zone.
      expect(store.effectiveTimezone).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone)
    })

    it('sets and persists a supported zone', () => {
      store.setDatetimeTimezone('America/New_York')
      expect(store.datetimeTimezone).toBe('America/New_York')
      expect(store.effectiveTimezone).toBe('America/New_York')
      expect(localStorage.getItem('datetimeTimezone')).toBe('America/New_York')
    })

    it('rejects an unsupported zone (no-op)', () => {
      store.setDatetimeTimezone('Not/AZone')
      expect(store.datetimeTimezone).toBe('')
    })

    it('clearing the override removes the persisted value', () => {
      store.setDatetimeTimezone('Asia/Kolkata')
      expect(localStorage.getItem('datetimeTimezone')).toBe('Asia/Kolkata')
      store.setDatetimeTimezone('')
      expect(store.datetimeTimezone).toBe('')
      expect(localStorage.getItem('datetimeTimezone')).toBeNull()
    })
  })

  describe('sidebar width', () => {
    beforeEach(() => localStorage.removeItem('sidebarWidth'))

    it('stores a dragged width, clamped to the shell bounds', () => {
      store.setSidebarWidth(333.4)
      expect(store.sidebarWidth).toBe(333)
      expect(localStorage.getItem('sidebarWidth')).toBe('333')

      store.setSidebarWidth(9000)
      expect(store.sidebarWidth).toBe(420)
      store.setSidebarWidth(10)
      expect(store.sidebarWidth).toBe(200)
    })

    it('restores a stored width and ignores a bad one', () => {
      localStorage.setItem('sidebarWidth', '300')
      setActivePinia(createPinia())
      expect(useUIStore().sidebarWidth).toBe(300)

      localStorage.setItem('sidebarWidth', 'wide')
      setActivePinia(createPinia())
      expect(useUIStore().sidebarWidth).toBe(260)
    })
  })
})
