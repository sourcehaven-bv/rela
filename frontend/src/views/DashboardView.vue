<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted, watch } from 'vue'
import { usePageTitle } from '@/composables/useDocumentTitle'
import { useSchemaStore, useUIStore } from '@/stores'
import {
  searchEntities,
  searchAllEntities,
  analyze,
  getDashboard,
  getDashboardCard,
  ApiError,
} from '@/api'
import { useWorld } from '@/composables/useWorld'
import type { RouteLocationRaw } from 'vue-router'
import DashboardMatrix from '@/components/dashboard/DashboardMatrix.vue'
import DashboardRatio from '@/components/dashboard/DashboardRatio.vue'
import DashboardRecordsChart from '@/components/dashboard/DashboardRecordsChart.vue'
import Badge from '@/components/ui/Badge.vue'
import {
  listBreakdown,
  queryBreakdown,
  summaryText,
  tableRows,
  type BreakdownItem,
  type TableRow,
} from '@/widgets/dashboardCards'

import type {
  Entity,
  AnalyzeResult,
  DashboardCardEntry,
  DashboardCardValue,
  DashboardResponse,
} from '@/types'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'
import PageHeaderContent from '@/components/common/PageHeaderContent'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'

/**
 * `name` is a dashboard under `dashboards:` (TKT-5LQNLY), from
 * `/dashboard/<name>` or a page tab. Without it this is the default
 * dashboard, which the schema store prefetches.
 *
 * `anchor` is set by a `scope: root` tab of an entity page: the entity an
 * entity dashboard (`entity_type:`) counts for. The server narrows each card
 * to what the anchor reaches. An entity dashboard opened without an anchor,
 * such as from a stale `/dashboard/<name>` bookmark, has nothing to count and
 * says so.
 */
const props = defineProps<{ name?: string; anchor?: string }>()
const { t } = useI18n()

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const { worldParam } = useWorld()

// The named dashboard's per-principal payload, fetched on entry since only
// the default one is prefetched. `notFound` is the server's 404 for a name
// that is not configured, such as a stale bookmark.
const named = ref<DashboardResponse | undefined>(undefined)
const notFound = ref(false)

// State
const loading = ref(true)

// pageState mirrors DynamicForm's `form-state-*` contract: a stable signal
// that this screen has finished resolving, so a screenshot{} capture can wait
// for it rather than hanging until its timeout.
const loadError = ref(false)
const pageState = computed<'pending' | 'loaded' | 'error'>(() => {
  if (loadError.value) return 'error'
  return loading.value ? 'pending' : 'loaded'
})
// Keyed by cardKey(), not by array index: the card list is per-principal
// (TKT-53KICM), so its length and contents can differ between loads. An
// index-keyed map survives such a change and binds one card's rows to
// another's tile — e.g. dropping the first of two cards leaves the survivor
// rendering the dropped card's count.
const cardData = ref<Map<string, { entities: Entity[]; count: number }>>(new Map())
// Server-computed values of `list:` cards (TKT-2SGPJZ), keyed like cardData.
const cardValues = ref<Map<string, DashboardCardValue>>(new Map())
// List cards whose value could not be loaded. One failing card (say, a list
// filter on a property this reader cannot see) fails only its own tile.
const cardErrors = ref<Set<string>>(new Set())
const analysisResult = ref<AnalyzeResult | null>(null)

/**
 * A stable identity for a card, derived from its content.
 *
 * Cards have no configured id, so this is the best available substitute:
 * `title` alone is not guaranteed unique, and `query` alone is genuinely
 * shared by cards that display the same data differently.
 *
 * JSON.stringify rather than string concatenation, so the parts cannot run
 * together: `{title:'a b', query:'c'}` and `{title:'a', query:'b c'}` must not
 * collide into one key.
 */
function cardKey(card: DashboardCardEntry): string {
  // A list card is addressed by its config index, so the index is part of
  // its identity; a query card keeps the key it always had.
  if (card.list) return JSON.stringify(['list', card.index, card.title, card.display, card.list])
  if (card.records) return JSON.stringify(['records', card.index, card.title])
  return JSON.stringify([card.title, card.query, card.display])
}

/**
 * Whether a computed value belongs to the card it was asked for. A config
 * reload between `/_dashboard` and `/_dashboard/card` can move the indexes;
 * a value for another card is dropped rather than shown on this tile.
 */
