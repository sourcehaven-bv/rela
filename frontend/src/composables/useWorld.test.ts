import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref } from 'vue'
import { useWorld, DEFAULT_WORLD } from './useWorld'
import { useSchemaStore } from '@/stores/schema'

// Mirrors useBackTarget.test.ts: a mutable query object behind a getter, so
// the composable's `computed` re-evaluates when the "URL" changes.
const mockRouteQuery = ref<Record<string, unknown>>({})
const push = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({
    get query() {
      return mockRouteQuery.value
    },
  }),
  useRouter: () => ({ push }),
}))

describe('useWorld', () => {
  beforeEach(() => {
    mockRouteQuery.value = {}
    push.mockClear()
  })

  describe('reading the world from the URL', () => {
    it('is the default world when no ?world= is present', () => {
      const { world, isWorldBound, worldParam } = useWorld()
      expect(world.value).toBe('')
      expect(isWorldBound.value).toBe(false)
      // undefined, not '', so spreading into params emits no ?world= at all.
      expect(worldParam.value).toBeUndefined()
    })

    it('reads a named world', () => {
      mockRouteQuery.value = { world: 'published' }
      const { world, isWorldBound, worldParam } = useWorld()
      expect(world.value).toBe('published')
      expect(isWorldBound.value).toBe(true)
      expect(worldParam.value).toBe('published')
    })

    // The API accepts `?world=default` as an explicit spelling of the implicit
    // default world. Mutation: drop the DEFAULT_WORLD term from isWorldBound —
    // the SPA then treats it as world-bound, hides search and suppresses
    // include for a world that is just... the default one.
    it('treats an explicit "default" as NOT world-bound', () => {
      mockRouteQuery.value = { world: DEFAULT_WORLD }
      const { world, isWorldBound, worldParam } = useWorld()
      // Preserved verbatim so a deep link round-trips unchanged...
      expect(world.value).toBe(DEFAULT_WORLD)
      // ...but it is the default world, so nothing is suppressed.
      expect(isWorldBound.value).toBe(false)
      expect(worldParam.value).toBeUndefined()
    })

    // A repeated param arrives as an array. The API REJECTS that (400
    // duplicate_world) rather than picking one, so the SPA must not invent a
    // precedence. Mutation: return the last element instead of '' — the SPA
    // silently sends a world the user never unambiguously asked for.
    it('resolves a duplicated ?world= to the default world', () => {
      mockRouteQuery.value = { world: ['published', 'site-nl'] }
      const { world, isWorldBound } = useWorld()
      expect(world.value).toBe('')
      expect(isWorldBound.value).toBe(false)
    })
  })

  // With `app.default_world` configured, an ABSENT param means the CONFIGURED
  // world to the server. So the bare faces have exactly one spelling —
  // `?world=default` — and worldParam must keep it. Before this it dropped
  // it, and "Go to draft" fetched the published face with every write guard
  // off (the page thought it was on the writable default world).
  describe('with a configured default world', () => {
    beforeEach(() => {
      useSchemaStore().defaultWorld = 'published'
    })

    it('a bare URL is the configured world, and sends it', () => {
      const { world, isWorldBound, worldParam } = useWorld()
      expect(world.value).toBe('published')
      expect(isWorldBound.value).toBe(true)
      expect(worldParam.value).toBe('published')
    })

    it('an explicit "default" is the bare faces, and SENDS it', () => {
      mockRouteQuery.value = { world: DEFAULT_WORLD }
      const { isWorldBound, worldParam } = useWorld()
      // Not world-bound: the bare faces are where writes land...
      expect(isWorldBound.value).toBe(false)
      // ...but the param must travel, or the server applies the default.
      expect(worldParam.value).toBe(DEFAULT_WORLD)
    })

    it('without a configured default, "default" still sends nothing', () => {
      useSchemaStore().defaultWorld = ''
      mockRouteQuery.value = { world: DEFAULT_WORLD }
      const { worldParam } = useWorld()
      expect(worldParam.value).toBeUndefined()
    })
  })

  describe('setWorld', () => {
    it('pushes the world onto the query', () => {
      const { setWorld } = useWorld()
      setWorld('published')
      expect(push).toHaveBeenCalledWith({ query: { world: 'published' } })
    })

    it('removes the param entirely when returning to the default world', () => {
      mockRouteQuery.value = { world: 'published' }
      const { setWorld } = useWorld()
      setWorld('')
      // Absent, not `world: ''` — an empty param is a distinct request shape.
      expect(push).toHaveBeenCalledWith({ query: {} })
    })

    it('removes the param when the default world is named explicitly', () => {
      mockRouteQuery.value = { world: 'published' }
      const { setWorld } = useWorld()
      setWorld(DEFAULT_WORLD)
      expect(push).toHaveBeenCalledWith({ query: {} })
    })

    it('preserves unrelated query params', () => {
      mockRouteQuery.value = { 'filter[status]': 'open', sort: '-updated' }
      const { setWorld } = useWorld()
      setWorld('published')
      expect(push).toHaveBeenCalledWith({
        query: { 'filter[status]': 'open', sort: '-updated', world: 'published' },
      })
    })

    // Page 3 of the draft world is not page 3 of the published world: the
    // published world may hold fewer entities than that offset, so keeping the
    // page lands the user on an empty page that reads as "nothing is
    // published". Mutation: remove `delete query.page` — the assertion sees
    // page survive, which is the silent-empty-result trap.
    it('resets pagination when the world changes', () => {
      mockRouteQuery.value = { page: '3' }
      const { setWorld } = useWorld()
      setWorld('published')
      expect(push).toHaveBeenCalledWith({ query: { world: 'published' } })
    })

    // Without this, every selector re-emit pushes a duplicate history entry,
    // so one back-press appears to do nothing.
    it('does not navigate when the world is unchanged', () => {
      mockRouteQuery.value = { world: 'published' }
      const { setWorld } = useWorld()
      setWorld('published')
      expect(push).not.toHaveBeenCalled()
    })
  })
})
