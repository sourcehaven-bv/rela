<script setup lang="ts">
/**
 * Flow diagram over the server's gated nodes and links.
 *
 * The server does the security-sensitive work (row-gating, redaction, caps; see
 * internal/dataentry/sankey_handler.go). This view fetches the nodes, lays them
 * out with the pure functions in utils/sankeyLayout.ts and renders the result
 * with v-for. Hovering or focusing a node highlights every path through it.
 */
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getSankey, getErrorMessage, type SankeyPip, type SankeyResponse } from '@/api'
import EntityDetailPanel from '@/components/entity/EntityDetailPanel.vue'
import { useDetailPanel } from '@/composables/useDetailPanel'
import { usePageTitle } from '@/composables/useDocumentTitle'
import { useSchemaStore } from '@/stores/schema'
import { fromPageQuery } from '@/utils/pageContext'
import { renderMarkdown } from '@/utils/markdown'
import { layoutSankey } from '@/utils/sankeyLayout'
import type { CalendarColor } from '@/types/config'

const props = defineProps<{ id: string }>()

const route = useRoute()
const router = useRouter()
const schemaStore = useSchemaStore()
const { t } = useI18n()

const config = computed(() => schemaStore.getSankey(props.id))
usePageTitle(() => config.value?.title || props.id)

const data = ref<SankeyResponse | null>(null)
const error = ref('')
let fetchSeq = 0

async function load() {
  const seq = ++fetchSeq
  try {
    const res = await getSankey(props.id)
    if (seq !== fetchSeq) return // the user already moved to another diagram
    data.value = res
    error.value = ''
  } catch (e) {
    if (seq !== fetchSeq) return
    data.value = null
    error.value = getErrorMessage(e)
  }
}
watch(
  () => props.id,
  () => {
    // Another diagram has other columns: drop the old nodes rather than lay
    // them out against the new configuration while the fetch is on its way.
    data.value = null
    error.value = ''
    void load()
  },
  { immediate: true }
)

/* The diagram does not scale: text keeps its size, and a narrow window scrolls
   sideways instead. The width follows the container between 760 and 1600. */
const mainEl = ref<HTMLElement | null>(null)
const containerW = ref(1200)
let observer: ResizeObserver | undefined
// mainEl sits on a v-else branch, so it can appear, go and come back as the
// load succeeds or fails; follow the element, not the mount.
watch(
  mainEl,
  (el) => {
    observer?.disconnect()
    observer = undefined
    if (!el) return
    containerW.value = el.clientWidth
    observer = new ResizeObserver(() => {
      if (mainEl.value) containerW.value = mainEl.value.clientWidth
    })
    observer.observe(el)
  },
  { flush: 'post' }
)
onBeforeUnmount(() => observer?.disconnect())

const CHAR_PX = 5.8
const ROW_PX = 17
const TYPE_GAP_PX = 28 // the layout's gap between stacked types in one column
const PIP_STEP = 11
const PIP_R = 4

const width = computed(() => Math.min(1600, Math.max(760, containerW.value - 24)))
const side = computed(() => Math.round(Math.min(420, Math.max(140, width.value * 0.19))))
const cutSide = computed(() => Math.max(8, Math.floor((side.value - 24) / CHAR_PX)))

const layout = computed(() => {
  const cfg = config.value
  if (!cfg || !data.value) return null
  // Tall enough for the busiest column's rows and gaps. There is no ceiling:
  // a ceiling squeezed a long column until its first nodes fell off the top.
  const need = cfg.columns.map(
    (col) =>
      data.value!.nodes.filter((n) => col.includes(n.type)).length * ROW_PX +
      col.length * TYPE_GAP_PX
  )
  const height = Math.max(360, Math.max(0, ...need) + 80)
  return layoutSankey(
    data.value.nodes.map((n) => ({ id: n.id, type: n.type, sortKey: n.sort_key })),
    data.value.links.map((l) => ({ source: l.source, target: l.target })),
    {
      columns: cfg.columns,
      fixedTypes: Object.entries(cfg.types ?? {})
        .filter(([, tc]) => tc.order)
        .map(([name]) => name),
      width: width.value,
      height,
      marginLeft: side.value,
      marginRight: side.value,
    }
  )
})

const svgHeight = computed(() =>
  layout.value ? Math.max(...layout.value.nodes.map((n) => n.y1), 0) + 20 : 0
)

const nodeInfo = computed(() => new Map((data.value?.nodes ?? []).map((n) => [n.id, n])))
/** Every pip by entity id, so a dot can be selected like a node. */
const pipInfo = computed(() => {
  const out = new Map<string, SankeyPip>()
  for (const n of data.value?.nodes ?? []) for (const p of n.pips ?? []) out.set(p.id, p)
  return out
})