/** Whether a card folds one property into one value. */
function isSummary(display: DashboardCardEntry['display']): boolean {
  return display === 'sum' || display === 'avg' || display === 'min' || display === 'max'
}

function valueMatches(card: DashboardCardEntry, value: DashboardCardValue): boolean {
  return value.title === card.title && value.display === card.display && value.list === card.list
}

// Computed
const dashboardConfig = computed(() => (props.name ? named.value : schemaStore.dashboard))
const title = computed(() => dashboardConfig.value?.title || t('dashboardView.title'))
usePageTitle(title)
const description = computed(() => dashboardConfig.value?.description)
const cards = computed(() => dashboardConfig.value?.cards || [])
// Every card of an entity dashboard has a scope, and no other card does.
const entityDashboard = computed(() => cards.value.some((c) => !!c.scope))
// An entity dashboard needs its anchor; without one no card is asked for.
const anchorMissing = computed(() => entityDashboard.value && !props.anchor)
// Whether the validation card shows. The server resolves the dashboard's
// `validation:` toggle for this principal (TKT-31822PN); an older server
// sends nothing and the card shows. The card covers the whole project, so an
// entity dashboard, which is about one entity, never shows it. Hiding it is a
// UX choice, not access control: /analyze keeps its own gate.
const showValidation = computed(
  () => !entityDashboard.value && dashboardConfig.value?.validation !== false
)

// Methods
// Bumped on every load, so a slow load for a dashboard the user has already
// left cannot overwrite the one on screen.
let generation = 0

async function loadData() {
  const gen = ++generation
  const stale = () => gen !== generation
  loading.value = true
  loadError.value = false
  notFound.value = false
  cardData.value = new Map()
  cardValues.value = new Map()
  cardErrors.value = new Set()

  try {
    if (props.name) {
      named.value = undefined
      try {
        const resp = await getDashboard(props.name)
        if (stale()) return
        named.value = resp
      } catch (err) {
        if (stale()) return
        if (err instanceof ApiError && err.status === 404) {
          notFound.value = true
          return
        }
        throw err
      }
    }
    if (anchorMissing.value) return
    // An entity dashboard counts for the anchor only; a plain one never
    // sends it, which the server would refuse.
    const anchor = entityDashboard.value ? props.anchor : undefined
    // Load card data in parallel
    const cardPromises = cards.value.map(async (card) => {
      // A line card loads its own series from the records API.
      if (card.records) return
      if (card.list) {
        // A list card counts in the page's world, as its list would.
        try {
          const value = await getDashboardCard(props.name, card.index, worldParam.value, anchor)
          if (stale()) return
          if (valueMatches(card, value)) cardValues.value.set(cardKey(card), value)
        } catch (err) {
          if (stale()) return
          console.error('Dashboard card load error:', err)
          cardErrors.value.add(cardKey(card))
        }
        return
      }
      // Query cards count in the default world. Counting one in another
      // world is a separate decision, like next-action's per-source
      // `source_world:`. A breakdown or a table is computed over every
      // row, so it reads every page; a count needs only meta.total, which
      // the first page carries (TKT-T0DK37).
      const query = card.query ?? ''
      const response =
        card.display === 'breakdown' || card.display === 'table'
          ? await searchAllEntities(query)
          : await searchEntities(query)
      if (stale()) return
      cardData.value.set(cardKey(card), {
        entities: response.data,
        count: response.meta.total,
      })
    })

    // A hidden validation card has nothing to load.
    const analysisPromise = showValidation.value
      ? analyze().then((r) => (analysisResult.value = r))
      : Promise.resolve()

    await Promise.all([...cardPromises, analysisPromise])
  } catch (err) {
    if (stale()) return
    console.error('Dashboard load error:', err)
    loadError.value = true
  } finally {
    if (!stale()) loading.value = false
  }
}

/** One rendered tile: the card plus everything the template needs derived. */
interface CardView {
  card: DashboardCardEntry
  key: string
  count: number
  breakdown: BreakdownItem[]
  rows: TableRow[]
  /** A table card's column headers, one per column, resolved once. */
  columnLabels: string[]
  /** Some rows hide the grouped value from this reader and are not counted. */
  partial: boolean
  /** The server-computed value of a list card; absent for a query card. */
  value?: DashboardCardValue
  /** A sum, avg, min or max card's formatted value; absent when it has none. */
  summary?: string
  /**
   * Where the card's link goes: its list, or the search for its query.
   * Absent on an entity dashboard, where the list would show the whole type.
   */
  link?: RouteLocationRaw
  /** True when a list card's value failed to load. */
  failed: boolean
}

