import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSchemaStore } from './schema'
import type { WorldInfo } from '@/types/schema'

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
    it('loads the per-caller world map from /_schema', async () => {
      // Pins the WIRING, not just the getter. Deleting the
      // `worlds.value = new Map(...)` line survives every component test,
      // because those seed the store directly — verified by mutation. The
      // only test that can catch it is one that goes through load().
      const { getSchema, getConfig } = await import('@/api/schema')
      vi.mocked(getSchema).mockResolvedValue({
        entities: {},
        relations: {},
        types: {},
        worlds: {
          default: { readable: true, default: true },
          published: { readable: false, select: ['published'], otherwise: 'exclude' },
        },
      })
      vi.mocked(getConfig).mockResolvedValue({
        app: { name: 'Test App' },
        forms: {},
        lists: {},
        views: {},
        kanbans: {},
        navigation: [],
      })

      const store = useSchemaStore()
      await store.load()

      expect(store.worlds.size).toBe(2)
      // `false` is the load-bearing value — the server never omits `readable`
      // precisely so a denial is distinguishable from an old server.
      expect(store.worldReadable('published')).toBe(false)
      expect(store.worldReadable('default')).toBe(true)
      // The empty name is how the SPA spells the default world.
      expect(store.worldReadable('')).toBe(true)
      // An undeclared world is readable, not denied: absence means "unknown",
      // and manufacturing a denial would hide a working affordance.
      expect(store.worldReadable('no-such-world')).toBe(true)
    })

    // history_enabled is a per-DEPLOYMENT capability (postgres-only content
    // versioning), and it is the FLAG DIRECTION that matters here.
    //
    // Absent must mean FALSE, unlike `readable` above where unknown means
    // permitted. The directions differ because the mistakes differ: hiding a
    // working History button is a missing feature someone reports, while
    // showing one on every fs deployment is a control that can only 501 —
    // the affordance-that-lies shape.
    //
    // This goes through load() deliberately. Component tests seed the store
    // directly, so they cannot catch a parse that reads the flag the wrong
    // way round; only a test that runs the real assignment can.
    describe('history_enabled from /_config', () => {
      async function loadWithApp(app: Record<string, unknown>) {
        const { getSchema, getConfig } = await import('@/api/schema')
        vi.mocked(getSchema).mockResolvedValue({
          entities: {}, relations: {}, types: {},
        } as never)
        vi.mocked(getConfig).mockResolvedValue({
          app: { name: 'Test App', ...app },
          forms: {}, lists: {}, views: {}, kanbans: {}, navigation: [],
        } as never)
        const store = useSchemaStore()
        await store.load()
        return store
      }

      it('is true when the server says so', async () => {
        expect((await loadWithApp({ history_enabled: true })).historyEnabled).toBe(true)
      })

      it('is false when the server says so', async () => {
        expect((await loadWithApp({ history_enabled: false })).historyEnabled).toBe(false)
      })

      it('is FALSE when the server omits it (a server too old to answer)', async () => {
        expect((await loadWithApp({})).historyEnabled).toBe(false)
      })
    })

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

    // A /_dashboard failure must NOT fail the boot. doLoad re-throws, and
    // App.vue turns a rejected load into the full-screen error state — so
    // letting this one endpoint reject would take down the sidebar, every
    // list and every form for the sake of a UX filter most deployments never
    // exercise. The concrete case is a newer SPA against an older server,
    // where the route simply does not exist.
    it('degrades to an empty dashboard when /_dashboard fails, without failing the boot', async () => {
      const { getSchema, getConfig } = await import('@/api/schema')
      const { getDashboard } = await import('@/api/dashboard')

      vi.mocked(getSchema).mockResolvedValue({ entities: {}, relations: {}, types: {} })
      vi.mocked(getConfig).mockResolvedValue({
        app: { name: 'test' },
        forms: {},
        lists: {},
        views: {},
        kanbans: {},
        navigation: [{ label: 'Tasks', list: 'tasks' }],
      })
      vi.mocked(getDashboard).mockRejectedValue(new Error('404 not found'))

      const store = useSchemaStore()
      await expect(store.load()).resolves.toBeUndefined()

      expect(store.loaded).toBe(true)
      expect(store.error).toBeNull()
      // The rest of the app is fully usable...
      expect(store.navigation).toEqual([{ label: 'Tasks', list: 'tasks' }])
      // ...and only the dashboard degrades.
      expect(store.dashboard).toBeUndefined()
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

  describe('worldForFace', () => {
    // Maps a stored POINTER to the world that serves it. `?world=` is the
    // read-selection grammar this API has; a bare face is not a world, and
    // sending one yields 400 unknown_world (measured: ?world=nl for a project
    // whose world is site-nl).
    async function loadWorlds(worlds: Record<string, WorldInfo>) {
      const { getSchema, getConfig } = await import('@/api/schema')
      vi.mocked(getSchema).mockResolvedValue({
        entities: {},
        relations: {},
        types: {},
        worlds,
      })
      vi.mocked(getConfig).mockResolvedValue({
        app: { name: 'Test App' },
        forms: {},
        lists: {},
        views: {},
        kanbans: {},
        navigation: [],
      })
      const store = useSchemaStore()
      await store.load()
      return store
    }

    it('matches the chain HEAD, not mere membership', async () => {
      // The rule that is easy to get wrong. site-nl is [nl, en], so `en`
      // appears in its chain as a FALLBACK — that does not make site-nl the
      // world which serves English. Matching on membership would route an
      // English face to the Dutch site.
      const store = await loadWorlds({
        default: { readable: true, default: true },
        'site-nl': { readable: true, select: ['nl', 'en'], otherwise: 'default' },
      })
      expect(store.worldForFace('blog-post', 'nl')).toBe('site-nl')
      expect(store.worldForFace('blog-post', 'en')).toBeUndefined()
    })

    it('prefers a per-type override over the world own select', async () => {
      // The resolver honours `overrides` ahead of `select`, so this must too.
      // Here `published` selects the `published` face generally but serves
      // blog-posts through `en` — so a policy face and a blog face map to the
      // SAME world through different chains.
      const store = await loadWorlds({
        default: { readable: true, default: true },
        published: {
          readable: true,
          select: ['published'],
          overrides: { 'blog-post': ['en'] },
          otherwise: 'exclude',
        },
      })
      expect(store.worldForFace('policy', 'published')).toBe('published')
      expect(store.worldForFace('blog-post', 'en')).toBe('published')
      // The override REPLACES select for that type, so the generic chain no
      // longer applies to it.
      expect(store.worldForFace('blog-post', 'published')).toBeUndefined()
    })

    // TKT-MFVH03: two worlds heading the same face used to be resolved by map
    // iteration order — insertion order, a property of how the config
    // serialized rather than of what the operator meant.
    it('uses primary_for to break a tie between two worlds heading the face', async () => {
      const store = await loadWorlds({
        default: { readable: true, default: true },
        'site-nl': {
          readable: true,
          select: ['nl', 'en'],
          otherwise: 'default',
          primary_for: ['nl'],
        },
        'editorial-nl': { readable: true, select: ['nl'], otherwise: 'exclude' },
      })
      expect(store.worldForFace('guide', 'nl')).toBe('site-nl')
    })

    it('is insensitive to the order the tied worlds arrive in', async () => {
      // The same two worlds, declared the other way round. Before the fix this
      // flipped the answer; the claim must decide it, not the ordering.
      const store = await loadWorlds({
        default: { readable: true, default: true },
        'editorial-nl': { readable: true, select: ['nl'], otherwise: 'exclude' },
        'site-nl': {
          readable: true,
          select: ['nl', 'en'],
          otherwise: 'default',
          primary_for: ['nl'],
        },
      })
      expect(store.worldForFace('guide', 'nl')).toBe('site-nl')
    })

    it('returns undefined for an unresolved tie rather than picking one', async () => {
      // The server refuses such a schema at load, so this is belt-and-braces —
      // but returning SOMETHING here is precisely the old bug, and the caller
      // already knows how to omit an affordance for undefined.
      const store = await loadWorlds({
        default: { readable: true, default: true },
        'site-nl': { readable: true, select: ['nl', 'en'], otherwise: 'default' },
        'editorial-nl': { readable: true, select: ['nl'], otherwise: 'exclude' },
      })
      expect(store.worldForFace('guide', 'nl')).toBeUndefined()
    })

    it('ignores a claim on a face the world does not head', async () => {
      // `en` is site-nl FALLBACK. The server rejects this schema, so the only
      // question here is that the client does not honour the claim anyway.
      const store = await loadWorlds({
        default: { readable: true, default: true },
        'site-nl': {
          readable: true,
          select: ['nl', 'en'],
          otherwise: 'default',
          primary_for: ['en'],
        },
      })
      expect(store.worldForFace('guide', 'en')).toBeUndefined()
    })

    it('omits the affordance when two distinguishable worlds lead the face', async () => {
      // Same head, different `otherwise:` — the server ACCEPTS this pair (they
      // answer different questions about entities lacking the face), so no
      // declaration is forced. But nothing says which one a face-switch means,
      // so the affordance is omitted rather than guessed.
      const store = await loadWorlds({
        default: { readable: true, default: true },
        published: { readable: true, select: ['published'], otherwise: 'exclude' },
        lenient: { readable: true, select: ['published'], otherwise: 'default' },
      })
      expect(store.worldForFace('note', 'published')).toBeUndefined()
    })

    it('honours a claim even between distinguishable worlds', async () => {
      const store = await loadWorlds({
        default: { readable: true, default: true },
        published: {
          readable: true,
          select: ['published'],
          otherwise: 'exclude',
          primary_for: ['published'],
        },
        lenient: { readable: true, select: ['published'], otherwise: 'default' },
      })
      expect(store.worldForFace('note', 'published')).toBe('published')
    })

    it('returns undefined when no declared world heads the face', async () => {
      // undefined is load-bearing: the caller omits the affordance rather than
      // inventing a `?world=` the server will reject. A falsy-but-defined ''
      // would be read as "the default world" and navigate somewhere wrong.
      const store = await loadWorlds({
        default: { readable: true, default: true },
        published: { readable: true, select: ['published'], otherwise: 'exclude' },
      })
      expect(store.worldForFace('policy', 'archived')).toBeUndefined()
    })

    it("maps the default face to '' — the world param is omitted", async () => {
      const store = await loadWorlds({
        default: { readable: true, default: true },
        published: { readable: true, select: ['published'], otherwise: 'exclude' },
      })
      expect(store.worldForFace('policy', '')).toBe('')
    })

    it('never returns the reserved default world for a real face', async () => {
      // `default` is total and implicit; it heads no face chain. Returning
      // it would send `?world=default`, which is a different request from
      // omitting the param on a deployment with app.default_world set.
      const store = await loadWorlds({
        default: { readable: true, default: true, select: ['draft'] },
      })
      expect(store.worldForFace('policy', 'draft')).toBeUndefined()
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
