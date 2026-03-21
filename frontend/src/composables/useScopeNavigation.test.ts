import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { useScopeNavigation } from './useScopeNavigation'

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

// Mock stores
const mockSchemaStore = {
  getList: vi.fn(),
}

const mockEntitiesStore = {
  fetchList: vi.fn(),
}

vi.mock('@/stores', () => ({
  useSchemaStore: () => mockSchemaStore,
  useEntitiesStore: () => mockEntitiesStore,
}))

describe('useScopeNavigation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockRouteQuery.value = {}
    mockPush.mockClear()
    mockSchemaStore.getList.mockReturnValue(null)
    mockEntitiesStore.fetchList.mockResolvedValue({ data: [] })
  })

  describe('loadScopeNav', () => {
    it('sets scopeNav to null when no from query param', async () => {
      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(scopeNav.value).toBeNull()
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

    it('sets scopeNav to null when entity not in list', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task', title: 'Tasks' })
      mockEntitiesStore.fetchList.mockResolvedValue({
        data: [{ id: 'TASK-002' }, { id: 'TASK-003' }],
      })

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
      mockEntitiesStore.fetchList.mockResolvedValue({
        data: [{ id: 'TASK-001' }, { id: 'TASK-002' }, { id: 'TASK-003' }],
      })

      const { scopeNav, loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-002'
      )

      await loadScopeNav()

      expect(scopeNav.value).toEqual({
        backUrl: '/list/tasks',
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
      mockEntitiesStore.fetchList.mockResolvedValue({
        data: [{ id: 'TASK-001' }, { id: 'TASK-002' }],
      })

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
      mockEntitiesStore.fetchList.mockResolvedValue({
        data: [{ id: 'TASK-001' }, { id: 'TASK-002' }],
      })

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
      mockEntitiesStore.fetchList.mockResolvedValue({
        data: [{ id: 'TASK-001' }],
      })

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
      mockEntitiesStore.fetchList.mockResolvedValue({ data: [{ id: 'TASK-001' }] })

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockEntitiesStore.fetchList).toHaveBeenCalledWith(
        'task',
        expect.objectContaining({ sort: '-created_at,title' })
      )
    })

    it('applies sort from query params over default', async () => {
      mockRouteQuery.value = { from: 'tasks', sort: 'priority' }
      mockSchemaStore.getList.mockReturnValue({
        entity: 'task',
        default_sort: [{ property: 'created_at', direction: 'desc' }],
      })
      mockEntitiesStore.fetchList.mockResolvedValue({ data: [{ id: 'TASK-001' }] })

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockEntitiesStore.fetchList).toHaveBeenCalledWith(
        'task',
        expect.objectContaining({ sort: 'priority' })
      )
    })

    it('applies list config filters', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({
        entity: 'task',
        filters: [{ property: 'status', operator: '=', value: 'open' }],
      })
      mockEntitiesStore.fetchList.mockResolvedValue({ data: [{ id: 'TASK-001' }] })

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockEntitiesStore.fetchList).toHaveBeenCalledWith(
        'task',
        expect.objectContaining({ 'filter[status][eq]': 'open' })
      )
    })

    it('applies user filters from query', async () => {
      mockRouteQuery.value = { from: 'tasks', filter_priority: 'high' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockEntitiesStore.fetchList.mockResolvedValue({ data: [{ id: 'TASK-001' }] })

      const { loadScopeNav } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()

      expect(mockEntitiesStore.fetchList).toHaveBeenCalledWith(
        'task',
        expect.objectContaining({ 'filter[priority]': 'high' })
      )
    })

    it('handles fetch errors gracefully', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockEntitiesStore.fetchList.mockRejectedValue(new Error('Network error'))

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
      mockEntitiesStore.fetchList.mockResolvedValue({
        data: [{ id: 'TASK-001' }, { id: 'TASK-002' }],
      })

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
      mockEntitiesStore.fetchList.mockResolvedValue({
        data: [{ id: 'TASK-001' }, { id: 'TASK-002' }],
      })

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
      mockEntitiesStore.fetchList.mockResolvedValue({
        data: [{ id: 'TASK-001' }],
      })

      const { loadScopeNav, navigateScope } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()
      navigateScope('prev')

      expect(mockPush).not.toHaveBeenCalled()
    })
  })

  describe('goBack', () => {
    it('navigates to back URL', async () => {
      mockRouteQuery.value = { from: 'tasks' }
      mockSchemaStore.getList.mockReturnValue({ entity: 'task' })
      mockEntitiesStore.fetchList.mockResolvedValue({
        data: [{ id: 'TASK-001' }],
      })

      const { loadScopeNav, goBack } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      await loadScopeNav()
      goBack()

      expect(mockPush).toHaveBeenCalledWith('/list/tasks')
    })

    it('does nothing when no scopeNav', () => {
      const { goBack } = useScopeNavigation(
        () => 'task',
        () => 'TASK-001'
      )

      goBack()

      expect(mockPush).not.toHaveBeenCalled()
    })
  })
})