/** A list card's table rows, shaped like the entities the template renders. */
function valueRows(value: DashboardCardValue): Entity[] {
  return (value.rows ?? []).map((r) => ({
    id: r.id,
    type: r.type,
    _title: r._title,
    properties: r.properties,
    _redacted: r._redacted,
  }))
}

function cardLink(card: DashboardCardEntry): RouteLocationRaw {
  if (card.list) {
    // The list opens in the world the card was counted in.
    return {
      path: `/list/${encodeURIComponent(card.list)}`,
      query: worldParam.value ? { world: worldParam.value } : {},
    }
  }
  return `/search?q=${encodeURIComponent(card.query ?? '')}`
}

function buildRows(card: DashboardCardEntry, entities: Entity[]): Entity[] {
  const sorted = [...entities]

  if (card.sort?.length) {
    const sort = card.sort[0]
    sorted.sort((a, b) => {
      const aVal = String(a.properties[sort.property] || '')
      const bVal = String(b.properties[sort.property] || '')
      const cmp = aVal.localeCompare(bVal)
      return sort.direction === 'desc' ? -cmp : cmp
    })
  }

  return card.limit ? sorted.slice(0, card.limit) : sorted
}

/**
 * The rendered tiles, with each card's display data derived exactly once
 * (TKT-ERHWL0).
 *
 * The template used to call `getBreakdown(card)` / `getTableRows(card)` twice
 * per card — once for `v-if="…length"`, once for `v-for` — and both are O(N)
 * or worse over the card's whole result set (a group-by; a copy plus a
 * `localeCompare` sort). Deriving into the view model the template iterates
 * means each derivation has exactly one call site, structurally, rather than
 * being deduplicated by a cache.
 *
 * One entry per card BY POSITION, deliberately not a Map keyed by `cardKey()`.
 * `cardKey` covers `[title, query, display]`, which is right for `cardData` —
 * two cards with the same query legitimately share one fetch — but wrong here,
 * because `group_by` / `sort` / `limit` change the derivation without changing
 * the key. Keying the derived data by `cardKey` let a "by status" card render a
 * "by priority" card's breakdown: the exact one-card's-data-on-another's-tile
 * bug `cardKey` was introduced (TKT-53KICM) to prevent.
 *
 * Deriving only for the display mode that renders it also keeps a count card
 * from paying for a table copy of a result set nothing shows.
 */
const cardViews = computed<CardView[]>(() =>
  cards.value.map((card) => {
    const view = buildCardView(card)
    const columnLabels =
      card.display === 'table'
        ? (card.columns ?? []).map((col) => getColumnLabel(col, view.rows))
        : []
    return { ...view, columnLabels }
  })
)

/** One tile without its column labels, which need the rows first. */
function buildCardView(card: DashboardCardEntry): Omit<CardView, 'columnLabels'> {
  const key = cardKey(card)
  const link = entityDashboard.value || card.records ? undefined : cardLink(card)
  if (card.list) {
    const value = cardValues.value.get(key)
    return {
      card,
      key,
      link,
      value,
      count: value?.total ?? 0,
      partial: value?.partial ?? false,
      breakdown:
        value && card.display === 'breakdown'
          ? listBreakdown(schemaStore, card, value, t('dashboard.unknown'))
          : [],
      rows:
        value && card.display === 'table'
          ? tableRows(schemaStore, card, valueRows(value), uiStore.effectiveTimezone)
          : [],
      summary: value ? summaryText(schemaStore, card, value, uiStore.effectiveTimezone) : undefined,
      failed: cardErrors.value.has(key),
    }
  }
  const entities = cardData.value.get(key)?.entities ?? []
  const breakdown =
    card.display === 'breakdown' && card.group_by
      ? queryBreakdown(schemaStore, card.group_by, entities, t('dashboard.unknown'))
      : { items: [], partial: false }
  return {
    link,
    failed: false,
    card,
    key,
    count: cardData.value.get(key)?.count || 0,
    partial: breakdown.partial,
    breakdown: breakdown.items,
    rows:
      card.display === 'table'
        ? tableRows(schemaStore, card, buildRows(card, entities), uiStore.effectiveTimezone)
        : [],
  }
}

