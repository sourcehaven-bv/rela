import { describe, it, expect, beforeEach } from 'vitest'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { useCreateTarget } from './useCreateTarget'
import { useSchemaStore } from '@/stores/schema'

// A create always writes the entity's DEFAULT face — it names no face, and no
// world reaches a write — so under a filtering `default_world` a newly created
// entity has no face in the world on screen. Landing the author there showed "not in
// this world" for something they had just made.
describe('useCreateTarget', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const schema = useSchemaStore()
    schema.worlds.set('published', { readable: true } as never)
    schema.worlds.set('preview', { readable: true } as never)
    schema.worlds.set('secret', { readable: false } as never)
  })

  it('opens the form in create_world, not the world on screen', () => {
    const { target, targetWorld } = useCreateTarget(
      ref('new_policy'), ref('preview'), ref('published'),
    )
    expect(targetWorld.value).toBe('preview')
    expect(target.value).toEqual({ path: '/form/new_policy', query: { world: 'preview' } })
  })

  it('falls back to the ambient world when create_world is unset', () => {
    const { target, targetWorld } = useCreateTarget(
      ref('new_policy'), ref(undefined), ref('published'),
    )
    expect(targetWorld.value).toBe('published')
    expect(target.value).toEqual({ path: '/form/new_policy', query: { world: 'published' } })
  })

  it('omits the query entirely in the default world', () => {
    const { target } = useCreateTarget(ref('new_policy'), ref(undefined), ref(''))
    expect(target.value).toEqual({ path: '/form/new_policy', query: {} })
  })

  it('is null when the list declares no create form', () => {
    const { target } = useCreateTarget(ref(undefined), ref('preview'), ref('published'))
    expect(target.value).toBeNull()
  })

  describe('readability is checked against the world the button LANDS in', () => {
    it('denies when the target world is unreadable, even though the ambient one is readable', () => {
      // Gating on the ambient world would wrongly show the button, and the
      // author would land on an empty page that reads as "nothing is there"
      // rather than as the denial it is.
      const { targetReadable } = useCreateTarget(
        ref('new_policy'), ref('secret'), ref('published'),
      )
      expect(targetReadable.value).toBe(false)
    })

    it('allows when the target world is readable, even though the ambient one is not', () => {
      const { targetReadable } = useCreateTarget(
        ref('new_policy'), ref('preview'), ref('secret'),
      )
      expect(targetReadable.value).toBe(true)
    })

    it('treats an unknown world as readable — the server is the authority', () => {
      const { targetReadable } = useCreateTarget(
        ref('new_policy'), ref('not-declared'), ref('published'),
      )
      expect(targetReadable.value).toBe(true)
    })

    it('allows the default world', () => {
      const { targetReadable } = useCreateTarget(ref('new_policy'), ref(undefined), ref(''))
      expect(targetReadable.value).toBe(true)
    })
  })
})
