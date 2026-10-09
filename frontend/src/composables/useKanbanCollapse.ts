import { ref, watch, type MaybeRefOrGetter, toValue } from 'vue'

/** Where a board's folded and unfolded columns are remembered, per browser. */
export function kanbanCollapseStorageKey(kanbanId: string): string {
  return `rela:kanban-columns:${kanbanId}`
}

/*
 * The reader's choices are kept as overrides of the config default, not as a
 * set of folded columns, so a column the config folds can be opened and stay
 * open across a reload. An override equal to the default is dropped, so a
 * later change to the config default still reaches a reader who toggled the
 * column back.
 */
type Overrides = Record<string, boolean>

function readOverrides(kanbanId: string): Overrides {
  try {
    const parsed: unknown = JSON.parse(
      localStorage.getItem(kanbanCollapseStorageKey(kanbanId)) ?? '{}'
    )
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    return Object.fromEntries(
      Object.entries(parsed).filter(
        (entry): entry is [string, boolean] => typeof entry[1] === 'boolean'
      )
    )
  } catch {
    // A corrupt entry costs the reader their folded columns, not the board.
    return {}
  }
}

/**
 * Which columns of a kanban board are collapsed. `defaults` is the config's
 * `collapsed:` per column value; the reader's toggles override it and are
 * stored in localStorage under the board's id. Other open tabs pick a change
 * up on their next load; there is no cross-tab sync.
 */
export function useKanbanCollapse(
  kanbanId: MaybeRefOrGetter<string>,
  defaults: MaybeRefOrGetter<Record<string, boolean>>
) {
  const overrides = ref<Overrides>({})

  watch(
    () => toValue(kanbanId),
    (id) => {
      overrides.value = readOverrides(id)
    },
    { immediate: true }
  )

  function isCollapsed(column: string): boolean {
    return overrides.value[column] ?? toValue(defaults)[column] ?? false
  }

  function setCollapsed(column: string, collapsed: boolean) {
    const next = { ...overrides.value }
    if (collapsed === (toValue(defaults)[column] ?? false)) delete next[column]
    else next[column] = collapsed
    overrides.value = next
    const key = kanbanCollapseStorageKey(toValue(kanbanId))
    try {
      if (Object.keys(next).length) localStorage.setItem(key, JSON.stringify(next))
      else localStorage.removeItem(key)
    } catch {
      // Storage full or blocked: the column still folds for this visit.
    }
  }

  return { isCollapsed, setCollapsed }
}