/**
 * Whether a `hide_empty:` card loaded and has no rows for this reader. A
 * card that failed or has not loaded is not empty: an error must not read
 * as "nothing to show".
 */
function loadedEmpty(view: CardView): boolean {
  if (view.failed) return false
  if (view.card.list) return view.value !== undefined && view.value.total === 0
  const data = cardData.value.get(view.key)
  return data !== undefined && data.count === 0
}

/** The tiles that show: every card but a `hide_empty:` card with no rows. */
const shownViews = computed(() =>
  cardViews.value.filter((view) => !(view.card.hide_empty && loadedEmpty(view)))
)

/**
 * The tile class for a card's `width:` (TKT-H21TYS). Any other value, which
 * the server rejects at config load, gets no class and renders as normal.
 */
function widthClass(width: DashboardCardEntry['width']): string | undefined {
  if (width === 'wide') return 'dashboard-card--wide'
  if (width === 'full') return 'dashboard-card--full'
  return undefined
}

/**
 * A table card's column header: its own label, then the schema label of the
 * property on the first row type that has one (the label forms show), then
 * the raw property name.
 */
function getColumnLabel(
  col: { property?: string; label?: string },
  rows: { entity: { type: string } }[]
): string {
  if (col.label) return col.label
  if (col.property) {
    for (const type of new Set(rows.map((r) => r.entity.type))) {
      const label = schemaStore.getEntityType(type)?.properties[col.property]?.label
      if (label) return label
    }
  }
  return col.property || ''
}

// Lifecycle
onMounted(async () => {
  // Belt-and-braces, NOT the thing preventing the empty-state flash: App.vue
  // renders <RouterView/> only after the store has loaded, so this view cannot
  // normally mount with `loaded === false`. It matters only if the boot
  // sequence changes, or when this component is mounted directly (as its unit
  // tests do). Without a gate somewhere, "No dashboard cards to show" — which
  // is indistinguishable from the all-filtered state — would flash on load.
  if (!schemaStore.loaded) {
    await schemaStore.load()
  }
  await loadData()
})

// The router reuses this view between two named dashboards, and a world
// change recounts the list cards.
watch([() => props.name, () => props.anchor, worldParam], () => void loadData())
</script>

