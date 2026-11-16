<script setup lang="ts">
/**
 * A `display: line` dashboard card (TKT-RK810V7): one line per owner over the
 * buckets of `/_records/{type}/_aggregate?per_owner=true`.
 *
 * The server folds only the owners this reader may read. An owner among
 * several that the reader cannot read is absent from the points, so it gets
 * no line, no legend entry and no title lookup. A card with one owner that
 * the reader cannot read gets a 404, which this card shows as no data.
 *
 * Colour follows the owner's position in `owners:`, never its rank among the
 * lines drawn, so a hidden owner does not repaint the others.
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { aggregateRecords, getEntity, getRecordTypes, shouldDropHeldContent } from '@/api'
import { intlLocale } from '@/i18n/locale'
import type { DashboardRecords } from '@/types'
import {
  DEFAULT_BUCKET,
  bucketStarts,
  buildSeries,
  formatBucket,
  niceTicks,
  rangeStart,
  type Series,
} from '@/utils/recordsChart'

const props = defineProps<{ records: DashboardRecords; title: string }>()
const { t } = useI18n()

const HEIGHT = 200
const PAD = { top: 12, right: 16, bottom: 28, left: 48 }
const END_LABEL_GAP = 12

const series = ref<Series[]>([])
const titles = ref<Map<string, string>>(new Map())
const loaded = ref(false)
const failed = ref(false)
const width = ref(480)
const active = ref<number | null>(null)
const root = ref<HTMLElement | null>(null)

let generation = 0
let observer: ResizeObserver | undefined

async function load() {
  const gen = ++generation
  const cfg = props.records
  failed.value = false
  try {
    const points = await aggregateRecords(cfg.type, {
      owners: cfg.owners,
      field: cfg.field,
      bucket: cfg.bucket ?? DEFAULT_BUCKET,
      from: rangeStart(cfg.range, new Date()),
      perOwner: true,
    })
    if (gen !== generation) return
    series.value = buildSeries(cfg, points)
    void loadTitles(gen, series.value)
  } catch (err) {
    if (gen !== generation) return
    // A denial drops what the card held: a reader who lost access must not
    // keep seeing the line.
    series.value = []
    if (!shouldDropHeldContent(err)) {
      console.error('Records card load error:', err)
      failed.value = true
    }
  } finally {
    if (gen === generation) loaded.value = true
  }
}

/**
 * Looks up the title of each owner that has a line. Only owners the server
 * returned are looked up; an owner whose title cannot be read keeps its id.
 */
async function loadTitles(gen: number, drawn: Series[]) {
  if (drawn.length === 0) return
  try {
    const ownerType = (await getRecordTypes()).find((r) => r.key === props.records.type)?.owner
    if (!ownerType) return
    const found = await Promise.all(
      drawn.map((s) =>
        getEntity(ownerType, s.owner)
          .then((e) => [s.owner, e._title || s.owner] as const)
          .catch(() => [s.owner, s.owner] as const)
      )
    )
    if (gen === generation) titles.value = new Map(found)
  } catch {
    // The ids stay as labels.
  }
}

function label(owner: string): string {
  return titles.value.get(owner) ?? owner
}

const locale = computed(() => intlLocale())
const numberFormat = computed(
  () => new Intl.NumberFormat(locale.value, { maximumFractionDigits: 2 })
)
const tickFormat = computed(
  () => new Intl.NumberFormat(locale.value, { notation: 'compact', maximumFractionDigits: 1 })
)

const starts = computed(() => bucketStarts(series.value))
const ticks = computed(() => {
  const values = series.value.flatMap((s) => s.points.map((p) => p.v))
  return values.length ? niceTicks(Math.min(...values), Math.max(...values)) : []
})
const plotWidth = computed(() => Math.max(width.value - PAD.left - PAD.right, 40))
const plotHeight = HEIGHT - PAD.top - PAD.bottom

function x(tms: number): number {
  const s = starts.value
  if (s.length < 2) return PAD.left + plotWidth.value / 2
  return PAD.left + ((tms - s[0]) / (s[s.length - 1] - s[0])) * plotWidth.value
}

