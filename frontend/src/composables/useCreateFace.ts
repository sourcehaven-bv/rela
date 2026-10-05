import { computed, ref, watch, type Ref } from 'vue'
import { listEntities } from '@/api'
import { useSchemaStore } from '@/stores/schema'
import type { EntityType } from '@/types'

/**
 * The face a create form writes when its world names none.
 *
 * A faced type has no implicit face, so a create must land on a declared
 * face. A world with `create:` names it; otherwise the form asks. The faces
 * offered are the ones the collection's `_actions` marks creatable
 * (`create@<face>`). Like every `_actions` key this is a hint: an absent key
 * is offered, and the server re-authorizes the write.
 */
export function useCreateFace(
  typeName: Ref<string | undefined>,
  entityType: Ref<EntityType | undefined>,
  // False in edit mode, or when the host pins the face of an embedded form.
  active: Ref<boolean>,
  // The world the create is issued from; '' or undefined for the default.
  world: Ref<string | undefined>,
) {
  const schemaStore = useSchemaStore()
  const face = ref('')
  const actions = ref<Record<string, boolean> | undefined>(undefined)

  const declared = computed(() => Object.keys(entityType.value?.faces ?? {}))

  const needsFace = computed(() => {
    if (!active.value || declared.value.length === 0) return false
    const name = world.value || schemaStore.defaultWorld
    return !schemaStore.worlds.get(name)?.create
  })

  const faces = computed(() => pickableFaces(declared.value, actions.value))

  watch(
    [needsFace, typeName],
    async ([need, type]) => {
      if (!need || !type) return
      try {
        actions.value = (await listEntities(type, { per_page: 1 }))._actions
      } catch {
        // Fail open: offer every declared face and let the server decide.
        actions.value = undefined
      }
    },
    { immediate: true },
  )

  watch(faces, (list) => {
    if (!list.includes(face.value)) face.value = list.length === 1 ? list[0] : ''
  })

  return { needsFace, faces, face }
}

/** The declared faces not marked uncreatable by `create@<face>`. */
export function pickableFaces(declared: string[], actions?: Record<string, boolean>): string[] {
  return declared.filter((f) => actions?.[`create@${f}`] !== false)
}
