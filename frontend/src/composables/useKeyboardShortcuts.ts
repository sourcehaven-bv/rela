import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { isInputFocused } from '@/utils/dom'
import { isAnyModalOpen } from './modalStack'

/**
 * Shared state for keyboard shortcuts modal.
 * Intentionally module-level to ensure single modal instance across app.
 * This is safe because:
 * 1. Only one modal should ever be open at a time
 * 2. The modal is rendered at App.vue level
 * 3. Multiple components may need to check/toggle this state
 */
export const shortcutsModalOpen = ref(false)

// Module-level ref so the global Cmd+K handler can flip it and the App-level
// CommandPaletteModal can react. Same single-instance rationale as above.
export const paletteOpen = ref(false)

/**
 * Global keyboard shortcuts composable.
 * Should be called once at App.vue level to register global handlers.
 * Handles G-prefix navigation sequences and modal toggling.
 */
export function useKeyboardShortcuts() {
  const router = useRouter()
  const route = useRoute()

  // G-sequence state
  let gPending = false
  let gTimer: ReturnType<typeof setTimeout> | null = null

  function isFormPage(): boolean {
    return route.name === 'form-create' || route.name === 'form-edit'
  }

  function isSearchPage(): boolean {
    return route.name === 'search'
  }

  function handleKeydown(e: KeyboardEvent) {
    // Cmd/Ctrl+K opens the command palette. Bypasses isInputFocused and
    // isAnyModalOpen on purpose — users expect the palette to open from
    // anywhere, including form fields and on top of other modals.
    // Idempotent: already-open is a no-op.
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault()
      paletteOpen.value = true
      return
    }

    // Stand down while any modal is open. The modal owns its own keyboard
    // semantics (Escape to close, etc.); we must not double-handle Escape
    // and accidentally trigger router.back() on a form page underneath.
    if (isAnyModalOpen()) return

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

    // / = focus search.
    //
    // List views own their own search box (TKT-603FQ), so when one is
    // visible we let useListKeyboard handle the keystroke instead of jumping
    // to the standalone /search page. Without this guard, the global handler
    // would race with the per-list handler and the user would lose their
    // list context on every `/`.
    if (e.key === '/') {
      if (document.querySelector('.entity-list .search-box')) return
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
    paletteOpen,
  }
}
