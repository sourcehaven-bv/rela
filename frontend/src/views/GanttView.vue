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
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getGantt, getErrorMessage, type GanttNode, type GanttResponse } from '@/api'
import { useSchemaStore } from '@/stores/schema'
import { renderMarkdown } from '@/utils/markdown'
import { cardFieldLabel, type KanbanCardField } from '@/types/config'
import { ICONS } from '@/utils/icons'
import {
  barSpan,
  findNode,
  flattenRows,
  forestSpan,
  isRowExpanded,
  parseDay,
  pct,
  ticksFor,
  type GanttZoom,
} from '@/utils/ganttLayout'

const props = defineProps<{ id: string }>()

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

async function fetchScope(root: string | null) {
  error.value = ''
  try {
    data.value = await getGantt(props.id, root ?? undefined)
    fetchedRoot.value = root
  } catch (e) {
    data.value = null
    error.value = getErrorMessage(e)
  }
}

/** Drill path from the URL (?path=id1,id2); [] means the full forest. */
const drillPath = computed<string[]>(() => {
  const raw = route.query.path
  if (typeof raw !== 'string' || raw === '') return []
  return raw.split(',')
})

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
  [() => props.id, drillPath] as const,
  ([id, path], old) => {
    const idChanged = !old || id !== old[0]
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
  { immediate: true },
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

const axis = computed(() => forestSpan(currentRoots.value))
const ticks = computed(() => (axis.value ? ticksFor(axis.value, zoom.value) : []))

const defaultDepth = computed(() => config.value?.default_depth ?? 2)
const rows = computed(() =>
  axis.value ? flattenRows(currentRoots.value, defaultDepth.value, expanded.value) : [],
)

const todayDay = computed(() => {
  // LOCAL calendar fields on purpose: the user's local "today" is what the
  // marker means, while every stored date is UTC-parsed. Rebuilding this from
  // toISOString() would shift the line a day for anyone west of Greenwich
  // after 00:00 UTC.
  const now = new Date()
  return parseDay(
    `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`,
  )
})

/** openEntity navigates to the node's entity page — the tree-column name's
 * click, and the fallback for chart clicks that cannot drill. */
function openEntity(node: GanttNode) {
  router.push(`/entity/${node.type}/${node.id}`)
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
  router.push({ query: { ...route.query, path: path.join(',') } })
}

function drillTo(index: number) {
  const path = drillPath.value.slice(0, index + 1)
  router.push({ query: { ...route.query, path: path.length ? path.join(',') : undefined } })
}

function toggleExpand(node: GanttNode) {
  const next = new Set(expanded.value)
  if (next.has(node.id)) next.delete(node.id)
  else next.add(node.id)
  expanded.value = next
}

/** Bar geometry for one row, all values 0-100 percentages of the axis. */
function barStyle(node: GanttNode) {
  const a = axis.value
  const span = barSpan(node)
  if (!a || span.start === null || span.end === null) return null
  const left = Math.max(0, pct(span.start, a))
  const width = Math.max(Math.min(100, pct(span.end, a)) - left, 0.6)
  return { left: `${left}%`, width: `${width}%` }
}

/**
 * Label placement: ABOVE the bar, anchored to its start (right-anchored when
 * the bar begins in the last stretch of the axis, so long names stay on
 * screen). Inside-the-bar labels sat on top of the breach textures
 * (unreadable) and forced accent-on-light text that failed WCAG AA
 * (4.16:1 < 4.5:1); above the bar, the label renders in the body text color
 * on the row background — the row is the label's own clean strip.
 */
function labelStyle(node: GanttNode) {
  const a = axis.value
  const span = barSpan(node)
  if (!a || span.start === null || span.end === null) return null
  const startPct = Math.max(0, pct(span.start, a))
  if (startPct <= 60) return { left: `${startPct}%` }
  return { right: `${100 - Math.min(100, pct(span.end, a))}%` }
}

/** The planned window inset, relative to the BAR (not the axis). */
function plannedStyle(node: GanttNode) {
  const span = barSpan(node)
  const ps = parseDay(node.planned?.start)
  const pe = parseDay(node.planned?.end)
  if (span.start === null || span.end === null || ps === null || pe === null) return null
  if (!node.breach?.before && !node.breach?.after) return null
  const total = Math.max(span.end - span.start, 1)
  return {
    left: `${((ps - span.start) / total) * 100}%`,
    width: `${((pe - ps) / total) * 100}%`,
  }
}

/** Overrun (dotted amber) regions inside the bar, one per breach direction. */
function overrunStyles(node: GanttNode) {
  const span = barSpan(node)
  const ps = parseDay(node.planned?.start)
  const pe = parseDay(node.planned?.end)
  if (span.start === null || span.end === null) return []
  const total = Math.max(span.end - span.start, 1)
  const out: { cls: string; style: Record<string, string> }[] = []
  if (node.breach?.before && ps !== null) {
    out.push({
      cls: 'overrun left',
      style: { left: '0%', width: `${((ps - span.start) / total) * 100}%` },
    })
  }
  if (node.breach?.after && pe !== null) {
    out.push({
      cls: 'overrun right',
      style: {
        left: `${((pe - span.start) / total) * 100}%`,
        width: `${((span.end - pe) / total) * 100}%`,
      },
    })
  }
  return out
}

/** Committed marker + past-commit rule, axis-relative. */
function committedStyle(node: GanttNode) {
  const a = axis.value
  const c = parseDay(node.committed)
  if (!a || c === null) return null
  return { left: `${pct(c, a)}%` }
}

function pastCommitStyle(node: GanttNode) {
  const a = axis.value
  const c = parseDay(node.committed)
  const span = barSpan(node)
  if (!a || c === null || span.end === null || span.end <= c) return null
  return {
    left: `${pct(c, a)}%`,
    width: `${Math.min(100, pct(span.end, a)) - pct(c, a)}%`,
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
  if (tip.value) tip.value = { ...tip.value, x: Math.min(ev.clientX, window.innerWidth - 320), y: ev.clientY }
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
    out.push({ text: `${span.end - c}d past the committed date (${node.committed})`, kind: 'commit' })
  }
  return out
}

const headerHtml = computed(() =>
  config.value?.header ? renderMarkdown(config.value.header) : '',
)
const footerHtml = computed(() =>
  config.value?.footer ? renderMarkdown(config.value.footer) : '',
)
</script>

<template>
  <div class="gantt-view">
    <div class="gantt-head">
      <h1>{{ config?.title || id }}</h1>
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
        <button class="crumb" :class="{ current: crumbs.length === 0 }" @click="drillTo(-1)">
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
        <span v-if="data?.truncated" class="truncated-flag" title="The tree was cut at the node cap; drill in to see more">
          truncated
        </span>
      </div>

      <div v-if="axis" class="chart">
        <div class="axis-row">
          <div class="tree-gutter" />
          <div class="axis">
            <span
              v-for="t in ticks"
              :key="t.day"
              class="tick"
              :style="{ left: pct(t.day, axis) + '%' }"
            >
              {{ t.label }}
            </span>
            <span
              v-if="todayDay !== null && todayDay >= axis.start && todayDay <= axis.end"
              class="today-flag"
              :style="{ left: pct(todayDay, axis) + '%' }"
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
          </div>

          <div class="cell-bars">
            <span
              v-for="t in ticks"
              :key="t.day"
              class="gridline"
              :style="{ left: pct(t.day, axis) + '%' }"
            />
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
                <div v-if="plannedStyle(row.node)" class="planned" :style="plannedStyle(row.node)!" />
                <div
                  v-for="(o, i) in overrunStyles(row.node)"
                  :key="i"
                  :class="o.cls"
                  :style="o.style"
                />
              </div>
              <button
                v-if="labelStyle(row.node)"
                class="bar-name"
                :style="labelStyle(row.node)!"
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
              :style="{ left: pct(todayDay, axis) + '%' }"
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
        <div v-for="(b, i) in breachLines(tip.node)" :key="i" class="tip-breach" :class="'tip-' + b.kind">
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
}
.zoom-seg {
  display: inline-flex;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  overflow: hidden;
}
.zoom-seg button {
  border: 0;
  background: var(--card-bg);
  padding: 0.35rem 0.7rem;
  font-size: 0.8125rem;
  cursor: pointer;
  color: var(--text-color);
  text-transform: capitalize;
}
.zoom-seg button.on {
  background: var(--accent-color);
  color: #fff;
  font-weight: 600;
}
.gantt-info {
  margin-bottom: 0.75rem;
  color: var(--muted-text);
  font-size: 0.9rem;
}
.gantt-error {
  color: var(--error-color);
  padding: 1rem;
}
.gantt-panel {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
}
.crumb-bar {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
  padding: 0.6rem 0.9rem;
  border-bottom: 1px solid var(--border-color);
}
.crumb {
  border: 1px solid var(--border-color);
  background: transparent;
  border-radius: 999px;
  padding: 0.2rem 0.7rem;
  font-size: 0.8125rem;
  cursor: pointer;
  color: var(--text-color);
}
/* Current crumb: accent BORDER + body text, not white-on-accent — the chip
   text is small, and white on the accent blue is 4.16:1 (< AA's 4.5:1). */
