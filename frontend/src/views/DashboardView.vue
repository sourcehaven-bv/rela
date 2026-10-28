<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted, watch } from 'vue'
import { useSchemaStore } from '@/stores'
import NextActionCard from '@/components/NextActionCard.vue'
import { searchEntities, analyze, getDashboard, getDashboardCard, ApiError } from '@/api'
import { useWorld } from '@/composables/useWorld'
import { isPropertyRedacted } from '@/utils/affordances'
import type { RouteLocationRaw } from 'vue-router'
import DashboardMatrix from '@/components/dashboard/DashboardMatrix.vue'
import DashboardRatio from '@/components/dashboard/DashboardRatio.vue'

import type {
  Entity,
  AnalyzeResult,
  DashboardCardEntry,
  DashboardCardValue,
  DashboardResponse,
} from '@/types'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'

/**
 * `name` is a dashboard under `dashboards:` (TKT-5LQNLY), from
 * `/dashboard/<name>` or a page tab. Without it this is the default
 * dashboard, which the schema store prefetches.
 */
const props = defineProps<{ name?: string }>()
const { t } = useI18n()

const schemaStore = useSchemaStore()
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
  return JSON.stringify([card.title, card.query, card.display])
}

/**
 * Whether a computed value belongs to the card it was asked for. A config
 * reload between `/_dashboard` and `/_dashboard/card` can move the indexes;
 * a value for another card is dropped rather than shown on this tile.
 */
function valueMatches(card: DashboardCardEntry, value: DashboardCardValue): boolean {
  return value.title === card.title && value.display === card.display && value.list === card.list
}

// Computed
const dashboardConfig = computed(() => (props.name ? named.value : schemaStore.dashboard))
const title = computed(() => dashboardConfig.value?.title || 'Dashboard')
const description = computed(() => dashboardConfig.value?.description)
const cards = computed(() => dashboardConfig.value?.cards || [])

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
    // Load card data in parallel
    const cardPromises = cards.value.map(async (card) => {
      if (card.list) {
        // A list card counts in the page's world, as its list would.
        try {
          const value = await getDashboardCard(props.name, card.index, worldParam.value)
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
      // `source_world:`.
      const response = await searchEntities(card.query ?? '')
      if (stale()) return
      cardData.value.set(cardKey(card), {
        entities: response.data,
        count: response.meta.total,
      })
    })

    // Load analysis
    const analysisPromise = analyze()

    await Promise.all([...cardPromises, analysisPromise.then((r) => (analysisResult.value = r))])
  } catch (err) {
    if (stale()) return
    console.error('Dashboard load error:', err)
    loadError.value = true
  } finally {
    if (!stale()) loading.value = false
  }
}

type Breakdown = Array<{ key?: string; value: string; count: number; percentage: number }>

/** One rendered tile: the card plus everything the template needs derived. */
interface CardView {
  card: DashboardCardEntry
  key: string
  count: number
  breakdown: Breakdown
  rows: Entity[]
  /** The server-computed value of a list card; absent for a query card. */
  value?: DashboardCardValue
  /** Where the card's link goes: its list, or the search for its query. */
  link: RouteLocationRaw
  /** True when a list card's value failed to load. */
  failed: boolean
}

function buildBreakdown(card: DashboardCardEntry, entities: Entity[]): Breakdown {
  if (!card.group_by) return []

  const groupBy = card.group_by
  const counts: Record<string, number> = {}
  let total = 0

  for (const entity of entities) {
    const value = String(entity.properties[groupBy] || 'Unknown')
    counts[value] = (counts[value] || 0) + 1
    total++
  }

  return Object.entries(counts)
    .map(([value, count]) => ({
      value,
      count,
      percentage: total > 0 ? (count / total) * 100 : 0,
    }))
    .sort((a, b) => b.count - a.count)
}

/** A list card's breakdown, from the groups the server counted. */
function valueBreakdown(value: DashboardCardValue): Breakdown {
  return (value.groups ?? []).map((g) => ({
    key: g.value,
    value: g.label || t('dashboard.unknown'),
    count: g.count,
    percentage: value.total > 0 ? (g.count / value.total) * 100 : 0,
  }))
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
    const key = cardKey(card)
    const link = cardLink(card)
    if (card.list) {
      const value = cardValues.value.get(key)
      return {
        card,
        key,
        link,
        value,
        count: value?.total ?? 0,
        breakdown: value && card.display === 'breakdown' ? valueBreakdown(value) : [],
        rows: value && card.display === 'table' ? valueRows(value) : [],
        failed: cardErrors.value.has(key),
      }
    }
    const entities = cardData.value.get(key)?.entities ?? []
    return {
      link,
      failed: false,
      card,
      key,
      count: cardData.value.get(key)?.count || 0,
      breakdown: card.display === 'breakdown' ? buildBreakdown(card, entities) : [],
      rows: card.display === 'table' ? buildRows(card, entities) : [],
    }
  })
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

function getColumnLabel(col: { property?: string; label?: string }): string {
  return col.label || col.property || ''
}

function getCellValue(entity: Entity, col: { property?: string }): string {
  if (!col.property) return ''
  return String(entity.properties[col.property] || '')
}

