import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useUIStore } from '@/stores'
import { deleteEntity, restoreEntity, getErrorMessage, ApiError } from '@/api'
import { isTextEntryFocused } from '@/utils/dom'
import { isAnyModalOpen } from './modalStack'

/** One row to delete: its bare id (the selection key) and its write address. */
export interface DeleteTarget {
  id: string
  type: string
  /** What the DELETE goes to: the id, or `ID@face` for a row served at a face. */
  ref: string
}

export interface BulkDeleteNoun {
  singular: string
  plural?: string
}

interface UseBulkDeleteOptions {
  /** What the toasts count, in the schema's words ("3 features"). */
  noun: () => BulkDeleteNoun
  /** Called before the requests go out, so the rows can leave the list at once. */
  onStart?: (ids: string[]) => void
  /**
   * Called once every delete has answered and the list has been refetched.
   * `failed` rows still exist on the server; the caller keeps them selected.
   */
  onSettled: (result: { deleted: string[]; failed: string[] }) => unknown
  /** Refetch the list. Runs after a delete, before `onSettled`, and after an undo. */
  refresh: () => unknown
}

/** How long the Undo toast stays up. The server keeps a deleted entity for 60s. */
export const UNDO_TOAST_MS = 10000

/** How many ids an error toast names before it summarises the rest. */
const NAMED_LIMIT = 5

function countLabel(count: number, noun: BulkDeleteNoun): string {
  const word = count === 1 ? noun.singular : noun.plural || `${noun.singular}s`
  return `${count} ${word}`
}

function nameIds(ids: string[]): string {
  if (ids.length <= NAMED_LIMIT) return ids.join(', ')
  return `${ids.slice(0, NAMED_LIMIT).join(', ')} and ${ids.length - NAMED_LIMIT} more`
}

function restoreErrorMessage(err: unknown): string {
  // A 404 means the server no longer holds the entity for restoring: the
  // grace period has passed, or the server has no soft delete at all.
  if (err instanceof ApiError && err.status === 404) return 'it can no longer be restored'
  return getErrorMessage(err, 'restore failed')
}

/**
 * Delete several rows at once, without a confirm, and offer Undo in a toast.
 *
 * There is no confirm because the delete is reversible: the server marks the
 * entity and only removes it for real after a grace period, and Undo calls
 * the restore endpoint for each deleted row within it.
 */
export function useBulkDelete(options: UseBulkDeleteOptions) {
  const uiStore = useUIStore()
  const deleting = ref(false)

  async function restoreMany(targets: DeleteTarget[]) {
    const results = await Promise.allSettled(targets.map((t) => restoreEntity(t.type, t.ref)))
    const failed: string[] = []
    let firstError: unknown
    results.forEach((r, i) => {
      if (r.status === 'rejected') {
        failed.push(targets[i].id)
        firstError ??= r.reason
      }
    })
    await options.refresh()

    const restoredCount = targets.length - failed.length
    if (restoredCount > 0) {
      uiStore.success(`Restored ${countLabel(restoredCount, options.noun())}`)
    }
    if (failed.length > 0) {
      uiStore.error(`Could not restore ${nameIds(failed)}: ${restoreErrorMessage(firstError)}`)
    }
  }

  async function deleteMany(targets: DeleteTarget[]) {
    if (targets.length === 0 || deleting.value) return
    deleting.value = true
    try {
      options.onStart?.(targets.map((t) => t.id))
      const results = await Promise.allSettled(targets.map((t) => deleteEntity(t.type, t.ref)))

      const deleted: DeleteTarget[] = []
      const failed: string[] = []
      let firstError: unknown
      results.forEach((r, i) => {
        if (r.status === 'fulfilled') {
          deleted.push(targets[i])
        } else {
          failed.push(targets[i].id)
          firstError ??= r.reason
        }
      })

      await options.refresh()
      await options.onSettled({ deleted: deleted.map((t) => t.id), failed })

      if (deleted.length > 0) {
        uiStore.showToast('success', `Deleted ${countLabel(deleted.length, options.noun())}`, UNDO_TOAST_MS, {
          label: 'Undo',
          onAction: () => void restoreMany(deleted),
        })
      }
      if (failed.length > 0) {
        uiStore.error(`Could not delete ${nameIds(failed)}: ${getErrorMessage(firstError, 'delete failed')}`)
      }
    } finally {
      deleting.value = false
    }
  }

  return { deleting, deleteMany, restoreMany }
}

/**
 * Delete and Backspace run `onDelete` while `enabled()` holds, except when
 * focus is in a text field or editable element, or a modal is open.
 * Backspace is included because Mac keyboards have no Delete key.
 */
export function useDeleteKey(options: { enabled: () => boolean; onDelete: () => void }) {
  function handleKeydown(e: KeyboardEvent) {
    if (e.key !== 'Delete' && e.key !== 'Backspace') return
    if (e.metaKey || e.ctrlKey || e.altKey) return
    if (isTextEntryFocused() || isAnyModalOpen()) return
    if (!options.enabled()) return
    e.preventDefault()
    options.onDelete()
  }

  onMounted(() => document.addEventListener('keydown', handleKeydown))
  onBeforeUnmount(() => document.removeEventListener('keydown', handleKeydown))
}
