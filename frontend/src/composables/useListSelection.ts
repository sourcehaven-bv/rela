import { ref, type Ref } from 'vue'

/**
 * Composable for managing multi-select state in list views.
 * Tracks selected entity IDs as a reactive Set.
 */
export function useListSelection() {
  const selectedIds: Ref<Set<string>> = ref(new Set())

  function toggle(id: string) {
    const next = new Set(selectedIds.value)
    if (next.has(id)) {
      next.delete(id)
    } else {
      next.add(id)
    }
    selectedIds.value = next
  }

  function clear() {
    selectedIds.value = new Set()
  }

  function isSelected(id: string): boolean {
    return selectedIds.value.has(id)
  }

  function selectAll(ids: string[]) {
    selectedIds.value = new Set(ids)
  }

  return {
    selectedIds,
    toggle,
    clear,
    isSelected,
    selectAll,
  }
}
