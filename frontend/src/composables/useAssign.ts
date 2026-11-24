import { ref, type Ref } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import { postAssign, type AssignSource } from '@/api/assign'
import { getErrorMessage } from '@/api/errors'
import { t } from '@/i18n'
import { assignTargetForHref, hasAssignDrag, readAssignDrag } from '@/utils/assignDrag'
import type { NavDropAttrs } from '@/components/common/sidebarNav'

/**
 * Applies an `assign:` block to entities and reports the outcome in one
 * toast: refused rows, and rows that still do not match the list's condition.
 */
export function useAssign() {
  const uiStore = useUIStore()
  async function assign(src: AssignSource, addresses: string[]): Promise<boolean> {
    try {
      const results = await postAssign(src, addresses)
      const refused = results.filter((r) => !r.ok).length
      const outside = results.filter((r) => r.ok && r.matches === false).length
      if (refused) {
        uiStore.showToast('error', t('assign.refused', { count: refused }, refused))
      } else if (outside) {
        uiStore.showToast('warning', t('assign.notInList', { count: outside }, outside))
      } else {
        uiStore.showToast('success', t('assign.done', { count: results.length }, results.length))
      }
      return refused === 0
    } catch (err) {
      uiStore.showToast('error', getErrorMessage(err))
      return false
    }
  }
  return { assign }
}

/**
 * Drop handlers for sidebar entries whose list or page has an `assign:`. The
 * entry accepts a drag only while it carries rows; the item type is checked on
 * drop, since a drag's payload is unreadable before it.
 */
export function useAssignDrop(): { dropAttrs: NavDropAttrs; dropOver: Ref<string | null> } {
  const schemaStore = useSchemaStore()
  const { assign } = useAssign()
  const dropOver = ref<string | null>(null)
  const lookup = {
    listAssignType: (id: string) => {
      const l = schemaStore.getList(id)
      return l?.assign ? l.entity : undefined
    },
    pageAssignType: (id: string) => schemaStore.getPageConfig(id)?.assign?.entity_type,
  }
  const dropAttrs: NavDropAttrs = (href) => {
    const target = assignTargetForHref(href, lookup)
    if (!target) return undefined
    return {
      'data-assign-drop': dropOver.value === href ? 'over' : 'idle',
      onDragover: (e: DragEvent) => {
        if (!hasAssignDrag(e.dataTransfer)) return
        e.preventDefault()
        if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
        dropOver.value = href
      },
      onDragleave: () => {
        if (dropOver.value === href) dropOver.value = null
      },
      onDrop: (e: DragEvent) => {
        dropOver.value = null
        const p = readAssignDrag(e.dataTransfer)
        if (!p) return
        e.preventDefault()
        if (p.entityType !== target.entityType) {
          useUIStore().showToast('warning', t('assign.wrongType'))
          return
        }
        void assign(target.src, p.addresses)
      },
    }
  }
  return { dropAttrs, dropOver }
}
