import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { reactive, nextTick, effectScope, type EffectScope } from 'vue'
import type { LocationQuery } from 'vue-router'
import {
  useVersionSelectionSync,
  parseSide,
  serializeSide,
  type Side,
  type UseVersionSelectionSyncOptions,
} from './useVersionSelectionSync'

// A reactive route-like object whose `query` we mutate to simulate navigation,
// mirroring useUrlFilterSync.test.ts.
const mockRoute = reactive<{ query: LocationQuery }>({ query: {} })
const mockReplace = vi.fn()
const mockPush = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => mockRoute,
  useRouter: () => ({
    replace: mockReplace,
    push: mockPush,
  }),
}))

// Real router.replace mutates route.query; wire the mock the same way so the
// watcher actually fires (and so an echo loop would be observable if the guard
// were broken).
mockReplace.mockImplementation(({ query }: { query: LocationQuery }) => {
  mockRoute.query = query
})

// Each test runs the composable in its own effect scope so watchers are torn
// down and don't re-fire against the shared mockRoute in later tests.
let scope: EffectScope

function setup(opts: Partial<UseVersionSelectionSyncOptions> = {}) {
  const full: UseVersionSelectionSyncOptions = {
    validVersions: () => [1, 2, 3],
    defaults: () => ({ base: 3, target: 'current' }),
    onChange: vi.fn(),
    ...opts,
  }
  const api = scope.run(() => useVersionSelectionSync(full))!
  return { ...api, onChange: full.onChange }
}

describe('parseSide', () => {
  const valid = [1, 2, 3]

  it("keeps 'current' as the sentinel string", () => {
    expect(parseSide('current', valid)).toBe('current')
  })

  it('coerces a valid ordinal to a NUMBER, not a numeric string', () => {
    // The <option value="current"> is a string while version options bind
    // :value="m.version" (a number). A string '3' would match no option and
    // v-model would render the dropdown blank.
    const parsed = parseSide('3', valid)
    expect(parsed).toBe(3)
    expect(typeof parsed).toBe('number')
  })

  it('takes the last value of a repeated param', () => {
    expect(parseSide(['1', '2'], valid)).toBe(2)
  })

  it.each([
    ['empty string', ''],
    ['non-numeric', 'abc'],
    ['negative', '-1'],
    ['zero (ordinals are 1-based)', '0'],
    ['fractional', '1.5'],
    ['exponent notation', '1e2'],
    ['out of range', '999'],
    ['path traversal attempt', '../../etc/passwd'],
    ['script tag', '<script>alert(1)</script>'],
    ['whitespace padded', ' 2 '],
    ['undefined', undefined],
    ['null', null],
    ['number type', 3],
    ['empty array', []],
  ])('rejects %s', (_label, input) => {
    expect(parseSide(input, valid)).toBeNull()
  })

  it('rejects an ordinal absent from the list even though it is numeric', () => {
    expect(parseSide('4', [1, 2, 3])).toBeNull()
  })

  it('rejects every ordinal while the version list is still empty', () => {
    expect(parseSide('1', [])).toBeNull()
    expect(parseSide('current', [])).toBe('current')
  })
})

describe('serializeSide', () => {
  it.each([
    ['current' as Side, 'current'],
    [3 as Side, '3'],
  ])('serializes %s to %s', (input, expected) => {
    expect(serializeSide(input)).toBe(expected)
  })
})

