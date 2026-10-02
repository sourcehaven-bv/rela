import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { spaceOf, stripSpace, useSpaceStore, withSpace } from './space'

describe('spaceOf', () => {
  it.each([
    ['/s/crm/list/deals', 'crm'],
    ['/s/crm', 'crm'],
    ['/s/crm/', 'crm'],
    ['/list/deals', undefined],
    ['/s/', undefined],
    ['/s/CRM/list', undefined],
    ['/sx/crm', undefined],
  ])('%s -> %s', (path, want) => {
    expect(spaceOf(path)).toBe(want)
  })
})

describe('stripSpace', () => {
  it.each([
    ['/s/crm/list/deals', '/list/deals'],
    ['/s/crm', '/'],
    ['/list/deals', '/list/deals'],
  ])('%s -> %s', (path, want) => {
    expect(stripSpace(path)).toBe(want)
  })
})

describe('withSpace', () => {
  it.each([
    ['/list/deals', 'crm', '/s/crm/list/deals'],
    ['/', 'crm', '/s/crm'],
    ['/s/isms/list', 'crm', '/s/isms/list'],
    ['/api/v1/x', 'crm', '/api/v1/x'],
    ['https://example.com/', 'crm', 'https://example.com/'],
    ['//evil.example/', 'crm', '//evil.example/'],
    ['/list/deals', undefined, '/list/deals'],
  ])('%s in %s -> %s', (path, space, want) => {
    expect(withSpace(path, space)).toBe(want)
  })

  it('round-trips through stripSpace', () => {
    expect(stripSpace(withSpace('/entity/deal/D-1', 'crm'))).toBe('/entity/deal/D-1')
  })
})

describe('useSpaceStore', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('leaves paths alone without spaces', () => {
    const store = useSpaceStore()
    store.set({})
    expect(store.enabled).toBe(false)
    expect(store.href('/list/deals')).toBe('/list/deals')
  })

  it('prefixes paths with the current space', () => {
    const store = useSpaceStore()
    store.set({
      spaces: [
        { id: 'crm', label: 'CRM', home: '/list/deals' },
        { id: 'isms', label: 'ISMS', home: '/dashboard' },
      ],
      space: 'isms',
      create: [{ type: 'risk', label: 'Risk', form: 'risk' }],
    })
    expect(store.enabled).toBe(true)
    expect(store.currentSpace?.label).toBe('ISMS')
    expect(store.href('/list/risks')).toBe('/s/isms/list/risks')
    expect(store.create).toHaveLength(1)
  })
})