<template>
  <div class="dashboard-view" :data-testid="`page-state-${pageState}`">
    <!--
      The title goes in the shell's header band, like a list's. A header of
      the dashboard's own sat below the page's notice, and on a phone it kept
      a slot for a menu button that the notice had pushed onto a row of its
      own (BUG-UX4P9T2).
    -->
    <PageHeaderContent :title="title" />
    <p v-if="description" class="description">{{ description }}</p>

    <RlStatusRegion v-if="loading">{{ t('dashboardView.loading') }}</RlStatusRegion>

    <RlEmptyState
      v-else-if="notFound"
      :title="t('dashboardView.notFound')"
      :description="t('dashboardView.notFoundDescription', { name })"
      data-testid="dashboard-not-found"
    />

    <RlEmptyState
      v-else-if="anchorMissing"
      :title="t('dashboardView.anchorMissing')"
      :description="t('dashboardView.anchorMissingDescription')"
      data-testid="dashboard-anchor-missing"
    />

    <template v-else>
      <!--
        No cards to show. Deliberately one state for three causes: no
        `dashboard:` configured, an empty `cards:`, and every card filtered out
        by `permission:` (TKT-53KICM). Distinguishing them would tell a user
        about cards they cannot use, which is the opposite of the point.
      -->
      <p v-if="cards.length === 0" class="no-data dashboard-empty">
        {{ t('dashboardView.noCards') }}
      </p>

      <div v-else class="dashboard-grid">
        <section
          v-for="view in shownViews"
          :key="view.key"
          class="dashboard-card"
          :class="widthClass(view.card.width)"
          :aria-labelledby="`dashboard-card-${view.key}-title`"
        >
          <div class="card-header">
            <h3 :id="`dashboard-card-${view.key}-title`">{{ view.card.title }}</h3>
            <router-link
              v-if="view.link"
              :to="view.link"
              class="card-link"
              :title="view.card.list ? t('dashboard.openList') : t('dashboard.viewInSearch')"
            >
              &#8599;
            </router-link>
          </div>

          <p v-if="view.failed" class="no-data" data-testid="card-failed">
            {{ t('dashboard.cardFailed') }}
          </p>

          <p v-else-if="view.partial" class="card-partial" data-testid="card-partial">
            {{ t('dashboard.partial') }}
          </p>

          <!-- Count display -->
          <template v-if="view.failed" />
          <div v-else-if="view.card.display === 'count'" class="card-count">
            <span class="count-number">{{ view.count }}</span>
          </div>

          <!-- Sum, avg, min and max displays (list cards only) -->
          <template v-else-if="isSummary(view.card.display)">
            <div v-if="view.summary !== undefined" class="card-count" data-testid="card-summary">
              <span class="count-number">{{ view.summary }}</span>
            </div>
            <div v-else class="no-data">{{ t('dashboard.noData') }}</div>
          </template>

          <!-- Breakdown display -->
          <div v-else-if="view.card.display === 'breakdown'" class="card-breakdown">
            <div v-for="item in view.breakdown" :key="item.key" class="breakdown-row">
              <span class="breakdown-label">
                <Badge
                  v-if="item.badge"
                  :value="item.key"
                  :property="view.card.group_by"
                  :entity-type="item.entityType"
                />
                <template v-else>{{ item.label }}</template>
              </span>
              <div class="breakdown-bar-track">
                <div
                  class="breakdown-bar-fill"
                  :style="{
                    width: `${item.percentage}%`,
                    ...(item.color ? { background: item.color } : {}),
                  }"
                />
              </div>
              <span class="breakdown-count">{{ item.count }}</span>
            </div>
            <div v-if="view.breakdown.length === 0" class="no-data">
              {{ t('dashboard.noData') }}
            </div>
          </div>

          <!-- Table display -->
          <!--
            The explicit roles keep the table a table for assistive technology
            on a phone, where the CSS below stacks each row into a block and a
            browser would otherwise drop the table semantics.
          -->
          <div v-else-if="view.card.display === 'table'" class="card-table">
            <table
              v-if="view.rows.length > 0"
              role="table"
              :aria-labelledby="`dashboard-card-${view.key}-title`"
            >
              <thead role="rowgroup">
                <tr role="row">
                  <th
                    v-for="(label, i) in view.columnLabels"
                    :key="view.card.columns?.[i]?.property ?? i"
                    role="columnheader"
                    scope="col"
                  >
                    {{ label }}
                  </th>
                </tr>
              </thead>
              <tbody role="rowgroup">
                <tr v-for="row in view.rows" :key="row.entity.id" role="row">
                  <td
                    v-for="(cell, i) in row.cells"
                    :key="cell.key"
                    role="cell"
                    :class="{ 'cell--empty': !cell.hidden && !cell.widget && !cell.text }"
                  >
                    <!-- The column name beside the value, shown only on a phone. -->
                    <span class="cell-label" aria-hidden="true">{{ view.columnLabels[i] }}</span>
                    <span class="cell-value" :class="{ 'cell-value--nowrap': cell.nowrap }">
                      <router-link v-if="cell.link" :to="cell.link" class="cell-link">
                        {{ cell.text }}
                      </router-link>
                      <span
                        v-else-if="cell.hidden"
                        class="cell-hidden"
                        :title="t('dashboard.hiddenTitle')"
                        >{{ t('dashboard.hidden') }}</span
                      >
                      <component
                        :is="cell.widget.component"
                        v-else-if="cell.widget"
                        :model-value="cell.widget.modelValue"
                        mode="display"
                        :property-name="cell.widget.propertyName"
                        :entity-type="row.entity.type"
                        :entity-id="row.entity.id"
                      />
                      <span v-else>{{ cell.text }}</span>
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
            <div v-else class="no-data">{{ t('dashboardView.noResults') }}</div>
          </div>

          <!-- Matrix display (list cards only) -->
          <template v-else-if="view.card.display === 'matrix'">
            <DashboardMatrix
              v-if="view.value?.matrix"
              :matrix="view.value.matrix"
              :list="view.card.list ?? ''"
              :title="view.card.title"
              :toned="!!view.card.tone_by"
              :world="worldParam"
              :unlinked="entityDashboard"
            />
            <div v-else class="no-data">{{ t('dashboard.noData') }}</div>
          </template>

          <!-- Line display: a trend over records (TKT-RK810V7) -->
          <DashboardRecordsChart
            v-else-if="view.card.display === 'line' && view.card.records"
            :records="view.card.records"
            :title="view.card.title"
          />

          <!-- Ratio display (list cards only) -->
          <template v-else-if="view.card.display === 'ratio'">
            <DashboardRatio
              v-if="view.value"
              :matched="view.value.matched ?? 0"
              :total="view.value.total"
              :groups="view.value.groups"
            />
            <div v-else class="no-data">{{ t('dashboard.noData') }}</div>
          </template>
        </section>
      </div>

      <!-- Validation card -->
      <div v-if="showValidation" class="validation-card" data-testid="dashboard-validation">
        <div class="card-header">
          <h3>&#9888; {{ t('dashboardView.validation') }}</h3>
          <router-link to="/analyze" class="card-link" :title="t('dashboardView.viewAnalysis')">
            &#8599;
          </router-link>
        </div>
        <div class="validation-content">
          <template v-if="analysisResult">
            <span
              v-if="analysisResult.errors === 0 && analysisResult.warnings === 0"
              class="validation-success"
            >
              &#10003; {{ t('dashboardView.allPassed') }}
            </span>
            <template v-else>
              <span v-if="analysisResult.errors > 0" class="badge badge-error">
                {{ t('dashboardView.errors', { n: analysisResult.errors }, analysisResult.errors) }}
              </span>
              <span v-if="analysisResult.warnings > 0" class="badge badge-warning">
                {{
                  t(
                    'dashboardView.warnings',
                    { n: analysisResult.warnings },
                    analysisResult.warnings
                  )
                }}
              </span>
              <router-link to="/analyze" class="view-details">
                {{ t('dashboardView.viewDetails') }} &rarr;
              </router-link>
            </template>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.dashboard-view {
  max-width: 1200px;
}

