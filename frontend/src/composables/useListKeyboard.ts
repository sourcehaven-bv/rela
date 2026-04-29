import { ref, onMounted, onBeforeUnmount, type Ref } from 'vue'
import { isInputFocused } from '@/utils/dom'
import { isAnyModalOpen } from './modalStack'

interface UseListKeyboardOptions {
  itemCount: Ref<number>
  onOpen?: (index: number) => void
  onEdit?: (index: number) => void
  onCreate?: () => void
  onDelete?: (index: number) => void
  onSelect?: (index: number) => void
  onClearSelection?: () => void
  hasSelection?: Ref<boolean>
  onPrevPage?: () => void
  onNextPage?: () => void
  hasPrevPage?: Ref<boolean>
  hasNextPage?: Ref<boolean>
  onFocusSearch?: () => void
  onOpenFilter?: () => void
}

/**
 * Keyboard navigation composable for list views.
 * Handles j/k navigation, Enter/o to open, e to edit, n to create.
 * Uses DOM queries for scroll behavior - tied to .entity-row class convention.
 */
export function useListKeyboard(options: UseListKeyboardOptions) {
  const selectedIndex = ref(-1)

  function moveSelection(delta: number) {
    const count = options.itemCount.value
    if (count === 0) return

    if (selectedIndex.value === -1) {
      // Nothing selected, select first or last depending on direction
      selectedIndex.value = delta > 0 ? 0 : count - 1
    } else {
      selectedIndex.value = Math.max(0, Math.min(count - 1, selectedIndex.value + delta))
    }

    // Scroll selected row into view
    scrollSelectedIntoView()
  }

  function scrollSelectedIntoView() {
    const rows = document.querySelectorAll('.entity-row')
    const row = rows[selectedIndex.value]
    if (row) {
      row.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    // Don't handle if in input field
    if (isInputFocused()) return

    // Don't handle if any modal is open. Uses an explicit stack registry
    // rather than a CSS-class query so unrelated modals (Command, etc.)
    // don't accidentally enable/disable list shortcuts based on class reuse.
    if (isAnyModalOpen()) return
    // Legacy fallback for the shortcuts modal which does not register with
    // the stack yet.
    if (document.querySelector('.shortcuts-overlay')) return

    switch (e.key) {
      case 'j':
      case 'ArrowDown':
        e.preventDefault()
        moveSelection(1)
        break

      case 'k':
      case 'ArrowUp':
        e.preventDefault()
        moveSelection(-1)
        break

      case ' ':
        if (selectedIndex.value >= 0 && options.onSelect) {
          e.preventDefault()
          options.onSelect(selectedIndex.value)
        }
        break

      case 'Escape':
        if (options.hasSelection?.value && options.onClearSelection) {
          e.preventDefault()
          options.onClearSelection()
        }
        break

      case 'Enter':
      case 'o':
        if (selectedIndex.value >= 0 && options.onOpen) {
          e.preventDefault()
          options.onOpen(selectedIndex.value)
        }
        break

      case 'e':
        if (selectedIndex.value >= 0 && options.onEdit) {
          e.preventDefault()
          options.onEdit(selectedIndex.value)
        }
        break

      case 'n':
        if (options.onCreate) {
          e.preventDefault()
          options.onCreate()
        }
        break

      case 'Delete':
      case 'Backspace':
        // Backspace is included so Mac users (no dedicated Delete key) can use
        // it. Guarded by selectedIndex >= 0, so browser back-nav only loses
        // when a row is explicitly selected — at which point a confirm modal
        // intercepts anyway. preventDefault stops the browser back-nav side
        // effect when we own the key.
        if (selectedIndex.value >= 0 && options.onDelete) {
          e.preventDefault()
          options.onDelete(selectedIndex.value)
        }
        break

      case 'h':
        if (options.onPrevPage && options.hasPrevPage?.value) {
          e.preventDefault()
          options.onPrevPage()
        }
        break

      case 'l':
        if (options.onNextPage && options.hasNextPage?.value) {
          e.preventDefault()
          options.onNextPage()
        }
        break

      case '/':
        if (options.onFocusSearch) {
          e.preventDefault()
          options.onFocusSearch()
        }
        break

      case 'f':
        if (options.onOpenFilter) {
          e.preventDefault()
          options.onOpenFilter()
        }
        break
    }
  }

  function clearSelection() {
    selectedIndex.value = -1
  }

  onMounted(() => {
    document.addEventListener('keydown', handleKeydown)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('keydown', handleKeydown)
  })

  return {
    selectedIndex,
    clearSelection,
  }
}
