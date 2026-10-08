<script setup lang="ts">
import { ref, computed, watch, type Component } from 'vue'
import { RouterLink, useRoute, useRouter, type RouteLocationRaw } from 'vue-router'
import { useQuery, useMutation, useQueryCache } from '@pinia/colada'
import { useEntitiesStore, useSchemaStore, useUIStore } from '@/stores'
import { listAllEntities, getErrorMessage } from '@/api'
import { entityKeys } from '@/queries/entities'
import { beginOptimistic, rollbackOptimistic, settleOptimistic } from '@/queries/optimisticList'
import type {
  Entity,
  KanbanConfig,
  KanbanCardField,
  KanbanColumn,
  KanbanSwimlane,
  ListParams,
  PageScope,
} from '@/types'
import { viewHeaderMarkdown, viewFooterMarkdown } from '@/types'
import type { FilterState } from '@/types/filters'
import FilterBar from '@/components/lists/FilterBar.vue'
import BackButton from '@/components/common/BackButton.vue'
import PageHeaderContent from '@/components/common/PageHeaderContent'
import EntityDetailPanel from '@/components/entity/EntityDetailPanel.vue'
import { useDetailPanel } from '@/composables/useDetailPanel'
import { useCreateModal } from '@/composables/useCreateModal'
import { usePageTabScope } from '@/composables/usePageTabScope'
import { useListReorder } from '@/composables/useListReorder'
import InlineCreateFormModal from '@/components/forms/InlineCreateFormModal.vue'
import { useBackTarget } from '@/composables/useBackTarget'
import { useUrlFilterSync } from '@/composables/useUrlFilterSync'
import { useWorld } from '@/composables/useWorld'
import { actionAllowed } from '@/utils/affordancesWarning'
import { filterStateToApiParams } from '@/utils/filters'
import { entityRef } from '@/utils/entityRef'
import { editFormRoute } from '@/utils/entityRoute'
import { fromPageQuery } from '@/utils/pageContext'
import { worldText } from '@/utils/worldText'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { renderMarkdown } from '@/utils/markdown'
import { hasIcon, resolveIcon } from '@/utils/icons'
import { formatCellValue } from '@/utils/format'
import { densePropertyRoutingHint } from '@/widgets/viewRouting'
import CardFieldList, { type ResolvedCardField } from '@/components/common/CardFieldList.vue'
import WorldBadge from '@/components/entity/WorldBadge.vue'
import { defaultRegistry } from '@/widgets/registry'
import type { DenseRoutingHint } from '@/widgets/viewRouting'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlBoard from 'rela-components/components/board/RlBoard.vue'
import RlSwimlaneBoard from 'rela-components/components/board/RlSwimlaneBoard.vue'
import type { Section, Swimlane } from 'rela-components/types'
import type { BoardDropPosition } from 'rela-components/composables/useBoardDnd'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'

const props = defineProps<{
  id: string
  /** Set when the board is a tab of an entity page: cards are those the anchor reaches. */
  pageScope?: PageScope
}>()

const router = useRouter()
const route = useRoute()
const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const queryCache = useQueryCache()

// Back affordance — renders when ?return_to= or ?from= is present.
const backTarget = useBackTarget()

// State

// The selected world (`?world=`). A board is a projection through a world
// exactly as a list is: it decides which face each card shows AND which
// entities are on the board at all, since an entity with no face in the world
// is omitted entirely.
//
// `worldParam` is undefined under the default world, so it spreads into the
// params object without emitting an empty `?world=` — and it is part of the
// CACHE KEY below for the same reason EntityList keys on its params: two
// worlds are two different boards, and serving one from the other's cache
// would show faces the reader did not ask for.
const { world, isWorldBound, worldParam } = useWorld()

// The operator's announcement for the world on screen, or '' to announce
// nothing — the same split the list and detail pages make. Config, not data.
const worldBanner = computed<string>(
  () => (world.value ? schemaStore.worlds.get(world.value)?.banner : '') || '',
)

// Affordance gates: `_actions` map from the server. `false` → hide;
// anything else → render. Helper keeps the contract DRY across
// components; see frontend/src/utils/affordancesWarning.ts.
//
// From `_actions` alone, under every world. The server computes the map for
// the FACE each card shows, and the drag writes to that card's ADDRESS
// (`entityRef`) rather than its bare id, so a card that accepts the gesture
// writes the row the reader is looking at. A stand-in face the principal may
// not write reports `update: false` and refuses the drag up front — the
// honest ordering for a write triggered by a gesture. An earlier revision
// ANDed in `!isWorldBound`, which made every board read-only under a
// configured `default_world` (atlas worlds issue 2).
function canCreate(): boolean {
  return actionAllowed({ _actions: collectionActions.value }, 'create')
}
function canUpdate(entity: Entity): boolean {
  return actionAllowed(entity, 'update')
}

