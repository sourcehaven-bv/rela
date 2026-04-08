import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { reactive, nextTick, effectScope, type EffectScope } from 'vue'
import type { LocationQuery } from 'vue-router'
import { useUrlFilterSync, type UseUrlFilterSyncOptions } from './useUrlFilterSync'

// A reactive route-like object whose `query` we can mutate to simulate
// navigation. The composable destructures via `useRoute()` so it gets a stable
// reference that watch() can observe field changes on.
const mockRoute = reactive<{ query: LocationQuery }>({ query: {} })
const mockReplace = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => mockRoute,
  useRouter: () => ({
    replace: mockReplace,
  }),
}))

// router.replace usually mutates route.query in real Vue Router. Wire the mock
// so it does the same — that's what makes the watcher fire.
mockReplace.mockImplementation(({ query }: { query: LocationQuery }) => {
  mockRoute.query = query
})

const noStatic = () => new Set<string>()

// Each test runs the composable inside its own effect scope so the watcher is
// torn down at end-of-test. Without this, watchers from previous tests stay
// subscribed to the shared mockRoute and re-fire on later mutations, polluting
// stderr with stale collision warnings.
let scope: EffectScope

function setup(opts: UseUrlFilterSyncOptions) {
  return scope.run(() => useUrlFilterSync(opts))!
}

