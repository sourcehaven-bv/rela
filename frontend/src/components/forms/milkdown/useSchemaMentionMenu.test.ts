import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { getEntity, listRecentlyModified, searchEntities } from '@/api'
import { useSchemaMentionMenu, type MentionSelf } from './useMentionMenu'

vi.mock('@/api', () => ({
  ApiError: class extends Error {},
  searchEntities: vi.fn(async () => ({ data: [] })),
  getEntity: vi.fn(async () => ({ id: 'X', type: 'page', properties: {}, included: {} })),
  listRecentlyModified: vi.fn(async () => ({ data: [] })),
}))

const pageWorld = ref<string | undefined>('site-nl')
vi.mock('@/composables/useWorld', () => ({
  useWorld: () => ({ worldParam: pageWorld }),
}))

// Two sites, each led by its own language.
const heads: Record<string, string> = { nl: 'site-nl', en: 'site-en' }
vi.mock('@/stores/schema', () => ({
  useSchemaStore: () => ({
    entityTypeList: [['page', {}]],
    worldForFace: (_type: string, face: string) => (face ? heads[face] : ''),
  }),
}))

vi.mock('@/utils/recentEntities', () => ({
  listRecentEntities: () => [{ id: 'PAGE-R', type: 'page' }],
}))

/** Opens the menu on a bare `@`, then searches, and lets both settle. */
async function exercise(self: MentionSelf | null): Promise<void> {
  const menu = useSchemaMentionMenu(() => self)
  menu.setQuery('')
  await vi.waitFor(() => expect(listRecentlyModified).toHaveBeenCalled())
  menu.setQuery('page:abc')
  await vi.waitFor(() => expect(searchEntities).toHaveBeenCalled())
  menu.dispose()
}

describe('useSchemaMentionMenu world', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    pageWorld.value = 'site-nl'
  })

  it('reads every source in the world that heads the edited face', async () => {
    await exercise({ id: 'PAGE-1', type: 'page', face: 'en' })
    expect(searchEntities).toHaveBeenCalledWith('abc', {
      type: 'page',
      signal: expect.anything(),
      world: 'site-en',
      limit: 100,
    })
    expect(listRecentlyModified).toHaveBeenCalledWith(
      ['page'],
      expect.any(Number),
      undefined,
      'site-en'
    )
    // Related: the edited face by its address, neighbours in its world.
    expect(getEntity).toHaveBeenCalledWith('page', 'PAGE-1@en', { include: '*', world: 'site-en' })
    // Recently viewed: reloaded in the same world.
    expect(getEntity).toHaveBeenCalledWith('page', 'PAGE-R', { world: 'site-en' })
  })

  it('falls back to the page world for an entity without a face', async () => {
    await exercise({ id: 'PAGE-1', type: 'page', face: '' })
    expect(searchEntities).toHaveBeenCalledWith('abc', {
      type: 'page',
      signal: expect.anything(),
      world: 'site-nl',
      limit: 100,
    })
    expect(getEntity).toHaveBeenCalledWith('page', 'PAGE-1', { include: '*', world: 'site-nl' })
  })

  it('sends no world when the page has none', async () => {
    pageWorld.value = undefined
    await exercise(null)
    expect(searchEntities).toHaveBeenCalledWith('abc', {
      type: 'page',
      signal: expect.anything(),
      world: undefined,
      limit: 100,
    })
    expect(getEntity).toHaveBeenCalledWith('page', 'PAGE-R', undefined)
  })
})
