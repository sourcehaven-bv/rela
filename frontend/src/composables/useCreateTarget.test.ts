import { describe, it, expect } from 'vitest'
import { ref } from 'vue'
import { useCreateTarget } from './useCreateTarget'

// A create always writes the entity's DEFAULT face — it names no face, and no
// world reaches a write — so under a filtering `default_world` a newly created
// entity has no face in the world on screen. Landing the author there showed "not in
// this world" for something they had just made.
//
// Whether the button SHOWS is `_actions.create` on the list response; this
// composable only computes where it goes.
describe('useCreateTarget', () => {
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
})
