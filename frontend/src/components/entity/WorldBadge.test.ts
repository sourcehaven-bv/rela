import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import WorldBadge from './WorldBadge.vue'
import type { EntityWorld } from '@/types'

// WorldBadge is an EXCEPTION marker: it renders ONLY when the world served a
// stand-in, and nothing at all when the reader is looking at the face the
// world asked for. A badge on every row is noise, and noise trains people to
// ignore it — so the presence of the badge is what carries the meaning.
//
// The regression these tests exist for (WORLDS-DEMO-ISSUES, blocker 1): a
// draft-only policy served under `?world=published` reported `via: 'chain'`,
// byte-identical to a genuine published hit, so the badge said nothing and
// the page framed draft content as published. `chain_position` carries the
// missing fact, and a position > 0 must read exactly like `fallback-default`.

const world = (over: Partial<EntityWorld>): EntityWorld => ({
  name: 'published',
  face: 'published',
  via: 'chain',
  ...over,
})

describe('WorldBadge substitute detection', () => {
  it('renders NOTHING for a first-choice chain hit', () => {
    // The rule, pinned directly: chain_position 0 is the face the world would
    // normally give you, so there is nothing to warn about and no badge.
    const w = mount(WorldBadge, {
      props: { world: world({ chain_position: 0 }) },
    })
    expect(w.find('.world-badge').exists()).toBe(false)
    expect(w.text()).toBe('')
  })

  it('flags a WITHIN-CHAIN fallback as a substitute', () => {
    // THE reported bug: `via` is still 'chain', because the chain did match —
    // its second element. Only the position separates this from the case
    // above.
    const w = mount(WorldBadge, {
      props: { world: world({ face: 'draft', chain_position: 1 }) },
    })
    expect(w.find('.world-badge').exists()).toBe(true)
    expect(w.find('.world-badge').classes()).toContain('is-fallback')
    expect(w.text()).toBe('draft')
    expect(w.find('.world-badge').attributes('title')).toBe(
      'No published face exists for this entity — showing draft instead',
    )
  })

  it('still flags the otherwise:default arm', () => {
    const w = mount(WorldBadge, {
      props: { world: world({ face: '', via: 'fallback-default' }) },
    })
    expect(w.find('.world-badge').exists()).toBe(true)
    expect(w.find('.world-badge').classes()).toContain('is-fallback')
    expect(w.text()).toBe('default')
    expect(w.find('.world-badge').attributes('title')).toBe(
      'No published face exists for this entity — showing the default state instead',
    )
  })

  it('treats a missing chain_position as a first-choice hit', () => {
    // An older server omits the field. `undefined > 0` is false, so the badge
    // stays quiet rather than inventing a warning it has no evidence for —
    // the pre-existing behaviour, deliberately unchanged.
    const w = mount(WorldBadge, { props: { world: world({}) } })
    expect(w.find('.world-badge').exists()).toBe(false)
  })

  it('renders nothing for unscoped', () => {
    const w = mount(WorldBadge, {
      props: { world: world({ via: 'unscoped', face: '' }) },
    })
    expect(w.find('.world-badge').exists()).toBe(false)
  })

  it('renders nothing with no world at all', () => {
    const w = mount(WorldBadge, { props: {} })
    expect(w.find('.world-badge').exists()).toBe(false)
  })

  it('never renders the removed non-substitute state', () => {
    // `is-chain` styled a quiet first-choice badge and was removed with it.
    // A reintroduction would put a badge back on every row, which is exactly
    // what this rule exists to prevent.
    for (const w of [world({ chain_position: 0 }), world({ chain_position: 3, face: 'draft' })]) {
      expect(mount(WorldBadge, { props: { world: w } }).find('.is-chain').exists()).toBe(false)
    }
  })
})
