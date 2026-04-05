import { ref, onMounted, onBeforeUnmount, type Ref } from 'vue'
import { isInputFocused } from '@/utils/dom'

interface UseListKeyboardOptions {
  itemCount: Ref<number>
  onOpen?: (index: number) => void
  onEdit?: (index: number) => void
  onCreate?: () => void
  onDelete?: (index: number) => void
  onPrevPage?: () => void
  onNextPage?: () => void
  hasPrevPage?: Ref<boolean>
  hasNextPage?: Ref<boolean>
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

    // Don't handle if a modal is open
    if (document.querySelector('.shortcuts-overlay, .modal-overlay')) return

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
        if (selectedIndex.value >= 0 && options.onDelete && e.key === 'Delete') {
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
