import { describe, it, expect } from 'vitest'
import { editFormRoute, entityDetailHref, ownedEntityHref } from './entityRoute'

describe('entityDetailHref', () => {
  it('returns /entity/:type/:id when no cellLink', () => {
    const href = entityDetailHref({ id: 'TKT-9', type: 'ticket' })
    expect(href).toBe('/entity/ticket/TKT-9')
  })

  it('returns cellLink verbatim when provided (table per-column links win)', () => {
    const href = entityDetailHref(
      { id: 'TKT-9', type: 'ticket' },
      { cellLink: '/document/spec/TKT-9' }
    )
    expect(href).toBe('/document/spec/TKT-9')
  })

  it('returns empty string when entity.type is empty (avoids /entity//id)', () => {
    const href = entityDetailHref({ id: 'X', type: '' })
    expect(href).toBe('')
  })

  it('returns empty string when entity.id is empty (avoids /entity/type/)', () => {
    const href = entityDetailHref({ id: '', type: 'ticket' })
    expect(href).toBe('')
  })

  it('cellLink wins even when type is empty', () => {
    const href = entityDetailHref({ id: 'X', type: '' }, { cellLink: '/somewhere' })
    expect(href).toBe('/somewhere')
  })
})

describe('ownedEntityHref', () => {
  it("links an owned entity to its owner's page, anchored at the entity", () => {
    const href = ownedEntityHref({
      id: 'TASK-2',
      type: 'task',
      _owner: { id: 'TASK-1', type: 'task' },
    })
    expect(href).toBe('/entity/task/TASK-1#TASK-2')
  })

  it('links an entity without an owner to its own page', () => {
    expect(ownedEntityHref({ id: 'TASK-2', type: 'task' })).toBe('/entity/task/TASK-2')
  })

  it('encodes the anchor', () => {
    const href = ownedEntityHref({
      id: 'a b',
      type: 'task',
      _owner: { id: 'TASK-1', type: 'task' },
    })
    expect(href).toBe('/entity/task/TASK-1#a%20b')
  })

  it('returns empty string when the owner has no type', () => {
    expect(
      ownedEntityHref({ id: 'TASK-2', type: 'task', _owner: { id: 'TASK-1', type: '' } })
    ).toBe('')
  })
})

describe('editFormRoute', () => {
  it('carries the world so the form loads the relations the page showed', () => {
    expect(editFormRoute('control', 'CTL-1', 'editorial')).toEqual({
      name: 'form-edit',
      params: { id: 'control', entityId: 'CTL-1' },
      query: { world: 'editorial' },
    })
  })

  it('omits the world for the default world', () => {
    expect(editFormRoute('policy', 'POL-1@draft', undefined)).toEqual({
      name: 'form-edit',
      params: { id: 'policy', entityId: 'POL-1@draft' },
      query: {},
    })
  })

  it('keeps the other query params', () => {
    expect(editFormRoute('control', 'CTL-1', 'editorial', { return_to: '/x' })).toEqual({
      name: 'form-edit',
      params: { id: 'control', entityId: 'CTL-1' },
      query: { return_to: '/x', world: 'editorial' },
    })
  })
})
