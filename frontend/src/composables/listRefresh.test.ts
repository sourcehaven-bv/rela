import { describe, it, expect } from 'vitest'
import type { ListConfig } from '@/types/config'
import { listTypesRefreshedBy } from './listRefresh'

const list = (entity: string, refresh_types?: string[]): ListConfig => {
  const config: ListConfig = { entity, columns: [], refresh_types }
  return config
}

describe('listTypesRefreshedBy', () => {
  const lists = [
    list('project', ['action', 'project', 'tag']),
    list('action', ['action', 'project', 'tag']),
    list('project', ['action', 'project', 'tag']),
    list('note'),
  ]

  it('names each other list type whose roll-ups read the changed type', () => {
    expect(listTypesRefreshedBy(lists, 'tag')).toEqual(['action', 'project'])
  })

  it('leaves out the changed type itself, which the type key already covers', () => {
    expect(listTypesRefreshedBy(lists, 'action')).toEqual(['project'])
  })

  it('names none for a type no roll-up reads', () => {
    expect(listTypesRefreshedBy(lists, 'note')).toEqual([])
  })
})
