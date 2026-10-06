import { describe, it, expect } from 'vitest'
import { editFormRoute, entityDetailHref } from './entityRoute'

describe('entityDetailHref', () => {
  it('returns /entity/:type/:id when no cellLink', () => {
    const href = entityDetailHref({ id: 'TKT-9', type: 'ticket' })
    expect(href).toBe('/entity/ticket/TKT-9')
  })

  it('returns cellLink verbatim when provided (table per-column links win)', () => {
    const href = entityDetailHref(
      { id: 'TKT-9', type: 'ticket' },
      { cellLink: '/document/spec/TKT-9' },
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
