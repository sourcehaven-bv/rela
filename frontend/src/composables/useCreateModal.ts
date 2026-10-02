import { ref } from 'vue'
import type { Entity } from '@/types'
import { shouldDeferToBrowser } from '@/utils/openIntent'

/**
 * A collection view's New button, opened as a dialog over the view rather
 * than as a navigation to the create form's page.
 *
 * The button stays a link to that page, so a modified or middle click still
 * opens the full form in a new tab; only a plain click is taken over. Bind
 * `onClick` with `@click.capture`: RouterLink's own handler runs first in the
 * bubble phase and would navigate before this one could cancel it.
 *
 * Bind `add-another` on the dialog with `createdAnother` for its event: a
 * collection has no link step, so more than one record per opening is fine.
 */
export function useCreateModal(onCreated: (entity: Entity) => void, onCreatedAnother?: (entity: Entity) => void) {
  const open = ref(false)

  function onClick(event: MouseEvent) {
    if (shouldDeferToBrowser(event)) return
    event.preventDefault()
    event.stopPropagation()
    open.value = true
  }

  function show() {
    open.value = true
  }

  function close() {
    open.value = false
  }

  function created(entity: Entity) {
    open.value = false
    onCreated(entity)
  }

  // "Create & add another": the dialog stays open for the next record, so the
  // host only refreshes and does not open the entity.
  function createdAnother(entity: Entity) {
    onCreatedAnother?.(entity)
  }

  return { open, onClick, show, close, created, createdAnother }
}