// The world note is a fact about a FACED type only: an entity of a type
// without faces has one state, present in every world, so no card can be
// missing from the board on the world's account (atlas worlds issue 1).
const boardTypeHasFaces = computed(() => {
  const def = schemaStore.getEntityType(kanbanConfig.value?.entity ?? '')
  return Object.keys(def?.faces ?? {}).length > 0
})
// The operator's `messages.projection` for the world, or nothing.
const projectionNote = computed<string>(() => {
  if (!boardTypeHasFaces.value) return ''
  const info = world.value ? schemaStore.worlds.get(world.value) : undefined
  return worldText(info?.messages?.projection, { world: world.value })
})

// Computed
const kanbanConfig = computed(() => schemaStore.getKanban(props.id) as KanbanConfig | undefined)

// The board's filter controls are the list's: the same FilterBar, the same
// URL sync, and the same server-side `filter[...]` params, so a relation
// control (whose options are the relation's targets) works on a board exactly
// as on a list. The board used to filter client-side on
// `entity.properties[control.property]`, which silently ignored every
// `relation:` control. Properties pinned by the board's static `filters:`
// cannot be overridden from the URL, as on a list. Declared after
// kanbanConfig: useUrlFilterSync reads staticFilterProperties during setup.
const { filters, writeToQuery } = useUrlFilterSync({
  staticFilterProperties: () =>
    new Set((kanbanConfig.value?.filters ?? []).map((f) => f.property)),
})

// Cards may render relation targets by ID; when any card field references a
// relation we must ask the server to embed the related entities (?include=*)
// so we can resolve those IDs to titles. Property-only boards fetch without
// includes, exactly as before.
const hasRelationFields = computed(
  () => kanbanConfig.value?.card.fields?.some((f) => !!f.relation) ?? false
)

// A tab of an entity page narrows the board to the anchor's cards.
const tabScope = usePageTabScope(() => props.pageScope)

// The board's list query (FEAT-XY2D1L). The key derives from the
// configured entity type, so switching boards (props.id) switches cache
// entries automatically, and useEvents' targeted SSE invalidation on
// ['entities', <type>] marks it stale and triggers a *background*
// refetch while this view is mounted. The template gates its spinner on
// `isPending` (no data yet), so refetches — including echoes of this
// client's own writes — never blank the board.
// listAllEntities, not listEntities: the board partitions the COMPLETE
// set by column property, and a single list call is one page (default
// 25) — treating it as the full set silently dropped page 2+ from the
// board (BUG-5OAQUG).
const boardParams = computed<ListParams | undefined>(() => {
  const params: ListParams = {}
  if (hasRelationFields.value) params.include = '*'
  if (worldParam.value) params.world = worldParam.value
  // See the same attachment in EntityList: the board's configured scope has to
  // travel on the request, since the endpoint is keyed by type.
  if (kanbanConfig.value?.query_scope) params.query_scope = kanbanConfig.value.query_scope
  // The user's filter controls, serialized exactly as EntityList does. They
  // are part of the params, so each filter state is its own cache entry.
  Object.assign(params, filterStateToApiParams(filters.value))
  Object.assign(params, tabScope.params.value)
  return Object.keys(params).length ? params : undefined
})

// The board whose data the query last resolved; see placeholderData below.
let heldBoardId: string | undefined

const boardQuery = useQuery({
  // listParams, not the param-free `list`: the world has to separate cache
  // entries. It still shares the `list(type)` prefix, so SSE invalidation
  // reaches every world's board in one go.
  key: () => entityKeys.listParams(kanbanConfig.value?.entity ?? '', boardParams.value),
  // The signal matters more here than on single-fetch queries: when a
  // refetch supersedes this call (drag-drop settle, SSE echo), it also
  // cancels the remaining page fetches of the superseded loop.
  query: async ({ signal }) => {
    const config = kanbanConfig.value
    if (!config) throw new Error(`unknown kanban view: ${props.id}`)
    const boardId = props.id
    const result = await listAllEntities(config.entity, boardParams.value, signal)
    heldBoardId = boardId
    return result
  },
  enabled: () => !!kanbanConfig.value,
  // A filter change is a new key, which would start out pending and blank the
  // board while every page reloads. Hold the previous cards instead, as
  // EntityList does — but only for the same board: this view is reused across
  // boards, and another board's cards must not appear under this one's columns.
  placeholderData: (prev) => (heldBoardId === props.id ? prev : undefined),
})

const entities = computed(() => boardQuery.data.value?.data ?? [])
const includedEntities = computed<Record<string, Entity>>(
  () => boardQuery.data.value?.included ?? {}
)
const collectionActions = computed(() => boardQuery.data.value?._actions)
const loading = computed(() => boardQuery.isPending.value)