function getCellLink(entity: Entity, col: { link?: string }): string | undefined {
  if (col.link === 'detail') {
    return `/entity/${entity.type}/${entity.id}`
  }
  return undefined
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
watch([() => props.name, worldParam], () => void loadData())
</script>

<template>
  <div class="dashboard-view" :data-testid="`page-state-${pageState}`">
    <header class="dashboard-header mobile-topbar mobile-topbar--with-menu">
      <h1>{{ title }}</h1>
      <p v-if="description" class="description">{{ description }}</p>
    </header>

    <RlStatusRegion v-if="loading">Loading dashboard...</RlStatusRegion>

    <RlEmptyState
      v-else-if="notFound"
      title="Dashboard not found"
      :description="`No dashboard is named “${name}”.`"
      data-testid="dashboard-not-found"
    />

    <template v-else>
      <!-- Above the cards, and independent of them: a suggestion is worth
           showing even on a dashboard with nothing else on it. A named
           dashboard shows it too, as it does the analysis summary: both are
           project-wide, not part of any one dashboard's cards. -->
      <NextActionCard />

      <!--
        No cards to show. Deliberately one state for three causes: no
        `dashboard:` configured, an empty `cards:`, and every card filtered out
        by `permission:` (TKT-53KICM). Distinguishing them would tell a user
        about cards they cannot use, which is the opposite of the point.
      -->
      <p v-if="cards.length === 0" class="no-data dashboard-empty">No dashboard cards to show.</p>

      <div v-else class="dashboard-grid">
        <section
          v-for="view in cardViews"
          :key="view.key"
          class="dashboard-card"
          :class="widthClass(view.card.width)"
          :aria-labelledby="`dashboard-card-${view.key}-title`"
        >
          <div class="card-header">
            <h3 :id="`dashboard-card-${view.key}-title`">{{ view.card.title }}</h3>
            <router-link
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

          <p v-else-if="view.value?.partial" class="card-partial" data-testid="card-partial">
            {{ t('dashboard.partial') }}
          </p>

          <!-- Count display -->
          <template v-if="view.failed" />
          <div v-else-if="view.card.display === 'count'" class="card-count">
            <span class="count-number">{{ view.count }}</span>
          </div>

          <!-- Breakdown display -->
          <div v-else-if="view.card.display === 'breakdown'" class="card-breakdown">
            <div v-for="item in view.breakdown" :key="item.key ?? item.value" class="breakdown-row">
              <span class="breakdown-label">{{ item.value }}</span>
              <div class="breakdown-bar-track">
                <div class="breakdown-bar-fill" :style="{ width: `${item.percentage}%` }" />
              </div>
              <span class="breakdown-count">{{ item.count }}</span>
            </div>
            <div v-if="view.breakdown.length === 0" class="no-data">No data</div>
          </div>

          <!-- Table display -->
          <div v-else-if="view.card.display === 'table'" class="card-table">
            <table
              v-if="view.rows.length > 0"
              :aria-labelledby="`dashboard-card-${view.key}-title`"
            >
              <thead>
                <tr>
                  <th v-for="col in view.card.columns" :key="col.property" scope="col">
                    {{ getColumnLabel(col) }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="entity in view.rows" :key="entity.id">
                  <td v-for="col in view.card.columns" :key="col.property">
                    <router-link
                      v-if="getCellLink(entity, col)"
                      :to="getCellLink(entity, col)!"
                      class="cell-link"
                    >
                      {{ getCellValue(entity, col) }}
                    </router-link>
                    <span
                      v-else-if="col.property && isPropertyRedacted(col.property, entity._redacted)"
                      class="cell-hidden"
                      :title="t('dashboard.hiddenTitle')"
                      >{{ t('dashboard.hidden') }}</span
                    >
                    <span v-else>{{ getCellValue(entity, col) }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
            <div v-else class="no-data">No results</div>
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
            />
            <div v-else class="no-data">{{ t('dashboard.noData') }}</div>
          </template>

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
      <div class="validation-card">
        <div class="card-header">
          <h3>&#9888; Validation</h3>
          <router-link to="/analyze" class="card-link" title="View full analysis">
            &#8599;
          </router-link>
        </div>
        <div class="validation-content">
          <template v-if="analysisResult">
            <span
              v-if="analysisResult.errors === 0 && analysisResult.warnings === 0"
              class="validation-success"
            >
              &#10003; All checks passed
            </span>
            <template v-else>
              <span v-if="analysisResult.errors > 0" class="badge badge-error">
                {{ analysisResult.errors }} {{ analysisResult.errors === 1 ? 'error' : 'errors' }}
              </span>
              <span v-if="analysisResult.warnings > 0" class="badge badge-warning">
                {{ analysisResult.warnings }}
                {{ analysisResult.warnings === 1 ? 'warning' : 'warnings' }}
              </span>
              <router-link to="/analyze" class="view-details"> View details &rarr; </router-link>
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

.dashboard-header {
  margin-bottom: 24px;
}

.dashboard-header h1 {
  margin: 0 0 8px;
}

.description {
  color: var(--rl-color-text-muted);
  margin: 0;
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
   12px gap at <= 768px; two fixed tracks on phones), so the whole row is "two
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
  /* .dashboard-header uses .mobile-topbar.mobile-topbar--with-menu from
     mobile-bars.css. Override only typography and hide the description
     to keep the bar compact. */
  .dashboard-header h1 {
    font-size: var(--font-size-lg);
    margin: 0;
  }

  .dashboard-header .description {
    display: none;
  }

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
    /* Two compact stat cards per row instead of one tall one. minmax(0, …)
       lets a track shrink below item min-content — with plain 1fr the
       full-width table cards blow the tracks past the viewport. */
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  /* Breakdown and table cards still want full width — too cramped at
     half width. */
  .dashboard-card:has(.card-breakdown),
  .dashboard-card:has(.card-table) {
    grid-column: 1 / -1;
  }
}
</style>