.description {
  color: var(--rl-color-text-muted);
  margin: 0 0 24px;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
  margin-bottom: 20px;
  /* The tiles query the grid's own width, not the viewport's: the sidebar
     changes it. */
  container: dashboard-grid / inline-size;
}

.dashboard-card {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-lg);
  padding: 16px;
}

/* Card width (TKT-H21TYS). The grid flows in config order, without
   `grid-auto-flow: dense`: dense would fill a hole with a later tile, so the
   visual order would no longer match the reading and Tab order. A `wide` tile
   that does not fit at the end of a row leaves one cell empty instead.

   A wide tile takes the whole row by default and spans two tracks only on a
   grid of at least 580px, which always holds two or more tracks. Below 580px
   the grid holds at most two tracks (280px tracks + 20px gap above; 200px +
   12px gap at <= 768px; a single track at <= 480px), so the whole row is "two
   tracks or one", and the tile never creates an implicit column. A browser
   without container queries gets the whole row, which never overflows.

   The threshold assumes the stock track and gap sizes. A container condition
   cannot read a custom property, so change 580px together with them. */
.dashboard-card--wide,
.dashboard-card--full {
  grid-column: 1 / -1;
}

@container dashboard-grid (width >= 580px) {
  .dashboard-card--wide {
    grid-column: span 2;
  }
}

.cell-hidden {
  color: var(--rl-color-text-muted);
  font-style: italic;
}

.card-partial {
  margin: 0 0 var(--space-xs);
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--rl-color-text);
}

.card-link {
  color: var(--rl-color-text-muted);
  text-decoration: none;
  font-size: var(--font-size-base);
}

.card-link:hover {
  color: var(--rl-color-accent);
}

/* Count display */
.card-count {
  padding: 16px 0;
}

.count-number {
  font-size: 48px;
  font-weight: 700;
  color: var(--rl-color-text);
}

