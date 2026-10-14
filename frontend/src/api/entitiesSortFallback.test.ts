import { describe, it, expect, vi } from 'vitest'
import { withSortFallback, isPropertyNotVisible } from './entities'
import { ApiError } from './errors'
import type { ListParams, ListResponse } from '@/types'

// BUG-WJSYQT: the list endpoint refuses a sort or filter on a property the
// reader cannot see on every row. A configured list's default_sort and
// group_by reach it as `sort`, so a refused sort is read again unsorted
// rather than turning the whole list into an error. A refused filter stands.

function refusal(use: 'sort' | 'filter'): ApiError {
  const detail = `property not visible: cannot ${use} on "secret" of "ticket", because it is hidden from you on some or all rows`
  return new ApiError(detail, {
    kind: 'http',
    status: 400,
    problem: {
      type: 'https://rela.dev/errors/property_not_visible',
      title: 'Cannot filter or sort on this property',
      status: 400,
      detail,
      use,
      property: 'secret',
    },
    original: null,
  })
}

const rows: ListResponse<string> = {
  data: ['T-1'],
  meta: { total: 1, page: 1, per_page: 25, has_more: false },
}

describe('withSortFallback', () => {
  it('returns the read unchanged when the server accepts it', async () => {
    const read = vi.fn().mockResolvedValue(rows)
    const res = await withSortFallback({ sort: 'title' }, read)
    expect(read).toHaveBeenCalledTimes(1)
    expect(res.sortDropped).toBeUndefined()
  })

  it('reads again without the sort when the sort is refused', async () => {
    const refused = refusal('sort')
    const read = vi.fn().mockRejectedValueOnce(refused).mockResolvedValueOnce(rows)
    const params: ListParams = { sort: 'secret,-title', list_id: 'tickets', per_page: 25 }

    const res = await withSortFallback(params, read)

    expect(read).toHaveBeenCalledTimes(2)
    expect(read.mock.calls[1][0]).toEqual({ list_id: 'tickets', per_page: 25 })
    expect(res.data).toEqual(rows.data)
    expect(res.sortDropped).toEqual({ sort: params.sort, reason: refused.message })
  })

  it('keeps a refused filter as an error, since dropping it would widen the list', async () => {
    const refused = refusal('filter')
    const read = vi.fn().mockRejectedValue(refused)

    await expect(
      withSortFallback({ sort: 'title', 'filter[secret]': 'x' } as ListParams, read)
    ).rejects.toBe(refused)
    // The problem body says the filter was refused, so no unsorted retry.
    expect(read).toHaveBeenCalledTimes(1)
  })

  it('does not retry a refusal that does not say it was the sort', async () => {
    const refused = refusal('sort')
    delete (refused.problem as { use?: string }).use
    const read = vi.fn().mockRejectedValue(refused)

    await expect(withSortFallback({ sort: 'secret' }, read)).rejects.toBe(refused)
    expect(read).toHaveBeenCalledTimes(1)
  })

  it('does not retry a read without a sort, or another error', async () => {
    const unsorted = vi.fn().mockRejectedValue(refusal('filter'))
    await expect(withSortFallback({}, unsorted)).rejects.toBeInstanceOf(ApiError)
    expect(unsorted).toHaveBeenCalledTimes(1)

    const other = new ApiError('boom', { kind: 'http', status: 500, original: null })
    const failing = vi.fn().mockRejectedValue(other)
    await expect(withSortFallback({ sort: 'title' }, failing)).rejects.toBe(other)
    expect(failing).toHaveBeenCalledTimes(1)
  })
})

describe('isPropertyNotVisible', () => {
  it('recognizes only the property_not_visible 400', () => {
    expect(isPropertyNotVisible(refusal('sort'))).toBe(true)
    expect(
      isPropertyNotVisible(new ApiError('x', { kind: 'http', status: 400, original: null }))
    ).toBe(false)
    expect(isPropertyNotVisible(new Error('x'))).toBe(false)
  })
})
