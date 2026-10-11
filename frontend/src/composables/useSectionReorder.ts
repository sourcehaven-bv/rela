import { ref, type Ref } from 'vue'
import type { RowMove } from 'rela-components/components/table/types'
import { moveRelation } from '@/api/entities'
import { getErrorMessage } from '@/api/errors'
import type { ViewResponse, ViewRow, ViewSection } from '@/api/views'
import { useUIStore } from '@/stores/ui'
import { createMoveQueue } from '@/utils/moveQueue'
import { orderMoveArgs, planRowMove } from '@/utils/relationOrder'

/**
 * What a section row is called in its move handle's label and in the move
 * announcement: its first cell, which is the column a reader scans, or its
 * id when that cell is empty.
 */
export function sectionRowLabel(row: ViewRow): string {
  return row.cells[0]?.values.filter(Boolean).join(', ') || row.entityId
}

/**
 * Moving rows of a detail-view section table shown in relation order.
 *
 * The same contract as `useListReorder`: the move shows at once, moves go
 * through a `createMoveQueue`, and the view is reloaded once the queue is
 * empty to show what the server stored. A failed move is reported and
 * undone by that reload. The view has no per-section read, so the reload
 * is the whole view.
 */
export function useSectionReorder(opts: {
  view: Ref<ViewResponse | null>
  reload: () => Promise<void>
}) {
  const ui = useUIStore()

  /**
   * What the live region last said about a keyboard move, and in which
   * section; see the note on `RlTable`. Each section has its own region, so
   * two reorderable sections do not both announce one move. Sections are
   * named by index: their ids come from headings, which need not be unique.
   */
  const status = ref({ section: -1, text: '' })

  function statusOf(sectionIndex: number): string {
    return status.value.section === sectionIndex ? status.value.text : ''
  }

  /** Whether the reader may reorder this section's rows. */
  function movable(section: ViewSection): boolean {
    return (
      section.display === 'table' && !section.isGrouped && section.relationOrder?.movable === true
    )
  }

  const enqueue = createMoveQueue(opts.reload)

  function onReorder(sectionIndex: number, move: RowMove): Promise<void> {
    const view = opts.view.value
    const section = view?.sections[sectionIndex]
    if (!view || !section?.relationOrder || !movable(section)) return Promise.resolve()
    const order = section.relationOrder
    const rows = (section.rows ?? []).map((row) => ({ id: row.entityId, row }))
    const plan = planRowMove(rows, move)
    if (!plan) return Promise.resolve()

    opts.view.value = {
      ...view,
      sections: view.sections.map((s) =>
        s === section ? { ...s, rows: plan.rows.map((r) => r.row) } : s
      ),
    }
    if ('step' in move) {
      const row = section.rows?.find((r) => r.entityId === move.itemId)
      const label = row ? sectionRowLabel(row) : move.itemId
      status.value = {
        section: sectionIndex,
        text: `${label} moved ${move.step < 0 ? 'up' : 'down'}`,
      }
    }

    return enqueue(async () => {
      try {
        await moveRelation(...orderMoveArgs(order, move.itemId, plan.position))
      } catch (err) {
        ui.error(getErrorMessage(err, 'Could not move the row'))
      }
    })
  }

  return { movable, onReorder, statusOf }
}
