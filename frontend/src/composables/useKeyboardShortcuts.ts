import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'

export const shortcutsModalOpen = ref(false)

export function useKeyboardShortcuts() {
  const router = useRouter()
  const route = useRoute()

  // G-sequence state
  let gPending = false
  let gTimer: ReturnType<typeof setTimeout> | null = null

  function isInputFocused(): boolean {
    const el = document.activeElement
    if (!el) return false
    const tag = el.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
    if ((el as HTMLElement).isContentEditable) return true
    // Check for CodeMirror (EasyMDE)
    if (el.closest && el.closest('.CodeMirror')) return true
    return false
  }

  function isFormPage(): boolean {
    return route.name === 'form-create' || route.name === 'form-edit'
  }

  function isSearchPage(): boolean {
    return route.name === 'search'
  }

  function handleKeydown(e: KeyboardEvent) {
    // Cmd/Ctrl+K: reserved for command palette (future)
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault()
      // TODO: implement command palette
      return
    }

    // Escape: close shortcuts modal first
    if (e.key === 'Escape') {
      if (shortcutsModalOpen.value) {
        shortcutsModalOpen.value = false
        return
      }
      // If in input, blur it
      if (isInputFocused()) {
        ;(document.activeElement as HTMLElement)?.blur()
        return
      }
      // On form page, go back
      if (isFormPage()) {
        router.back()
        return
      }
      return
    }

    // Don't handle single-key shortcuts when in input
    if (isInputFocused()) return

    // ? = show keyboard shortcuts
    if (e.key === '?') {
      shortcutsModalOpen.value = true
      return
    }

    // G-prefix sequences
    if (gPending) {
      gPending = false
      if (gTimer) clearTimeout(gTimer)

      if (e.key === 'd') {
        router.push('/dashboard')
        return
      }
      if (e.key === 'g') {
        router.push('/graph')
        return
      }
      if (e.key === 's') {
        router.push('/search')
        return
      }
      if (e.key === 'a') {
        router.push('/analyze')
        return
      }
      return
    }

    // g = start G-sequence
    if (e.key === 'g') {
      gPending = true
      gTimer = setTimeout(() => {
        gPending = false
      }, 1000)
      return
    }

    // / = focus search (go to search page if not there)
    if (e.key === '/') {
      e.preventDefault()
      if (!isSearchPage()) {
        router.push('/search')
      }
      // Focus will be handled by SearchView
      return
    }
  }

  onMounted(() => {
    document.addEventListener('keydown', handleKeydown)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('keydown', handleKeydown)
  })

  return {
    shortcutsModalOpen,
  }
}
