import { ref, onMounted, onBeforeUnmount, computed, type Ref } from 'vue'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { updateEntity } from '@/api/entities'
import { runAction } from '@/api/actions'
import { isInputFocused } from '@/utils/dom'
import { isAnyModalOpen } from './modalStack'
import type { ActionConfig, Entity } from '@/types'

interface UseListActionsOptions {
  listId: Ref<string>
  selectedIds: Ref<Set<string>>
  entities: Ref<Entity[]>
  onClearSelection: () => void
  onRequestConfirm: (action: ActionConfig, actionId: string) => void
  onComplete: () => void
  /** Remove entity IDs from the local list optimistically (for transition animation). */
  onRemoveEntities?: (ids: string[]) => void
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

  /** Resolve action configs for the current list. */
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

  /** Interpolate template variables in set values. */
  function interpolate(value: string): string {
    return value.replace(/\{\{today\}\}/g, new Date().toISOString().slice(0, 10))
  }

  /** Execute a confirmed action against all selected entities. */
  async function executeAction(actionId: string, action: ActionConfig) {
    const ids = Array.from(options.selectedIds.value)
    if (ids.length === 0) return

    processing.value = true
    let successCount = 0
    let errorCount = 0

    const list = schemaStore.getList(options.listId.value)
    const entityType = list?.entity || ''

    for (const entityId of ids) {
      try {
        if (action.set) {
          // Declarative property mutation via PATCH
          const properties: Record<string, unknown> = {}
          for (const [prop, val] of Object.entries(action.set)) {
            properties[prop] = interpolate(val)
          }
          await updateEntity(entityType, entityId, { properties })
        } else if (action.script) {
          // Lua script action with entity context
          await runAction(actionId, entityId, entityType)
        }
        successCount++
      } catch {
        errorCount++
      }
    }

    processing.value = false

    if (errorCount > 0) {
      uiStore.error(`${action.label}: ${errorCount} failed, ${successCount} succeeded`)
    } else {
      uiStore.success(`${action.label}: ${successCount} updated`)
    }

    const selectedCopy = [...ids]
    options.onClearSelection()

    // Optimistically remove rows so TransitionGroup can animate them out,
    // then do a full re-fetch after the animation completes.
    if (options.onRemoveEntities && successCount > 0) {
      options.onRemoveEntities(selectedCopy)
      setTimeout(() => {
        entitiesStore.invalidateAll()
        options.onComplete()
      }, 350) // match CSS transition duration
    } else {
      entitiesStore.invalidateAll()
      options.onComplete()
    }
  }

  /** Trigger an action — either immediately or via confirmation. */
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