.crumb.current {
  border: 2px solid var(--accent-color);
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
.chart {
  overflow-x: auto;
}
.axis-row {
  display: flex;
  border-bottom: 1px solid var(--border-color);
  height: 36px;
}
.tree-gutter {
  width: 280px;
  min-width: 280px;
  border-right: 1px solid var(--border-color);
}
.axis {
  flex: 1;
  position: relative;
  min-width: 300px;
}
.tick {
  position: absolute;
  bottom: 4px;
  font-size: 0.75rem;
  color: var(--muted-text);
  padding-left: 4px;
  border-left: 1px solid var(--border-color);
}
.today-flag {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--error-color);
}
.today-flag::after {
  content: 'today';
  position: absolute;
  top: 0;
  left: 4px;
  font-size: 0.6875rem;
  color: var(--error-color);
  font-weight: 700;
}
/* Two-tier row: a full-contrast label strip on top, the bar beneath it.
   The strip is the row's own background, so the name can never collide with
   the bar fill or the breach textures. */
.row {
  display: flex;
  height: 58px;
  border-bottom: 1px solid var(--border-color);
}
.row:hover {
  background: var(--hover-bg);
}
.cell-tree {
  width: 280px;
  min-width: 280px;
  display: flex;
  align-items: center;
  gap: 0.3rem;
  border-right: 1px solid var(--border-color);
  overflow: hidden;
  white-space: nowrap;
}
.twisty {
  border: 0;
  background: transparent;
  width: 20px;
  cursor: pointer;
  color: var(--text-color);
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
  color: var(--text-color);
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1 1 auto;
  min-width: 4rem;
  text-align: left;
  padding: 0;
}
.tname:hover {
  color: var(--accent-color);
  text-decoration: underline;
}
.kind {
  font-size: 0.6875rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--muted-text);
  border: 1px solid var(--border-color);
  border-radius: 3px;
  padding: 0 4px;
  margin-right: 0.5rem;
  flex: 0 0 auto;
}
.cell-bars {
  flex: 1;
  position: relative;
  min-width: 300px;
  overflow: hidden;
}
.gridline {
  position: absolute;
  top: 0;
  bottom: 0;
  border-left: 1px solid var(--border-color);
  opacity: 0.5;
}
/* The label strip: body text color on the row background (≥12:1 in both
   themes), anchored above the bar's start. */