// has_more on the MERGED response means listAllEntities hit its page cap
// — the one case where the board is knowingly incomplete. Should never
// occur in practice (~5,000 entities), but silent truncation is exactly
// this view's bug class, so it gets a visible banner, not a console line.
const truncated = computed(() => boardQuery.data.value?.meta.has_more === true)
const totalCount = computed(() => boardQuery.data.value?.meta.total ?? 0)
const loadError = computed(() => {
  const err = boardQuery.error.value
  if (!err) return null
  return getErrorMessage(err, 'Failed to load board')
})

// pageState mirrors DynamicForm's `form-state-*` contract: a stable signal
// that this screen has finished resolving, so a screenshot{} capture can wait
// for it rather than hanging until its timeout.
const pageState = computed<'pending' | 'loaded' | 'error'>(() => {
  if (loadError.value) return 'error'
  return loading.value ? 'pending' : 'loaded'
})

// Admin-authored info regions from data-entry.yaml, rendered as sanitized
// markdown (renderMarkdown) above and below the board. Shares its resolvers and
// its .view-info styles with EntityList so both views behave identically.
const headerHtml = computed(() => renderMarkdown(viewHeaderMarkdown(kanbanConfig.value)))
const footerHtml = computed(() => renderMarkdown(viewFooterMarkdown(kanbanConfig.value)))

const entityType = computed(() => {
  if (!kanbanConfig.value) return undefined
  return schemaStore.getEntityType(kanbanConfig.value.entity)
})

const columns = computed(() => {
  if (!kanbanConfig.value) return []

  // Use defined columns or generate from unique values
  if (kanbanConfig.value.columns?.length) {
    return kanbanConfig.value.columns
  }

  // Default to the property's DECLARED enum values, in declaration order.
  //
  // `column_property` is required to be an enum (validate.go rejects anything
  // else), so the correct, ordered list is known at config-load time. Deriving
  // it from the data instead produced a board that was a picture of the current
  // rows rather than of the workflow: a state nobody is currently in had no
  // column at all — so a card could not be dragged BACK to it — and the order
  // followed entity insertion, which put `done` left of `doing` (TKT-R7H6G1).
  const property = kanbanConfig.value.column_property
  const declared = schemaStore.enumValuesForProperty(property, kanbanConfig.value.entity)
  if (declared?.length) {
    return declared.map(
      (v): KanbanColumn => ({
        value: v,
        label: schemaStore.getEnumLabel(v, property, kanbanConfig.value?.entity) ?? v,
      }),
    )
  }

  // Last resort for a non-enum property. The server rejects that today, so
  // this is unreachable in a valid config — kept so a schema that somehow
  // slips through renders a board instead of nothing.
  const values = new Set<string>()
  for (const entity of entities.value) {
    const val = String(entity.properties[property] || '')
    if (val) values.add(val)
  }
  return Array.from(values).map((v): KanbanColumn => ({ value: v, label: v }))
})

const filteredEntities = computed(() => {
  let result = [...entities.value]

  // Apply kanban config filters
  if (kanbanConfig.value?.filters) {
    for (const filter of kanbanConfig.value.filters) {
      result = result.filter((entity) => {
        const val = String(entity.properties[filter.property] || '')
        switch (filter.operator) {
          case '=':
          case '==':
            return val === filter.value
          case '!=':
            return val !== filter.value
          default:
            return true
        }
      })
    }
  }

  return result
})

// Swimlanes (rows in 2D grid layout)
const swimlanes = computed(() => {
  if (!kanbanConfig.value?.swimlane_property) return []

  // Use defined swimlanes or generate from unique values
  if (kanbanConfig.value.swimlanes?.length) {
    return kanbanConfig.value.swimlanes
  }

  // Same rule as the columns above: prefer the declared enum order. The old
  // path sorted alphabetically, which is just as arbitrary for a workflow as
  // insertion order — and equally hid a swimlane nobody currently occupies.
  const property = kanbanConfig.value.swimlane_property
  const declared = schemaStore.enumValuesForProperty(property, kanbanConfig.value.entity)
  if (declared?.length) {
    return declared.map(
      (v): KanbanSwimlane => ({
        value: v,
        label: schemaStore.getEnumLabel(v, property, kanbanConfig.value?.entity) ?? v,
      }),
    )
  }

  // Unreachable for the same reason as the column fallback — swimlane_property
  // is enum-required too (validate.go) — but kept, and sorted, so a config that
  // slips through renders deterministically rather than not at all.
  const values = new Set<string>()
  for (const entity of entities.value) {
    const val = String(entity.properties[property] || '')
    if (val) values.add(val)
  }
  return Array.from(values).sort().map((v): KanbanSwimlane => ({ value: v, label: v }))
})

const hasSwimmlanes = computed(() => swimlanes.value.length > 0)

// Accessible name for the board region. The columns are sections inside it, so
// the group needs its own name to be distinguishable from the rest of the page.
const boardLabel = computed(() => `${kanbanConfig.value?.title || 'Kanban'} board`)

