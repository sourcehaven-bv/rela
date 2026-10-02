import { describe, expect, it } from 'vitest'
import { groupedSort, listBaseParams, sortParam } from './listParams'
import type { ListConfig } from '@/types'

describe('listBaseParams', () => {
  it('carries the list id, static filters, default sort and scope', () => {
    const config = {
      entity: 'ticket',
      columns: [],
      page_size: 10,
      query_scope: 'open',
      filters: [
        { property: 'status', operator: '=', value: 'todo' },
        { property: 'status', operator: '=', value: 'doing' },
        { property: 'owner', operator: '', value: 'x' },
      ],
      default_sort: [{ property: 'due', direction: 'desc' }],
    } as unknown as ListConfig

    expect(listBaseParams('open_tickets', config)).toEqual({
      list_id: 'open_tickets',
      per_page: 10,
      'filter[status][eq]': 'todo,doing',
      sort: '-due',
      query_scope: 'open',
    })
  })

  it('defaults the page size and omits what the config does not set', () => {
    expect(listBaseParams('all', { entity: 'ticket', columns: [] } as ListConfig)).toEqual({
      list_id: 'all',
      per_page: 25,
    })
  })
})

describe('sortParam', () => {
  it('prefixes a descending key with a minus', () => {
    expect(
      sortParam([
        { property: 'a', direction: 'asc' },
        { property: 'b', direction: 'desc' },
      ])
    ).toBe('a,-b')
  })
})

describe('groupedSort', () => {
  it('sorts by the group property first, so each section arrives together', () => {
    expect(
      groupedSort({ property: 'status', max_rows: 500 }, [
        { property: 'due', direction: 'desc' },
        { property: 'status', direction: 'desc' },
      ])
    ).toEqual([
      { property: 'status', direction: 'asc' },
      { property: 'due', direction: 'desc' },
    ])
  })

  it('leaves the sort alone for a flat list', () => {
    const specs = [{ property: 'due', direction: 'desc' as const }]
    expect(groupedSort(undefined, specs)).toBe(specs)
  })

  it('reaches listBaseParams', () => {
    const config = {
      entity: 'ticket',
      columns: [],
      group_by: { property: 'status', max_rows: 500 },
      default_sort: [{ property: 'due', direction: 'desc' }],
    } as unknown as ListConfig
    expect(listBaseParams('open', config).sort).toBe('status,-due')
  })
})