/* Breakdown display */
.card-breakdown {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.breakdown-row {
  display: flex;
  align-items: center;
  gap: var(--space-md);
}

.breakdown-label {
  min-width: 80px;
  font-size: var(--font-size-dense);
  color: var(--rl-color-text-muted);
}

.breakdown-bar-track {
  flex: 1;
  height: 8px;
  background: var(--rl-color-bg-hover);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.breakdown-bar-fill {
  height: 100%;
  background: var(--rl-color-accent, #6366f1);
  border-radius: var(--radius-sm);
  transition: width 0.3s ease;
}

.breakdown-count {
  min-width: 32px;
  text-align: right;
  font-size: var(--font-size-dense);
  font-weight: 600;
  color: var(--rl-color-text);
}

/* Table display */
.card-table {
  overflow-x: auto;
}

.card-table table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--font-size-dense);
}

.card-table th {
  text-align: left;
  padding: 8px;
  border-bottom: 1px solid var(--rl-color-border);
  font-weight: 600;
  color: var(--rl-color-text-muted);
}

.card-table td {
  padding: 8px;
  border-bottom: 1px solid var(--rl-color-border);
  color: var(--rl-color-text);
}

/* The column name beside each value; only the stacked phone layout shows it. */
.cell-label {
  display: none;
}

.cell-value--nowrap {
  white-space: nowrap;
}

.cell-link {
  color: var(--rl-color-accent);
  text-decoration: none;
}

.cell-link:hover {
  text-decoration: underline;
}

.no-data {
  color: var(--rl-color-text-muted);
  font-size: var(--font-size-dense);
  padding: 8px 0;
}

/* Validation card */
.validation-card {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-lg);
  padding: 16px;
}

.validation-content {
  display: flex;
  align-items: center;
  gap: var(--space-md);
}

.validation-success {
  color: var(--rl-color-status-green);
  font-weight: 600;
  font-size: var(--font-size-base);
}

.badge {
  font-size: var(--font-size-sm);
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  font-weight: 500;
}

.badge-error {
  background: color-mix(in srgb, var(--rl-color-danger) 15%, transparent);
  color: var(--rl-color-danger);
}

.badge-warning {
  background: color-mix(in srgb, var(--rl-color-status-amber) 15%, transparent);
  color: var(--rl-color-status-amber);
}

.view-details {
  margin-left: auto;
  font-size: var(--font-size-dense);
  color: var(--rl-color-accent);
  text-decoration: none;
  font-weight: 500;
}

.view-details:hover {
  text-decoration: underline;
}

@media (max-width: 768px) {
  .dashboard-grid {
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: var(--space-md);
  }

  .dashboard-card {
    padding: 12px;
  }

  .card-header {
    margin-bottom: 8px;
  }

  /* Count cards: compact on mobile so the big number doesn't waste a
     screenful of vertical space. The number sits inline with the header
     instead of below it. */
  .card-count {
    padding: 0;
  }

  .count-number {
    font-size: var(--font-size-3xl);
  }

  .breakdown-label {
    min-width: 60px;
    font-size: var(--font-size-sm);
  }

  .breakdown-row {
    gap: var(--space-sm);
  }
}

@media (max-width: 480px) {
  .dashboard-grid {
    /* One card per row on phones, whatever its `width`. minmax(0, …) lets
       the track shrink below item min-content — with plain 1fr a table card
       blows the track past the viewport. */
    grid-template-columns: minmax(0, 1fr);
  }

  /* A table card stacks each row into a block on a phone: the first column
     as the row's heading, each other column as a "name  value" line. A
     table that scrolled sideways cut its columns off at the card's edge,
     and a reader could not tell more columns were there. */
  .card-table thead {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }

  .card-table table,
  .card-table tbody,
  .card-table tr,
  .card-table td {
    display: block;
  }

  .card-table tr {
    position: relative;
    padding: 8px 0;
    border-bottom: 1px solid var(--rl-color-border);
  }

  .card-table tr:last-child {
    border-bottom: none;
  }

  /* The value column is at least as wide as an unbreakable short value, so
     an id never overflows the card; the label column gives way. */
  .card-table td {
    display: grid;
    grid-template-columns: minmax(0, 2fr) minmax(min-content, 3fr);
    align-items: center;
    gap: var(--space-sm);
    padding: 2px 0;
    border-bottom: none;
  }

  .card-table td:first-child {
    grid-template-columns: minmax(0, 1fr);
    font-weight: 600;
  }

  /* A long value without spaces, such as an email address, wraps rather
     than widening its column past the labels of the other lines. A short
     token keeps its line (cell-value--nowrap). */
  .card-table .cell-value:not(.cell-value--nowrap) {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .card-table td:not(:first-child) .cell-label {
    display: block;
    color: var(--rl-color-text-muted);
    overflow-wrap: anywhere;
  }

  /* A value with nothing in it would leave a label with no answer. The cell
     is hidden from sight only: a screen reader still counts it, so the cells
     after it keep their column headers. */
  .card-table td.cell--empty:not(:first-child) {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
}
</style>