// Column header text. An explicit kanban-config column label wins; otherwise
// fall back to the enum's display label for the grouping value, then the raw
// value. Keeps headers consistent with the card badges (which resolve labels
// via Badge). `column.label` defaults to the value for auto-generated columns
// (see the columns computed), so treat label===value as "no explicit label".
function columnTitle(column: { value: string; label?: string }): string {
  if (column.label && column.label !== column.value) return column.label
  const property = kanbanConfig.value?.column_property
  const entityTypeName = kanbanConfig.value?.entity
  return (
    schemaStore.getEnumLabel(column.value, property, entityTypeName) ??
    column.label ??
    column.value
  )
}

const entitiesByColumn = computed(() => {
  const grouped: Record<string, Entity[]> = {}
  const property = kanbanConfig.value?.column_property || ''

  for (const column of columns.value) {
    grouped[column.value] = []
  }

  for (const entity of filteredEntities.value) {
    const val = String(entity.properties[property] || '')
    if (grouped[val]) {
      grouped[val].push(entity)
    }
  }

  return grouped
})

// 2D grouping for swimlane mode: entitiesByCell[column][swimlane] = entities
const entitiesByCell = computed(() => {
  if (!hasSwimmlanes.value) return {}

  const cells: Record<string, Record<string, Entity[]>> = {}
  const colProp = kanbanConfig.value?.column_property || ''
  const swimProp = kanbanConfig.value?.swimlane_property || ''

  // Initialize all cells
  for (const column of columns.value) {
    cells[column.value] = {}
    for (const swimlane of swimlanes.value) {
      cells[column.value][swimlane.value] = []
    }
  }

  // Group entities into cells
  for (const entity of filteredEntities.value) {
    const colVal = String(entity.properties[colProp] || '')
    const swimVal = String(entity.properties[swimProp] || '')
    if (cells[colVal] && cells[colVal][swimVal]) {
      cells[colVal][swimVal].push(entity)
    }
  }

  return cells
})

// A card as the library board holds it: the id and title it reads, and the
// entity our card slot renders.
interface BoardCard {
  id: string
  title: string
  entity: Entity
}

function toCard(entity: Entity): BoardCard {
  return { id: entity.id, title: getCardTitle(entity), entity }
}

function iconFor(name?: string) {
  return hasIcon(name) ? resolveIcon(name) : undefined
}

const boardSections = computed((): Section<BoardCard>[] =>
  columns.value.map((column) => ({
    id: column.value,
    title: columnTitle(column),
    icon: iconFor(column.icon),
    items: (entitiesByColumn.value[column.value] ?? []).map(toCard),
  }))
)

// Lanes the user folded away. View state only, so it resets with the page.
const collapsedLanes = ref(new Set<string>())

const boardLanes = computed((): Swimlane<BoardCard>[] =>
  swimlanes.value.map((lane) => ({
    id: lane.value,
    title: lane.label || lane.value,
    icon: iconFor(lane.icon),
    collapsed: collapsedLanes.value.has(lane.value),
    sections: boardSections.value.map((column) => ({
      ...column,
      items: (entitiesByCell.value[column.id]?.[lane.value] ?? []).map(toCard),
    })),
  }))
)

function toggleLane(lane: Swimlane<BoardCard>) {
  const next = new Set(collapsedLanes.value)
  if (!next.delete(lane.id)) next.add(lane.id)
  collapsedLanes.value = next
}

// Drag-drop write path: optimistic copy-on-write against the query
// cache, rollback + toast on failure, reconcile with server truth via
// invalidation on settle. Cached entities are never mutated in place —
// other subscribers may hold references to the same objects.
interface MoveCardVars {
  entity: Entity
  updates: Record<string, string>
}

const entitiesStore = useEntitiesStore()

const { mutateAsync: moveCard } = useMutation({
  mutation: ({ entity, updates }: MoveCardVars) => {
    const config = kanbanConfig.value
    if (!config) throw new Error(`unknown kanban view: ${props.id}`)
    // To the card's ADDRESS, face included — see utils/entityRef. Through the
    // entities store so the edit form, which reads its cache, sees the move
    // without waiting for the SSE invalidation.
    return entitiesStore.update(config.entity, entityRef(entity), { properties: updates })
  },
  onMutate({ entity, updates }: MoveCardVars) {
    return beginOptimistic(
      queryCache,
      entityKeys.list(kanbanConfig.value?.entity ?? ''),
      entity.id,
      (e) => ({ ...e, properties: { ...e.properties, ...updates } })
    )
  },
  onError(err, _vars, context) {
    rollbackOptimistic(queryCache, context)
    console.error('Failed to update entity:', err)
    uiStore.error(getErrorMessage(err, 'Failed to move card'))
  },
  async onSettled(_data, _err, _vars, context) {
    await settleOptimistic(queryCache, context)
  },
})

function getCardTitle(entity: Entity): string {
  if (!kanbanConfig.value) return entity.id
  return String(entity.properties[kanbanConfig.value.card.title] || entity.id)
}

