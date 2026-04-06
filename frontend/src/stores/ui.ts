import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'

export interface Toast {
  id: string
  type: 'success' | 'error' | 'warning' | 'info'
  message: string
  timeout?: number
}

type ThemeMode = 'light' | 'dark' | 'system'

function getInitialDarkMode(): boolean {
  const stored = localStorage.getItem('theme')
  if (stored === 'dark') return true
  if (stored === 'light') return false
  // Default to system preference
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function getInitialThemeMode(): ThemeMode {
  const stored = localStorage.getItem('theme')
  if (stored === 'dark' || stored === 'light' || stored === 'system') {
    return stored
  }
  return 'system'
}

export const useUIStore = defineStore('ui', () => {
  // State
  const sidebarCollapsed = ref(false)
  const sidebarMobileOpen = ref(false)
  const commandPaletteOpen = ref(false)
  const toasts = ref<Toast[]>([])
  const currentModal = ref<string | null>(null)
  const modalData = ref<unknown>(null)
  const themeMode = ref<ThemeMode>(getInitialThemeMode())
  const darkMode = ref(getInitialDarkMode())

  // Getters
  const isSidebarVisible = computed(
    () => !sidebarCollapsed.value || sidebarMobileOpen.value
  )

  const isDark = computed(() => darkMode.value)

  // Actions
  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function openMobileSidebar() {
    sidebarMobileOpen.value = true
  }

  function closeMobileSidebar() {
    sidebarMobileOpen.value = false
  }

  function toggleCommandPalette() {
    commandPaletteOpen.value = !commandPaletteOpen.value
  }

  function openModal(name: string, data?: unknown) {
    currentModal.value = name
    modalData.value = data
  }

  function closeModal() {
    currentModal.value = null
    modalData.value = null
  }

  function showToast(type: Toast['type'], message: string, timeout = 5000): string {
    const id = crypto.randomUUID()
    toasts.value.push({ id, type, message, timeout })

    if (timeout > 0) {
      setTimeout(() => {
        dismissToast(id)
      }, timeout)
    }

    return id
  }

  function dismissToast(id: string) {
    const index = toasts.value.findIndex((t) => t.id === id)
    if (index !== -1) {
      toasts.value.splice(index, 1)
    }
  }

  function success(message: string) {
    return showToast('success', message)
  }

  function error(message: string) {
    return showToast('error', message, 10000)
  }

  function warning(message: string) {
    return showToast('warning', message)
  }

  function info(message: string) {
    return showToast('info', message)
  }

  function toggleDarkMode() {
    // Toggle cycles: current -> opposite, sets mode to explicit (not system)
    darkMode.value = !darkMode.value
    themeMode.value = darkMode.value ? 'dark' : 'light'
  }

  /* v8 ignore start - theme mode tested via e2e */
  function setThemeMode(mode: ThemeMode) {
    themeMode.value = mode
    if (mode === 'system') {
      darkMode.value = window.matchMedia('(prefers-color-scheme: dark)').matches
    } else {
      darkMode.value = mode === 'dark'
    }
  }

  // Apply palette CSS variables to the document root
  function applyPalette(palette: Record<string, string>) {
    const root = document.documentElement
    for (const [key, value] of Object.entries(palette)) {
      if (value) {
        root.style.setProperty(key, value)
      }
    }
  }

  // Clear palette overrides (revert to CSS defaults)
  function clearPalette() {
    const root = document.documentElement
    // Remove any inline style properties we may have set
    root.removeAttribute('style')
  }

  // Apply dark mode class and persist
  watch(
    [darkMode, themeMode],
    ([dark, mode]) => {
      document.documentElement.classList.toggle('dark', dark)
      localStorage.setItem('theme', mode)
    },
    { immediate: true }
  )

  // Listen for system preference changes when in system mode
  if (typeof window !== 'undefined') {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
      if (themeMode.value === 'system') {
        darkMode.value = e.matches
      }
    })
  }
  /* v8 ignore stop */

  return {
    // State
    sidebarCollapsed,
    sidebarMobileOpen,
    commandPaletteOpen,
    toasts,
    currentModal,
    modalData,
    darkMode,
    themeMode,

    // Getters
    isSidebarVisible,
    isDark,

    // Actions
    toggleSidebar,
    openMobileSidebar,
    closeMobileSidebar,
    toggleCommandPalette,
    openModal,
    closeModal,
    showToast,
    dismissToast,
    success,
    error,
    warning,
    info,
    toggleDarkMode,
    setThemeMode,
    applyPalette,
    clearPalette,
  }
})
