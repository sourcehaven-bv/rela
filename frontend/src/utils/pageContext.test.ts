import { describe, it, expect } from 'vitest'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { currentPageTab, fromPageQuery, pageTabPath, readFromPage } from './pageContext'

function route(name: string, params: Record<string, string>): RouteLocationNormalizedLoaded {
  return { name, params } as unknown as RouteLocationNormalizedLoaded
}

describe('pageContext', () => {
  it('names the page tab the route shows', () => {
    expect(currentPageTab(route('page', { page: 'tickets', tab: 'table' }))).toEqual({
      page: 'tickets',
      tab: 'table',
    })
    expect(fromPageQuery(route('page', { page: 'tickets', tab: 'table' }))).toEqual({
      from_page: 'tickets/table',
    })
  })

  it('adds nothing outside a page tab', () => {
    expect(fromPageQuery(route('list', { id: 'tickets' }))).toEqual({})
    expect(fromPageQuery(route('page', { page: 'tickets', tab: '' }))).toEqual({})
  })

  it('reads only a page id and a tab id back', () => {
    expect(readFromPage({ from_page: 'tickets/table' })).toEqual({ page: 'tickets', tab: 'table' })
    for (const bad of ['//evil.com/x', '/p/tickets/table', 'tickets', 'a/b/c/d', 'A/b', 'a/b.c/d', 'a/-b/c']) {
      expect(readFromPage({ from_page: bad })).toBeNull()
    }
    expect(readFromPage({ from_page: ['tickets/table'] })).toBeNull()
  })

  it('builds the tab path', () => {
    expect(pageTabPath({ page: 'tickets', tab: 'board' })).toBe('/p/tickets/board')
    expect(pageTabPath({ page: 'topic', entity: 'TOP-1', tab: 'board' })).toBe('/p/topic/TOP-1/board')
  })

  it('carries the entity of an entity page', () => {
    const r = route('entity-page', { page: 'topic', entity: 'TOP-1', tab: 'board' })
    expect(currentPageTab(r)).toEqual({ page: 'topic', entity: 'TOP-1', tab: 'board' })
    expect(fromPageQuery(r)).toEqual({ from_page: 'topic/TOP-1/board' })
    expect(readFromPage({ from_page: 'topic/TOP-1/board' })).toEqual({ page: 'topic', entity: 'TOP-1', tab: 'board' })
    expect(fromPageQuery(route('entity-page', { page: 'topic', entity: '', tab: 'board' }))).toEqual({})
  })
})
