import { ref, onMounted, onBeforeUnmount, computed, type Ref } from 'vue'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { updateEntity } from '@/api/entities'
import { runAction } from '@/api/actions'
import { isInputFocused } from '@/utils/dom'
import { isAnyModalOpen } from './modalStack'
import type { ActionConfig, Entity } from '@/types'

/** Must match the CSS .row-leave-active transition duration in EntityList.vue. */
const ROW_ANIMATION_MS = 350

interface UseListActionsOptions {
  listId: Ref<string>
  selectedIds: Ref<Set<string>>
  entities: Ref<Entity[]>
  onClearSelection: () => void
  onRequestConfirm: (action: ActionConfig, actionId: string) => void
  onComplete: () => void
}

/**
 * Composable for handling keyboard-triggered list actions.
 * Registers keydown handlers for configured action keys and applies
 * mutations (set or script) to all selected entities.
 */
export function useListActions(options: UseListActionsOptions) {
  const schemaStore = useSchemaStore()
  const entitiesStore = useEntitiesStore()
  const uiStore = useUIStore()
  const processing = ref(false)

  const resolvedActions = computed(() => {
    const list = schemaStore.getList(options.listId.value)
    if (!list?.actions) return []
    const result: { id: string; config: ActionConfig }[] = []
    for (const actionId of list.actions) {
      const config = schemaStore.getAction(actionId)
      if (config) {
        result.push({ id: actionId, config })
      }
    }
    return result
  })

  function interpolate(value: string): string {
    return value.replace(/\{\{today\}\}/g, new Date().toISOString().slice(0, 10))
  }

  async function executeAction(actionId: string, action: ActionConfig) {
    const ids = Array.from(options.selectedIds.value)
    if (ids.length === 0) return

    processing.value = true

    const list = schemaStore.getList(options.listId.value)
    const entityType = list?.entity || ''

    const results = await Promise.allSettled(
      ids.map((entityId) => {
        if (action.set) {
          const properties: Record<string, unknown> = {}
          for (const [prop, val] of Object.entries(action.set)) {
            properties[prop] = interpolate(val)
          }
          return updateEntity(entityType, entityId, { properties })
        }
        return runAction(actionId, entityId, entityType)
      }),
    )

    const successCount = results.filter((r) => r.status === 'fulfilled').length
    const errorCount = results.length - successCount

    processing.value = false

    if (errorCount > 0) {
      uiStore.error(`${action.label}: ${errorCount} failed, ${successCount} succeeded`)
    } else {
      uiStore.success(`${action.label}: ${successCount} updated`)
    }

    options.onClearSelection()

    // Optimistically remove affected rows so TransitionGroup can animate
    // them out, then do a full re-fetch after the animation completes.
    if (successCount > 0) {
      const removed = new Set(ids)
      options.entities.value = options.entities.value.filter((e) => !removed.has(e.id))
      setTimeout(() => {
        entitiesStore.invalidateAll()
        options.onComplete()
      }, ROW_ANIMATION_MS)
    } else {
      entitiesStore.invalidateAll()
      options.onComplete()
    }
  }

  function triggerAction(actionId: string, action: ActionConfig) {
    if (options.selectedIds.value.size === 0) return
    if (processing.value) return

    if (action.confirm) {
      options.onRequestConfirm(action, actionId)
    } else {
      executeAction(actionId, action)
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (isInputFocused()) return
    if (isAnyModalOpen()) return
    if (processing.value) return
    if (options.selectedIds.value.size === 0) return

    for (const { id, config } of resolvedActions.value) {
      if (config.key === e.key) {
        e.preventDefault()
        triggerAction(id, config)
        return
      }
    }
  }

  onMounted(() => {
    document.addEventListener('keydown', handleKeydown)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('keydown', handleKeydown)
  })

  return {
    resolvedActions,
    processing,
    executeAction,
    triggerAction,
  }
}
