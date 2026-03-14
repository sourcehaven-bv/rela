import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface Toast {
  id: string
  type: 'success' | 'error' | 'warning' | 'info'
  message: string
  timeout?: number
}

export const useUIStore = defineStore('ui', () => {
  // State
  const sidebarCollapsed = ref(false)
  const sidebarMobileOpen = ref(false)
  const commandPaletteOpen = ref(false)
  const toasts = ref<Toast[]>([])
  const currentModal = ref<string | null>(null)
  const modalData = ref<unknown>(null)

  // Getters
  const isSidebarVisible = computed(
    () => !sidebarCollapsed.value || sidebarMobileOpen.value
  )

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

  return {
    // State
    sidebarCollapsed,
    sidebarMobileOpen,
    commandPaletteOpen,
    toasts,
    currentModal,
    modalData,

    // Getters
    isSidebarVisible,

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
  }
})