// relationCardKey resolves the key under which a card field's relation
// targets are serialized on the entity's `relations` map. Outgoing edges
// are keyed by the relation name itself; incoming edges are keyed by the
// relation's declared inverse (schemaStore.getInverseName), falling back to
// `<relation>_inverse` when no inverse is declared. This mirrors the wire
// contract shared with EntityList relation columns (TKT-ODHV2D).
//
// MERGE-ORDER DEPENDENCY (RR-M8IIHV): the INCOMING branch only resolves once
// TKT-ODHV2D's server change lands. The list endpoint on this branch
// serializes OUTGOING edges only (see entityserializer.forWireRelated, fed by
// entityReader.outgoingRelations) — it does NOT populate the inverse key for
// incoming edges. Until ODHV2D merges, an incoming card field computes an
// inverse key that is absent from `relations`, so getCardFieldValue below
// returns '' and the card renders the '-' placeholder (degrades visibly, not a
// silent blank). The Go contract test `TestListEndpoint_IncomingEdge_InverseKey_ODHV2DContract`
// in internal/dataentry pins the server side of this inverse-key contract and
// activates once ODHV2D is integrated.
function relationCardKey(field: KanbanCardField): string {
  const rel = field.relation || ''
  if (field.direction === 'incoming') {
    return schemaStore.getInverseName(rel) || `${rel}_inverse`
  }
  return rel
}

function getCardFieldValue(entity: Entity, field: KanbanCardField): string {
  if (field.relation) {
    const ids = entity.relations?.[relationCardKey(field)] || []
    return ids
      .map((id) => {
        const included = includedEntities.value[id]
        // Unresolved target → raw ID fallback, matching EntityList.vue's
        // getFormattedCellValue (`included ? title : id`). Intentionally NOT
        // divergent: kanban and the list share one relation-cell contract
        // (RR-XM5ZEB). ACL-hidden targets do not leak their IDs here because
        // TKT-ODHV2D's server gate (`visibleRelationIDs`) removes hidden
        // neighbour IDs from the `relations` map before it reaches this
        // fallback — the SPA never sees a hidden ID to fall back to.
        return included ? entityDisplayTitle(included) : id
      })
      .join(', ')
  }
  if (!field.property) return ''
  // Formatted via the shared cell formatter so cards agree with list cells
  // (dates render human-readably, booleans as Yes/No, rrules as text). This
  // used to be a bare String(v || ''), which showed raw ISO datetimes and
  // "true" on cards -- see TKT-S9C14S.
  return formatCellValue(
    entity.properties[field.property],
    field.property,
    entityType.value,
    uiStore.effectiveTimezone
  )
}

// The stored property value, unformatted. PROPERTY fields only -- a relation
// field has no stored property and callers must use getCardFieldValue for it.
// (An earlier version returned the joined relation string from here, which
// made a function named "raw" hand pre-formatted text to a widget the moment
// anyone added a relation widget.)
function getCardFieldStoredValue(entity: Entity, field: KanbanCardField): unknown {
  if (!field.property) return undefined
  return entity.properties[field.property]
}

// Widget resolution for property card-fields, computed once per configured
// field rather than per card (RR-UD2A). Relation fields are absent on
// purpose: they have no PropertyDef and no relation widget, so they keep the
// joined-titles string path.
const cardFieldWidgets = computed(() => {
  const byProperty = new Map<string, { component: Component; hint: DenseRoutingHint }>()
  const type = entityType.value
  if (!type) return byProperty
  for (const field of kanbanConfig.value?.card.fields ?? []) {
    if (!field.property || field.relation || byProperty.has(field.property)) continue
    const hint = densePropertyRoutingHint(type.properties[field.property], field.property)
    byProperty.set(field.property, { component: defaultRegistry.resolveFromHint(hint), hint })
  }
  return byProperty
})

// One resolved card field: the widget plus the value shaped the way it wants.
// undefined means render the plain string span (relation fields, or a
// property with no widget entry).
//
// Unlike EntityList this needs no empty-value guard: visibleCardFields has
// already dropped empty fields before the template renders, so a widget's
// "no value" placeholder (MultiSelectWidget's em-dash, RR-UD2C) is
// unreachable here.
/**
 * Every visible field for one card, resolved once.
 *
 * The template used to call `resolveCardField` three times per field per card
 * (once for `v-if`, once for the component, once for each bound prop), so a
 * board of 50 cards with 3 fields did 450 resolutions per render where 150
 * would do — and each one re-derives a value the widget map already holds.
 *
 * Empty fields are dropped here rather than in the renderer, keeping the
 * dense-surface rule (empty renders as nothing, not a placeholder) with the
 * code that knows how a value is formatted.
 */
