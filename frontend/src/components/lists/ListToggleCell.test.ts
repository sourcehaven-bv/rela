import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ListToggleCell from './ListToggleCell.vue'
import { toggleCellState } from '@/utils/toggleCell'
import type { Entity, ListColumn } from '@/types'

const updateEntityMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
}))

const row = (props: Record<string, unknown>, extra: Partial<Entity> = {}) =>
  ({ id: 'T-1', type: 'todo', properties: props, ...extra }) as unknown as Entity
const enumCol: ListColumn = {
  property: 'status',
  widget: 'toggle',
  on: 'done',
  off: 'open',
  other: 'disabled',
}
const boolCol: ListColumn = { property: 'flagged', widget: 'switch' }

describe('toggleCellState', () => {
  it.each([
    [
      'boolean off',
      boolCol,
      row({}),
      { checked: false, target: true, disabled: false, hidden: false },
    ],
    [
      'boolean on',
      boolCol,
      row({ flagged: true }),
      { checked: true, target: false, disabled: false, hidden: false },
    ],
    [
      'enum on',
      enumCol,
      row({ status: 'done' }),
      { checked: true, target: 'open', disabled: false, hidden: false },
    ],
    [
      'enum off',
      enumCol,
      row({ status: 'open' }),
      { checked: false, target: 'done', disabled: false, hidden: false },
    ],
    [
      'other disabled',
      enumCol,
      row({ status: 'waiting' }),
      { checked: false, target: 'done', disabled: true, hidden: false },
    ],
    [
      'other on',
      { ...enumCol, other: 'on' as const },
      row({ status: 'waiting' }),
      { checked: true, target: 'open', disabled: false, hidden: false },
    ],
    [
      'other hidden',
      { ...enumCol, other: 'hidden' as const },
      row({ status: 'waiting' }),
      { checked: false, target: undefined, disabled: true, hidden: true },
    ],
    [
      'redacted',
      enumCol,
      row({}, { _redacted: ['status'] }),
      { checked: false, target: undefined, disabled: true, hidden: true },
    ],
    [
      'no update',
      enumCol,
      row({ status: 'open' }, { _actions: { update: false } }),
      { checked: false, target: 'done', disabled: true, hidden: false },
    ],
    [
      'read-only field',
      enumCol,
      row({ status: 'open' }, { _fields: { status: { writable: false } } }),
      { checked: false, target: 'done', disabled: true, hidden: false },
    ],
    [
      'transition refused',
      enumCol,
      row({ status: 'open' }, { _fields: { status: { options: { done: false } } } }),
      { checked: false, target: 'done', disabled: true, hidden: false },
    ],
  ])('%s', (_name, column, entity, want) => {
    expect(toggleCellState(entity, column)).toEqual(want)
  })
})

describe('toggleCellState with a state machine', () => {
  const moves = { 'in-progress': ['done'], done: ['in-progress'], open: ['in-progress'] }
  const col: ListColumn = {
    property: 'status',
    widget: 'toggle',
    on: 'done',
    off: 'open',
    other: 'on',
  }
  it('disables a move the machine does not declare', () => {
    expect(toggleCellState(row({ status: 'open' }), col, moves).disabled).toBe(true)
    expect(toggleCellState(row({ status: 'done' }), col, moves).disabled).toBe(true)
  })
  it('allows a declared move', () => {
    expect(toggleCellState(row({ status: 'in-progress' }), col, moves)).toMatchObject({
      checked: true,
      target: 'open',
      disabled: true,
    })
    const flip: ListColumn = { ...col, off: 'in-progress', other: 'disabled' }
    expect(toggleCellState(row({ status: 'in-progress' }), flip, moves)).toMatchObject({
      checked: false,
      target: 'done',
      disabled: false,
    })
  })
})

describe('ListToggleCell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    updateEntityMock.mockReset()
  })

  const mountCell = (entity: Entity, column: ListColumn = enumCol) =>
    mount(ListToggleCell, { props: { entity, column, entityType: 'todo' } })

  it('PATCHes the on value and asks for a refresh', async () => {
    updateEntityMock.mockResolvedValue(row({ status: 'done' }))
    const w = mountCell(row({ status: 'open' }))
    await w.find('input[type="checkbox"]').setValue(true)
    await flushPromises()
    expect(updateEntityMock).toHaveBeenCalledWith('todo', 'T-1', { properties: { status: 'done' } })
    expect(w.emitted('changed')).toHaveLength(1)
  })

  it('reverts when the PATCH fails', async () => {
    updateEntityMock.mockRejectedValue(new Error('refused'))
    const w = mountCell(row({ status: 'open' }))
    await w.find('input[type="checkbox"]').setValue(true)
    await flushPromises()
    expect(w.emitted('changed')).toBeUndefined()
    expect((w.find('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
  })

  it('sends nothing when disabled', async () => {
    const w = mountCell(row({ status: 'open' }, { _actions: { update: false } }))
    expect(w.find('input[type="checkbox"]').attributes('disabled')).toBeDefined()
    expect(updateEntityMock).not.toHaveBeenCalled()
  })

  it('renders nothing for a hidden other value', () => {
    const w = mountCell(row({ status: 'waiting' }), { ...enumCol, other: 'hidden' })
    expect(w.find('.list-toggle-cell').exists()).toBe(false)
  })

  it('renders a switch', () => {
    const w = mountCell(row({}), boolCol)
    expect(w.find('[role="switch"]').exists()).toBe(true)
  })
})
