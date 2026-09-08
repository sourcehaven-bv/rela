import { computed, type Ref } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { useSchemaStore } from '@/stores/schema'

/**
 * Where a list's "create" button goes, and whether it may be shown.
 *
 * ## Why a create button has a world of its own
 *
 * A create always writes the entity's DEFAULT face: it names no face, and a
 * world is a read-side routing rule that no write passes through. So under a
 * filtering `default_world` the new entity has no face in the world on screen,
 * and landing the author there showed "not in this world" for something they
 * had just made. That is how a demo ended up with POL-002 AND POL-003: the
 * natural response to "not found" is to create it again.
 *
 * `create_world` on the LIST says which world to open the form in. It is on the
 * list's create button rather than on the form because the form is generic —
 * the same `new_policy` form is reachable from several places, and which face a
 * new entity starts in is a property of the workflow that opened it, not of the
 * field layout. The form receives the world as a runtime parameter and carries
 * it onto the post-create redirect.
 *
 * ## Why readability is checked against the TARGET world
 *
 * World-read is a global, role-level grant the SPA already holds from
 * `/_schema`.worlds, so this costs no request. Gating on the ambient world
 * would be the wrong question: the button lands somewhere else. Offering a link
 * into a world the principal cannot read produces an empty page that reads as
 * "nothing is there" rather than as the denial it is.
 *
 * An unknown world name reads as READABLE, matching `schemaStore.worldReadable`
 * — absence means "unknown", and the server is the authority that will refuse.
 */
export function useCreateTarget(
  createForm: Ref<string | undefined>,
  createWorld: Ref<string | undefined>,
  ambientWorld: Ref<string | undefined>,
) {
  const schemaStore = useSchemaStore()

  /** The world the form opens in: the list's `create_world`, else the ambient one. */
  const targetWorld = computed(() => createWorld.value || ambientWorld.value)

  /** Whether the principal may read the world the button LANDS in. */
  const targetReadable = computed(() => {
    const target = targetWorld.value
    if (!target) return true
    return schemaStore.worldReadable(target)
  })

  /** The route to push, or null when this list has no create form. */
  const target = computed<RouteLocationRaw | null>(() => {
    if (!createForm.value) return null
    return {
      path: `/form/${createForm.value}`,
      query: targetWorld.value ? { world: targetWorld.value } : {},
    }
  })

  return { targetWorld, targetReadable, target }
}