function y(v: number): number {
  const tk = ticks.value
  const lo = tk[0]
  const hi = tk[tk.length - 1]
  return PAD.top + plotHeight - ((v - lo) / (hi - lo)) * plotHeight
}

function path(s: Series): string {
  return s.points
    .map((p, i) => `${i ? 'L' : 'M'}${x(p.t).toFixed(1)},${y(p.v).toFixed(1)}`)
    .join('')
}

/** At most five x labels, evenly spread and always including the last. */
const xLabels = computed(() => {
  const s = starts.value
  const n = Math.min(s.length, Math.max(2, Math.floor(plotWidth.value / 110)))
  if (s.length <= n) return s
  const step = (s.length - 1) / (n - 1)
  return Array.from({ length: n }, (_, i) => s[Math.round(i * step)])
})

function fmtBucket(tms: number): string {
  return formatBucket(tms, props.records.bucket, locale.value)
}

/** Value labels at the line ends, for up to four lines, skipping collisions. */
const endLabels = computed(() => {
  if (series.value.length > 4) return []
  const placed: { owner: string; text: string; y: number }[] = []
  const ends = series.value
    .map((s) => ({ s, last: s.points[s.points.length - 1] }))
    .sort((a, b) => y(a.last.v) - y(b.last.v))
  for (const { s, last } of ends) {
    const ly = y(last.v)
    if (placed.some((p) => Math.abs(p.y - ly) < END_LABEL_GAP)) continue
    placed.push({ owner: s.owner, text: numberFormat.value.format(last.v), y: ly })
  }
  return placed
})

const lastX = computed(() => (starts.value.length ? x(starts.value[starts.value.length - 1]) : 0))

/** Every line's value at the active bucket, for the tooltip. */
const readout = computed(() => {
  if (active.value === null) return null
  const tms = starts.value[active.value]
  if (tms === undefined) return null
  const rows = series.value.flatMap((s) => {
    const p = s.points.find((pt) => pt.t === tms)
    return p ? [{ owner: s.owner, slot: s.slot, value: numberFormat.value.format(p.v) }] : []
  })
  return { tms, x: x(tms), when: fmtBucket(tms), rows }
})

function onPointer(ev: PointerEvent) {
  const svg = ev.currentTarget as SVGSVGElement
  const box = svg.getBoundingClientRect()
  const px = ev.clientX - box.left
  let best = 0
  starts.value.forEach((tms, i) => {
    if (Math.abs(x(tms) - px) < Math.abs(x(starts.value[best]) - px)) best = i
  })
  active.value = starts.value.length ? best : null
}

function onKey(ev: KeyboardEvent) {
  const n = starts.value.length
  if (!n) return
  if (ev.key === 'ArrowLeft' || ev.key === 'ArrowRight') {
    ev.preventDefault()
    const cur = active.value ?? n - 1
    active.value = Math.min(n - 1, Math.max(0, cur + (ev.key === 'ArrowRight' ? 1 : -1)))
  } else if (ev.key === 'Escape') {
    active.value = null
  }
}

/** The table view: one row per bucket, one column per line. */
const tableRows = computed(() =>
  starts.value.map((tms) => ({
    tms,
    when: fmtBucket(tms),
    cells: series.value.map((s) => {
      const p = s.points.find((pt) => pt.t === tms)
      return p ? numberFormat.value.format(p.v) : ''
    }),
  }))
)

onMounted(() => {
  void load()
  if (root.value && typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width
      if (w) width.value = w
    })
    observer.observe(root.value)
  }
})
onBeforeUnmount(() => {
  generation++
  observer?.disconnect()
})
watch(
  () => props.records,
  () => void load(),
  { deep: true }
)
</script>

