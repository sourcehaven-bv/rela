import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSchemaStore } from './schema'

// Mock the API
vi.mock('@/api/schema', () => ({
  getSchema: vi.fn(),
  getConfig: vi.fn(),
}))

describe('Schema Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('initial state', () => {
    it('starts with empty state', () => {
      const store = useSchemaStore()

      expect(store.loaded).toBe(false)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.entityTypes.size).toBe(0)
      expect(store.relationTypes.size).toBe(0)
      expect(store.forms.size).toBe(0)
      expect(store.lists.size).toBe(0)
      expect(store.views.size).toBe(0)
      expect(store.kanbans.size).toBe(0)
      expect(store.navigation).toEqual([])
    })

    it('has default app config', () => {
      const store = useSchemaStore()
      expect(store.app).toEqual({ name: 'rela' })
    })
  })

  describe('load', () => {
    it('loads schema and config from API', async () => {
      const { getSchema, getConfig } = await import('@/api/schema')

      vi.mocked(getSchema).mockResolvedValue({
        entities: {
          task: { label: 'Task', description: 'A task', properties: {} },
          bug: { label: 'Bug', description: 'A bug', properties: {} },
        },
        relations: {
          blocks: { label: 'blocks', from: ['task'], to: ['task'] },
        },
        types: {
          priority: { values: ['low', 'medium', 'high'] },
        },
      })

      vi.mocked(getConfig).mockResolvedValue({
        app: { name: 'Test App', description: 'A test app' },
        styles: { status: { open: 'badge-blue' } },
        forms: { 'task-create': { entity: 'task' } },
        lists: { tasks: { entity: 'task', columns: [] } },
        views: { 'task-view': { entity: 'task', sections: [] } },
        kanbans: { 'task-board': { entity: 'task', column_property: 'status', card: { title: 'title' } } },
        documents: { report: { command: 'echo test' } },
        dashboard: { cards: [] },
        navigation: [{ label: 'Tasks', list: 'tasks' }],
      })

      const store = useSchemaStore()
      await store.load()

      expect(store.loaded).toBe(true)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()

      // Check schema
      expect(store.entityTypes.size).toBe(2)
      expect(store.entityTypes.get('task')).toEqual({ label: 'Task', description: 'A task', properties: {} })
      expect(store.relationTypes.size).toBe(1)
      expect(store.customTypes.size).toBe(1)

      // Check config
      expect(store.app).toEqual({ name: 'Test App', description: 'A test app' })
      expect(store.styles).toEqual({ status: { open: 'badge-blue' } })
      expect(store.forms.size).toBe(1)
      expect(store.lists.size).toBe(1)
      expect(store.views.size).toBe(1)
      expect(store.kanbans.size).toBe(1)
      expect(store.documents.size).toBe(1)
      expect(store.dashboard).toEqual({ cards: [] })
      expect(store.navigation).toEqual([{ label: 'Tasks', list: 'tasks' }])
    })

    it('does not load twice when already loaded', async () => {
      const { getSchema, getConfig } = await import('@/api/schema')

      vi.mocked(getSchema).mockResolvedValue({ entities: {}, relations: {}, types: {} })
      vi.mocked(getConfig).mockResolvedValue({ app: { name: 'test' }, forms: {}, lists: {}, views: {}, kanbans: {}, navigation: [] })

      const store = useSchemaStore()
      await store.load()
      await store.load()

      expect(getSchema).toHaveBeenCalledTimes(1)
      expect(getConfig).toHaveBeenCalledTimes(1)
    })

    it('does not load when already loading', async () => {
      const { getSchema, getConfig } = await import('@/api/schema')

      let resolveSchema: () => void
      vi.mocked(getSchema).mockReturnValue(
        new Promise((resolve) => {
          resolveSchema = () => resolve({ entities: {}, relations: {}, types: {} })
        })
      )
      vi.mocked(getConfig).mockResolvedValue({ app: { name: 'test' }, forms: {}, lists: {}, views: {}, kanbans: {}, navigation: [] })

      const store = useSchemaStore()

      // Start first load
      const loadPromise1 = store.load()
      expect(store.loading).toBe(true)

      // Start second load - should not actually call API again
      const loadPromise2 = store.load()

      // Resolve the schema
      resolveSchema!()
      await loadPromise1
      await loadPromise2

      expect(getSchema).toHaveBeenCalledTimes(1)
    })

    it('handles errors gracefully', async () => {
      const { getSchema, getConfig } = await import('@/api/schema')

      vi.mocked(getSchema).mockRejectedValue(new Error('Network error'))
      vi.mocked(getConfig).mockResolvedValue({ app: { name: 'test' }, forms: {}, lists: {}, views: {}, kanbans: {}, navigation: [] })

      const store = useSchemaStore()

      await expect(store.load()).rejects.toThrow('Network error')

      expect(store.loaded).toBe(false)
      expect(store.loading).toBe(false)
      expect(store.error).toBe('Network error')
    })

    it('handles non-Error exceptions', async () => {
      const { getSchema, getConfig } = await import('@/api/schema')

      vi.mocked(getSchema).mockRejectedValue('string error')
      vi.mocked(getConfig).mockResolvedValue({ app: { name: 'test' }, forms: {}, lists: {}, views: {}, kanbans: {}, navigation: [] })

      const store = useSchemaStore()

      await expect(store.load()).rejects.toBe('string error')

      expect(store.error).toBe('Failed to load schema')
    })

    it('handles missing optional fields', async () => {
      const { getSchema, getConfig } = await import('@/api/schema')

      vi.mocked(getSchema).mockResolvedValue({ entities: {}, relations: {}, types: {} })
      vi.mocked(getConfig).mockResolvedValue({ app: { name: 'rela' }, forms: {}, lists: {}, views: {}, kanbans: {}, navigation: [] })

      const store = useSchemaStore()
      await store.load()

      expect(store.loaded).toBe(true)
      expect(store.entityTypes.size).toBe(0)
      expect(store.app).toEqual({ name: 'rela' })
      expect(store.styles).toEqual({})
      expect(store.navigation).toEqual([])
    })
  })

  describe('reload', () => {
    it('resets loaded state and loads again', async () => {
      const { getSchema, getConfig } = await import('@/api/schema')

      vi.mocked(getSchema).mockResolvedValue({
        entities: { task: { label: 'Task', description: '', properties: {} } },
        relations: {},
        types: {},
      })
      vi.mocked(getConfig).mockResolvedValue({ app: { name: 'test' }, forms: {}, lists: {}, views: {}, kanbans: {}, navigation: [] })

      const store = useSchemaStore()
      await store.load()

      expect(getSchema).toHaveBeenCalledTimes(1)

      await store.reload()

      expect(getSchema).toHaveBeenCalledTimes(2)
      expect(store.loaded).toBe(true)
    })
  })

  describe('getters', () => {
    beforeEach(async () => {
      const { getSchema, getConfig } = await import('@/api/schema')

      vi.mocked(getSchema).mockResolvedValue({
        entities: {
          task: { label: 'Task', description: 'A task', properties: {} },
        },
        relations: {
          blocks: { label: 'blocks', from: ['task'], to: ['task'] },
        },
        types: {},
      })

      vi.mocked(getConfig).mockResolvedValue({
        app: { name: 'test' },
        forms: { 'task-form': { entity: 'task' } },
        lists: { tasks: { entity: 'task', columns: [] } },
        views: { 'task-view': { entity: 'task', sections: [] } },
        kanbans: { 'task-board': { entity: 'task', column_property: 'status', card: { title: 'title' } } },
        navigation: [],
      })
    })

    it('getEntityType returns entity type', async () => {
      const store = useSchemaStore()
      await store.load()

      expect(store.getEntityType('task')).toEqual({ label: 'Task', description: 'A task', properties: {} })
      expect(store.getEntityType('nonexistent')).toBeUndefined()
    })

    it('getRelationType returns relation type', async () => {
      const store = useSchemaStore()
      await store.load()

      expect(store.getRelationType('blocks')).toEqual({ label: 'blocks', from: ['task'], to: ['task'] })
      expect(store.getRelationType('nonexistent')).toBeUndefined()
    })

    it('getForm returns form config', async () => {
      const store = useSchemaStore()
      await store.load()

      expect(store.getForm('task-form')).toEqual({ entity: 'task' })
      expect(store.getForm('nonexistent')).toBeUndefined()
    })

    it('getList returns list config', async () => {
      const store = useSchemaStore()
      await store.load()

      expect(store.getList('tasks')).toEqual({ entity: 'task', columns: [] })
      expect(store.getList('nonexistent')).toBeUndefined()
    })

    it('findListIdForEntityType returns the list ID for a given entity type', async () => {
      const store = useSchemaStore()
      await store.load()

      expect(store.findListIdForEntityType('task')).toBe('tasks')
      expect(store.findListIdForEntityType('nonexistent-type')).toBeUndefined()
    })

    it('getView returns view config', async () => {
      const store = useSchemaStore()
      await store.load()

      expect(store.getView('task-view')).toEqual({ entity: 'task', sections: [] })
      expect(store.getView('nonexistent')).toBeUndefined()
    })

    it('getKanban returns kanban config', async () => {
      const store = useSchemaStore()
      await store.load()

      expect(store.getKanban('task-board')).toEqual({
        entity: 'task',
        column_property: 'status',
        card: { title: 'title' },
      })
      expect(store.getKanban('nonexistent')).toBeUndefined()
    })

    it('entityTypeList returns entries array', async () => {
      const store = useSchemaStore()
      await store.load()

      expect(store.entityTypeList).toEqual([
        ['task', { label: 'Task', description: 'A task', properties: {} }],
      ])
    })

    it('relationTypeList returns entries array', async () => {
      const store = useSchemaStore()
      await store.load()

      expect(store.relationTypeList).toEqual([
        ['blocks', { label: 'blocks', from: ['task'], to: ['task'] }],
      ])
    })
  })
})
