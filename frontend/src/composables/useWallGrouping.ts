import { useEntitiesStore, useSchemaStore, useUIStore } from '@/stores'
import { createRelation, getErrorMessage, moveRelation } from '@/api'
import { entityRef } from '@/utils/entityRef'
import { actionAllowed } from '@/utils/affordancesWarning'
import { t } from '@/i18n'
import { OTHER_COLUMN, type RelationColumn } from '@/composables/useRelationColumns'
import type { Entity } from '@/types'
import type { BoardSectionDropPosition } from 'rela-components/composables/useBoardDnd'

/** The longest group title taken from a card, before it is cut with an ellipsis. */
export const GROUP_TITLE_MAX = 40

/**
 * Grouping on a wall (`layout: wall` with `columns_from`, TKT-0VBQW0H).
 *
 * A group is a target of the `columns_from` relation; a card is in a group
 * when its edge points at it, and loose (in the pile) when it has none.
 * Dropping a card on a loose card creates a group, offered by the page's
 * entity when the board has `offered_by`.
 *
 * Every write goes through the ordinary entity and relation endpoints, so
 * the server's ACL and validation apply. Creating a group takes up to four
 * requests, not one. They run in an order where each partial state is a
 * valid board (an empty group, then a group with one card), so a failure
 * halfway shows the error and the board refetched from the server. A write
 * the reader's `_actions` already deny is refused before the first request,
 * so a denied drop does not leave an empty group behind.
 *
 * Drops are handled one at a time, each against the cards as the previous
 * one left them: a second drop on the same loose card while its group is
 * being made joins that group instead of making another.
 */
export function useWallGrouping(opts: {
  /** The type of the cards. */
  cardType: () => string | undefined
  /** The `columns_from` relation, from card to group. */
  relation: () => string | undefined
  columns: {
    columns: { value: RelationColumn[] }
    columnOf: (card: Entity) => string
    targetType: { value: string | undefined }
    anchor: { value: { type: string; id: string; relation: string } | undefined }
    targetActions: { value: Record<string, boolean> | undefined }
  }
  /** Whether the reader may update an entity, from its `_actions`. */
  canUpdate: (entity: Entity) => boolean
  cards: () => Entity[]
  cardTitle: (card: Entity) => string
  /** Refetches the groups and the cards, after every write. */
  refresh: () => Promise<void>
}) {
  const entitiesStore = useEntitiesStore()
  const schemaStore = useSchemaStore()
  const uiStore = useUIStore()

  /**
   * The property a group's title goes in: the one its display title is read
   * from. Without one the title is derived or the id, so there is nothing to
   * write a name into; a group is then created untitled and not renamed.
   */
  function titleProperty(type: string): string | undefined {
    return schemaStore.getEntityType(type)?.primary
  }

  /** Whether the reader can edit a group's name in place. */
  function renamable(groupId: string): boolean {
    const type = opts.columns.targetType.value
    const group = opts.columns.columns.value.find((c) => c.value === groupId)?.entity
    return !!type && !!titleProperty(type) && !!group && opts.canUpdate(group)
  }

  // The drop being handled; the next one waits for it.
  let pending: Promise<void> = Promise.resolve()

  function point(card: Entity, groupId: string | undefined) {
    const relation = opts.relation()
    const type = opts.cardType()
    const target = opts.columns.targetType.value
    if (!relation || !type || !target) return Promise.resolve()
    return entitiesStore.update(type, entityRef(card), {
      relations: { [relation]: { data: groupId ? [{ type: target, id: groupId }] : [] } },
    })
  }

  /**
   * `card` was dropped on the middle of `ontoId`. A grouped target means the
   * card joins its group; a loose one means a new group, titled after it.
   */
  function groupOnto(card: Entity, ontoId: string): Promise<void> {
    pending = pending.then(() => groupNow(card.id, ontoId))
    return pending
  }

  async function groupNow(cardId: string, ontoId: string): Promise<void> {
    const cards = opts.cards()
    const card = cards.find((e) => e.id === cardId)
    const onto = cards.find((e) => e.id === ontoId)
    const target = opts.columns.targetType.value
    const relation = opts.relation()
    if (!card || !onto || !target || !relation || onto.id === card.id) return
    let groupId = opts.columns.columnOf(onto)
    if (groupId !== OTHER_COLUMN && groupId === opts.columns.columnOf(card)) return
    if (groupId === OTHER_COLUMN) {
      // A card can be in a group this board does not show, such as one another
      // page offers. It only looks loose here; grouping it would move it out.
      if (onto.relations?.[relation]?.[0]) {
        uiStore.error(t('kanban.groupElsewhere'))
        return
      }
      if (!opts.canUpdate(onto) || !actionAllowed({ _actions: opts.columns.targetActions.value }, 'create')) {
        uiStore.error(t('kanban.groupForbidden'))
        return
      }
    }
    try {
      if (groupId === OTHER_COLUMN) {
        const prop = titleProperty(target)
        const title = opts.cardTitle(onto)
        const chars = Array.from(title)
        const short = chars.length > GROUP_TITLE_MAX ? `${chars.slice(0, GROUP_TITLE_MAX).join('')}…` : title
        const created = await entitiesStore.create(target, {
          properties: prop ? { [prop]: short } : {},
        })
        groupId = created.id
        const anchor = opts.columns.anchor.value
        if (anchor) await createRelation(anchor.type, anchor.id, anchor.relation, groupId)
        await point(onto, groupId)
      }
      await point(card, groupId)
    } catch (err) {
      uiStore.error(getErrorMessage(err, t('kanban.groupFailed')))
    }
    await opts.refresh()
  }

  /** Takes a card out of its group, back to the pile. */
  async function ungroup(card: Entity): Promise<void> {
    if (opts.columns.columnOf(card) === OTHER_COLUMN) return
    try {
      await point(card, undefined)
    } catch (err) {
      uiStore.error(getErrorMessage(err, t('kanban.moveFailed')))
    }
    await opts.refresh()
  }

  /**
   * Renames a group. Returns the title to show: the new one, or the old one
   * when the new one is empty or unchanged.
   */
  async function renameGroup(groupId: string, value: string): Promise<string | undefined> {
    const column = opts.columns.columns.value.find((c) => c.value === groupId)
    const type = opts.columns.targetType.value
    const prop = type ? titleProperty(type) : undefined
    if (!column?.entity || !type || !prop) return column?.label
    const name = value.trim()
    if (!name || name === column.label) return column.label
    try {
      await entitiesStore.update(type, entityRef(column.entity), { properties: { [prop]: name } })
    } catch (err) {
      uiStore.error(getErrorMessage(err, t('kanban.renameFailed')))
      await opts.refresh()
      return column.label
    }
    await opts.refresh()
    return name
  }

  /** Moves a group before or after another, in the `offered_by` relation's order. */
  async function moveGroup(move: BoardSectionDropPosition): Promise<void> {
    const anchor = opts.columns.anchor.value
    if (!anchor || move.sectionId === move.targetId) return
    if (move.sectionId === OTHER_COLUMN || move.targetId === OTHER_COLUMN) return
    const position = move.placement === 'before' ? { before: move.targetId } : { after: move.targetId }
    try {
      await moveRelation(anchor.type, anchor.id, anchor.relation, move.sectionId, position)
    } catch (err) {
      uiStore.error(getErrorMessage(err, t('kanban.moveGroupFailed')))
    }
    await opts.refresh()
  }

  return { groupOnto, ungroup, renameGroup, moveGroup, renamable }
}