<template>
  <div ref="root" class="records-chart">
    <p v-if="failed" class="records-chart__note" data-testid="card-failed">
      {{ t('dashboard.cardFailed') }}
    </p>
    <p
      v-else-if="loaded && series.length === 0"
      class="records-chart__note"
      data-testid="records-empty"
    >
      {{ t('dashboard.noData') }}
    </p>
    <template v-else-if="series.length">
      <ul v-if="series.length > 1" class="records-chart__legend" data-testid="records-legend">
        <li v-for="s in series" :key="s.owner">
          <span
            class="records-chart__key"
            :class="`records-chart__s${s.slot + 1}`"
            aria-hidden="true"
          />
          {{ label(s.owner) }}
        </li>
      </ul>
      <div class="records-chart__plot">
        <svg
          :width="width"
          :height="HEIGHT"
          :viewBox="`0 0 ${width} ${HEIGHT}`"
          role="img"
          :aria-label="t('dashboard.lineChartLabel', { title })"
          tabindex="0"
          data-testid="records-svg"
          @pointermove="onPointer"
          @pointerleave="active = null"
          @focus="active = starts.length - 1"
          @blur="active = null"
          @keydown="onKey"
        >
          <g class="records-chart__grid">
            <line
              v-for="tk in ticks"
              :key="tk"
              :x1="PAD.left"
              :x2="PAD.left + plotWidth"
              :y1="y(tk)"
              :y2="y(tk)"
              :class="{ 'records-chart__baseline': tk === 0 }"
            />
          </g>
          <g class="records-chart__axis">
            <text
              v-for="tk in ticks"
              :key="tk"
              :x="PAD.left - 8"
              :y="y(tk)"
              text-anchor="end"
              dy="0.32em"
            >
              {{ tickFormat.format(tk) }}
            </text>
            <text
              v-for="tms in xLabels"
              :key="tms"
              :x="x(tms)"
              :y="HEIGHT - 8"
              :text-anchor="
                tms === starts[0] && starts.length > 1
                  ? 'start'
                  : tms === starts[starts.length - 1] && starts.length > 1
                    ? 'end'
                    : 'middle'
              "
            >
              {{ fmtBucket(tms) }}
            </text>
          </g>
          <line
            v-if="readout"
            class="records-chart__crosshair"
            :x1="readout.x"
            :x2="readout.x"
            :y1="PAD.top"
            :y2="PAD.top + plotHeight"
          />
          <g
            v-for="s in series"
            :key="s.owner"
            :class="`records-chart__s${s.slot + 1}`"
            data-testid="records-line"
          >
            <path v-if="s.points.length > 1" class="records-chart__line" :d="path(s)" />
            <circle
              class="records-chart__dot"
              :cx="x(s.points[s.points.length - 1].t)"
              :cy="y(s.points[s.points.length - 1].v)"
              r="4"
            />
          </g>
          <text
            v-for="l in endLabels"
            :key="l.owner"
            class="records-chart__end"
            :x="Math.min(lastX + 8, width - 2)"
            :y="l.y"
            :text-anchor="lastX + 48 > width ? 'end' : 'start'"
            dy="-0.6em"
          >
            {{ l.text }}
          </text>
        </svg>
        <div
          v-if="readout"
          class="records-chart__tip"
          :class="{ 'records-chart__tip--left': readout.x > width / 2 }"
          :style="{ left: `${readout.x}px` }"
          role="status"
          data-testid="records-tooltip"
        >
          <div class="records-chart__tip-when">{{ readout.when }}</div>
          <div v-for="r in readout.rows" :key="r.owner" class="records-chart__tip-row">
            <span
              class="records-chart__tip-key"
              :class="`records-chart__s${r.slot + 1}`"
              aria-hidden="true"
            />
            <strong>{{ r.value }}</strong>
            <span class="records-chart__tip-name">{{ label(r.owner) }}</span>
          </div>
        </div>
      </div>
      <details class="records-chart__table">
        <summary>{{ t('dashboard.showTable') }}</summary>
        <table data-testid="records-table">
          <thead>
            <tr>
              <th scope="col">{{ t('dashboard.period') }}</th>
              <th v-for="s in series" :key="s.owner" scope="col">{{ label(s.owner) }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in tableRows" :key="row.tms">
              <th scope="row">{{ row.when }}</th>
              <td v-for="(c, i) in row.cells" :key="i">{{ c }}</td>
            </tr>
          </tbody>
        </table>
      </details>
    </template>
  </div>
</template>

<style scoped src="./DashboardRecordsChart.css"></style>
