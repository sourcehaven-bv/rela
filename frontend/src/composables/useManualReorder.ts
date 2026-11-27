import { computed } from 'vue'
import { useQueryCache } from '@pinia/colada'
import type { RowMove } from 'rela-components/components/table/types'
import { moveListRow } from '@/api/entities'
import { getErrorMessage } from '@/api/errors'
import { entityKeys } from '@/queries/entities'
import { beginOptimisticReorder, rollbackOptimistic } from '@/queries/optimisticList'
import { useUIStore } from '@/stores/ui'
import { createMoveQueue } from '@/utils/moveQueue'
import { planRowMove } from '@/utils/relationOrder'
import type { Entity, ListParams } from '@/types'
import { t } from '@/i18n'

/**
 * Moving rows of a list with `order: manual` (TKT-AVNEX5T). Each reader
 * keeps their own order; the server stores it.
 *
 * On a grouped list a row moves only inside its own section: the sections
 * keep the group sort, so a row dropped in another section would jump back.
 * A step past the first or last row on screen does nothing, because the
 * server needs a neighbour to place the row next to.
 *
 * Like useListReorder, a move shows at once, is sent through a move queue,
 * and the list is refetched once the queue is empty.
 */
export function useManualReorder(opts: {
  /** Whether the rows on screen are in the reader's manual order. */
  active: () => boolean
  /** The rows on screen, in order. */
  rows: () => Entity[]
  /** The sections on screen; one section for a flat list. */
  sections: () => { items: Entity[] }[]
  /** The params of the list read on screen. */
  params: () => ListParams
  /** The cache key of the list on screen. */
  key: () => readonly string[]
  /** The rows' entity type, whose lists are refetched after a move. */
  type: () => string
}) {
  const queryCache = useQueryCache()
  const ui = useUIStore()
  const reorderable = computed(() => opts.active())

  const enqueue = createMoveQueue(async () => {
    await queryCache.invalidateQueries({ key: entityKeys.list(opts.type()) })
  })
  let latest = 0

  function onReorder(move: RowMove): Promise<void> {
    if (!reorderable.value) return Promise.resolve()
    const section = opts.sections().find((s) => s.items.some((r) => r.id === move.itemId))
    if (!section) return Promise.resolve()
    const plan = planRowMove(section.items, move)
    if (!plan || 'step' in plan.position) return Promise.resolve()
    const position = plan.position
    const rows = replaceRun(opts.rows(), section.items, plan.rows)
    const ctx = beginOptimisticReorder(queryCache, opts.key(), rows)
    const id = ++latest
    const params = opts.params()
    return enqueue(async () => {
      try {
        await moveListRow(params, move.itemId, position)
      } catch (err) {
        if (id === latest) rollbackOptimistic(queryCache, ctx)
        ui.error(getErrorMessage(err, t('list.moveFailed')))
      }
    })
  }

  return { reorderable, onReorder }
}

/**
 * `all` with the rows of `run` replaced, in place, by `next`: the same rows
 * in a new order.
 */
export function replaceRun(all: Entity[], run: Entity[], next: Entity[]): Entity[] {
  const ids = new Set(run.map((r) => r.id))
  let i = 0
  return all.map((r) => (ids.has(r.id) ? next[i++] : r))
}
