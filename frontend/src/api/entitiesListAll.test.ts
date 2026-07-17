import { describe, it, expect, vi, beforeEach } from 'vitest'
import { listAllEntities, _setEntityPluralForTest } from './entities'
import { api } from './client'
import type { Entity, ListResponse } from '@/types'

vi.mock('./client', () => ({
  api: { get: vi.fn() },
}))

// BUG-5OAQUG: the kanban board consumed one page of the list endpoint as
// the complete set, silently dropping page 2+. listAllEntities is the
// fetch-complete-set helper: response-driven paging (follow meta.has_more
// / meta.page, assume nothing about the honored per_page), dedupe by ID
// (offset skew from concurrent writes), merged included/_actions, and a
// page cap that surfaces truncation via has_more instead of hiding it.

function makeEntity(id: string, title = id): Entity {
  return { id, type: 'ticket', properties: { title }, relations: {} }
}

function page(
  data: Entity[],
  meta: Partial<ListResponse<Entity>['meta']>,
  extra?: Partial<ListResponse<Entity>>
): ListResponse<Entity> {
  return {
    data,
    meta: { total: data.length, page: 1, per_page: 100, has_more: false, ...meta },
    _actions: { create: true },
    ...extra,
  }
}

const getMock = vi.mocked(api.get)

describe('listAllEntities', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    _setEntityPluralForTest('ticket', 'tickets')
  })

  it('returns a single page unchanged (one request, has_more false)', async () => {
    getMock.mockResolvedValueOnce(page([makeEntity('T-1')], { total: 1 }))

    const res = await listAllEntities('ticket')

    expect(getMock).toHaveBeenCalledTimes(1)
    expect(getMock.mock.calls[0][0]).toBe('/tickets')
    expect(getMock.mock.calls[0][1]).toMatchObject({ page: 1, per_page: 100 })
    expect(res.data.map((e) => e.id)).toEqual(['T-1'])
    expect(res.meta).toEqual({ total: 1, page: 1, per_page: 1, has_more: false })
  })

  it('follows has_more across pages and merges data and included', async () => {
    getMock
      .mockResolvedValueOnce(
        page([makeEntity('T-1'), makeEntity('T-2')], { total: 3, page: 1, has_more: true }, {
          included: { 'C-1': makeEntity('C-1') },
        })
      )
      .mockResolvedValueOnce(
        page([makeEntity('T-3')], { total: 3, page: 2 }, { included: { 'C-2': makeEntity('C-2') } })
      )

    const res = await listAllEntities('ticket', { include: '*' })

    expect(getMock).toHaveBeenCalledTimes(2)
    expect(getMock.mock.calls[0][1]).toMatchObject({ include: '*', page: 1 })
    expect(getMock.mock.calls[1][1]).toMatchObject({ include: '*', page: 2 })
    expect(res.data.map((e) => e.id)).toEqual(['T-1', 'T-2', 'T-3'])
    expect(Object.keys(res.included ?? {}).sort()).toEqual(['C-1', 'C-2'])
    expect(res.meta.has_more).toBe(false)
    expect(res.meta.total).toBe(3)
  })

  it('keeps _actions from the first page', async () => {
    getMock
      .mockResolvedValueOnce(
        page([makeEntity('T-1')], { page: 1, has_more: true }, { _actions: { create: true } })
      )
      .mockResolvedValueOnce(
        page([makeEntity('T-2')], { page: 2 }, { _actions: { create: false } })
      )

    const res = await listAllEntities('ticket')
    expect(res._actions).toEqual({ create: true })
  })

  it('dedupes an entity that appears on two pages, keeping the later copy', async () => {
    // A concurrent write between page fetches shifts offsets, so T-2 shows
    // up again on page 2 — with a fresher property value.
    getMock
      .mockResolvedValueOnce(
        page([makeEntity('T-1'), makeEntity('T-2', 'stale')], { total: 3, page: 1, has_more: true })
      )
      .mockResolvedValueOnce(
        page([makeEntity('T-2', 'fresh'), makeEntity('T-3')], { total: 3, page: 2 })
      )

    const res = await listAllEntities('ticket')

    expect(res.data.map((e) => e.id)).toEqual(['T-1', 'T-2', 'T-3'])
    expect(res.data.find((e) => e.id === 'T-2')?.properties.title).toBe('fresh')
    expect(res.meta.per_page).toBe(3)
  })

  it('advances from the response meta when the server ignores per_page', async () => {
    // parseV1Pagination silently falls back to 25 on out-of-range values;
    // the loop must follow meta.page + 1, not offsets derived from the
    // requested page size.
    getMock
      .mockResolvedValueOnce(page([makeEntity('T-1')], { total: 2, page: 1, per_page: 25, has_more: true }))
      .mockResolvedValueOnce(page([makeEntity('T-2')], { total: 2, page: 2, per_page: 25 }))

    const res = await listAllEntities('ticket')

    expect(getMock.mock.calls[1][1]).toMatchObject({ page: 2 })
    expect(res.data).toHaveLength(2)
  })

  it('forwards the abort signal to every page request and stops when it fires', async () => {
    // Mirrors axios behavior: a request started with an aborted signal
    // rejects immediately. Aborting after page 1 must fail the second
    // request and end the loop — a superseded Colada refetch would
    // otherwise let the old loop keep paging to the cap.
    const controller = new AbortController()
    getMock.mockImplementation(async (_url, params, signal) => {
      if ((signal as AbortSignal | undefined)?.aborted) throw new Error('canceled')
      const p = (params as { page: number }).page
      controller.abort()
      return page([makeEntity(`T-${p}`)], { total: 300, page: p, has_more: true })
    })

    await expect(listAllEntities('ticket', undefined, controller.signal)).rejects.toThrow(
      'canceled'
    )
    expect(getMock).toHaveBeenCalledTimes(2)
    expect(getMock.mock.calls[0][2]).toBe(controller.signal)
    expect(getMock.mock.calls[1][2]).toBe(controller.signal)
  })

  it('breaks on an empty page even when has_more stays true, keeping the anomaly visible', async () => {
    // A server that reports has_more with no rows would otherwise spin
    // the loop to the page cap re-fetching nothing.
    getMock
      .mockResolvedValueOnce(page([makeEntity('T-1')], { total: 5, page: 1, has_more: true }))
      .mockResolvedValueOnce(page([], { total: 5, page: 2, has_more: true }))

    const res = await listAllEntities('ticket')

    expect(getMock).toHaveBeenCalledTimes(2)
    expect(res.data.map((e) => e.id)).toEqual(['T-1'])
    expect(res.meta.has_more).toBe(true)
  })

  it('stops at the page cap and preserves has_more so truncation is visible', async () => {
    let n = 0
    getMock.mockImplementation(async () =>
      page([makeEntity(`T-${++n}`)], { total: 9999, page: n, has_more: true })
    )

    const res = await listAllEntities('ticket')

    expect(getMock).toHaveBeenCalledTimes(50)
    expect(res.data).toHaveLength(50)
    expect(res.meta.has_more).toBe(true)
    expect(res.meta.total).toBe(9999)
  })
})
