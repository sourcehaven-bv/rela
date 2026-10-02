import { beforeEach, describe, expect, it } from 'vitest'
import { resetFlyout, useFlyout } from './useFlyout'
import type { Entity } from '@/types'

const mine = { navId: 'href:/list/mine', title: 'Mine', listId: 'mine', href: '/list/mine' }
const all = { navId: 'href:/list/all', title: 'All', listId: 'all', href: '/list/all' }
const entity: Entity = { id: 'T-1', type: 'ticket', properties: {}, relations: {} }

beforeEach(resetFlyout)

describe('useFlyout', () => {
  it('opens a list and closes it on a second toggle of the same entry', () => {
    const flyout = useFlyout()
    flyout.toggle(mine)
    expect(flyout.openNavId.value).toBe(mine.navId)
    flyout.toggle(mine)
    expect(flyout.list.value).toBeNull()
  })

  it('switches to another list and drops the open row', () => {
    const flyout = useFlyout()
    flyout.toggle(mine)
    flyout.openEntity(entity)
    flyout.toggle(all)
    expect(flyout.list.value?.listId).toBe('all')
    expect(flyout.entity.value).toBeNull()
  })

  it('closes the row without the list, and both on close', () => {
    const flyout = useFlyout()
    flyout.toggle(mine)
    flyout.openEntity(entity)
    flyout.closeEntity()
    expect(flyout.list.value).not.toBeNull()
    flyout.openEntity(entity)
    flyout.close()
    expect(flyout.list.value).toBeNull()
    expect(flyout.entity.value).toBeNull()
  })
})
