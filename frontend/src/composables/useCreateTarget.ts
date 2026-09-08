import { computed, type Ref } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

/**
 * Where a list's "create" button goes.
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
 * ## Whether the button SHOWS is the server's answer, not this composable's
 *
 * The list response's `_actions.create` is the create verdict; this only
 * computes the destination. An earlier revision also checked the target
 * world's readability from `/_schema` and hid the button on that basis — a
 * second, client-side gate re-deriving something the server already decides,
 * which is the pattern that made every world-bound page read-only (atlas
 * worlds issue 3). A world the principal cannot read answers with its ordinary
 * empty result on arrival, the same as a typed URL.
 */
export function useCreateTarget(
  createForm: Ref<string | undefined>,
  createWorld: Ref<string | undefined>,
  ambientWorld: Ref<string | undefined>,
) {
  /** The world the form opens in: the list's `create_world`, else the ambient one. */
  const targetWorld = computed(() => createWorld.value || ambientWorld.value)

  /** The route to push, or null when this list has no create form. */
  const target = computed<RouteLocationRaw | null>(() => {
    if (!createForm.value) return null
    return {
      path: `/form/${createForm.value}`,
      query: targetWorld.value ? { world: targetWorld.value } : {},
    }
  })

  return { targetWorld, target }
}
