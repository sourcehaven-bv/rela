<script setup lang="ts">
/**
 * Hierarchical gantt over the server-folded containment tree.
 *
 * The server does the security-sensitive work (row-gating, field redaction,
 * roll-up, caps — see internal/dataentry/gantt_handler.go); this view is pure
 * rendering plus navigation state. Two navigation modes coexist:
 *
 * - DRILL: clicking a bar re-roots the chart on that node and rescales the
 *   axis to its subtree (the flame-graph idiom — the answer to unbounded
 *   self-referential depth). The path lives in the URL so a drilled state is
 *   linkable and back-button-able.
 * - EXPAND: the twisty opens a subtree in place without re-rooting.
 *
 * All geometry lives in utils/ganttLayout.ts as pure functions.
 */
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { fromPageQuery } from '@/utils/pageContext'
import { getGantt, getErrorMessage, type GanttNode, type GanttResponse } from '@/api'
import { useSchemaStore } from '@/stores/schema'
import { renderMarkdown } from '@/utils/markdown'
import { cardFieldLabel, type KanbanCardField } from '@/types/config'
import { ICONS } from '@/utils/icons'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import {
  barSpan,
  findNode,
  flattenRows,
  forestSpan,
  isRowExpanded,
  parseDay,
  pct,
  PX_PER_DAY,
  SCROLL_UNIT_DAYS,
  ticksFor,
  withToday,
  type GanttZoom,
} from '@/utils/ganttLayout'

const props = defineProps<{
  id: string
  /**
   * Pins the chart to one entity's subtree, for a gantt tab of an entity page
   * (`scope: root`). The drill path in the URL then continues below it, and
   * there is no way up to the whole forest.
   */
  root?: string
}>()

const route = useRoute()
const router = useRouter()
const schemaStore = useSchemaStore()

const config = computed(() => schemaStore.getGantt(props.id))

const data = ref<GanttResponse | null>(null)
/** The root id `data` was fetched for; null means the full forest. */
const fetchedRoot = ref<string | null>(null)
const error = ref('')

/** Titles remembered per drilled id, so breadcrumbs stay labelled even when a
 * re-scoped fetch no longer carries the ancestors. */
const crumbTitles = ref<Map<string, string>>(new Map())

/** Numbers each fetch so a slow response for a scope the user already left
 * cannot overwrite the newer one. */
let fetchSeq = 0
/** True while a fetch for the shown scope is in flight. */
const loading = ref(false)

/** Drops any fetch still in flight: the scope it was for is no longer shown. */
function cancelFetch() {
  fetchSeq++
  loading.value = false
}

async function fetchScope(root: string | null) {
  const seq = ++fetchSeq
  loading.value = true
  error.value = ''
  try {
    const res = await getGantt(props.id, root ?? undefined)
    if (seq !== fetchSeq) return
    data.value = res
    fetchedRoot.value = root
  } catch (e) {
    if (seq !== fetchSeq) return
    data.value = null
    error.value = getErrorMessage(e)
  } finally {
    if (seq === fetchSeq) loading.value = false
  }
}

/**
 * Drill path from the URL (?path=id1,id2), below the pinned root when there
 * is one; [] means the full forest.
 */
const drillPath = computed<string[]>(() => {
  const raw = route.query.path
  const below = typeof raw !== 'string' || raw === '' ? [] : raw.split(',')
  return props.root ? [props.root, ...below] : below
})

/** The `?path=` for a drill path: the part below the pinned root. */
function pathQuery(path: string[]): string | undefined {
  const below = props.root ? path.slice(1) : path
  return below.length ? below.join(',') : undefined
}

const zoom = ref<GanttZoom>('month')
const expanded = ref<Set<string>>(new Set())

/**
 * Fetch policy: drilling is client-side (snappy — the subtree is usually
 * already here) EXCEPT when the fetched data cannot answer: the target is
 * missing from it, or the response was truncated (its children may have been
 * cut — this is what makes the truncated hint's "drill in to see more" true).
 * Going back up above the fetched scope refetches likewise.
 */
watch(
  [() => props.id, drillPath, () => props.root] as const,
  ([id, path, root], old) => {
    // Every navigation settles the scope anew, whether or not it fetches.
    cancelFetch()
    const idChanged = !old || id !== old[0] || root !== old[2]
    if (idChanged) {
      crumbTitles.value = new Map()
      expanded.value = new Set()
      data.value = null
      fetchedRoot.value = null
    } else {
      // A drill is a change of context; expansion set in the old context
      // rarely means anything in the new one, and would grow unboundedly.
      expanded.value = new Set()
    }
    const target = path.length ? path[path.length - 1] : null
    if (!idChanged && target === fetchedRoot.value) return
    const found = target !== null && data.value ? findNode(data.value.roots, target) : null
    const canAnswerLocally =
      !idChanged &&
      data.value !== null &&
      !data.value.truncated &&
      fetchedRoot.value === null &&
      (target === null || (found !== null && !found.has_more_children))
    if (!canAnswerLocally) void fetchScope(target)
  },
  { immediate: true }
)

/** The roots currently shown: the forest, or the drilled node's subtree. */
const currentRoots = computed<GanttNode[]>(() => {
  const roots = data.value?.roots ?? []
  const path = drillPath.value
  if (path.length === 0) return roots
  const node = findNode(roots, path[path.length - 1])
  return node ? [node] : roots
})