function resolvedCardFields(entity: Entity): ResolvedCardField[] {
  const out: ResolvedCardField[] = []

  for (const field of kanbanConfig.value?.card.fields ?? []) {
    const text = getCardFieldValue(entity, field)
    if (text === '') continue

    // Relation fields have no PropertyDef and so no widget: they render as
    // joined target titles, the contract shared with list relation columns.
    const entry = field.property && !field.relation
      ? cardFieldWidgets.value.get(field.property)
      : undefined

    out.push({
      field,
      component: entry?.component,
      propertyName: entry?.hint.propertyName,
      modelValue: entry
        ? entry.hint.preformatted
          ? text
          : getCardFieldStoredValue(entity, field)
        : undefined,
      text,
    })
  }
  return out
}

function canMoveCard(card: BoardCard): boolean {
  return canUpdate(card.entity)
}

// On a board whose cards the reader may reorder, every card can be picked up:
// moving one within its column writes the anchor's edge, not the card.
function boardCanMove(card: BoardCard): boolean {
  return reorder.reorderable.value || canMoveCard(card)
}

// Card order on a tab shown in relation order: the order the reader sets by
// dropping a card before or after another. One order runs through every
// column, so a card placed before another lands there whichever column it
// came from.
const reorder = useListReorder({
  order: () => boardQuery.data.value?.meta.relation_order,
  active: () => tabScope.relationOrdered.value,
  rows: () => entities.value,
  key: () => entityKeys.listParams(kanbanConfig.value?.entity ?? '', boardParams.value),
  type: () => kanbanConfig.value?.entity ?? '',
})

// The board reports a drop on another column (and lane), and with `reorder`
// the card it landed against. The move writes the values the card now sits
// under, then its place: in that order, so a failed column change leaves the
// card where it was rather than half moved.
async function onMove({
  item,
  to,
  lane,
  at,
}: {
  item: BoardCard
  to: Section<BoardCard>
  lane?: Swimlane<BoardCard>
  at?: BoardDropPosition
}) {
  const config = kanbanConfig.value
  const entity = item.entity
  if (!config) return

  const colProp = config.column_property
  const swimProp = config.swimlane_property

  // The values the card now sits under that differ from its own.
  const updates: Record<string, string> = {}
  if (String(entity.properties[colProp] || '') !== to.id) {
    updates[colProp] = to.id
  }
  if (swimProp && lane && String(entity.properties[swimProp] || '') !== lane.id) {
    updates[swimProp] = lane.id
  }

  if (Object.keys(updates).length > 0) {
    // On a reorderable board a card the reader may not update can still be
    // picked up, to move it within its column; a drop into another column
    // would change the card itself, so it is refused out loud.
    if (!canUpdate(entity)) {
      uiStore.error('You may not move this card to another column')
      return
    }
    try {
      await moveCard({ entity, updates })
    } catch {
      return // reported by the mutation
    }
  }
  if (at) await reorder.onReorder({ itemId: entity.id, ...at })
}

// cardTarget is the single source of truth for where a card goes, bound to each
// card's RouterLink. The edit-form branch must be reproduced exactly, or a
// cmd-clicked tab would land on the detail page while a plain click opens the
// form.
//
// The card is now a real <a> (TKT-3CSZRG), which supersedes the earlier
// role="button" + @keydown shim: an anchor is natively focusable and
// Enter-activatable, so the keyboard half of the contract comes for free and
// cmd/middle-click open a tab, which the shim could never do.
function cardTarget(entity: Entity): RouteLocationRaw {
  // Inside a page tab, Back and Cancel on the destination return to the tab.
  const fromPage = fromPageQuery(route)
  // The form opens on the card's ADDRESS, face included, so an edit from a
  // world-bound board edits the face the card showed and not its bare id, in
  // the board's world so the form loads the relations that world serves.
  if (kanbanConfig.value?.edit_form) {
    return editFormRoute(kanbanConfig.value.edit_form, entityRef(entity), worldParam.value, fromPage)
  }
  // The world rides along so the detail resolves the face the card showed.
  const path = `/entity/${entity.type}/${entity.id}`
  const query = { ...fromPage, ...(worldParam.value ? { world: worldParam.value } : {}) }
  return Object.keys(query).length ? { path, query } : path
}

/*
 * A plain click on a card opens the entity in the shell's overlay panel, over
 * the board, with its id in `?selected=` so a reload or a shared link reopens
 * it. Same contract as a list row (see EntityList): a modified or middle
 * click stays the link's own, so cmd-click still opens the card's page in a
 * new tab.
 */
const panel = useDetailPanel()

const selectedEntityId = computed(() => {
  const raw = route.query.selected
  const value = Array.isArray(raw) ? raw[raw.length - 1] : raw
  return typeof value === 'string' && value !== '' ? value : null
})

const selectedEntity = computed(() => {
  const id = selectedEntityId.value
  return id ? (entities.value.find((e) => e.id === id) ?? null) : null
})

