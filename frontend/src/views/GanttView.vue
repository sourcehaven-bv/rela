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
    const canAnswerLocally =
      !idChanged &&
      data.value !== null &&
      !data.value.truncated &&
      fetchedRoot.value === null &&
      (target === null || findNode(data.value.roots, target) !== null)
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

function drill(node: GanttNode) {
  if (!node.children?.length) {
    router.push(`/entity/${node.type}/${node.id}`)
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

/** Peek segments: children drawn as a recessed strip inside the parent bar. */
function peekStyles(node: GanttNode) {
  const span = barSpan(node)
  if (span.start === null || span.end === null || !node.children?.length) return []
  const total = Math.max(span.end - span.start, 1)
  const out: { deep: boolean; title: string; style: Record<string, string> }[] = []
  for (const child of node.children) {
    const cs = barSpan(child)
    if (cs.start === null || cs.end === null) continue
    const left = Math.max(0, ((cs.start - span.start) / total) * 100)
    out.push({
      deep: Boolean(child.children?.length),
      title: child.title || child.id,
      style: {
        left: `${left}%`,
        width: `${Math.max(1, Math.min(100 - left, ((cs.end - cs.start) / total) * 100))}%`,
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
            <button class="tname" :title="row.node.id" @click="drill(row.node)">
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
              >
                <div v-if="plannedStyle(row.node)" class="planned" :style="plannedStyle(row.node)!" />
                <div
                  v-for="(o, i) in overrunStyles(row.node)"
                  :key="i"
                  :class="o.cls"
                  :style="o.style"
                />
                <div v-if="peekStyles(row.node).length" class="peek-lane">
                  <span
                    v-for="(p, i) in peekStyles(row.node)"
                    :key="i"
                    class="peek"
                    :class="{ deep: p.deep }"
                    :style="p.style"
                    :title="p.title"
                  />
                </div>
                <span class="bar-label">
                  {{ row.node.title || row.node.id }}
                  <span v-if="row.node.children?.length" class="bar-count">{{
                    row.node.children!.length
                  }}</span>
                </span>
              </div>
              <div
                v-if="committedStyle(row.node)"
                class="commit"
                :style="committedStyle(row.node)!"
                :title="'Committed ' + row.node.committed"
              />
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
        <span><i class="sw parent-sw" /> derived envelope</span>
        <span><i class="sw planned-sw" /> planned window</span>
        <span><i class="sw overrun-sw" /> ● outside planned window</span>
        <span><i class="sw commit-sw" /> ╱ past committed date</span>
      </div>
    </div>

    <!-- eslint-disable-next-line vue/no-v-html -- admin-authored, sanitized in renderMarkdown -->
    <div v-if="footerHtml" class="gantt-info" v-html="footerHtml" />
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
  padding: 0.3rem 0.65rem;
  font-size: 0.78rem;
  cursor: pointer;
  color: var(--muted-text);
  text-transform: capitalize;
}
.zoom-seg button.on {
  background: var(--accent-color);
  color: #fff;
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
  gap: 0.35rem;
  flex-wrap: wrap;
  padding: 0.6rem 0.9rem;
  border-bottom: 1px solid var(--border-color);
}
.crumb {
  border: 1px solid var(--border-color);
  background: transparent;
  border-radius: 999px;
  padding: 0.15rem 0.6rem;
  font-size: 0.78rem;
  cursor: pointer;
  color: var(--muted-text);
}
.crumb.current {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: #fff;
}
.truncated-flag {
  font-size: 0.68rem;
  color: #b45309;
  background: #fef3c7;
  border: 1px solid #fde68a;
  border-radius: 3px;
  padding: 0 4px;
  cursor: help;
}
.chart {
  overflow-x: auto;
}
.axis-row {
  display: flex;
  border-bottom: 1px solid var(--border-color);
  height: 34px;
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
  bottom: 3px;
  font-size: 0.68rem;
  color: var(--muted-text);
  padding-left: 4px;
  border-left: 1px solid var(--border-color);
}
.today-flag {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: #dc2626;
}
.today-flag::after {
  content: 'today';
  position: absolute;
  top: 0;
  left: 3px;
  font-size: 0.6rem;
  color: #dc2626;
  font-weight: 600;
}
.row {
  display: flex;
  height: 56px;
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
  width: 18px;
  cursor: pointer;
  color: var(--muted-text);
  font-size: 0.7rem;
  padding: 0;
}
.twisty.leaf {
  visibility: hidden;
}
.tname {
  border: 0;
  background: transparent;
  font-size: 0.82rem;
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
  font-size: 0.58rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--muted-text);
  border: 1px solid var(--border-color);
  border-radius: 3px;
  padding: 0 3px;
  margin-right: 0.5rem;
  flex: 0 0 auto;
}
.cell-bars {
  flex: 1;
  position: relative;
  min-width: 300px;
}
.gridline {
  position: absolute;
  top: 0;
  bottom: 0;
  border-left: 1px solid var(--border-color);
  opacity: 0.5;
}
.bar {
  position: absolute;
  border-radius: 5px;
  cursor: pointer;
  overflow: hidden;
}
.bar.leaf {
  top: 18px;
  height: 20px;
  background: var(--accent-color);
}
.bar.parent {
  top: 10px;
  height: 32px;
  background: color-mix(in srgb, var(--accent-color) 7%, transparent);
  border: 1px solid color-mix(in srgb, var(--accent-color) 50%, transparent);
}
.bar.breached {
  border-color: rgba(217, 119, 6, 0.7);
}
.planned {
  position: absolute;
  top: 0;
  bottom: 0;
  background: color-mix(in srgb, var(--accent-color) 13%, transparent);
  border-left: 2px solid color-mix(in srgb, var(--accent-color) 75%, transparent);
  border-right: 2px solid color-mix(in srgb, var(--accent-color) 75%, transparent);
  pointer-events: none;
}
/* Overrun: amber DOTS. Texturally distinct from the past-commit stripes so
   the two warning states survive colour-blindness and greyscale. */
.overrun {
  position: absolute;
  top: 0;
  bottom: 0;
  pointer-events: none;
  background-image: radial-gradient(rgba(180, 83, 9, 0.85) 1.2px, transparent 1.3px);
  background-size: 5px 5px;
  background-color: rgba(217, 119, 6, 0.14);
}
.peek-lane {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 12px;
  background: color-mix(in srgb, var(--accent-color) 6%, transparent);
  border-top: 1px solid color-mix(in srgb, var(--accent-color) 22%, transparent);
}
.peek {
  position: absolute;
  top: 3px;
  height: 6px;
  background: var(--accent-color);
  border-radius: 2px;
}
.peek.deep {
  opacity: 0.5;
}
.bar-label {
  position: absolute;
  left: 7px;
  right: 7px;
  top: 2px;
  font-size: 0.7rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  pointer-events: none;
  color: var(--accent-color);
  font-weight: 600;
}
.bar.leaf .bar-label {
  color: #fff;
  top: 50%;
  transform: translateY(-50%);
  font-weight: 400;
}
.bar-count {
  font-weight: 400;
  opacity: 0.6;
}
.commit {
  position: absolute;
  top: 8px;
  bottom: 12px;
  width: 2px;
  background: #dc2626;
  z-index: 2;
}
/* Past-commit: red diagonal STRIPES on their own tier under the bar. */
.past-commit {
  position: absolute;
  top: 45px;
  height: 4px;
  border-radius: 2px;
  background: repeating-linear-gradient(45deg, #dc2626 0 2px, rgba(220, 38, 38, 0.18) 2px 6px);
}
.today-line {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: #dc2626;
  opacity: 0.45;
  pointer-events: none;
}
.empty {
  padding: 2rem;
  text-align: center;
  color: var(--muted-text);
  font-size: 0.85rem;
}
.legend {
  display: flex;
  gap: 1.1rem;
  flex-wrap: wrap;
  padding: 0.55rem 0.9rem;
  border-top: 1px solid var(--border-color);
  font-size: 0.72rem;
  color: var(--muted-text);
}
.legend span {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
}
.sw {
  width: 20px;
  height: 9px;
  border-radius: 3px;
  display: inline-block;
}
.leaf-sw {
  background: var(--accent-color);
}
.parent-sw {
  background: color-mix(in srgb, var(--accent-color) 7%, transparent);
  border: 1px solid color-mix(in srgb, var(--accent-color) 50%, transparent);
}
.planned-sw {
  background: color-mix(in srgb, var(--accent-color) 13%, transparent);
  border-left: 2px solid var(--accent-color);
  border-right: 2px solid var(--accent-color);
}
.overrun-sw {
  background-image: radial-gradient(rgba(180, 83, 9, 0.85) 1.2px, transparent 1.3px);
  background-size: 5px 5px;
  background-color: rgba(217, 119, 6, 0.14);
}
.commit-sw {
  background: repeating-linear-gradient(45deg, #dc2626 0 2px, rgba(220, 38, 38, 0.18) 2px 6px);
}
</style>
