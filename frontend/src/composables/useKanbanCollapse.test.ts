import { describe, it, expect, beforeEach, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { kanbanCollapseStorageKey, useKanbanCollapse } from './useKanbanCollapse'

describe('useKanbanCollapse', () => {
  beforeEach(() => localStorage.clear())

  it('starts from the config default', () => {
    const { isCollapsed } = useKanbanCollapse('board', { postponed: true })
    expect(isCollapsed('postponed')).toBe(true)
    expect(isCollapsed('open')).toBe(false)
  })

  it('remembers a toggle per board', () => {
    useKanbanCollapse('board', {}).setCollapsed('done', true)

    expect(useKanbanCollapse('board', {}).isCollapsed('done')).toBe(true)
    expect(useKanbanCollapse('other', {}).isCollapsed('done')).toBe(false)
  })

  it('keeps a default-collapsed column open once the reader opens it', () => {
    useKanbanCollapse('board', { postponed: true }).setCollapsed('postponed', false)

    expect(useKanbanCollapse('board', { postponed: true }).isCollapsed('postponed')).toBe(false)
  })

  it('drops an override that matches the default, so a new default applies', () => {
    const board = useKanbanCollapse('board', {})
    board.setCollapsed('done', true)
    board.setCollapsed('done', false)

    expect(localStorage.getItem(kanbanCollapseStorageKey('board'))).toBeNull()
    expect(useKanbanCollapse('board', { done: true }).isCollapsed('done')).toBe(true)
  })

  it('writes under the current board after a switch', async () => {
    const id = ref('a')
    const { setCollapsed } = useKanbanCollapse(id, {})
    id.value = 'b'
    await nextTick()

    setCollapsed('done', true)

    expect(localStorage.getItem(kanbanCollapseStorageKey('a'))).toBeNull()
    expect(JSON.parse(localStorage.getItem(kanbanCollapseStorageKey('b'))!)).toEqual({ done: true })
  })

  it('still folds the column when storage refuses the write', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError')
    })
    try {
      const board = useKanbanCollapse('board', {})
      board.setCollapsed('done', true)
      expect(board.isCollapsed('done')).toBe(true)
    } finally {
      setItem.mockRestore()
    }
  })

  it('reloads when the board changes', async () => {
    localStorage.setItem(kanbanCollapseStorageKey('b'), JSON.stringify({ done: true }))
    const id = ref('a')
    const { isCollapsed } = useKanbanCollapse(id, {})
    expect(isCollapsed('done')).toBe(false)

    id.value = 'b'
    await nextTick()
    expect(isCollapsed('done')).toBe(true)
  })

  it.each([
    ['corrupt JSON', '{not json'],
    ['an array', '["done"]'],
    ['a non-boolean value', '{"done":"yes"}'],
  ])('ignores %s', (_, stored) => {
    localStorage.setItem(kanbanCollapseStorageKey('board'), stored)
    expect(useKanbanCollapse('board', { done: false }).isCollapsed('done')).toBe(false)
  })
})