.bar-name {
  position: absolute;
  top: 4px;
  border: 0;
  background: transparent;
  padding: 0;
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--text-color);
  cursor: pointer;
  white-space: nowrap;
  max-width: 60%;
  overflow: hidden;
  text-overflow: ellipsis;
}
.bar-name:hover {
  color: var(--accent-color);
  text-decoration: underline;
}
.bar-count {
  font-weight: 400;
  color: var(--muted-text);
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
  background: var(--accent-color);
}
/* A parent is ONE slim bar: its own planned window as the body, the
   children's spill before/after it as the dotted regions. Child detail
   lives one drill away, not in a second tier. */
.bar.parent {
  top: 30px;
  height: 18px;
  background: color-mix(in srgb, var(--accent-color) 10%, transparent);
  /* Solid accent border: a meaningful boundary needs ≥3:1 (WCAG 1.4.11);
     the earlier 50%-alpha border was ~1.6:1 against the card. */
  border: 1px solid var(--accent-color);
}
.bar.breached {
  border-color: #b45309; /* 4.6:1 vs white, 3.4:1 vs the dark bg */
}
.planned {
  position: absolute;
  top: 0;
  bottom: 0;
  background: color-mix(in srgb, var(--accent-color) 14%, transparent);
  border-left: 2px solid var(--accent-color);
  border-right: 2px solid var(--accent-color);
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
  top: 2px;
  bottom: 4px;
  width: 0;
  z-index: 2;
}
.commit-flag {
  position: absolute;
  top: 0;
  left: -2px;
  width: 15px;
  height: 15px;
  color: var(--error-color);
}
.commit-line {
  position: absolute;
  top: 13px;
  bottom: 0;
  left: 0;
  width: 2px;
  background: var(--error-color);
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
  background: var(--error-color);
  opacity: 0.7;
  pointer-events: none;
}
.empty {
  padding: 2rem;
  text-align: center;
  color: var(--muted-text);
  font-size: 0.875rem;
}
.legend {
  display: flex;
  gap: 1.1rem;
  flex-wrap: wrap;
  padding: 0.6rem 0.9rem;
  border-top: 1px solid var(--border-color);
  font-size: 0.8125rem;
  color: var(--text-color);
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
  background: var(--accent-color);
}
.parent-sw {
  background: color-mix(in srgb, var(--accent-color) 8%, transparent);
  border: 1px solid var(--accent-color);
}
.planned-sw {
  background: color-mix(in srgb, var(--accent-color) 14%, transparent);
  border-left: 2px solid var(--accent-color);
  border-right: 2px solid var(--accent-color);
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
  background: var(--card-bg);
  color: var(--text-color);
  border: 1px solid var(--border-color);
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
  color: var(--muted-text);
}
.tip-grid {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.1rem 0.6rem;
  margin: 0;
}
.tip-grid dt {
  color: var(--muted-text);
}
.tip-grid dd {
  margin: 0;
  font-variant-numeric: tabular-nums;
}
.tip-muted {
  color: var(--muted-text);
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
