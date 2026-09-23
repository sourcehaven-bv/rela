import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import SidebarEntityQuery, { type SidebarEntityRow } from './SidebarEntityQuery.vue'
import { _setEntityPluralForTest } from '@/api/entities'
import { ApiError } from '@/api/errors'
import { useQueryCache } from '@pinia/colada'
import type { Entity, ListResponse, SidebarEntities } from '@/types'

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

const mockRoute = { query: {} as Record<string, string>, path: '/' }
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => mockRoute,
}))

function response(entities: Entity[], total = entities.length): ListResponse<Entity> {
  return {
    data: entities,
    meta: { total, page: 1, per_page: 100, has_more: total > entities.length },
    included: {},
  }
}

interface SlotProps {
  rows: SidebarEntityRow[]
  overflow: number
  failed: boolean
}

/**
 * Mounts the renderless component and captures what it hands its slot, since
 * the slot is its whole output.
 */
async function mountQuery(entities: SidebarEntities) {
  let last: SlotProps = { rows: [], overflow: 0, failed: false }
  const pinia = createPinia()
  setActivePinia(pinia)
  const wrapper = mount(
    defineComponent({
      setup() {
        return () =>
          h(
            SidebarEntityQuery,
            { entities },
            {
              default: (p: SlotProps) => {
                last = p
                return h('div')
              },
            }
          )
      },
    }),
    { global: { plugins: [pinia, PiniaColada] } }
  )
  await flushPromises()
  return { wrapper, slot: () => last, inner: () => wrapper.findComponent(SidebarEntityQuery) }
}

describe('SidebarEntityQuery', () => {
  beforeEach(() => {
    listEntitiesMock.mockReset()
    mockRoute.query = {}
    _setEntityPluralForTest('project', 'projects')
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('requests one page with the served scope and sort', async () => {
    listEntitiesMock.mockResolvedValue(response([]))
    await mountQuery({ type: 'project', query_scope: 'active', sort: '-title' })

    expect(listEntitiesMock).toHaveBeenCalledWith(
      'project',
      { per_page: 100, query_scope: 'active', sort: '-title' },
      expect.anything()
    )
  })

  it('omits scope and sort when the entry has none', async () => {
    listEntitiesMock.mockResolvedValue(response([]))
    await mountQuery({ type: 'project' })

    expect(listEntitiesMock.mock.calls[0][1]).toEqual({ per_page: 100 })
  })

  it('carries the world on the request and on each link', async () => {
    mockRoute.query = { world: 'draft' }
    listEntitiesMock.mockResolvedValue(
      response([{ id: 'PRJ-1', type: 'project', properties: {}, _title: 'Apollo' } as Entity])
    )
    const { slot } = await mountQuery({ type: 'project' })

    expect(listEntitiesMock.mock.calls[0][1]).toMatchObject({ world: 'draft' })
    expect(slot().rows[0].to).toMatchObject({ query: { world: 'draft' } })
  })

  it('labels rows with the display title and falls back to the id', async () => {
    listEntitiesMock.mockResolvedValue(
      response([
        { id: 'PRJ-1', type: 'project', properties: {}, _title: 'Apollo' } as Entity,
        { id: 'PRJ-2', type: 'project', properties: {} } as Entity,
      ])
    )
    const { slot } = await mountQuery({ type: 'project' })

    expect(slot().rows.map((r) => r.title)).toEqual(['Apollo', 'PRJ-2'])
    expect(slot().rows[0].path).toContain('PRJ-1')
    expect(slot().failed).toBe(false)
  })

  it('reports rows beyond the served page as overflow', async () => {
    listEntitiesMock.mockResolvedValue(
      response([{ id: 'PRJ-1', type: 'project', properties: {} } as Entity], 130)
    )
    const { slot } = await mountQuery({ type: 'project' })

    expect(slot().overflow).toBe(129)
  })

  it('emits shown(false) when nothing matched and shown(true) otherwise', async () => {
    listEntitiesMock.mockResolvedValue(response([]))
    const none = await mountQuery({ type: 'project' })
    expect(none.inner().emitted('shown')?.slice(-1)[0]).toEqual([false])

    listEntitiesMock.mockResolvedValue(
      response([{ id: 'PRJ-1', type: 'project', properties: {} } as Entity])
    )
    const some = await mountQuery({ type: 'project' })
    expect(some.inner().emitted('shown')?.slice(-1)[0]).toEqual([true])
  })

  it('flags a failed load instead of reporting it as empty', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    listEntitiesMock.mockRejectedValue(new Error('boom'))
    const { slot, inner } = await mountQuery({ type: 'project', query_scope: 'gone' })

    expect(slot().failed).toBe(true)
    expect(slot().rows).toEqual([])
    // A failure must not hide the group as if nothing matched.
    expect(inner().emitted('shown')?.slice(-1)[0]).toEqual([true])
  })

  describe('a refetch that fails after a success', () => {
    const held = response([
      { id: 'PRJ-1', type: 'project', properties: {}, _title: 'Apollo' } as Entity,
    ])

    function refusal(status: number): ApiError {
      return new ApiError(`Request failed (${status})`, {
        kind: 'http',
        status,
        original: new Error('denied'),
      })
    }

    async function refetchFailing(err: unknown) {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      listEntitiesMock.mockResolvedValue(held)
      const q = await mountQuery({ type: 'project' })
      expect(q.slot().rows.map((r) => r.title)).toEqual(['Apollo'])

      listEntitiesMock.mockRejectedValue(err)
      await useQueryCache()
        .invalidateQueries({ key: ['entities'] })
        .catch(() => {})
      await flushPromises()
      return q
    }

    it.each([401, 403, 404])('drops the held rows on a %i refusal', async (status) => {
      const { slot } = await refetchFailing(refusal(status))
      expect(slot().rows).toEqual([])
      expect(slot().overflow).toBe(0)
      expect(slot().failed).toBe(true)
    })

    it('keeps the held rows on a transient failure', async () => {
      const { slot } = await refetchFailing(refusal(500))
      expect(slot().rows.map((r) => r.title)).toEqual(['Apollo'])
      expect(slot().failed).toBe(false)
    })
  })
})