function typeLabel(type: string): string {
  const def = schemaStore.getEntityType(type)
  return def?.label_plural || def?.plural || def?.label || type
}

function cut(s: string, n: number): string {
  return s.length > n ? s.slice(0, n - 1).trimEnd() + '…' : s
}
function nodeName(id: string): string {
  const n = nodeInfo.value.get(id)
  return n?.title || id
}
/** The width, in px, of a node's dots and its "+n", with a gap after them. */
function pipsWidth(id: string): number {
  const n = nodeInfo.value.get(id)
  const slots = (n?.pips?.length ?? 0) + (n?.pips_more ? 2 : 0)
  return slots ? slots * PIP_STEP + 4 : 0
}
function nodeText(id: string): string {
  const room = Math.floor((side.value - 24 - pipsWidth(id)) / CHAR_PX)
  return cut(nodeName(id), Math.max(6, Math.min(cutSide.value, room)))
}
type Placed = { col: number; x0: number; x1: number; id: string }
/** The dots sit against the node, so the label starts after them. */
function labelX(n: Placed): number {
  return n.col === 0 ? n.x0 - 6 - pipsWidth(n.id) : n.x1 + 6 + pipsWidth(n.id)
}
function pipX(n: Placed, i: number): number {
  return n.col === 0 ? n.x0 - 6 - PIP_R - i * PIP_STEP : n.x1 + 6 + PIP_R + i * PIP_STEP
}

/* Colours are palette tokens mapped onto the theme's own variables, so the
   diagram follows light and dark mode like the rest of the app. */
const TOKEN_VAR: Record<CalendarColor, string> = {
  blue: 'var(--rl-color-status-blue)',
  green: 'var(--rl-color-status-green)',
  amber: 'var(--rl-color-status-amber)',
  red: 'var(--rl-color-status-red)',
  violet: 'var(--rl-color-accent)',
  slate: 'var(--rl-color-status-grey)',
}
const CYCLE: CalendarColor[] = ['blue', 'green', 'amber', 'red', 'violet', 'slate']

const typeColor = computed(() => {
  const out = new Map<string, string>()
  const cfg = config.value
  if (!cfg) return out
  cfg.columns.flat().forEach((type, i) => {
    out.set(type, TOKEN_VAR[cfg.types?.[type]?.color ?? CYCLE[(i + 4) % CYCLE.length]])
  })
  return out
})
/** Equal tone values get equal colours: sorted, then dealt out around the cycle. */
const toneColor = computed(() => {
  const tones = [
    ...new Set((data.value?.nodes ?? []).map((n) => n.tone).filter((v): v is string => !!v)),
  ].sort()
  return new Map(tones.map((v, i) => [v, TOKEN_VAR[CYCLE[i % CYCLE.length]]]))
})
/** Pip tones are dealt colours like node tones, in a set of their own. */
const pipToneColor = computed(() => {
  const tones = [...pipInfo.value.values()].map((p) => p.tone).filter((v): v is string => !!v)
  return new Map([...new Set(tones)].sort().map((v, i) => [v, TOKEN_VAR[CYCLE[i % CYCLE.length]]]))
})
function pipFill(p: SankeyPip): string {
  return (p.tone && pipToneColor.value.get(p.tone)) || 'var(--rl-color-status-grey)'
}

function fillOf(id: string): string {
  const n = nodeInfo.value.get(id)
  if (!n) return 'var(--rl-color-status-grey)'
  return (
    (n.tone && toneColor.value.get(n.tone)) ||
    typeColor.value.get(n.type) ||
    'var(--rl-color-status-grey)'
  )
}

/* Hover and focus: every node on a path through the active one is on. */
const active = ref<string | null>(null)
const adjacency = computed(() => {
  const out = new Map<string, string[]>()
  const inn = new Map<string, string[]>()
  const push = (m: Map<string, string[]>, k: string, v: string) => {
    const list = m.get(k)
    if (list) list.push(v)
    else m.set(k, [v])
  }
  for (const l of layout.value?.links ?? []) {
    push(out, l.source, l.target)
    push(inn, l.target, l.source)
  }
  return { out, inn }
})
const lit = computed(() => {
  const on = new Set<string>()
  const id = active.value
  if (!id || !layout.value) return on
  const { out, inn } = adjacency.value
  const walk = (adj: Map<string, string[]>) => {
    const stack = [id]
    while (stack.length) {
      const cur = stack.pop()!
      for (const next of adj.get(cur) ?? []) {
        if (!on.has(next)) {
          on.add(next)
          stack.push(next)
        }
      }
    }
  }
  on.add(id)
  walk(out)
  walk(inn)
  return on
})
const litLinks = computed(() => {
  const set = new Set<number>()
  layout.value?.links.forEach((l, i) => {
    if (lit.value.has(l.source) && lit.value.has(l.target)) set.add(i)
  })
  return set
})

