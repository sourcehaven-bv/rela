import { describe, it, expect } from 'vitest'
import { useSidebarEmptyGroups } from './useSidebarEmptyGroups'
import type { SidebarGroup } from '@/types'

const entitiesOnly: SidebarGroup = {
  group: 'Projects',
  items: [
    { label: '', entities: { type: 'project' } },
    { label: '', entities: { type: 'note' } },
  ],
}
const mixed: SidebarGroup = {
  group: 'Work',
  items: [
    { label: 'All', href: '/list/all' },
    { label: '', entities: { type: 'project' } },
  ],
}

describe('useSidebarEmptyGroups', () => {
  it('hides an entities-only group until one item has something to show', () => {
    const g = useSidebarEmptyGroups()
    // Nothing reported yet: no heading flashes while the fetch is in flight.
    expect(g.groupIsEmpty(entitiesOnly, 0)).toBe(true)

    g.setEntryShown(g.entryKey(0, 1), true)
    expect(g.groupIsEmpty(entitiesOnly, 0)).toBe(false)

    // The last row leaving hides the heading again.
    g.setEntryShown(g.entryKey(0, 1), false)
    expect(g.groupIsEmpty(entitiesOnly, 0)).toBe(true)
  })

  it('never hides a group holding an ordinary link', () => {
    const g = useSidebarEmptyGroups()
    expect(g.groupIsEmpty(mixed, 0)).toBe(false)
  })

  it('keeps state per group index', () => {
    const g = useSidebarEmptyGroups()
    g.setEntryShown(g.entryKey(0, 0), true)
    expect(g.groupIsEmpty(entitiesOnly, 0)).toBe(false)
    expect(g.groupIsEmpty(entitiesOnly, 1)).toBe(true)
  })

  it('forgets everything on reset', () => {
    const g = useSidebarEmptyGroups()
    g.setEntryShown(g.entryKey(0, 0), true)
    g.reset()
    expect(g.groupIsEmpty(entitiesOnly, 0)).toBe(true)
  })

  it('gives entities items distinct keys that change with the load generation', () => {
    const g = useSidebarEmptyGroups()
    const [a, b] = entitiesOnly.items
    expect(g.itemKey(a, 0, 0)).not.toBe(g.itemKey(b, 0, 1))
    expect(g.itemKey(a, 0, 0, 1)).not.toBe(g.itemKey(a, 0, 0, 2))
    // Ordinary items keep a load-independent key, so a reload does not remount them.
    expect(g.itemKey(mixed.items[0], 0, 0, 1)).toBe('All/list/all')
    expect(g.itemKey(mixed.items[0], 0, 0, 2)).toBe('All/list/all')
  })
})
