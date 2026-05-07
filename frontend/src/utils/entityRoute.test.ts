import { describe, it, expect } from 'vitest'
import { entityDetailHref } from './entityRoute'

describe('entityDetailHref', () => {
  const noDetailViews = () => undefined

  it('returns the configured detail_view path when present', () => {
    const getDetailView = (type: string) => (type === 'idea' ? 'idea_detail' : undefined)
    const href = entityDetailHref({ id: 'IDEA-001', type: 'idea' }, getDetailView)
    expect(href).toBe('/view/idea_detail/IDEA-001')
  })

  it('falls back to /entity/:type/:id when no detail_view configured', () => {
    const href = entityDetailHref({ id: 'TKT-9', type: 'ticket' }, noDetailViews)
    expect(href).toBe('/entity/ticket/TKT-9')
  })

  it('returns cellLink verbatim when provided (table per-column links win)', () => {
    const href = entityDetailHref({ id: 'TKT-9', type: 'ticket' }, () => 'ticket_detail', {
      cellLink: '/document/spec/TKT-9',
    })
    expect(href).toBe('/document/spec/TKT-9')
  })

  it('returns empty string when entity.type is empty (avoids /entity//id)', () => {
    const href = entityDetailHref({ id: 'X', type: '' }, () => 'unused')
    expect(href).toBe('')
  })

  it('returns empty string when entity.id is empty (avoids /entity/type/)', () => {
    const href = entityDetailHref({ id: '', type: 'ticket' }, () => 'ticket_detail')
    expect(href).toBe('')
  })

  it('cellLink wins even when type is empty', () => {
    const href = entityDetailHref({ id: 'X', type: '' }, noDetailViews, { cellLink: '/somewhere' })
    expect(href).toBe('/somewhere')
  })
})
