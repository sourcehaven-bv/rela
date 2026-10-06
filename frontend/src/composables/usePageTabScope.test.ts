import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { pageCreateLink, usePageTabScope } from './usePageTabScope'
import { usePageStore } from '@/stores/pages'
import { useUIStore } from '@/stores/ui'
import type { Entity, PageScope, SidebarPage, SidebarPageLink } from '@/types'

const createRelationMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  createRelation: (...args: unknown[]) => createRelationMock(...args),
}))

const created = { id: 'TAAK-9', type: 'taak', properties: {} } as Entity

function seedPage(direction: 'outgoing' | 'incoming') {
  usePageStore().set({
    topic: {
      label: 'Topic',
      entity_type: 'topic',
      tabs: [{
        id: 'tabel', label: 'Tabel', view: 'list', target: 'taken', scope: 'relation', relation: 'bestaat_uit', direction,
        links: [{ type: 'taak', relation: 'bestaat_uit', direction }],
      }],
    },
  })
}

const scope: PageScope = { page: 'topic', tab: 'tabel', entity: 'TOP-1' }

beforeEach(() => {
  setActivePinia(createPinia())
  createRelationMock.mockReset()
})

describe('usePageTabScope', () => {
  it('narrows the read to the page, tab and anchor', () => {
    seedPage('outgoing')
    const { params } = usePageTabScope(() => scope)
    expect(params.value).toEqual({ scope_page: 'topic', scope_tab: 'tabel', anchor: 'TOP-1' })
  })

  it('is inert outside an entity page', async () => {
    seedPage('outgoing')
    const { params, linkCreated } = usePageTabScope(() => undefined)
    expect(params.value).toEqual({})
    await linkCreated(created)
    expect(createRelationMock).not.toHaveBeenCalled()
  })

  it.each(['outgoing', 'incoming'] as const)('links a new row from the anchor, %s', async (direction) => {
    seedPage(direction)
    createRelationMock.mockResolvedValue(undefined)
    await usePageTabScope(() => scope).linkCreated(created)
    expect(createRelationMock).toHaveBeenCalledWith('topic', 'TOP-1', 'bestaat_uit', 'TAAK-9', undefined, direction)
  })

  it('links from the face on screen, and narrows by the bare anchor', async () => {
    seedPage('outgoing')
    createRelationMock.mockResolvedValue(undefined)
    const faced: PageScope = { ...scope, ref: 'TOP-1@draft' }
    const { params, linkCreated } = usePageTabScope(() => faced)
    expect(params.value).toEqual({ scope_page: 'topic', scope_tab: 'tabel', anchor: 'TOP-1' })
    await linkCreated(created)
    expect(createRelationMock).toHaveBeenCalledWith('topic', 'TOP-1@draft', 'bestaat_uit', 'TAAK-9', undefined, 'outgoing')
  })

  it('names the row to link by hand when the link fails', async () => {
    seedPage('outgoing')
    createRelationMock.mockRejectedValue(new Error('forbidden'))
    const errorSpy = vi.spyOn(useUIStore(), 'error')
    await usePageTabScope(() => scope).linkCreated(created)
    expect(errorSpy).toHaveBeenCalledWith(expect.stringContaining('TAAK-9 was created'))
  })
})

describe('pageCreateLink', () => {
  const contains: SidebarPageLink = { type: 'task', relation: 'contains', direction: 'outgoing' }
  const blocks: SidebarPageLink = { type: 'task', relation: 'blocks', direction: 'incoming' }
  const page = (...tabLinks: SidebarPageLink[][]): SidebarPage => ({
    label: 'Topic',
    entity_type: 'topic',
    tabs: tabLinks.map((links, i) => ({ id: `t${i}`, label: `T${i}`, view: 'list', links })),
  })

  it.each([
    {
      name: 'the open tab wins over another tab',
      p: page([blocks], [contains]),
      tab: 't1',
      want: contains,
    },
    {
      name: 'the page link serves a tab without one',
      p: page([contains], []),
      tab: 't1',
      want: contains,
    },
    {
      name: 'the same link on two tabs is one link',
      p: page([contains], [contains], []),
      tab: 't2',
      want: contains,
    },
    {
      name: 'two relations on the page link nothing',
      p: page([contains], [blocks], []),
      tab: 't2',
      want: undefined,
    },
    {
      name: 'two relations on the open tab link nothing',
      p: page([contains, blocks]),
      tab: 't0',
      want: undefined,
    },
    {
      name: 'another type links nothing',
      p: page([{ ...contains, type: 'note' }]),
      tab: 't0',
      want: undefined,
    },
  ])('$name', ({ p, tab, want }) => {
    expect(pageCreateLink(p, tab, 'task')).toEqual(want)
  })
})
