import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useUIStore } from './ui'

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

    it('auto-dismisses toast after timeout', () => {
      store.showToast('info', 'Test message', 3000)
      expect(store.toasts).toHaveLength(1)

      vi.advanceTimersByTime(3000)
      expect(store.toasts).toHaveLength(0)
    })

    it('does not auto-dismiss when timeout is 0', () => {
      store.showToast('info', 'Persistent message', 0)
      expect(store.toasts).toHaveLength(1)

      vi.advanceTimersByTime(60000)
      expect(store.toasts).toHaveLength(1)
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
})