function onCardClick(entity: Entity, event: MouseEvent) {
  if (event.defaultPrevented) return
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  if (event.button !== 0) return
  // Capture phase, so RouterLink's own handler does not navigate as well.
  event.preventDefault()
  event.stopPropagation()
  if (entity.id === selectedEntityId.value) return
  void router.replace({ query: { ...route.query, selected: entity.id } })
}

function closePanel() {
  const query = { ...route.query }
  delete query.selected
  void router.replace({ query })
}

function expandPanel() {
  if (selectedEntity.value) void router.push(cardTarget(selectedEntity.value))
}

watch(
  [selectedEntity, () => kanbanConfig.value?.entity],
  ([entity, entityType]) => {
    if (!entity || !entityType) {
      panel.clear()
      return
    }
    panel.show({
      component: EntityDetailPanel,
      props: {
        entityType: entity.type || entityType,
        entityId: entityRef(entity),
        onClose: closePanel,
        onExpand: expandPanel,
      },
      // Over the board rather than beside it: a board needs its full width.
      mode: 'overlay',
    })
  },
  { immediate: true }
)

// New opens the create form in a dialog over the board; the new card then
// opens in the detail panel, as if it had been clicked.
function refreshAfterCreate() {
  void queryCache.invalidateQueries({ key: entityKeys.list(kanbanConfig.value?.entity ?? '') })
}
// In an entity-page tab a new card is linked to the anchor first, so the
// refresh already shows it on the board.
const createModal = useCreateModal(async (entity) => {
  await tabScope.linkCreated(entity)
  refreshAfterCreate()
  void router.replace({ query: { ...route.query, selected: entity.id } })
}, async (entity) => {
  await tabScope.linkCreated(entity)
  refreshAfterCreate()
})

// No lifecycle plumbing: the query fetches on mount, re-keys when
// props.id switches boards, and refetches in the background when
// useEvents invalidates ['entities', <type>] on SSE entity events.
</script>

<template>
  <div class="kanban-view" :data-testid="`page-state-${pageState}`">
    <!-- Painted by the app shell in its fixed header band; see PageHeaderContent. -->
    <PageHeaderContent :title="kanbanConfig?.title || props.id">
      <template #actions>
        <BackButton v-if="backTarget" :target="backTarget" />
        <RlButton
          v-if="kanbanConfig?.create_form && canCreate()"
          variant="primary"
          icon="plus"
          @click="createModal.show"
        >
          New
        </RlButton>
      </template>
    </PageHeaderContent>

    <!--
      A board under a world is a PROJECTION: each card is one entity at the
      face the world resolved, and an entity with no face here has no card.
      Cards move — a drag writes the face the card shows — so the banner no
      longer claims the board is read-only; a card the principal may not write
      simply refuses the drag through `_actions`, as in the default world.
    -->
    <div v-if="isWorldBound && !loadError && (worldBanner || projectionNote)" class="world-banner">
      <!--
        Both halves are operator config: the ANNOUNCEMENT (`banner:`) and the
        NOTE (`messages.projection`, only on a board of a faced type). Neither
        declared: no banner (TKT-5SZG2L).
      -->
      <span v-if="worldBanner" class="world-banner__label">
        {{ worldBanner }}
      </span>
      <span v-if="projectionNote" class="world-banner__note">
        {{ projectionNote }}
      </span>
    </div>

    <FilterBar
      v-if="kanbanConfig?.filter_controls?.length"
      :config="kanbanConfig"
      :entity-type="entityType"
      :filters="filters"
      @filter="(f: FilterState) => writeToQuery(f)"
    />

    <!-- eslint-disable-next-line vue/no-v-html -- sanitized by renderMarkdown -->
    <div v-if="headerHtml" class="view-info view-info--top" v-html="headerHtml"/>

    <div v-if="truncated" class="truncation-banner" role="alert">
      Showing {{ entities.length }} of {{ totalCount }} items — the board is incomplete.
    </div>

    <RlStatusRegion v-if="loading">Loading board...</RlStatusRegion>

    <RlStatusRegion v-else-if="loadError" tone="error">{{ loadError }}</RlStatusRegion>

    <!-- The card is a real link, so cmd/middle-click opens a tab. The board
         wraps it in the drag source. -->
    <RlSwimlaneBoard
      v-else-if="boardLanes.length"
      class="kanban-board"
      role="group"
      :aria-label="boardLabel"
      :lanes="boardLanes"
      :columns="boardSections"
      :show-add="false"
      :show-add-section="false"
      :can-move="canMoveCard"
      :selected-id="selectedEntityId ?? undefined"
      @move="onMove"
      @toggle-lane="toggleLane"
    >
          <template #card="{ item }">
            <RouterLink
              class="kanban-card"
              :to="cardTarget(item.entity)"
              :aria-label="item.title"
              @click.capture="onCardClick(item.entity, $event)"
            >
              <div class="card-id">{{ item.id }}</div>
              <!--
                Per-CARD face provenance (TKT-ILT1WD), beside the title, the same
                place and the same component the list uses. WorldBadge renders
                only for a substitute, so an ordinary board shows nothing.
              -->
              <div class="card-title text-wrap-anywhere">
                {{ item.title }}<WorldBadge :world="item.entity._world" :entity-type="item.entity.type" />
              </div>
              <CardFieldList
                :fields="resolvedCardFields(item.entity)"
                :entity-type="kanbanConfig?.entity"
              />
            </RouterLink>
          </template>
    </RlSwimlaneBoard>

    <RlBoard
      v-else
      class="kanban-board"
      role="group"
      :aria-label="boardLabel"
      :sections="boardSections"
      :show-add="false"
      :show-add-section="false"
      :can-move="boardCanMove"
      :reorder="reorder.reorderable.value"
      :selected-id="selectedEntityId ?? undefined"
      @move="onMove"
    >
          <template #card="{ item }">
            <RouterLink
              class="kanban-card"
              :to="cardTarget(item.entity)"
              :aria-label="item.title"
              @click.capture="onCardClick(item.entity, $event)"
            >
              <div class="card-id">{{ item.id }}</div>
              <!--
                Per-CARD face provenance (TKT-ILT1WD), beside the title, the same
                place and the same component the list uses. WorldBadge renders
                only for a substitute, so an ordinary board shows nothing.
              -->
              <div class="card-title text-wrap-anywhere">
                {{ item.title }}<WorldBadge :world="item.entity._world" :entity-type="item.entity.type" />
              </div>
              <CardFieldList
                :fields="resolvedCardFields(item.entity)"
                :entity-type="kanbanConfig?.entity"
              />
            </RouterLink>
          </template>
    </RlBoard>

    <!-- Sits after every board branch (loading/error/simple/swimlane) so it
         renders once regardless of state, and outside the board's horizontal
         scroll container so it stays visible on a wide board. -->
    <!-- eslint-disable-next-line vue/no-v-html -- sanitized by renderMarkdown -->
    <div v-if="footerHtml" class="view-info view-info--bottom" v-html="footerHtml"/>

    <InlineCreateFormModal
      v-if="createModal.open.value && kanbanConfig?.create_form"
      :show="true"
      :form-id="kanbanConfig.create_form"
      :entity-type="kanbanConfig.entity"
      :world="worldParam"
      add-another
      @close="createModal.close"
      @created="createModal.created"
      @created-another="createModal.createdAnother"
    />
  </div>
