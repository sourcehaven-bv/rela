import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { useScopeNavigation } from './useScopeNavigation'
import type { EntityPosition, ScopeDescriptor } from '@/api/entities'

// Mock vue-router
const mockRouteQuery = ref<Record<string, string>>({})
const mockPush = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: mockRouteQuery.value,
  }),
  useRouter: () => ({
    push: mockPush,
  }),
}))

// Mock schema store
const mockSchemaStore = {
  getList: vi.fn(),
}

vi.mock('@/stores', () => ({
  useSchemaStore: () => mockSchemaStore,
}))

// Mock the position API — the composable now asks the server for position
// rather than fetching the whole list and scanning client-side (#844).
const mockGetEntityPosition = vi.fn()
vi.mock('@/api/entities', () => ({
  getEntityPosition: (id: string, scope: ScopeDescriptor) => mockGetEntityPosition(id, scope),
}))

/**
 * positionFromIds derives the {prev, next, current, total} the server would
 * return for `currentId` within an ordered `ids` array — letting the existing
 * list-based test cases drive the new position-based composable. A currentId
 * not in `ids` rejects with a 404-shaped error, matching the backend.
 */
function positionFromIds(ids: string[], currentId: string): Promise<EntityPosition> {
  const idx = ids.indexOf(currentId)
  if (idx === -1) {
    return Promise.reject(new Error('not_in_scope'))
  }
  return Promise.resolve({
    prev: idx > 0 ? ids[idx - 1] : null,
    next: idx < ids.length - 1 ? ids[idx + 1] : null,
    current: idx + 1,
    total: ids.length,
  })
}

/** mockPositionForList wires getEntityPosition to derive from a fixed id set. */
function mockPositionForList(ids: string[]) {
  mockGetEntityPosition.mockImplementation((id: string) => positionFromIds(ids, id))
}

