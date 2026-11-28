import { describe, it, expect } from 'vitest'
import type { ListConfig } from '@/types/config'
import type { ListParams } from '@/types/entity'
import { entityKeys } from '@/queries/entities'
import { isListQueryOf, listsRefreshedBy } from './listRefresh'

const list = (entity: string, refresh_types?: string[]): ListConfig => {
  const config: ListConfig = { entity, columns: [], refresh_types }
  return config
}

describe('listsRefreshedBy', () => {
  const lists: [string, ListConfig][] = [
    ['stalled', list('project', ['action', 'project', 'tag'])],
    ['available', list('action', ['action', 'project', 'tag'])],
    ['all_projects', list('project')],
    ['notes', list('note')],
  ]

  it('names each list of another type whose roll-ups read the changed type', () => {
    expect([...listsRefreshedBy(lists, 'tag')]).toEqual([
      ['stalled', 'project'],
      ['available', 'action'],
    ])
  })

  it('leaves out lists of the changed type itself, which the type key already covers', () => {
    expect([...listsRefreshedBy(lists, 'action')]).toEqual([['stalled', 'project']])
  })

  it('names none for a type no roll-up reads', () => {
    expect(listsRefreshedBy(lists, 'note').size).toBe(0)
  })
})

describe('isListQueryOf', () => {
  const ids = new Map([['stalled', 'project']])
  const params = (listId: string): ListParams => ({ list_id: listId, per_page: 25 })

  it('matches a page and a whole-collection read of the declaring list', () => {
    expect(isListQueryOf(entityKeys.listParams('project', params('stalled')), ids)).toBe(true)
    expect(isListQueryOf(entityKeys.listAll('project', params('stalled'), 500), ids)).toBe(true)
  })

  it('leaves other lists, kanbans and details of the same type alone', () => {
    expect(isListQueryOf(entityKeys.listParams('project', params('all_projects')), ids)).toBe(false)
    expect(isListQueryOf(entityKeys.list('project'), ids)).toBe(false)
    expect(isListQueryOf(entityKeys.listParams('project'), ids)).toBe(false)
    expect(isListQueryOf(entityKeys.detail('project', 'PRJ-1'), ids)).toBe(false)
  })

  it('does not match the list id under another type', () => {
    expect(isListQueryOf(entityKeys.listParams('action', params('stalled')), ids)).toBe(false)
  })
})
