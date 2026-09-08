import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import WorldBadge from './WorldBadge.vue'
import { useSchemaStore } from '@/stores/schema'
import type { EntityWorld } from '@/types'

// WorldBadge is an EXCEPTION marker: it renders ONLY when the world served a
// stand-in, and nothing at all when the reader is looking at the face the
// world asked for. A badge on every row is noise, and noise trains people to
// ignore it — so the presence of the badge is what carries the meaning.
//
// WHETHER a row is a stand-in is the server's answer (`_world`); WHAT the
// badge says is the operator's `messages.stand_in` for the world, and with
// nothing declared nothing renders (TKT-5SZG2L). The badge has no words of
// its own — the coordinate names and tooltips it used to print were rela's
// storage vocabulary shown to a reader.
//
// The regression the substitute detection exists for (WORLDS-DEMO-ISSUES,
// blocker 1): a draft-only policy served under `?world=published` reported
// `via: 'chain'`, byte-identical to a genuine published hit. `chain_position`
// carries the missing fact, and a position > 0 must read exactly like
// `fallback-default`.

const world = (over: Partial<EntityWorld>): EntityWorld => ({
  name: 'published',
  face: 'published',
  via: 'chain',
  ...over,
})

function seed(standIn?: string) {
  const store = useSchemaStore()
  store.worlds.set('published', { readable: true, messages: standIn ? { stand_in: standIn } : undefined } as never)
  store.entityTypes.set('policy', {
    name: 'policy',
    label: 'Policy',
    properties: {},
    faces: { draft: { label: 'Concept' }, published: { label: 'Vastgesteld' } },
    bare_face: 'draft',
  } as never)
}

function badge(w: EntityWorld | undefined) {
  return mount(WorldBadge, { props: { world: w, entityType: 'policy' } })
}

describe('WorldBadge', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  describe('says only what the operator declared', () => {
    it('renders NOTHING for a stand-in when the world declares no stand_in text', () => {
      seed()
      const w = badge(world({ face: 'draft', chain_position: 1 }))
      expect(w.find('.world-badge').exists()).toBe(false)
      expect(w.text()).toBe('')
    })

    it('renders the declared text with {face} as the served face LABEL', () => {
      seed('Nog {face}')
      const w = badge(world({ face: 'draft', chain_position: 1 }))
      expect(w.find('.world-badge').exists()).toBe(true)
      expect(w.find('.world-badge').classes()).toContain('is-fallback')
      expect(w.text()).toBe('Nog Concept')
      // No tooltip: it was a rela sentence.
      expect(w.find('.world-badge').attributes('title')).toBeUndefined()
    })

    it('renders the declared text on the otherwise:default arm too', () => {
      seed('{bare_face}')
      const w = badge(world({ face: '', via: 'fallback-default' }))
      expect(w.text()).toBe('Concept')
    })
  })

  describe('substitute detection', () => {
    // Constant text: the arm under test may serve a face with no label, and
    // an empty `{face}` would hide the badge for a reason unrelated to
    // detection.
    beforeEach(() => seed('stand-in'))

    it('renders NOTHING for a first-choice chain hit', () => {
      // chain_position 0 is the face the world would normally give you, so
      // there is nothing to warn about and no badge — even with text declared.
      const w = badge(world({ chain_position: 0 }))
      expect(w.find('.world-badge').exists()).toBe(false)
    })

    it('flags a WITHIN-CHAIN fallback as a substitute', () => {
      // THE reported bug: `via` is still 'chain', because the chain did match —
      // its second element. Only the position separates this from the case
      // above.
      const w = badge(world({ face: 'draft', chain_position: 1 }))
      expect(w.find('.world-badge').classes()).toContain('is-fallback')
    })

    it('still flags the otherwise:default arm', () => {
      const w = badge(world({ face: '', via: 'fallback-default' }))
      expect(w.find('.world-badge').classes()).toContain('is-fallback')
    })

    it('treats a missing chain_position as a first-choice hit', () => {
      // An older server omits the field. `undefined > 0` is false, so the badge
      // stays quiet rather than inventing a warning it has no evidence for.
      expect(badge(world({})).find('.world-badge').exists()).toBe(false)
    })

    it('renders nothing for unscoped', () => {
      expect(badge(world({ via: 'unscoped', face: '' })).find('.world-badge').exists()).toBe(false)
    })

    it('renders nothing with no world at all', () => {
      expect(badge(undefined).find('.world-badge').exists()).toBe(false)
    })
  })
})
