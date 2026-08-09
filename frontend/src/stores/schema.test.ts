import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSchemaStore } from './schema'

// Mock the API
vi.mock('@/api/schema', () => ({
  getSchema: vi.fn(),
  getConfig: vi.fn(),
}))

// The per-principal dashboard is loaded alongside schema/config (TKT-53KICM).
// Defaulted in beforeEach so the many tests that don't care about cards need
// no per-test setup.
vi.mock('@/api/dashboard', () => ({
  getDashboard: vi.fn(),
}))

describe('Schema Store', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    const { getDashboard } = await import('@/api/dashboard')
    vi.mocked(getDashboard).mockResolvedValue({ cards: [] })
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
        // The UNFILTERED config block, deliberately different from what
        // /_dashboard returns below, so the assertion can tell which source
        // the store actually read (TKT-53KICM).
        dashboard: { cards: [{ title: 'Unfiltered', query: 'q', display: 'count' }] },
        navigation: [{ label: 'Tasks', list: 'tasks' }],
      })

      const { getDashboard } = await import('@/api/dashboard')
      vi.mocked(getDashboard).mockResolvedValue({
        title: 'Overview',
        cards: [{ title: 'Filtered', query: 'q', display: 'count' }],
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
      // From /_dashboard, NOT the /_config block: reading the latter would
      // show every principal the unfiltered card list.
      expect(store.dashboard).toEqual({
        title: 'Overview',
        cards: [{ title: 'Filtered', query: 'q', display: 'count' }],
      })
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

      // getErrorMessage surfaces a thrown string as-is — better than the
      // old generic fallback.
      expect(store.error).toBe('string error')
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

  describe('getEnumLabel', () => {
    it('returns the inline-enum label for a value', () => {
      const store = useSchemaStore()
      store.entityTypes = new Map([
        [
          'ticket',
          {
            label: 'Ticket',
            properties: {
              status: { type: 'enum', values: ['in_progress'], labels: { in_progress: 'In Progress' } },
            },
          },
        ],
      ]) as never
      expect(store.getEnumLabel('in_progress', 'status')).toBe('In Progress')
    })

    it('resolves a label from a referenced custom type', () => {
      const store = useSchemaStore()
      store.entityTypes = new Map([
        ['ticket', { label: 'Ticket', properties: { priority: { type: 'priority_t' } } }],
      ]) as never
      store.customTypes = new Map([
        ['priority_t', { values: ['hi'], labels: { hi: 'High' } }],
      ]) as never
      expect(store.getEnumLabel('hi', 'priority')).toBe('High')
    })

    it('returns undefined when no label is configured (caller falls back to value)', () => {
      const store = useSchemaStore()
      store.entityTypes = new Map([
        ['ticket', { label: 'Ticket', properties: { status: { type: 'enum', values: ['done'] } } }],
      ]) as never
      expect(store.getEnumLabel('done', 'status')).toBeUndefined()
    })

    it('returns undefined when property is absent', () => {
      const store = useSchemaStore()
      expect(store.getEnumLabel('x')).toBeUndefined()
    })

    it('prefers the given entity type when the same property name differs across types', () => {
      const store = useSchemaStore()
      store.entityTypes = new Map([
        [
          'bug',
          { label: 'Bug', properties: { status: { type: 'enum', values: ['x'], labels: { x: 'Bug X' } } } },
        ],
        [
          'ticket',
          { label: 'Ticket', properties: { status: { type: 'enum', values: ['x'], labels: { x: 'Ticket X' } } } },
        ],
      ]) as never
      expect(store.getEnumLabel('x', 'status', 'ticket')).toBe('Ticket X')
    })

    it('falls back to first-inserted type on collision when no entity type given', () => {
      const store = useSchemaStore()
      // Insertion order fixes the tie-break: `bug` first, so `bug`'s label wins
      // when the caller does not disambiguate. Pins the documented behavior.
      store.entityTypes = new Map([
        [
          'bug',
          { label: 'Bug', properties: { status: { type: 'enum', values: ['x'], labels: { x: 'Bug X' } } } },
        ],
        [
          'ticket',
          { label: 'Ticket', properties: { status: { type: 'enum', values: ['x'], labels: { x: 'Ticket X' } } } },
        ],
      ]) as never
      expect(store.getEnumLabel('x', 'status')).toBe('Bug X')
    })
  })

  describe('stylesForProperty', () => {
    // The server keys `styles` by custom-type name (buildStyleMap /
    // validateStyles), so resolution must go property -> custom type -> styles.
    it('resolves styles via the custom-type name when property ≠ type (BUG-28Y0Y2)', () => {
      const store = useSchemaStore()
      store.entityTypes = new Map([
        ['ticket', { label: 'Ticket', properties: { status: { type: 'ticket-status' } } }],
      ]) as never
      store.customTypes = new Map([['ticket-status', { values: ['todo', 'doing'] }]]) as never
      store.styles = { 'ticket-status': { todo: 'badge-yellow' } }
      expect(store.stylesForProperty('status')).toEqual({ todo: 'badge-yellow' })
    })

    it('prefers the type-keyed entry over a property-keyed one', () => {
      const store = useSchemaStore()
      store.entityTypes = new Map([
        ['ticket', { label: 'Ticket', properties: { status: { type: 'ticket-status' } } }],
      ]) as never
      store.customTypes = new Map([['ticket-status', { values: ['todo'] }]]) as never
      store.styles = {
        'ticket-status': { todo: 'badge-yellow' },
        status: { todo: 'badge-red' },
      }
      expect(store.stylesForProperty('status')).toEqual({ todo: 'badge-yellow' })
    })

    it('resolves a TYPE name passed as the property (EntityDetail view sections pass PropType)', () => {
      const store = useSchemaStore()
      store.entityTypes = new Map([
        ['ticket', { label: 'Ticket', properties: { status: { type: 'ticket-status' } } }],
      ]) as never
      store.customTypes = new Map([['ticket-status', { values: ['todo'] }]]) as never
      store.styles = { 'ticket-status': { todo: 'badge-yellow' } }
      // No property is *named* ticket-status; the direct-key fallback must hit.
      expect(store.stylesForProperty('ticket-status')).toEqual({ todo: 'badge-yellow' })
    })

    it('falls back to the property name when the property has no custom type', () => {
      const store = useSchemaStore()
      store.entityTypes = new Map([
        ['ticket', { label: 'Ticket', properties: { status: { type: 'enum', values: ['open'] } } }],
      ]) as never
      store.styles = { status: { open: 'badge-blue' } }
      expect(store.stylesForProperty('status')).toEqual({ open: 'badge-blue' })
    })

    it('disambiguates via the given entity type when property types differ across entities', () => {
      const store = useSchemaStore()
      store.entityTypes = new Map([
        ['bug', { label: 'Bug', properties: { status: { type: 'bug-status' } } }],
        ['ticket', { label: 'Ticket', properties: { status: { type: 'ticket-status' } } }],
      ]) as never
      store.customTypes = new Map([
        ['bug-status', { values: ['open'] }],
        ['ticket-status', { values: ['open'] }],
      ]) as never
      store.styles = {
        'bug-status': { open: 'badge-red' },
        'ticket-status': { open: 'badge-blue' },
      }
      expect(store.stylesForProperty('status', 'ticket')).toEqual({ open: 'badge-blue' })
      // Without a disambiguator the first-inserted type wins (documented tie-break).
      expect(store.stylesForProperty('status')).toEqual({ open: 'badge-red' })
    })

    it('resolves relation-type properties too', () => {
      const store = useSchemaStore()
      store.relationTypes = new Map([
        ['blocks', { label: 'blocks', properties: { weight: { type: 'weight_t' } } }],
      ]) as never
      store.customTypes = new Map([['weight_t', { values: ['high'] }]]) as never
      store.styles = { weight_t: { high: 'badge-orange' } }
      expect(store.stylesForProperty('weight')).toEqual({ high: 'badge-orange' })
    })

    it('returns undefined when nothing is styled', () => {
      const store = useSchemaStore()
      expect(store.stylesForProperty('status')).toBeUndefined()
    })
  })
})
