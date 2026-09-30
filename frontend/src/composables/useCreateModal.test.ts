import { describe, it, expect, vi } from 'vitest'
import { useCreateModal } from './useCreateModal'
import type { Entity } from '@/types'

function click(init: MouseEventInit = {}) {
  return new MouseEvent('click', { button: 0, cancelable: true, ...init })
}

describe('useCreateModal', () => {
  it('opens on a plain click and cancels the link navigation', () => {
    const modal = useCreateModal(vi.fn())
    const event = click()
    modal.onClick(event)
    expect(modal.open.value).toBe(true)
    expect(event.defaultPrevented).toBe(true)
  })

  it.each([
    ['cmd', { metaKey: true }],
    ['ctrl', { ctrlKey: true }],
    ['shift', { shiftKey: true }],
    ['middle button', { button: 1 }],
  ])('leaves a %s click to the link', (_name, init) => {
    const modal = useCreateModal(vi.fn())
    const event = click(init)
    modal.onClick(event)
    expect(modal.open.value).toBe(false)
    expect(event.defaultPrevented).toBe(false)
  })

  it('closes and hands over the created entity', () => {
    const onCreated = vi.fn()
    const modal = useCreateModal(onCreated)
    modal.show()
    const entity = { id: 'T-1', type: 'ticket', properties: {}, relations: {} } as Entity
    modal.created(entity)
    expect(modal.open.value).toBe(false)
    expect(onCreated).toHaveBeenCalledWith(entity)
  })

  it('keeps the dialog open for "Create & add another"', () => {
    const onCreated = vi.fn()
    const onCreatedAnother = vi.fn()
    const modal = useCreateModal(onCreated, onCreatedAnother)
    modal.show()

    const entity = { id: 'IDEA-1', type: 'idea' } as Entity
    modal.createdAnother(entity)

    expect(modal.open.value).toBe(true)
    expect(onCreatedAnother).toHaveBeenCalledWith(entity)
    expect(onCreated).not.toHaveBeenCalled()
  })
})
