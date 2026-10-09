import { computed } from 'vue'
import { useQueryCache } from '@pinia/colada'
import type { RowMove } from 'rela-components/components/table/types'
import { moveRelation } from '@/api/entities'
import { getErrorMessage } from '@/api/errors'
import { entityKeys } from '@/queries/entities'
import { beginOptimisticReorder, rollbackOptimistic } from '@/queries/optimisticList'
import { useUIStore } from '@/stores/ui'
import { createMoveQueue } from '@/utils/moveQueue'
import { orderMoveArgs, planRowMove } from '@/utils/relationOrder'
import type { Entity, RelationOrder } from '@/types'

/**
 * Moving rows of a list shown in relation order (see `RelationOrder`).
 *
 * A move shows at once: the cached page takes the new order before the
 * call returns, so a keyboard move can refocus its row on the next tick.
 * The move is then sent through a `createMoveQueue`, and the list is
 * refetched once the queue is empty to show what the server stored. A
 * failed move puts the rows back, unless a later move has already changed
 * them, and says why; the refetch corrects the rest.
 */
export function useListReorder(opts: {
  /** The list's relation order, from the last read. */
  order: () => RelationOrder | undefined
  /**
   * Whether the reader is looking at the relation order: not while a sort of
   * their own or a grouping puts the rows in another order.
   */
  active: () => boolean
  /** The rows on screen, in their order. */
  rows: () => Entity[]
  /** The cache key of the page on screen. */
  key: () => readonly string[]
  /** The rows' entity type, whose lists are refetched after a move. */
  type: () => string
}) {
  const queryCache = useQueryCache()
  const ui = useUIStore()

  const reorderable = computed(() => opts.active() && opts.order()?.movable === true)

  const enqueue = createMoveQueue(async () => {
    await queryCache.invalidateQueries({ key: entityKeys.list(opts.type()) })
  })
  let latest = 0

  function onReorder(move: RowMove): Promise<void> {
    const order = opts.order()
    if (!reorderable.value || !order) return Promise.resolve()
    const plan = planRowMove(opts.rows(), move)
    if (!plan) return Promise.resolve()
    const ctx = beginOptimisticReorder(queryCache, opts.key(), plan.rows)
    const id = ++latest
    return enqueue(async () => {
      try {
        await moveRelation(...orderMoveArgs(order, move.itemId, plan.position))
      } catch (err) {
        if (id === latest) rollbackOptimistic(queryCache, ctx)
        ui.error(getErrorMessage(err, 'Could not move the row'))
      }
    })
  }

  return { reorderable, onReorder }
}
