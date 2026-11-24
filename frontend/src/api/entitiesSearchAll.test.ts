import { describe, it, expect, vi, beforeEach } from 'vitest'
import { searchAllEntities } from './entities'
import { api } from './client'
import type { Entity, ListResponse } from '@/types'

vi.mock('./client', () => ({
  api: { get: vi.fn() },
}))

// TKT-T0DK37: one /_search read returns at most one page. searchAllEntities
// follows meta.has_more so a caller that computes over the whole result
// sees every row.

function makeEntity(id: string): Entity {
  return { id, type: 'ticket', properties: { title: id }, relations: {} }
}

function page(data: Entity[], meta: Partial<ListResponse<Entity>['meta']>): ListResponse<Entity> {
  return { data, meta: { total: data.length, page: 1, per_page: 1000, has_more: false, ...meta } }
}

const getMock = vi.mocked(api.get)

describe('searchAllEntities', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('reads one page when has_more is false, without a page parameter', async () => {
    getMock.mockResolvedValueOnce(page([makeEntity('T-1')], {}))

    const res = await searchAllEntities('type:ticket')

    expect(getMock).toHaveBeenCalledTimes(1)
    expect(getMock.mock.calls[0][0]).toBe('/_search')
    expect(getMock.mock.calls[0][1]).toEqual({ q: 'type:ticket' })
    expect(res.data.map((e) => e.id)).toEqual(['T-1'])
    expect(res.meta).toEqual({ total: 1, page: 1, per_page: 1, has_more: false })
  })

  it('follows has_more and dedupes rows a write shifted between pages', async () => {
    getMock
      .mockResolvedValueOnce(page([makeEntity('T-1'), makeEntity('T-2')], { total: 3, has_more: true }))
      .mockResolvedValueOnce(page([makeEntity('T-2'), makeEntity('T-3')], { total: 3, page: 2 }))

    const res = await searchAllEntities('type:ticket', { world: 'draft' })

    expect(getMock).toHaveBeenCalledTimes(2)
    expect(getMock.mock.calls[1][1]).toEqual({ q: 'type:ticket', world: 'draft', page: '2' })
    expect(res.data.map((e) => e.id)).toEqual(['T-1', 'T-2', 'T-3'])
    expect(res.meta).toEqual({ total: 3, page: 1, per_page: 3, has_more: false })
  })

  it('stops on an empty page that claims has_more and keeps the flag', async () => {
    getMock
      .mockResolvedValueOnce(page([makeEntity('T-1')], { total: 5, has_more: true }))
      .mockResolvedValueOnce(page([], { total: 5, page: 2, has_more: true }))

    const res = await searchAllEntities('type:ticket')

    expect(getMock).toHaveBeenCalledTimes(2)
    expect(res.meta.has_more).toBe(true)
  })
})