/*
 * A click selects the node: its id goes in `?selected=` so a reload or a shared
 * link reopens it, and the shell's overlay panel shows the entity (the same
 * gated detail view a list row opens). The panel's expand control goes to the
 * entity's own page.
 */
const panel = useDetailPanel()

const selectedId = computed(() => {
  const raw = route.query.selected
  const value = Array.isArray(raw) ? raw[raw.length - 1] : raw
  return typeof value === 'string' && value !== '' ? value : null
})
/** The selected entity: a node, or a pip drawn beside one. */
const selectedNode = computed(() => {
  const id = selectedId.value
  if (!id) return null
  return nodeInfo.value.get(id) ?? pipInfo.value.get(id) ?? null
})

function select(id: string) {
  if (id === selectedId.value) return
  void router.replace({ query: { ...route.query, selected: id } })
}
function closePanel() {
  const query = { ...route.query }
  delete query.selected
  void router.replace({ query })
}
function expandPanel() {
  const n = selectedNode.value
  if (!n) return
  const path = `/entity/${n.type}/${n.id}`
  const query = fromPageQuery(route)
  void router.push(Object.keys(query).length ? { path, query } : path)
}

/*
 * Opening the panel narrows the diagram's area, which moves the selected item
 * out of view. Bring it back once the node exists and again after the panel's
 * resize has settled.
 */
let scrollTimer: ReturnType<typeof setTimeout> | undefined
function scrollSelectedIntoView() {
  const el = mainEl.value?.querySelector('.node-group.selected rect.node, circle.pip.selected')
  el?.scrollIntoView({ block: 'center', inline: 'center' })
}
// Keyed on the resolved entity, not the raw id: on a cold load the id is in the
// URL before the data that holds the node has arrived.
watch(
  () => selectedNode.value?.id,
  (id) => {
    clearTimeout(scrollTimer)
    if (!id) return
    void nextTick(scrollSelectedIntoView)
    scrollTimer = setTimeout(scrollSelectedIntoView, 300)
  },
  { immediate: true }
)
onBeforeUnmount(() => clearTimeout(scrollTimer))

watch(
  selectedNode,
  (n) => {
    if (!n) {
      panel.clear()
      return
    }
    panel.show({
      component: EntityDetailPanel,
      props: {
        entityType: n.type,
        // The address of the face the diagram shows, as a list row passes it.
        entityId: n.face ? `${n.id}@${n.face}` : n.id,
        onClose: closePanel,
        onExpand: expandPanel,
      },
      // Over the diagram rather than beside it: it needs its full width.
      mode: 'overlay',
    })
  },
  { immediate: true }
)

const headerHtml = computed(() => (config.value?.header ? renderMarkdown(config.value.header) : ''))
const footerHtml = computed(() => (config.value?.footer ? renderMarkdown(config.value.footer) : ''))
</script>

