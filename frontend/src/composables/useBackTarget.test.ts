import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref } from 'vue'
import { useBackTarget } from './useBackTarget'

// Mock vue-router's useRoute to expose a mutable query object. The composable
// wraps its read in `computed`, so invalidation hinges on the ref reading
// through route.query.* at evaluation time.
const mockRouteQuery = ref<Record<string, unknown>>({})
vi.mock('vue-router', () => ({
  useRoute: () => ({
    get query() {
      return mockRouteQuery.value
    },
  }),
}))

describe('useBackTarget', () => {
  beforeEach(() => {
    mockRouteQuery.value = {}
  })

  describe('precedence', () => {
    it('returns null when no query params present', () => {
      const target = useBackTarget()
      expect(target.value).toBeNull()
    })

    it('uses return_to when present', () => {
      mockRouteQuery.value = { return_to: '/entity/ticket/TKT-001' }
      const target = useBackTarget()
      expect(target.value).toEqual({ to: '/entity/ticket/TKT-001', labelHint: null })
    })

    it('uses return_to ahead of from when both present', () => {
      mockRouteQuery.value = { return_to: '/document/release_notes/REL-1', from: 'all_tasks' }
      const target = useBackTarget()
      expect(target.value).toEqual({
        to: '/document/release_notes/REL-1',
        labelHint: null,
      })
    })

    it('uses from when return_to absent', () => {
      mockRouteQuery.value = { from: 'all_tasks' }
      const target = useBackTarget()
      expect(target.value).toEqual({
        to: '/list/all_tasks',
        labelHint: { kind: 'list', id: 'all_tasks' },
      })
    })

    it('falls through to from when return_to is unsafe (open-redirect guard)', () => {
      mockRouteQuery.value = { return_to: '//evil.com', from: 'all_tasks' }
      const target = useBackTarget()
      expect(target.value).toEqual({
        to: '/list/all_tasks',
        labelHint: { kind: 'list', id: 'all_tasks' },
      })
    })

    it('falls through to null when return_to is unsafe and no from', () => {
      mockRouteQuery.value = { return_to: '//evil.com' }
      const target = useBackTarget()
      expect(target.value).toBeNull()
    })
  })

  describe('open-redirect guard', () => {
    // The composable delegates to isSafeReturnPath / readReturnTo, which
    // already have an exhaustive test suite of their own. These cases are
    // the surface-level smoke test that we're actually calling the guard.
    it.each([
      '//evil.com',
      '/\\evil.com',
      '/%5Cevil.com',
      '/%5cevil.com',
      '/%2Fevil.com',
      '/%2fevil.com',
      'https://evil.com',
      'javascript:alert(1)',
      'mailto:a@b.c',
    ])('rejects %s', (hostile) => {
      mockRouteQuery.value = { return_to: hostile }
      const target = useBackTarget()
      expect(target.value).toBeNull()
    })
  })

  describe('edge cases', () => {
    it('ignores array-valued return_to (vue-router duplicate key)', () => {
      mockRouteQuery.value = { return_to: ['/a', '/b'] }
      const target = useBackTarget()
      expect(target.value).toBeNull()
    })

    it('ignores non-string from', () => {
      mockRouteQuery.value = { from: ['a', 'b'] }
      const target = useBackTarget()
      expect(target.value).toBeNull()
    })

    it('empty return_to falls through to from', () => {
      mockRouteQuery.value = { return_to: '', from: 'all' }
      const target = useBackTarget()
      expect(target.value).toEqual({
        to: '/list/all',
        labelHint: { kind: 'list', id: 'all' },
      })
    })

    it('preserves query + fragment on return_to value', () => {
      mockRouteQuery.value = {
        return_to: '/entity/category/backend?doc=overview#edit-tkt-1-0',
      }
      const target = useBackTarget()
      expect(target.value).toEqual({
        to: '/entity/category/backend?doc=overview#edit-tkt-1-0',
        labelHint: null,
      })
    })
  })

  describe('reactivity', () => {
    it('re-evaluates when route.query changes', () => {
      const target = useBackTarget()
      expect(target.value).toBeNull()
      mockRouteQuery.value = { return_to: '/a' }
      expect(target.value).toEqual({ to: '/a', labelHint: null })
      mockRouteQuery.value = { from: 'b' }
      expect(target.value).toEqual({
        to: '/list/b',
        labelHint: { kind: 'list', id: 'b' },
      })
    })
  })
})