describe('useScopeNavigation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockRouteQuery.value = {}
    mockPush.mockClear()
    mockSchemaStore.getList.mockReturnValue(null)
    mockGetEntityPosition.mockResolvedValue({ prev: null, next: null, current: 1, total: 1 })
  })

  describe('loadScopeNav', () => {
    it('sets scopeNav to null when no from query param', async () => {
      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(scopeNav.value).toBeNull()
      expect(mockGetEntityPosition).not.toHaveBeenCalled()
    })

    it('sets scopeNav to null when list config not found', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue(null)

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(scopeNav.value).toBeNull()
    })

    it('sets scopeNav to null when entity not in scope (server 404)', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task', title: 'Tasks' })
      mockPositionForList(['TASK-002', 'TASK-003'])

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(scopeNav.value).toBeNull()
    })

    it('calculates scope navigation for entity in list', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task', title: 'Tasks' })
      mockPositionForList(['TASK-001', 'TASK-002', 'TASK-003'])

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-002'
      )

      await loadScopeNav()

      expect(scopeNav.value).toEqual({
        prevId: 'TASK-001',
        nextId: 'TASK-003',
        current: 2,
        total: 3,
        label: 'Tasks',
      })
    })

    it('sets prevId to null for first item', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task', title: 'Tasks' })
      mockPositionForList(['TASK-001', 'TASK-002'])

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(scopeNav.value?.prevId).toBeNull()
      expect(scopeNav.value?.nextId).toBe('TASK-002')
    })

    it('sets nextId to null for last item', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task', title: 'Tasks' })
      mockPositionForList(['TASK-001', 'TASK-002'])

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-002'
      )

      await loadScopeNav()

      expect(scopeNav.value?.prevId).toBe('TASK-001')
      expect(scopeNav.value?.nextId).toBeNull()
    })

    it('uses list ID as label when title not provided', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' }) // No title
      mockPositionForList(['TASK-001'])

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(scopeNav.value?.label).toBe('tasks')
    })

    it('applies default sort from list config', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({
        entity: 'task',
        default_sort: [
          { property: 'created_at', direction: 'desc' },
          { property: 'title', direction: 'asc' },
        ],
      })
      mockPositionForList(['TASK-001'])

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockGetEntityPosition).toHaveBeenCalledWith(
        'TASK-001',
        expect.objectContaining({ source: 'list', type: 'task', sort: '-created_at,title' })
      )
    })

    it('applies sort from query params over default', async () => {
      mockRouteQuery.value = { from: 'tasks', sort: 'priority' }
      mockSchemaStore.getList.mockReturnValue({
        entity: 'task',
        default_sort: [{ property: 'created_at', direction: 'desc' }],
      })
      mockPositionForList(['TASK-001'])

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockGetEntityPosition).toHaveBeenCalledWith(
        'TASK-001',
        expect.objectContaining({ sort: 'priority' })
      )
    })

    it('applies list config filters', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({
        entity: 'task',
        filters: [{ property: 'status', operator: '=', value: 'open' }],
      })
      mockPositionForList(['TASK-001'])

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockGetEntityPosition).toHaveBeenCalledWith(
        'TASK-001',
        expect.objectContaining({
          filters: expect.objectContaining({ 'filter[status][eq]': 'open' }),
        })
      )
    })

    it('applies user filters from query (bracket format)', async () => {
      mockRouteQuery.value = { from: 'tasks', 'filter[priority]': 'high' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockPositionForList(['TASK-001'])

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockGetEntityPosition).toHaveBeenCalledWith(
        'TASK-001',
        expect.objectContaining({
          filters: expect.objectContaining({ 'filter[priority]': 'high' }),
        })
      )
    })

    it('applies user filters with non-default operator', async () => {
      mockRouteQuery.value = { from: 'tasks', 'filter[due_date][lte]': '$today' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockPositionForList(['TASK-001'])

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockGetEntityPosition).toHaveBeenCalledWith(
        'TASK-001',
        expect.objectContaining({
          filters: expect.objectContaining({ 'filter[due_date][lte]': '$today' }),
        })
      )
    })

    it('honors free-text q within a list scope (source=search)', async () => {
      mockRouteQuery.value = { from: 'tasks', q: 'urgent' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockPositionForList(['TASK-001'])

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockGetEntityPosition).toHaveBeenCalledWith(
        'TASK-001',
        expect.objectContaining({ source: 'search', q: 'urgent' })
      )
    })

    it('builds a search-origin scope from from=search (no list config)', async () => {
      // from=search is the dedicated search origin: no getList lookup, q is the
      // full search query, navigation can span types. getList must NOT be
      // consulted.
      mockRouteQuery.value = { from: 'search', q: 'type:ticket auth' }
      mockPositionForList(['TASK-001'])

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockSchemaStore.getList).not.toHaveBeenCalled()
      expect(mockGetEntityPosition).toHaveBeenCalledWith(
        'TASK-001',
        expect.objectContaining({ source: 'search', q: 'type:ticket auth' })
      )
      expect(scopeNav.value?.label).toBe('Search: type:ticket auth')
    })

    it('search origin with no q yields no scope nav', async () => {
      mockRouteQuery.value = { from: 'search' }

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(scopeNav.value).toBeNull()
      expect(mockGetEntityPosition).not.toHaveBeenCalled()
    })

    it('handles fetch errors gracefully', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockGetEntityPosition.mockRejectedValue(new Error('Network error'))

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(scopeNav.value).toBeNull()
    })
  })

  describe('navigateScope', () => {
    it('navigates to previous entity', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockPositionForList(['TASK-001', 'TASK-002'])

      const { loadScopeNav, navigateScope } = useScopeNavigation(
        () => 'task',
        () => 'TASK-002'
      )

      await loadScopeNav()
      navigateScope('prev')

      expect(mockPush).toHaveBeenCalledWith({
        path: '/entity/task/TASK-001',
        query: { from: 'tasks' },
      })
    })

    it('navigates to next entity', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockPositionForList(['TASK-001', 'TASK-002'])

      const { loadScopeNav, navigateScope } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()
      navigateScope('next')

      expect(mockPush).toHaveBeenCalledWith({
        path: '/entity/task/TASK-002',
        query: { from: 'tasks' },
      })
    })

    it('does nothing when no scopeNav', () => {
      const { navigateScope } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      navigateScope('prev')

      expect(mockPush).not.toHaveBeenCalled()
    })

    it('does nothing when no prev/next available', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockPositionForList(['TASK-001'])

      const { loadScopeNav, navigateScope } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()
      navigateScope('prev')

      expect(mockPush).not.toHaveBeenCalled()
    })
  })

  describe('navigateScope preserves query params', () => {
    // TKT-JIEKC RR-97NAZ: navigating through a list via Prev/Next must
    // preserve other query params — crucially `return_to`, so the Back
    // button still points at the original source across in-list
    // navigation. The composable achieves this by passing `query:
    // route.query` (all keys) through to router.push; this test pins
    // the behaviour so a future 'clean up query on nav' commit doesn't
    // silently break it.
    it('preserves return_to and other query keys on prev/next push', async () => {
      mockRouteQuery.value = {
        from: 'tasks',
        return_to: '/document/release_notes/REL-1',
        sort: '-priority',
      }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockPositionForList(['TASK-001', 'TASK-002'])

      const { loadScopeNav, navigateScope } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()
      navigateScope('next')

      expect(mockPush).toHaveBeenCalledWith({
        path: '/entity/task/TASK-002',
        query: {
          from: 'tasks',
          return_to: '/document/release_notes/REL-1',
          sort: '-priority',
        },
      })
    })
  })
})