<template>
  <div class="sankey-view">
    <div class="sankey-head">
      <h1>{{ config?.title || id }}</h1>
      <span v-if="data?.truncated" class="truncated-flag" :title="t('sankey.truncatedTitle')">
        {{ t('sankey.truncated') }}
      </span>
    </div>

    <!-- eslint-disable-next-line vue/no-v-html -- admin-authored, sanitized in renderMarkdown -->
    <div v-if="headerHtml" class="sankey-info" v-html="headerHtml" />

    <div v-if="!config" class="sankey-error">{{ t('sankey.unknown') }}</div>
    <div v-else-if="error" class="sankey-error">{{ error }}</div>
    <p v-else-if="data && data.nodes.length === 0" class="sankey-empty">{{ t('sankey.empty') }}</p>

    <div v-else ref="mainEl" class="sankey-main">
      <svg
        v-if="layout"
        :width="width"
        :height="svgHeight"
        :class="{ dim: active }"
        role="group"
        :aria-label="t('sankey.diagramLabel', { title: config.title })"
      >
        <text v-for="h in layout.headings" :key="h.type" class="col-head" :x="h.x" :y="h.y">
          {{ typeLabel(h.type) }}
        </text>
        <g class="links">
          <path
            v-for="(l, i) in layout.links"
            :key="i"
            class="link"
            :class="{ on: litLinks.has(i) }"
            :d="l.path"
            :stroke-width="Math.max(l.width, 1)"
          />
        </g>
        <g
          v-for="n in layout.nodes"
          :key="n.id"
          class="node-group"
          :class="{ on: lit.has(n.id), selected: n.id === selectedId }"
        >
          <rect
            class="node"
            tabindex="0"
            role="button"
            :aria-label="t('sankey.nodeLabel', { type: typeLabel(n.type), name: nodeName(n.id) })"
            :x="n.x0"
            :y="n.y0"
            :width="n.x1 - n.x0"
            :height="Math.max(n.y1 - n.y0, 2)"
            :fill="fillOf(n.id)"
            @mouseenter="active = n.id"
            @mouseleave="active = null"
            @focus="active = n.id"
            @blur="active = null"
            @click="select(n.id)"
            @keydown.enter.prevent="select(n.id)"
            @keydown.space.prevent="select(n.id)"
          >
            <title>{{ nodeName(n.id) }}</title>
          </rect>
          <text
            class="label"
            :x="labelX(n)"
            :y="(n.y0 + n.y1) / 2"
            :text-anchor="n.col === 0 ? 'end' : 'start'"
            dominant-baseline="middle"
          >
            {{ nodeText(n.id) }}
          </text>
          <circle
            v-for="(p, i) in nodeInfo.get(n.id)?.pips ?? []"
            :key="p.id"
            class="pip"
            :class="{ selected: p.id === selectedId }"
            tabindex="0"
            role="button"
            :aria-label="t('sankey.pipLabel', { type: typeLabel(p.type), name: p.title || p.id })"
            :cx="pipX(n, i)"
            :cy="(n.y0 + n.y1) / 2"
            :r="PIP_R"
            :fill="pipFill(p)"
            @click.stop="select(p.id)"
            @keydown.enter.prevent="select(p.id)"
            @keydown.space.prevent="select(p.id)"
          >
            <title>{{ p.title || p.id }}</title>
          </circle>
          <text
            v-if="nodeInfo.get(n.id)?.pips_more"
            class="pips-more"
            :x="pipX(n, nodeInfo.get(n.id)?.pips?.length ?? 0) + (n.col === 0 ? PIP_R : -PIP_R)"
            :y="(n.y0 + n.y1) / 2"
            :text-anchor="n.col === 0 ? 'end' : 'start'"
            dominant-baseline="middle"
          >
            +{{ nodeInfo.get(n.id)?.pips_more }}
          </text>
        </g>
      </svg>
    </div>

    <!-- eslint-disable-next-line vue/no-v-html -- admin-authored, sanitized in renderMarkdown -->
    <div v-if="footerHtml" class="sankey-info" v-html="footerHtml" />
  </div>
</template>

<style scoped>
.sankey-view {
  padding: 1rem 1.25rem;
}
.sankey-head {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}
.sankey-head h1 {
  font-size: 1.25rem;
  margin: 0;
}
.sankey-error {
  color: var(--rl-color-danger);
  padding: 1rem;
}
.sankey-empty {
  color: var(--rl-color-text-muted);
}
.sankey-main {
  overflow-x: auto;
}
svg {
  display: block;
  margin: 0 auto;
}
.col-head {
  font-size: 12px;
  font-weight: 600;
  fill: var(--rl-color-text-muted);
}
.link {
  fill: none;
  stroke: var(--rl-color-text-muted);
  stroke-opacity: 0.3;
}
.node {
  cursor: pointer;
}
.pip {
  cursor: pointer;
  stroke: var(--rl-color-bg);
  stroke-width: 1px;
}
.pip:focus-visible {
  outline: 2px solid var(--rl-color-text);
  outline-offset: 1px;
}
.pip.selected {
  stroke: var(--rl-color-text);
  stroke-width: 2px;
}
.pips-more {
  font-size: 10px;
  fill: var(--rl-color-text-muted);
  pointer-events: none;
}
.node-group.selected .node {
  stroke: var(--rl-color-text);
  stroke-width: 2px;
}
.node:focus-visible {
  outline: 2px solid var(--rl-color-text);
  outline-offset: 1px;
}
.label {
  font-size: 11px;
  fill: var(--rl-color-text);
  pointer-events: none;
  paint-order: stroke;
  stroke: var(--rl-color-bg);
  stroke-width: 3px;
}
.dim .link {
  stroke-opacity: 0.04;
}
.dim .link.on {
  stroke-opacity: 0.75;
  stroke: var(--rl-color-text);
}
.dim .node-group {
  opacity: 0.2;
}
.dim .node-group.on {
  opacity: 1;
}
.truncated-flag {
  font-size: 0.75rem;
  color: var(--rl-color-warning-fg);
  background: var(--rl-color-warning-bg);
  border: 1px solid var(--rl-color-warning-fg);
  border-radius: 3px;
  padding: 0 5px;
  cursor: help;
}
@media (prefers-reduced-motion: no-preference) {
  .link,
  .node-group {
    transition:
      opacity 0.12s,
      stroke-opacity 0.12s;
  }
}
</style>
