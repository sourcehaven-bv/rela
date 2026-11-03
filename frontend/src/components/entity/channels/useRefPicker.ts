import { computed, ref } from 'vue'
import type { Entity } from '@/types'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import type { ChannelReadableRef } from 'rela-components/components/channel/types'

/**
 * The promise plumbing between the composer's `pickRef` prop and the entity
 * picker modal. `pickRef` opens the picker for the allowed types and
 * settles when the user picks (`onPicked`) or closes it (`onClose`). A
 * second `pickRef` while one is open settles the first with null.
 */
export function useRefPicker() {
  const pending = ref<{ types: string[]; resolve: (r: ChannelReadableRef | null) => void } | null>(
    null
  )

  function settle(r: ChannelReadableRef | null) {
    const p = pending.value
    pending.value = null
    p?.resolve(r)
  }

  function pickRef(to: string[]): Promise<ChannelReadableRef | null> {
    settle(null)
    return new Promise((resolve) => {
      pending.value = { types: to, resolve }
    })
  }

  function onPicked(_id: string, entity: Entity) {
    settle({ id: entity.id, type: entity.type, title: entityDisplayTitle(entity) })
  }

  return {
    /** Whether the picker is open. */
    open: computed(() => pending.value !== null),
    /** The entity types the open picker allows. */
    types: computed(() => pending.value?.types),
    pickRef,
    onPicked,
    onClose: () => settle(null),
  }
}