/** Breadcrumbs: id + best-known title (from the drill click, else the tree,
 * else the id — a deep link into a truncated tree has nothing better). */
const crumbs = computed<{ id: string; title: string }[]>(() => {
  const roots = data.value?.roots ?? []
  return drillPath.value.map((id) => ({
    id,
    title: crumbTitles.value.get(id) || findNode(roots, id)?.title || id,
  }))
})

const todayDay = computed(() => {
  // LOCAL calendar fields on purpose: the user's local "today" is what the
  // marker means, while every stored date is UTC-parsed. Rebuilding this from
  // toISOString() would shift the line a day for anyone west of Greenwich
  // after 00:00 UTC.
  const now = new Date()
  return parseDay(
    `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
  )
})

const axis = computed(() => {
  const span = forestSpan(currentRoots.value)
  return span && todayDay.value !== null ? withToday(span, todayDay.value, zoom.value) : span
})
const ticks = computed(() => {
  if (!axis.value) return []
  const all = ticksFor(axis.value, zoom.value)
  // Only a compressed timeline packs periods closer than a label's width.
  const stride = Math.max(1, Math.ceil(MIN_TICK_GAP_PX / (SCROLL_UNIT_DAYS[zoom.value] * pxPerDay.value)))
  return stride === 1 ? all : all.filter((_, i) => i % stride === 0)
})

/** The tree column's width; the timeline scrolls beside it. */
const TREE_W = 280
/** The widest timeline drawn; see pxPerDay. */
const MAX_TIMELINE_PX = 250_000
/** The closest two tick labels may sit; a compressed timeline thins them. */
const MIN_TICK_GAP_PX = 56
/** The narrowest bar, so a one-day item stays visible and hoverable. */
const MIN_BAR_PX = 6
/** Room a start-anchored label needs; a bar starting closer than this to the
 * timeline's end gets an end-anchored label instead. */
const LABEL_ROOM_PX = 240

const chartEl = ref<HTMLElement | null>(null)
/** Visible timeline width: the chart's width minus the tree column. */
const viewportW = ref(0)
/** The chart's height cap: the room left below its top edge, so its
 * horizontal scrollbar stays on screen whatever sits above it. */
const chartMaxH = ref(0)
let resizeObserver: ResizeObserver | undefined

function measure() {
  const el = chartEl.value
  if (!el) return
  viewportW.value = Math.max(el.clientWidth - TREE_W, 0)
  chartMaxH.value = Math.max(320, window.innerHeight - el.getBoundingClientRect().top - 24)
}
window.addEventListener('resize', measure)
onBeforeUnmount(() => window.removeEventListener('resize', measure))

watch(chartEl, (el) => {
  resizeObserver?.disconnect()
  resizeObserver = undefined
  if (!el) return
  measure()
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(measure)
    resizeObserver.observe(el)
  }
})
onBeforeUnmount(() => resizeObserver?.disconnect())

/**
 * Width of one day in px: the zoom's fixed width, stretched when the whole
 * span is narrower than the screen so a short plan still fills it, and
 * squeezed when the timeline would pass MAX_TIMELINE_PX. That cap keeps a
 * typo such as 2062 for 2026 from producing a timeline wider than the
 * browser can lay out; the chart then says it is compressed.
 */
const pxPerDay = computed(() => {
  const a = axis.value
  if (!a) return PX_PER_DAY[zoom.value]
  const days = Math.max(a.end - a.start, 1)
  return Math.min(Math.max(PX_PER_DAY[zoom.value], viewportW.value / days), MAX_TIMELINE_PX / days)
})
const compressed = computed(() => pxPerDay.value < PX_PER_DAY[zoom.value])
const timelineW = computed(() =>
  axis.value ? (axis.value.end - axis.value.start) * pxPerDay.value : 0
)

/** The shared horizontal scale, a percentage of the timeline width and linear
 * in days. Every positioned element — bars, gridlines, ticks, markers — goes
 * through it, so nothing drifts. */
const scale = computed(() => {
  const a = axis.value
  return a ? (day: number) => pct(day, a) : null
})

/** Today, when the axis holds it; null disables "Now". */
const nowDay = computed<number | null>(() => {
  const a = axis.value
  const t = todayDay.value
  return a && t !== null && t >= a.start && t <= a.end ? t : null
})

/** scrollTo moves the timeline; smooth for the buttons unless the user asked
 * for reduced motion, instant for re-anchoring after a layout change. */
function scrollTo(left: number, smooth: boolean) {
  const el = chartEl.value
  if (!el) return
  const reduce =
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (smooth && !reduce && typeof el.scrollTo === 'function')
    el.scrollTo({ left, behavior: 'smooth' })
  else el.scrollLeft = left
}

/** Scroll offset that centres a day in the visible timeline. */
function centreOffset(day: number, axisStart: number, ppd: number): number {
  return (day - axisStart) * ppd - viewportW.value / 2
}

function goNow() {
  const a = axis.value
  if (a && nowDay.value !== null)
    scrollTo(centreOffset(nowDay.value, a.start, pxPerDay.value), true)
}

function step(dir: -1 | 1) {
  const el = chartEl.value
  if (el) scrollTo(el.scrollLeft + dir * SCROLL_UNIT_DAYS[zoom.value] * pxPerDay.value, true)
}

/**
 * Scroll position across layout changes. A zoom change, a resize or a
 * refetch of the same scope keeps the day at the left edge where it was. A
 * new scope (first load, drill, breadcrumb) starts at today when the axis
 * holds it, else at the start.
 *
 * Dates are inclusive: a bar ends at the END of its end day, pos(end + 1).
 */
let shownScope: string | null = null
watch([axis, pxPerDay], ([a, ppd], old) => {
  // While a drill's fetch is in flight the chart shows a stand-in axis; the
  // scope opens when its own data arrives.
  if (!a || loading.value) return
  const scope = `${props.id}|${drillPath.value.join(',')}`
  const [oldA, oldPpd] = old ?? [null, 0]
  const el = chartEl.value
  const leftDay = scope === shownScope && oldA && el ? oldA.start + el.scrollLeft / oldPpd : null
  shownScope = scope
  void nextTick(() => {
    if (leftDay !== null) scrollTo((leftDay - a.start) * ppd, false)
    else scrollTo(nowDay.value !== null ? centreOffset(nowDay.value, a.start, ppd) : 0, false)
  })
})

/**
 * Gridlines as ONE shared multi-background style instead of per-row spans:
 * rows × ticks gridline elements dominated the DOM (34k of 52k nodes on a
 * 2000-row view, measured) for purely decorative lines. A 1px gradient per
 * tick, positioned by percentage, costs zero nodes.
 */
const gridStyle = computed(() => {
  const a = axis.value
  if (!a || !ticks.value.length) return {}
  return {
    backgroundImage: ticks.value
      .map(() => 'linear-gradient(to right, var(--rl-color-border) 1px, transparent 1px)')
      .join(', '),
    backgroundPosition: ticks.value.map((t) => `${pct(t.day, a)}% 0`).join(', '),
    backgroundSize: '1px 100%',
    backgroundRepeat: 'no-repeat',
  }
})

const defaultDepth = computed(() => config.value?.default_depth ?? 2)
const rows = computed(() =>
  axis.value ? flattenRows(currentRoots.value, defaultDepth.value, expanded.value) : []
)

/** cycleLabel describes the containment loop a node sits on. Worded
 * neutrally: whether a loop is a data error or a legitimate mutual
 * containment depends on the operator's schema, so the UI reports the shape
 * and the consequence without calling it a mistake. */
function cycleLabel(node: GanttNode): string {
  return `${node.title || node.id} is part of a containment loop — one repeating edge is not shown`
}

/** openEntity navigates to the node's entity page — the tree-column name's
 * click, and the fallback for chart clicks that cannot drill. */
function openEntity(node: GanttNode) {
  // Inside a page tab, Back on the entity returns to the tab.
  const path = `/entity/${node.type}/${node.id}`
  const query = fromPageQuery(route)
  router.push(Object.keys(query).length ? { path, query } : path)
}

function drill(node: GanttNode) {
  // The drilled root's own row (always the top of a drilled view) cannot
  // drill further into itself, and a leaf has nothing to drill into —
  // both fall back to the entity page rather than a dead click.
  const atDrilledRoot = drillPath.value[drillPath.value.length - 1] === node.id
  if (atDrilledRoot || (!node.children?.length && !node.has_more_children)) {
    openEntity(node)
    return
  }
  const titles = new Map(crumbTitles.value)
  titles.set(node.id, node.title || node.id)
  crumbTitles.value = titles
  const path = [...drillPath.value, node.id]
  router.push({ query: { ...route.query, path: pathQuery(path) } })
}

function drillTo(index: number) {
  const path = drillPath.value.slice(0, index + 1)
  router.push({ query: { ...route.query, path: pathQuery(path) } })
}

function toggleExpand(node: GanttNode) {
  const next = new Set(expanded.value)
  if (next.has(node.id)) next.delete(node.id)
  else next.add(node.id)
  expanded.value = next
}

/** Bar geometry for one row, all values 0-100 percentages of the axis. */
function barStyle(node: GanttNode) {
  const pos = scale.value
  const span = barSpan(node)
  if (!pos || span.start === null || span.end === null) return null
  const left = Math.max(0, pos(span.start))
  const minWidth = timelineW.value ? (MIN_BAR_PX / timelineW.value) * 100 : 0
  const width = Math.max(Math.min(100, pos(span.end + 1)) - left, minWidth)
  return { left: `${left}%`, width: `${width}%` }
}

/**
 * Label placement: ABOVE the bar. Inside-the-bar labels sat on top of the
 * breach textures (unreadable) and forced accent-on-light text that failed
 * WCAG AA (4.16:1 < 4.5:1); above the bar, the label renders in the body text
 * color on the row background — the row is the label's own clean strip.
 *
 * The label sits in a track spanning the bar (at least LABEL_ROOM_PX wide)
 * and sticks to the left edge of the visible timeline inside it, so a bar
 * that starts off-screen still shows its name. A bar too close to the
 * timeline's end for the name to fit gets a LABEL_ROOM_PX track ending at the
 * bar's end, with the label aligned right.
 */
function labelTrack(node: GanttNode): { style: Record<string, string>; end: boolean } | null {
  const pos = scale.value
  const span = barSpan(node)
  if (!pos || span.start === null || span.end === null) return null
  const startPct = Math.max(0, pos(span.start))
  const endPct = Math.min(100, pos(span.end + 1))
  if (((100 - startPct) / 100) * timelineW.value >= LABEL_ROOM_PX) {
    return {
      style: { left: `${startPct}%`, width: `max(${endPct - startPct}%, ${LABEL_ROOM_PX}px)` },
      end: false,
    }
  }
  return { style: { right: `${100 - endPct}%`, width: `${LABEL_ROOM_PX}px` }, end: true }
}

/** The planned window inset, relative to the BAR (not the axis). */
function plannedStyle(node: GanttNode) {
  const pos = scale.value
  const span = barSpan(node)
  const ps = parseDay(node.planned?.start)
  const pe = parseDay(node.planned?.end)
  if (!pos || span.start === null || span.end === null || ps === null || pe === null) return null
  if (!node.breach?.before && !node.breach?.after) return null
  const barLeft = pos(span.start)
  const barW = Math.max(pos(span.end + 1) - barLeft, 0.001)
  return {
    left: `${((pos(ps) - barLeft) / barW) * 100}%`,
    width: `${((pos(pe + 1) - pos(ps)) / barW) * 100}%`,
  }
}

/** Overrun (dotted amber) regions inside the bar, one per breach direction. */
function overrunStyles(node: GanttNode) {
  const pos = scale.value
  const span = barSpan(node)
  const ps = parseDay(node.planned?.start)
  const pe = parseDay(node.planned?.end)
  if (!pos || span.start === null || span.end === null) return []
  const barLeft = pos(span.start)
  const barW = Math.max(pos(span.end + 1) - barLeft, 0.001)
  const rel = (d: number) => ((pos(d) - barLeft) / barW) * 100
  const out: { cls: string; style: Record<string, string> }[] = []
  if (node.breach?.before && ps !== null) {
    out.push({
      cls: 'overrun left',
      style: { left: '0%', width: `${rel(ps)}%` },
    })
  }
  if (node.breach?.after && pe !== null) {
    out.push({
      cls: 'overrun right',
      style: { left: `${rel(pe + 1)}%`, width: `${100 - rel(pe + 1)}%` },
    })
  }
  return out
}

/** Committed marker + past-commit rule, axis-relative. The marker stands at
 * the end of the committed day, where the work was due. */
function committedStyle(node: GanttNode) {
  const pos = scale.value
  const c = parseDay(node.committed)
  if (!pos || c === null) return null
  return { left: `${pos(c + 1)}%` }
}

function pastCommitStyle(node: GanttNode) {
  const pos = scale.value
  const c = parseDay(node.committed)
  const span = barSpan(node)
  if (!pos || c === null || span.end === null || span.end <= c) return null
  return {
    left: `${pos(c + 1)}%`,
    width: `${Math.min(100, pos(span.end + 1)) - pos(c + 1)}%`,
  }
}

function pastCommitDays(node: GanttNode): number {
  const c = parseDay(node.committed)
  const span = barSpan(node)
  if (c === null || span.end === null) return 0
  return span.end - c
}

/**
 * Tooltip: one fixed-position card following the hovered bar/label, carrying
 * what the bar geometry can only gesture at — exact dates, the derived-vs-
 * declared distinction, and breach MAGNITUDES in days. Shown on hover and on
 * keyboard focus; dismissed on leave/blur/Escape (WCAG 1.4.13).
 */
const tip = ref<{ node: GanttNode; x: number; y: number } | null>(null)

function showTip(node: GanttNode, ev: MouseEvent | FocusEvent) {
  const x = 'clientX' in ev ? ev.clientX : (ev.target as HTMLElement).getBoundingClientRect().left
  const y = 'clientY' in ev ? ev.clientY : (ev.target as HTMLElement).getBoundingClientRect().top
  tip.value = { node, x: Math.min(x, window.innerWidth - 320), y }
}
function moveTip(ev: MouseEvent) {
  if (tip.value)
    tip.value = { ...tip.value, x: Math.min(ev.clientX, window.innerWidth - 320), y: ev.clientY }
}
function hideTip() {
  tip.value = null
}

/** The configured tooltip rows that have a value on this node. Labels via the
 * shared kanban-card resolution; enum values through the schema's label map. */
function tooltipRows(node: GanttNode): { label: string; value: string }[] {
  const fields: KanbanCardField[] = config.value?.tooltip?.fields ?? []
  const out: { label: string; value: string }[] = []
  for (const f of fields) {
    if (!f.property) continue
    const raw = node.props?.[f.property]
    if (raw === undefined) continue
    out.push({
      label: cardFieldLabel(f),
      value: schemaStore.getEnumLabel(raw, f.property, node.type) || raw,
    })
  }
  return out
}

function fmtRange(span?: { start?: string; end?: string }): string {
  if (!span || (!span.start && !span.end)) return '—'
  return `${span.start || '…'} → ${span.end || '…'}`
}

function countDescendants(node: GanttNode): number {
  return (node.children ?? []).reduce((n, c) => n + 1 + countDescendants(c), 0)
}

/** Breach lines with day magnitudes — the actionable part of a breach. */
function breachLines(node: GanttNode): { text: string; kind: 'overrun' | 'commit' }[] {
  const out: { text: string; kind: 'overrun' | 'commit' }[] = []
  const ps = parseDay(node.planned?.start)
  const pe = parseDay(node.planned?.end)
  const rs = parseDay(node.rolled?.start)
  const re = parseDay(node.rolled?.end)
  if (node.breach?.before && ps !== null && rs !== null) {
    out.push({ text: `children start ${ps - rs}d before the planned start`, kind: 'overrun' })
  }
  if (node.breach?.after && pe !== null && re !== null) {
    out.push({ text: `children end ${re - pe}d after the planned end`, kind: 'overrun' })
  }
  const c = parseDay(node.committed)
  const span = barSpan(node)
  if (c !== null && span.end !== null && span.end > c) {
    out.push({
      text: `${span.end - c}d past the committed date (${node.committed})`,
      kind: 'commit',
    })
  }
  return out
}

const headerHtml = computed(() => (config.value?.header ? renderMarkdown(config.value.header) : ''))
const footerHtml = computed(() => (config.value?.footer ? renderMarkdown(config.value.footer) : ''))
</script>

<template>
  <div class="gantt-view">
    <div class="gantt-head">
      <h1>{{ config?.title || id }}</h1>
      <div class="gantt-nav">
        <RlIconButton icon="chevron-left" label="Earlier" :disabled="!axis" @click="step(-1)" />
        <span v-if="axis && nowDay === null" class="now-hint">Today is outside this plan</span>
        <RlButton variant="secondary" :disabled="nowDay === null" @click="goNow">Now</RlButton>
        <RlIconButton icon="chevron-right" label="Later" :disabled="!axis" @click="step(1)" />
      </div>
      <div class="zoom-seg" role="group" aria-label="Time zoom">
        <button
          v-for="z in ['quarter', 'month', 'week'] as GanttZoom[]"
          :key="z"
          :class="{ on: zoom === z }"
          @click="zoom = z"
        >
          {{ z }}
        </button>
      </div>
    </div>

    <!-- eslint-disable-next-line vue/no-v-html -- admin-authored, sanitized in renderMarkdown -->
    <div v-if="headerHtml" class="gantt-info" v-html="headerHtml" />

    <div v-if="error" class="gantt-error">{{ error }}</div>

    <div v-else class="gantt-panel">
      <div class="crumb-bar">
        <button
          v-if="!root"
          class="crumb"
          :class="{ current: crumbs.length === 0 }"
          @click="drillTo(-1)"
        >
          All work
        </button>
        <button
          v-for="(c, i) in crumbs"
          :key="c.id"
          class="crumb"
          :class="{ current: i === crumbs.length - 1 }"
          @click="drillTo(i)"
        >
          {{ c.title }}
        </button>
        <span
          v-if="data?.truncated"
          class="truncated-flag"
          title="The tree was cut at the node cap; drill in to see more"
        >
          truncated
        </span>
        <span
          v-if="compressed"
          class="truncated-flag"
          title="The plan spans too long to draw at this zoom; days are drawn narrower"
        >
          compressed
        </span>
      </div>

      <div
        v-if="axis"
        ref="chartEl"
        class="chart"
        :style="{
          '--timeline-w': timelineW + 'px',
          '--tree-w': TREE_W + 'px',
          '--chart-max-h': chartMaxH ? chartMaxH + 'px' : 'none',
        }"
      >
        <div class="axis-row">
          <div class="tree-gutter" />
          <div class="axis">
            <span
              v-for="t in ticks"
              :key="t.day"
              class="tick"
              :style="{ left: scale!(t.day) + '%' }"
            >
              {{ t.label }}
            </span>
            <span
              v-if="todayDay !== null && todayDay >= axis.start && todayDay <= axis.end"
              class="today-flag"
              :style="{ left: scale!(todayDay) + '%' }"
            />
          </div>
        </div>

        <div
          v-for="row in rows"
          :key="row.node.id + ':' + row.indent"
          class="row"
          :data-node-id="row.node.id"
        >
          <div class="cell-tree" :style="{ paddingLeft: 8 + row.indent * 14 + 'px' }">
            <button
              class="twisty"
              :class="{ leaf: !row.node.children?.length }"
              :aria-expanded="isRowExpanded(row, defaultDepth, expanded)"
              @click="toggleExpand(row.node)"
            >
              {{ isRowExpanded(row, defaultDepth, expanded) ? '▾' : '▸' }}
            </button>
            <button class="tname" :title="row.node.id" @click="openEntity(row.node)">
              {{ row.node.title || row.node.id }}
            </button>
            <span class="kind">{{ row.node.type }}</span>
            <span
              v-if="row.node.in_cycle"
              class="cycle-flag"
              role="img"
              :aria-label="cycleLabel(row.node)"
              :title="cycleLabel(row.node)"
              >&#8635;</span
            >
          </div>

          <div class="cell-bars" :style="gridStyle">
            <template v-if="barStyle(row.node)">
              <div
                class="bar"
                :class="[
                  row.node.children?.length ? 'parent' : 'leaf',
                  { breached: row.node.breach?.before || row.node.breach?.after },
                ]"
                :style="barStyle(row.node)!"
                @click="drill(row.node)"
                @mouseenter="showTip(row.node, $event)"
                @mousemove="moveTip"
                @mouseleave="hideTip"
              >
                <div
                  v-if="plannedStyle(row.node)"
                  class="planned"
                  :style="plannedStyle(row.node)!"
                />
                <div
                  v-for="(o, i) in overrunStyles(row.node)"
                  :key="i"
                  :class="o.cls"
                  :style="o.style"
                />
              </div>
              <div
                v-if="labelTrack(row.node)"
                class="bar-label-track"
                :class="{ end: labelTrack(row.node)!.end }"
                :style="labelTrack(row.node)!.style"
              >
                <button
                  class="bar-name"
                  @click="drill(row.node)"
                  @mouseenter="showTip(row.node, $event)"
                  @mousemove="moveTip"
                  @mouseleave="hideTip"
                  @focus="showTip(row.node, $event)"
                  @blur="hideTip"
                  @keydown.escape="hideTip"
                >
                  {{ row.node.title || row.node.id }}
                  <span v-if="row.node.children?.length" class="bar-count">{{
                    row.node.children!.length
                  }}</span>
                </button>
              </div>
              <div
                v-if="committedStyle(row.node)"
                class="commit-marker"
                :style="committedStyle(row.node)!"
                :title="'Committed ' + row.node.committed"
              >
                <component :is="ICONS.flag" class="commit-flag" />
                <span class="commit-line" />
              </div>
              <div
                v-if="pastCommitStyle(row.node)"
                class="past-commit"
                :style="pastCommitStyle(row.node)!"
                :title="pastCommitDays(row.node) + 'd past the committed date'"
              />
            </template>
            <span
              v-if="todayDay !== null && todayDay >= axis.start && todayDay <= axis.end"
              class="today-line"
              :style="{ left: scale!(todayDay) + '%' }"
            />
          </div>
        </div>

        <div v-if="rows.length === 0" class="empty">Nothing to show.</div>
      </div>
      <div v-else class="empty">No dated entities yet.</div>

      <div class="legend">
        <span><i class="sw leaf-sw" /> work item</span>
        <span><i class="sw parent-sw" /> project window</span>
        <span><i class="sw planned-sw" /> planned window</span>
        <span><i class="sw overrun-sw" /> ● outside planned window</span>
        <span><i class="sw commit-sw" /> ╱ past committed date</span>
      </div>
    </div>

    <!-- eslint-disable-next-line vue/no-v-html -- admin-authored, sanitized in renderMarkdown -->
    <div v-if="footerHtml" class="gantt-info" v-html="footerHtml" />

    <Teleport to="body">
      <div
        v-if="tip"
        class="gantt-tip"
        role="tooltip"
        :style="{ left: tip.x + 14 + 'px', top: tip.y + 16 + 'px' }"
      >
        <div class="tip-head">
          <strong>{{ tip.node.title || tip.node.id }}</strong>
          <span class="tip-kind">{{ tip.node.type }}</span>
        </div>
        <dl class="tip-grid">
          <template v-if="tip.node.planned">
            <dt>Planned</dt>
            <dd>{{ fmtRange(tip.node.planned) }}</dd>
          </template>
          <template v-if="tip.node.rolled">
            <dt>Rolled up</dt>
            <dd>
              {{ fmtRange(tip.node.rolled) }}
              <span class="tip-muted">from {{ countDescendants(tip.node) }} items</span>
            </dd>
          </template>
          <template v-if="!tip.node.planned && tip.node.rolled">
            <dt />
            <dd class="tip-muted">no dates of its own — span derived from children</dd>
          </template>
          <template v-if="tip.node.committed">
            <dt>Committed</dt>
            <dd>{{ tip.node.committed }}</dd>
          </template>
          <template v-for="row in tooltipRows(tip.node)" :key="row.label">
            <dt>{{ row.label }}</dt>
            <dd>{{ row.value }}</dd>
          </template>
        </dl>
        <div
          v-for="(b, i) in breachLines(tip.node)"
          :key="i"
          class="tip-breach"
          :class="'tip-' + b.kind"
        >
          <span class="tip-glyph">{{ b.kind === 'commit' ? '╱' : '●' }}</span> {{ b.text }}
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.gantt-view {
  padding: 1rem 1.25rem;
}
.gantt-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.75rem;
}
.gantt-head h1 {
  font-size: 1.25rem;
  margin: 0;
  margin-right: auto;
}
.gantt-nav {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  margin-right: 0.75rem;
}
.zoom-seg {
  display: inline-flex;
  border: 1px solid var(--rl-color-border);
  border-radius: 6px;
  overflow: hidden;
}
.zoom-seg button {
  border: 0;
  background: var(--rl-color-bg-raised);
  padding: 0.35rem 0.7rem;
  font-size: 0.8125rem;
  cursor: pointer;
  color: var(--rl-color-text);
  text-transform: capitalize;
}
.zoom-seg button.on {
  background: var(--rl-color-accent);
  color: #fff;
  font-weight: 600;
}
.gantt-info {
  margin-bottom: 0.75rem;
  color: var(--rl-color-text-muted);
  font-size: 0.9rem;
}
/* The loop marker sits with the type label, not on the bar itself: the bar
   is positioned by date and a node in a loop may have no dates at all. */
.cycle-flag {
  margin-left: 4px;
  font-size: 12px;
  line-height: 1;
  color: var(--text-muted, #888);
  cursor: help;
}

.gantt-error {
  color: var(--rl-color-danger);
  padding: 1rem;
}
.gantt-panel {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 8px;
  overflow: hidden;
}
.crumb-bar {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
  padding: 0.6rem 0.9rem;
  border-bottom: 1px solid var(--rl-color-border);
}
.crumb {
  border: 1px solid var(--rl-color-border);
  background: transparent;
  border-radius: 999px;
  padding: 0.2rem 0.7rem;
  font-size: 0.8125rem;
  cursor: pointer;
  color: var(--rl-color-text);
}
/* Current crumb: accent BORDER + body text, not white-on-accent — the chip
   text is small, and white on the accent blue is 4.16:1 (< AA's 4.5:1). */
.crumb.current {
  border: 2px solid var(--rl-color-accent);
  font-weight: 600;
}
.truncated-flag {
  font-size: 0.75rem;
  color: #92400e; /* 6.3:1 on the amber chip; #b45309 was borderline 4.5 */
  background: #fef3c7;
  border: 1px solid #fde68a;
  border-radius: 3px;
  padding: 0 5px;
  cursor: help;
}
/* The chart scrolls on both axes. The axis row sticks to the top and the
   tree column to the left, so names and dates stay readable while scrolling;
   both need an opaque background to cover what scrolls beneath them. Rows
   are as wide as the tree column plus the timeline (--timeline-w). */
.chart {
  overflow: auto;
  max-height: var(--chart-max-h);
  /* Keeps a focused label from scrolling under the sticky axis or tree. */
  scroll-padding-top: 36px;
  scroll-padding-left: var(--tree-w);
}
.now-hint {
  font-size: 0.75rem;
  color: var(--rl-color-text-muted);
  margin-right: 0.25rem;
}
.axis-row {
  display: flex;
  border-bottom: 1px solid var(--rl-color-border);
  height: 36px;
  width: max-content;
  min-width: 100%;
  position: sticky;
  top: 0;
  z-index: 4;
  background: var(--rl-color-bg-raised);
}
.tree-gutter {
  width: var(--tree-w);
  min-width: var(--tree-w);
  border-right: 1px solid var(--rl-color-border);
  position: sticky;
  left: 0;
  z-index: 1;
  background: var(--rl-color-bg-raised);
}
.axis {
  flex: 0 0 var(--timeline-w);
  position: relative;
}
.tick {
  position: absolute;
  bottom: 4px;
  font-size: 0.75rem;
  color: var(--rl-color-text-muted);
  padding-left: 4px;
  border-left: 1px solid var(--rl-color-border);
}
.today-flag {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--rl-color-danger);
}
.today-flag::after {
  content: 'today';
  position: absolute;
  top: 0;
  left: 4px;
  font-size: 0.6875rem;
  color: var(--rl-color-danger);
  font-weight: 700;
}
/* Two-tier row: a full-contrast label strip on top, the bar beneath it.
   The strip is the row's own background, so the name can never collide with
   the bar fill or the breach textures. */
.row {
  display: flex;
  height: 58px;
  border-bottom: 1px solid var(--rl-color-border);
  width: max-content;
  min-width: 100%;
}
.row:hover,
.row:hover .cell-tree {
  background: var(--rl-color-bg-hover);
}
.cell-tree {
  width: var(--tree-w);
  min-width: var(--tree-w);
  display: flex;
  align-items: center;
  gap: 0.3rem;
  border-right: 1px solid var(--rl-color-border);
  overflow: hidden;
  white-space: nowrap;
  position: sticky;
  left: 0;
  z-index: 3;
  background: var(--rl-color-bg-raised);
}
.twisty {
  border: 0;
  background: transparent;
  width: 20px;
  cursor: pointer;
  color: var(--rl-color-text);
  font-size: 0.8125rem;
  padding: 0;
}
.twisty.leaf {
  visibility: hidden;
}
.tname {
  border: 0;
  background: transparent;
  font-size: 0.875rem;
  color: var(--rl-color-text);
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1 1 auto;
  min-width: 4rem;
  text-align: left;
  padding: 0;
}
.tname:hover {
  color: var(--rl-color-accent);
  text-decoration: underline;
}
.kind {
  font-size: 0.6875rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--rl-color-text-muted);
  border: 1px solid var(--rl-color-border);
  border-radius: 3px;
  padding: 0 4px;
  margin-right: 0.5rem;
  flex: 0 0 auto;
}
/* clip, not hidden: hidden would make each row its own scroll container and
   the sticky bar labels would stick to it instead of to the chart. */
.cell-bars {
  flex: 0 0 var(--timeline-w);
  position: relative;
  overflow: clip;
}
/* The label strip: body text color on the row background (≥12:1 in both
   themes), anchored above the bar's start. */
/* The label's track spans its bar; the label sticks just right of the
   sticky tree column while the bar scrolls under it. It paints above the
   commit flag and today line (they may slide over it) and below the tree. */
.bar-label-track {
  position: absolute;
  z-index: 2;
  top: 4px;
  display: flex;
  pointer-events: none;
}
.bar-label-track.end {
  justify-content: flex-end;
}
.bar-name {
  position: sticky;
  left: calc(var(--tree-w) + 8px);
  pointer-events: auto;
  border: 0;
  background: transparent;
  padding: 0;
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--rl-color-text);
  cursor: pointer;
  white-space: nowrap;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
}
.bar-name:hover {
  color: var(--rl-color-accent);
  text-decoration: underline;
}
.bar-count {
  font-weight: 400;
  color: var(--rl-color-text-muted);
}
.bar {
  position: absolute;
  border-radius: 5px;
  cursor: pointer;
  overflow: hidden;
}
.bar.leaf {
  top: 30px;
  height: 18px;
  background: var(--rl-color-accent);
}
/* A parent is ONE slim bar: its own planned window as the body, the
   children's spill before/after it as the dotted regions. Child detail
   lives one drill away, not in a second tier. */
.bar.parent {
  top: 30px;
  height: 18px;
  background: color-mix(in srgb, var(--rl-color-accent) 10%, transparent);
  /* Solid accent border: a meaningful boundary needs ≥3:1 (WCAG 1.4.11);
     the earlier 50%-alpha border was ~1.6:1 against the card. */
  border: 1px solid var(--rl-color-accent);
}
.bar.breached {
  border-color: #b45309; /* 4.6:1 vs white, 3.4:1 vs the dark bg */
}
.planned {
  position: absolute;
  top: 0;
  bottom: 0;
  background: color-mix(in srgb, var(--rl-color-accent) 14%, transparent);
  border-left: 2px solid var(--rl-color-accent);
  border-right: 2px solid var(--rl-color-accent);
  pointer-events: none;
}
/* Overrun: amber DOTS. Texturally distinct from the past-commit stripes so
   the two warning states survive colour-blindness and greyscale. */
.overrun {
  position: absolute;
  top: 0;
  bottom: 0;
  pointer-events: none;
  background-image: radial-gradient(rgba(180, 83, 9, 0.9) 1.3px, transparent 1.4px);
  background-size: 5px 5px;
  background-color: rgba(217, 119, 6, 0.14);
}
/* Committed-date marker: a flag at label height with its pole dropping
   through the bar — a bare line was too easy to read as a gridline. */
.commit-marker {
  position: absolute;
  top: 9px;
  bottom: 4px;
  width: 0;
  z-index: 1;
}
.commit-flag {
  position: absolute;
  top: 0;
  /* Offset so the icon's own pole (x=4 in the 24-unit viewBox) sits exactly
     over the dropline — one continuous staff, not two misaligned strokes. */
  left: -1.75px;
  width: 15px;
  height: 15px;
  color: var(--rl-color-danger);
  stroke-width: 2.4; /* renders ~1.5px at this size, matching the line */
}
.commit-line {
  position: absolute;
  top: 12px;
  bottom: 0;
  left: 0;
  width: 1.5px;
  background: var(--rl-color-danger);
}
/* Past-commit: red diagonal STRIPES on their own tier under the bar. */
.past-commit {
  position: absolute;
  top: 51px;
  height: 4px;
  border-radius: 2px;
  background: repeating-linear-gradient(45deg, #dc2626 0 2px, rgba(220, 38, 38, 0.18) 2px 6px);
}
.today-line {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: var(--rl-color-danger);
  opacity: 0.7;
  pointer-events: none;
}
.empty {
  padding: 2rem;
  text-align: center;
  color: var(--rl-color-text-muted);
  font-size: 0.875rem;
}
.legend {
  display: flex;
  gap: 1.1rem;
  flex-wrap: wrap;
  padding: 0.6rem 0.9rem;
  border-top: 1px solid var(--rl-color-border);
  font-size: 0.8125rem;
  color: var(--rl-color-text);
}
.legend span {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}
.sw {
  width: 20px;
  height: 10px;
  border-radius: 3px;
  display: inline-block;
}
.leaf-sw {
  background: var(--rl-color-accent);
}
.parent-sw {
  background: color-mix(in srgb, var(--rl-color-accent) 8%, transparent);
  border: 1px solid var(--rl-color-accent);
}
.planned-sw {
  background: color-mix(in srgb, var(--rl-color-accent) 14%, transparent);
  border-left: 2px solid var(--rl-color-accent);
  border-right: 2px solid var(--rl-color-accent);
}
.overrun-sw {
  background-image: radial-gradient(rgba(180, 83, 9, 0.9) 1.3px, transparent 1.4px);
  background-size: 5px 5px;
  background-color: rgba(217, 119, 6, 0.14);
}
.commit-sw {
  background: repeating-linear-gradient(45deg, #dc2626 0 2px, rgba(220, 38, 38, 0.18) 2px 6px);
}

/* Tooltip card — fixed, above everything, theme-token colored. Text sizes
   and colors hold WCAG AA on the card background in both themes. */
.gantt-tip {
  position: fixed;
  z-index: 1000;
  max-width: 300px;
  background: var(--rl-color-bg-raised);
  color: var(--rl-color-text);
  border: 1px solid var(--rl-color-border);
  border-radius: 8px;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.18);
  padding: 0.6rem 0.75rem;
  font-size: 0.8125rem;
  pointer-events: none;
}
.tip-head {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 0.35rem;
}
.tip-head strong {
  font-size: 0.875rem;
}
.tip-kind {
  font-size: 0.6875rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--rl-color-text-muted);
}
.tip-grid {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.1rem 0.6rem;
  margin: 0;
}
.tip-grid dt {
  color: var(--rl-color-text-muted);
}
.tip-grid dd {
  margin: 0;
  font-variant-numeric: tabular-nums;
}
.tip-muted {
  color: var(--rl-color-text-muted);
}
.tip-breach {
  margin-top: 0.35rem;
  font-weight: 500;
}
/* amber-800 / red-700: ≥6:1 on the light card, and both hold ≥4.5 on the
   dark card via the shared glyphs carrying the distinction anyway. */
.tip-breach.tip-overrun {
  color: #92400e;
}
.tip-breach.tip-commit {
  color: #b91c1c;
}
:root.dark .tip-breach.tip-overrun {
  color: #fbbf24;
}
:root.dark .tip-breach.tip-commit {
  color: #f87171;
}
.tip-glyph {
  font-weight: 700;
}
</style>
