import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import CommentsPanel from './CommentsPanel.vue'
import { useSchemaStore } from '@/stores/schema'
import type { ViewResponse } from '@/api'
import { listComments, type Comment } from '@/api/comments'

// The page keeps one EntityDetail across entity switches, so the comment
// thread asked for the entity the user left can answer after they moved on.
// It must not land on the entity now on screen (TKT-7LTLHW3).

const fetchViewMock = vi.fn()

vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: vi.fn().mockResolvedValue([]),
}))
vi.mock('@/api/comments', async (orig) => ({
  ...(await orig<typeof import('@/api/comments')>()),
  listComments: vi.fn(),
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/entity/ticket/TKT-1', name: 'entity' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

function view(id: string): ViewResponse {
  return {
    entry: {
      id,
      type: 'ticket',
      _title: id,
      properties: { title: id },
      content: 'Body.',
      _actions: { update: true },
    },
    sections: [],
  }
}

const onLeft: Comment = {
  id: 'c1',
  author: 'alice',
  created_at: '2026-09-01T10:00:00Z',
  anchor: { kind: 'property', ref: 'title' },
  body: 'About TKT-1.',
  resolved: false,
  editable: true,
  deletable: true,
  acceptable: false,
}

describe('EntityDetail comments across an entity switch', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    useSchemaStore().entityTypes.set('ticket', {
      name: 'ticket',
      label: 'Ticket',
      properties: { title: { type: 'string' } },
      commentable: true,
    } as never)
    fetchViewMock
      .mockReset()
      .mockImplementation((_t: string, id: string) => Promise.resolve(view(id)))
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it.each([
    ['a thread', (late: Late) => late.resolve([onLeft])],
    ['a failure', (late: Late) => late.reject(new Error('gone'))],
  ] as const)('ignores %s for the entity the page has left', async (_name, settle) => {
    const mine: Comment = { ...onLeft, id: 'c2', body: 'About TKT-2.' }
    let late!: Late
    vi.mocked(listComments)
      .mockReset()
      .mockImplementationOnce(() => new Promise((resolve, reject) => (late = { resolve, reject })))
      .mockResolvedValue([mine])

    const w = mount(EntityDetail, {
      props: { entityType: 'ticket', entityId: 'TKT-1' },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    await w.setProps({ entityId: 'TKT-2' })
    await flushPromises()
    expect(w.findComponent(CommentsPanel).props('comments')).toEqual([mine])

    settle(late)
    await flushPromises()
    expect(w.findComponent(CommentsPanel).props('comments')).toEqual([mine])
  })
})

interface Late {
  resolve: (v: Comment[]) => void
  reject: (e: Error) => void
}