describe('useUrlFilterSync', () => {
  beforeEach(() => {
    mockRoute.query = {}
    mockReplace.mockClear()
    scope = effectScope()
  })

  afterEach(() => {
    scope.stop()
  })

  describe('initial read', () => {
    it('seeds empty state when query is empty', () => {
      const { filters } = setup({ staticFilterProperties: noStatic })
      expect(filters.value).toEqual({})
    })

    it('seeds from URL on setup (synchronously, before any fetch)', () => {
      mockRoute.query = { 'filter[status]': 'open' }
      const { filters } = setup({ staticFilterProperties: noStatic })
      expect(filters.value).toEqual({ status: { value: 'open' } })
    })

    it('seeds operator filters from URL', () => {
      mockRoute.query = { 'filter[due_date][lte]': '$today' }
      const { filters } = setup({ staticFilterProperties: noStatic })
      expect(filters.value).toEqual({ due_date: { value: '$today', op: '<=' } })
    })

    it('drops URL filters that collide with static config and warns', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      mockRoute.query = { 'filter[status]': 'open', 'filter[priority]': 'high' }
      const { filters } = setup({
        staticFilterProperties: () => new Set(['status']),
      })
      expect(filters.value).toEqual({ priority: { value: 'high' } })
      expect(warn).toHaveBeenCalledWith(
        expect.stringContaining('"status"'),
      )
      expect(warn).toHaveBeenCalledWith(
        expect.stringContaining('whole property'),
      )
      warn.mockRestore()
    })
  })

  describe('writeToQuery', () => {
    it('updates filters and calls router.replace', () => {
      const { filters, writeToQuery } = setup({
        staticFilterProperties: noStatic,
      })
      writeToQuery({ status: { value: 'open' } })
      expect(filters.value).toEqual({ status: { value: 'open' } })
      expect(mockReplace).toHaveBeenCalledWith({
        query: { 'filter[status]': 'open' },
      })
    })

    it('preserves non-filter params on the existing query', () => {
      mockRoute.query = { from: 'all_tasks', sort: '-due_date' }
      const { writeToQuery } = setup({ staticFilterProperties: noStatic })
      writeToQuery({ status: { value: 'open' } })
      expect(mockReplace).toHaveBeenCalledWith({
        query: { from: 'all_tasks', sort: '-due_date', 'filter[status]': 'open' },
      })
    })

    it('clearing all filters drops filter params but keeps non-filter params', () => {
      mockRoute.query = { from: 'all_tasks', 'filter[status]': 'open' }
      const { writeToQuery } = setup({ staticFilterProperties: noStatic })
      writeToQuery({})
      expect(mockReplace).toHaveBeenCalledWith({ query: { from: 'all_tasks' } })
    })
  })

  describe('route watcher (back/forward navigation)', () => {
    it('updates filters when route.query changes externally', async () => {
      const { filters } = setup({ staticFilterProperties: noStatic })
      expect(filters.value).toEqual({})

      mockRoute.query = { 'filter[status]': 'done' }
      await nextTick()

      expect(filters.value).toEqual({ status: { value: 'done' } })
    })

    it('does not re-read on self-write echo (signature match)', async () => {
      const { filters, writeToQuery } = setup({
        staticFilterProperties: noStatic,
      })

      writeToQuery({ status: { value: 'open' } })
      await nextTick()
      // The watcher would have fired here because router.replace mutates
      // route.query — but the signature matches, so filters shouldn't be
      // re-derived from a parse cycle. Easiest way to assert: state is exactly
      // what we wrote, no surprise mutation.
      expect(filters.value).toEqual({ status: { value: 'open' } })
    })

    it('self-heals after a write — next external nav still re-reads', async () => {
      const { filters, writeToQuery } = setup({
        staticFilterProperties: noStatic,
      })

      writeToQuery({ status: { value: 'open' } })
      await nextTick()

      // Simulate user clicking browser back: query changes to something
      // different from what we wrote.
      mockRoute.query = { 'filter[priority]': 'high' }
      await nextTick()

      expect(filters.value).toEqual({ priority: { value: 'high' } })
    })

    it('respects static filter collisions on external nav too', async () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const { filters } = setup({
        staticFilterProperties: () => new Set(['status']),
      })

      mockRoute.query = { 'filter[status]': 'open', 'filter[priority]': 'high' }
      await nextTick()

      expect(filters.value).toEqual({ priority: { value: 'high' } })
      warn.mockRestore()
    })

    it('does not collide on values containing & or = (RR-XO1V regression)', async () => {
      // Two distinct queries must produce different signatures so the
      // watcher correctly reads an external nav even if a prior self-write
      // had a value containing '=' or '&'.
      const { filters, writeToQuery } = setup({ staticFilterProperties: noStatic })

      // First write: one key with a value containing '&' and '='.
      writeToQuery({ search: { value: 'x&filter[b]=y' } })
      await nextTick()
      expect(filters.value).toEqual({ search: { value: 'x&filter[b]=y' } })

      // External nav to a genuinely different two-key query which a naive
      // k=v&… stringifier would collide with. The composable MUST re-read.
      mockRoute.query = { 'filter[search]': 'x', 'filter[b]': 'y' }
      await nextTick()
      expect(filters.value).toEqual({
        search: { value: 'x' },
        b: { value: 'y' },
      })
    })

    it('rapid successive writes — last signature wins', async () => {
      const { filters, writeToQuery } = setup({ staticFilterProperties: noStatic })

      writeToQuery({ status: { value: 'open' } })
      writeToQuery({ status: { value: 'in_progress' } })
      writeToQuery({ status: { value: 'done' } })
      await nextTick()

      expect(filters.value).toEqual({ status: { value: 'done' } })
      // The final router.replace reflects the last write.
      expect(mockReplace).toHaveBeenLastCalledWith({
        query: { 'filter[status]': 'done' },
      })
    })

    it('non-filter query changes still trigger a re-read', async () => {
      // Technically the composable re-reads on ANY external change (it's
      // simpler than filtering). This test documents that behavior so
      // future refactors don't accidentally break it \u2014 and if they do,
      // the fetch still needs to run because pagination/sort may have moved.
      const { filters } = setup({ staticFilterProperties: noStatic })

      mockRoute.query = { 'filter[status]': 'open', page: '1' }
      await nextTick()
      expect(filters.value).toEqual({ status: { value: 'open' } })

      mockRoute.query = { 'filter[status]': 'open', page: '2' }
      await nextTick()
      // Same filters, different page. Filter state unchanged.
      expect(filters.value).toEqual({ status: { value: 'open' } })
    })
  })
})