describe('useVersionSelectionSync', () => {
  beforeEach(() => {
    mockRoute.query = {}
    mockReplace.mockClear()
    mockPush.mockClear()
    scope = effectScope()
  })

  afterEach(() => {
    scope.stop()
  })

  describe('seeding', () => {
    it('falls back to defaults when no params are present', () => {
      const { base, target } = setup()
      expect(base.value).toBe(3)
      expect(target.value).toBe('current')
    })

    it('seeds both sides from the URL', () => {
      mockRoute.query = { base: '1', target: '2' }
      const { base, target } = setup()
      expect(base.value).toBe(1)
      expect(target.value).toBe(2)
    })

    it('seeds one side and defaults the other when only one param is given', () => {
      mockRoute.query = { base: '2' }
      const { base, target } = setup()
      expect(base.value).toBe(2)
      expect(target.value).toBe('current')
    })

    it('allows an identical pair (an empty diff is linkable)', () => {
      mockRoute.query = { base: '2', target: '2' }
      const { base, target } = setup()
      expect(base.value).toBe(2)
      expect(target.value).toBe(2)
    })

    it('falls back to defaults for malformed params without throwing', () => {
      mockRoute.query = { base: 'abc', target: '999' }
      const { base, target } = setup()
      expect(base.value).toBe(3)
      expect(target.value).toBe('current')
    })

    it('re-seeds against the real ordinals once the version list loads', () => {
      // The synchronous setup pass sees an empty list, so ?base=2 can't be
      // validated yet; seedFromUrl re-runs after listVersions resolves.
      mockRoute.query = { base: '2', target: 'current' }
      let loaded = false
      const { base, seedFromUrl } = setup({
        validVersions: () => (loaded ? [1, 2, 3] : []),
        defaults: () => ({ base: loaded ? 3 : 'current', target: 'current' }),
      })
      expect(base.value).toBe('current') // not yet validatable

      loaded = true
      seedFromUrl()
      expect(base.value).toBe(2)
    })

    it('does not seed a stale ordinal after the list shrinks', () => {
      mockRoute.query = { base: '3', target: 'current' }
      let versions = [1, 2, 3]
      const { base, seedFromUrl } = setup({
        validVersions: () => versions,
        defaults: () => ({ base: versions[versions.length - 1], target: 'current' }),
      })
      expect(base.value).toBe(3)

      versions = [1, 2]
      seedFromUrl()
      expect(base.value).toBe(2) // default, not the now-absent v3
    })
  })

  describe('writing to the URL', () => {
    it('writes the pair on select and recomputes', () => {
      const { select, onChange } = setup()
      select({ base: 1 })
      expect(mockReplace).toHaveBeenCalledWith({ query: { base: '1', target: 'current' } })
      expect(onChange).toHaveBeenCalledTimes(1)
    })

    it('uses replace, never push, so the Back button is not buried', () => {
      const { select } = setup()
      select({ base: 1 })
      select({ target: 2 })
      expect(mockReplace).toHaveBeenCalledTimes(2)
      expect(mockPush).not.toHaveBeenCalled()
    })

    it("serializes the 'current' sentinel verbatim rather than as a number", () => {
      const { select } = setup()
      select({ base: 'current', target: 'current' })
      expect(mockReplace).toHaveBeenCalledWith({
        query: { base: 'current', target: 'current' },
      })
    })

    it('preserves unrelated query params', () => {
      mockRoute.query = { return_to: '/list/features', from: 'dashboard' }
      const { select } = setup()
      select({ base: 2 })
      expect(mockReplace).toHaveBeenCalledWith({
        query: { return_to: '/list/features', from: 'dashboard', base: '2', target: 'current' },
      })
    })

    it('swap reverses the two sides and publishes both', () => {
      mockRoute.query = { base: '1', target: '3' }
      const { base, target, swap } = setup()
      swap()
      expect(base.value).toBe(3)
      expect(target.value).toBe(1)
      expect(mockReplace).toHaveBeenCalledWith({ query: { base: '3', target: '1' } })
    })

    it('publish writes the resolved pair without recomputing', () => {
      const { publish, onChange } = setup()
      publish()
      expect(mockReplace).toHaveBeenCalledWith({ query: { base: '3', target: 'current' } })
      expect(onChange).not.toHaveBeenCalled()
    })

    it('publishes a correcting write even when there are NO versions', () => {
      // The view calls publish() unconditionally so a stale ordinal left in the
      // URL doesn't survive to be re-applied once the sweep captures versions.
      mockRoute.query = { base: '999', target: 'nonsense' }
      const { publish } = setup({
        validVersions: () => [],
        defaults: () => ({ base: 'current', target: 'current' }),
      })
      publish()
      expect(mockReplace).toHaveBeenCalledWith({
        query: { base: 'current', target: 'current' },
      })
    })
  })

  describe('type coercion', () => {
    // parseSide guarantees numbers for URL-sourced values, but sides also come
    // from callers. A numeric STRING matches no <option> and blanks the control,
    // so the invariant is enforced on every write rather than per caller.
    it('coerces a numeric string passed to select', () => {
      const { base, select } = setup()
      select({ base: '2' as unknown as Side })
      expect(base.value).toBe(2)
      expect(typeof base.value).toBe('number')
    })

    it('coerces a numeric string coming from the view defaults', () => {
      const { base } = setup({
        defaults: () => ({ base: '3' as unknown as Side, target: 'current' }),
      })
      expect(base.value).toBe(3)
      expect(typeof base.value).toBe('number')
    })

    it('leaves the current sentinel alone', () => {
      const { base, select } = setup()
      select({ base: 'current' })
      expect(base.value).toBe('current')
    })
  })

  describe('resetToDefaults', () => {
    it('resets both sides to the defaults, ignoring the URL', () => {
      // The post-restore path: the version list changed underneath, so the URL's
      // pair must NOT be resurrected.
      mockRoute.query = { base: '1', target: '2' }
      const { base, target, resetToDefaults } = setup()
      expect(base.value).toBe(1)

      resetToDefaults()
      expect(base.value).toBe(3)
      expect(target.value).toBe('current')
    })

    it('does not write or recompute on its own', () => {
      // The view publishes and recomputes explicitly after resetting, so the
      // reset itself must stay side-effect free.
      const { resetToDefaults, onChange } = setup()
      resetToDefaults()
      expect(mockReplace).not.toHaveBeenCalled()
      expect(onChange).not.toHaveBeenCalled()
    })
  })

  describe('external navigation', () => {
    it('re-seeds and recomputes on an external query change', async () => {
      const { base, target, onChange } = setup()
      mockRoute.query = { base: '1', target: '2' }
      await nextTick()
      expect(base.value).toBe(1)
      expect(target.value).toBe(2)
      expect(onChange).toHaveBeenCalledTimes(1)
    })

    it('ignores the echo of its own write (no recompute, no loop)', async () => {
      const { select, onChange } = setup()
      select({ base: 1 })
      expect(onChange).toHaveBeenCalledTimes(1)

      await nextTick()
      // The write mutated route.query, firing the watcher. The signature guard
      // must recognize it as our own and not recompute or write again.
      expect(onChange).toHaveBeenCalledTimes(1)
      expect(mockReplace).toHaveBeenCalledTimes(1)
    })

    it('settles after repeated selections rather than looping', async () => {
      const { select, base } = setup()
      select({ base: 1 })
      select({ base: 2 })
      select({ base: 3 })
      await nextTick()
      await nextTick()
      expect(base.value).toBe(3)
      expect(mockReplace).toHaveBeenCalledTimes(3)
    })

    it('falls back to defaults when navigated to a malformed query', async () => {
      mockRoute.query = { base: '1', target: '2' }
      const { base, target } = setup()
      expect(base.value).toBe(1)

      mockRoute.query = { base: '999', target: 'nonsense' }
      await nextTick()
      expect(base.value).toBe(3)
      expect(target.value).toBe('current')
    })

    it('ignores changes to unrelated query params', async () => {
      const { onChange } = setup()
      mockRoute.query = { ...mockRoute.query, q: 'search text' }
      await nextTick()
      expect(onChange).not.toHaveBeenCalled()
    })
  })
})
