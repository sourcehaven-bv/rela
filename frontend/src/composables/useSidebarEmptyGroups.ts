import { ref } from 'vue'
import type { SidebarGroup, SidebarItem } from '@/types'

/**
 * Tracks which `entities:` sidebar items have something to show (TKT-PEKL8L),
 * so a group made only of such items can hide its heading until one does.
 *
 * That matches the server rule that drops a group with no items: a bare
 * heading reads as a rendering bug. An item counts as empty until it reports
 * otherwise, so a group whose scope matches nothing never flashes its heading
 * while the first fetch is in flight. The items stay mounted (the caller uses
 * v-show) so their queries keep running and the group appears when a row
 * arrives.
 */
export function useSidebarEmptyGroups() {
  const shownEntries = ref<Set<string>>(new Set())

  function entryKey(groupIndex: number, itemIndex: number): string {
    return `${groupIndex}:${itemIndex}`
  }

  /**
   * v-for key for a sidebar row. An `entities:` item has no label or href, so
   * the label+href key every other item uses would collide. Its key also
   * carries `load`, the sidebar load generation: a reload then remounts it so
   * it reports again after reset, since a cached query result would not.
   */
  function itemKey(item: SidebarItem, groupIndex: number, itemIndex: number, load = 0): string {
    if (item.entities) return `entities:${load}:${entryKey(groupIndex, itemIndex)}`
    return item.label + (item.href || item.action || '')
  }

  function setEntryShown(key: string, shown: boolean) {
    const next = new Set(shownEntries.value)
    if (shown) next.add(key)
    else next.delete(key)
    shownEntries.value = next
  }

  function groupIsEmpty(group: SidebarGroup, groupIndex: number): boolean {
    return (
      group.items.length > 0 &&
      group.items.every(
        (item, i) => item.entities !== undefined && !shownEntries.value.has(entryKey(groupIndex, i))
      )
    )
  }

  /** Forget every recorded state; call when the sidebar payload is replaced. */
  function reset() {
    shownEntries.value = new Set()
  }

  return { entryKey, itemKey, setEntryShown, groupIsEmpty, reset }
}