</template>

<style scoped>
/* The horizontal scroll belongs to the board containers, NOT this page wrapper.
   With overflow-x here, a board wider than the viewport dragged the page title,
   filter bar, truncation banner, and info regions sideways along with the
   columns. Scoping it to the boards keeps page furniture fixed. */
.kanban-view {
  max-width: 100%;
  /* Fill the pane so the board scrolls inside it and the filters and column
     headings stay put. */
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

/* The board renders the list's FilterBar, whose bottom border separates the
   filters from a list's table. A board has no table to separate from. */
.filter-bar {
  border-bottom: none;
}

.truncation-banner {
  padding: 10px 16px;
  margin-bottom: 16px;
  border: 1px solid #f59e0b;
  border-radius: var(--radius-lg);
  background: rgba(245, 158, 11, 0.12);
  color: var(--rl-color-text);
  font-size: var(--font-size-base);
}



.world-banner {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-xs);
  margin-bottom: var(--space-md);
  padding: var(--space-sm) var(--space-md);
  background: color-mix(in srgb, var(--rl-color-accent) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--rl-color-accent) 30%, transparent);
  border-radius: var(--radius-md);
}

.world-banner__label {
  font-size: var(--font-size-base);
  color: var(--rl-color-text);
  font-weight: 500;
}

.world-banner__note {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

/* The list item exists purely for <ul>/<li> semantics; `display: contents`
   removes its box so the .kanban-card anchor inside remains the flex item of
   .column-cards / .swimlane-cell, exactly as it was before the card became a
   link. RouterLink cannot render an <li> itself. */
/* The library boards set their own gutters; the page already has one. */
.kanban-board {
  flex: 1;
  min-height: 0;
  padding-left: 0;
  padding-right: 0;
}

/* Matches the library's task card, so the board reads as one component. */
.kanban-card {
  display: block;
  padding: var(--rl-space-3) var(--rl-space-4);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg);
  transition: box-shadow 120ms ease, border-color 120ms ease;
  /* A real link, so it must not pick up link colour or underline. */
  color: inherit;
  text-decoration: none;
}

.kanban-card:hover {
  border-color: var(--rl-color-border-strong);
  box-shadow: var(--rl-shadow-sm);
}

.kanban-card:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}

.card-id {
  font-family: monospace;
  font-size: var(--font-size-xs);
  color: var(--rl-color-text-muted);
  margin-bottom: 4px;
}

.card-title {
  font-size: var(--font-size-base);
  font-weight: 500;
  color: var(--rl-color-text);
  margin-bottom: 8px;
}
</style>
