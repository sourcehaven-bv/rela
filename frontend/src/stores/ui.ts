import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { browserTimeZone } from '@/utils/format'
import { useToasts } from 'rela-components/components/feedback/useToasts'
import type { MessageTone, ToastAction } from 'rela-components/components/feedback/types'

export type { ToastAction }

export interface Toast {
  id: string
  type: 'success' | 'error' | 'warning' | 'info'
  message: string
  timeout?: number
  action?: ToastAction
}

// rela's toast kinds, in the library's tone vocabulary. `error` is the only
// one whose name differs.
const TONE_OF: Record<Toast['type'], MessageTone> = {
  success: 'success',
  error: 'danger',
  warning: 'warning',
  info: 'info',
}
const TYPE_OF: Record<MessageTone, Toast['type']> = {
  success: 'success',
  danger: 'error',
  warning: 'warning',
  info: 'info',
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

// isSupportedTimeZone guards against a stale/tampered localStorage value: an
// unknown IANA zone silently falls back to the browser default rather than
// breaking every datetime widget. We probe with the Intl.DateTimeFormat
// constructor rather than checking supportedValuesOf membership, because the
// latter returns only canonical names on some engines (e.g. "Asia/Calcutta"
// but not its alias "Asia/Kolkata", and no "UTC") — yet the constructor
// accepts both. The constructor is the authoritative "does this zone work"
// test; supportedValuesOf is used only to populate the picker list.
function isSupportedTimeZone(tz: string): boolean {
  if (!tz) return false
  try {
    Intl.DateTimeFormat(undefined, { timeZone: tz })
    return true
  } catch {
    return false
  }
}

// The datetime display-timezone override. Empty string means "use the browser
// zone". A stored value that is no longer a supported zone is ignored.
function getInitialDatetimeTimezone(): string {
  const stored = localStorage.getItem('datetimeTimezone') ?? ''
  return isSupportedTimeZone(stored) ? stored : ''
}

// The sidebar's dragged width. The bounds are RlAppShell's defaults, so a
// stored width and the width the shell renders cannot disagree; a stored value
// outside them, or not a number, is clamped or ignored.
export const SIDEBAR_DEFAULT_WIDTH = 260
const SIDEBAR_MIN_WIDTH = 200
const SIDEBAR_MAX_WIDTH = 420

function clampSidebarWidth(width: number): number {
  return Math.round(Math.min(SIDEBAR_MAX_WIDTH, Math.max(SIDEBAR_MIN_WIDTH, width)))
}

function getInitialSidebarWidth(): number {
  const stored = Number(localStorage.getItem('sidebarWidth'))
  return stored ? clampSidebarWidth(stored) : SIDEBAR_DEFAULT_WIDTH
}

export const useUIStore = defineStore('ui', () => {
  // State
  const sidebarCollapsed = ref(false)
  const sidebarMobileOpen = ref(false)
  const sidebarWidth = ref(getInitialSidebarWidth())
  const commandPaletteOpen = ref(false)
  // Toasts live in the library's queue, which the one RlToastHost in App.vue
  // renders. The store is a facade over it so its callers keep their API;
  // `toasts` is a read-only view in rela's own shape.
  const toastQueue = useToasts()
  const toasts = computed<Toast[]>(() =>
    toastQueue.toasts.value.map((t) => ({
      id: t.id,
      type: TYPE_OF[t.tone ?? 'info'],
      message: t.title,
      timeout: t.duration,
      action: t.action,
    }))
  )
  const currentModal = ref<string | null>(null)
  const modalData = ref<unknown>(null)
  const themeMode = ref<ThemeMode>(getInitialThemeMode())
  const darkMode = ref(getInitialDarkMode())
  // '' = follow the browser zone; otherwise a chosen IANA zone.
  const datetimeTimezone = ref<string>(getInitialDatetimeTimezone())

  // Getters
  const isSidebarVisible = computed(
    () => !sidebarCollapsed.value || sidebarMobileOpen.value
  )

  const isDark = computed(() => darkMode.value)

  // The zone datetime widgets interpret input in and display values in.
  const effectiveTimezone = computed(() => datetimeTimezone.value || browserTimeZone())

  // Actions
  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function setSidebarWidth(width: number) {
    sidebarWidth.value = clampSidebarWidth(width)
    localStorage.setItem('sidebarWidth', String(sidebarWidth.value))
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

  // The timer belongs to RlToast, which pauses it while the toast is hovered
  // or focused; a timeout of 0 keeps the toast until it is dismissed.
  function showToast(
    type: Toast['type'],
    message: string,
    timeout = 5000,
    action?: ToastAction
  ): string {
    return toastQueue.show({ title: message, tone: TONE_OF[type], duration: timeout, action })
  }

  function dismissToast(id: string) {
    toastQueue.dismiss(id)
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

  // Set the display-timezone override. Pass '' to follow the browser zone.
  // An unsupported zone is rejected (no-op) so a bad caller can't wedge the UI.
  // Persistence is synchronous (not a watch) so callers see localStorage
  // updated immediately after the call.
  function setDatetimeTimezone(tz: string) {
    if (tz !== '' && !isSupportedTimeZone(tz)) return
    datetimeTimezone.value = tz
    if (tz) {
      localStorage.setItem('datetimeTimezone', tz)
    } else {
      localStorage.removeItem('datetimeTimezone')
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

  // Apply dark mode class and persist.
  //
  // `.dark` is rela's own switch: tokens.css keys its dark palette off
  // `:root.dark` and nothing else, so an absent class means light.
  //
  // `.light` exists for the rela-components palette, which resolves a theme
  // differently: as well as `.dark`, it honours the OS via
  // `@media (prefers-color-scheme: dark) { :root:not(.light) }`. Without an
  // explicit `.light`, a user who has chosen Light on a dark-OS machine gets
  // rela's own chrome light and every Rl component dark. Writing both classes
  // states the choice positively, which is the only thing that media query
  // will accept as "no, really, light".
  //
  // Deliberately NOT written in `system` mode: there the OS preference IS the
  // answer, and pinning a class would override the thing it is meant to follow.
  watch(
    [darkMode, themeMode],
    ([dark, mode]) => {
      document.documentElement.classList.toggle('dark', dark)
      document.documentElement.classList.toggle('light', mode === 'light')
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
    sidebarWidth,
    commandPaletteOpen,
    toasts,
    currentModal,
    modalData,
    darkMode,
    themeMode,
    datetimeTimezone,

    // Getters
    isSidebarVisible,
    isDark,
    effectiveTimezone,

    // Actions
    toggleSidebar,
    setSidebarWidth,
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
    setDatetimeTimezone,
    applyPalette,
    clearPalette,
  }
})
