import { describe, it, expect } from 'vitest'
import router from './index'

/**
 * The two document route shapes correspond to the two document kinds in
 * data-entry.yaml: entity-anchored (`entity_type:` set) and standalone
 * (no `entity_type:`, reachable from a `document:` navigation entry).
 *
 * Route ORDER matters here — `/document/:name/:entityId` must be declared
 * before `/document/:name`, or the two-segment URL could resolve against the
 * one-segment route. These tests pin the resolution, not just the existence.
 */
describe('document routes', () => {
  it('resolves an entity-anchored document to both params', () => {
    const r = router.resolve('/document/spec/TKT-9')
    expect(r.name).toBe('document')
    expect(r.params).toEqual({ name: 'spec', entityId: 'TKT-9' })
  })

  it('resolves a standalone document with no entityId', () => {
    const r = router.resolve('/document/sales_review')
    expect(r.name).toBe('standalone-document')
    expect(r.params.name).toBe('sales_review')
    expect(r.params.entityId).toBeUndefined()
  })

  it('passes params through as props for both shapes', () => {
    for (const path of ['/document/spec/TKT-9', '/document/sales_review']) {
      const matched = router.resolve(path).matched
      expect(matched.length).toBeGreaterThan(0)
      expect(matched[matched.length - 1].props.default).toBe(true)
    }
  })
})
