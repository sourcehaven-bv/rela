<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUIStore } from '@/stores'
import { getErrorMessage, ApiError } from '@/api/errors'
import {
  listRelationVersions,
  getRelationVersion,
  restoreRelationVersion,
  type RelationVersionMeta,
} from '@/api/history'
import { lineDiff, propertyDiff, type DiffLine, type PropertyChange } from '@/utils/lineDiff'

const route = useRoute()
const router = useRouter()
const uiStore = useUIStore()

// A relation is addressed by fromType/from/relType/to (fromType lets the read
// gate evaluate the source entity's per-type verdict).
const fromType = computed(() => String(route.params.fromType))
const from = computed(() => String(route.params.from))
const relType = computed(() => String(route.params.relType))
const to = computed(() => String(route.params.to))

const loading = ref(true)
const unsupported = ref(false)
const error = ref('')
const versions = ref<RelationVersionMeta[]>([])

// A comparison side is a version ordinal or 'current' (the live relation, taken
// from the newest snapshot since relations have no separate live-fetch endpoint
// here — the newest version reflects the current settled state).
type Side = number | 'current'
const baseSel = ref<Side>('current')
const targetSel = ref<Side>('current')
const contentDiff = ref<DiffLine[]>([])
const propDiff = ref<PropertyChange[]>([])
const restoring = ref(false)

const selectedVersion = computed<number | null>(() =>
  typeof baseSel.value === 'number' ? baseSel.value : null
)

function sideLabel(s: Side): string {
  return s === 'current' ? 'latest' : `v${s}`
}

function propertyLabel(name: string): string {
  const spaced = name.replace(/[_-]+/g, ' ').trim()
  return spaced.charAt(0).toUpperCase() + spaced.slice(1)
}

function displayValue(v: unknown): string {
  if (v === null || v === undefined || v === '') return '∅'
  if (Array.isArray(v)) return v.join(', ')
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    versions.value = await listRelationVersions(fromType.value, from.value, relType.value, to.value)
    if (versions.value.length) {
      const last = versions.value[versions.value.length - 1].version
      baseSel.value =
        versions.value.length > 1 ? versions.value[versions.value.length - 2].version : last
      targetSel.value = last
      await recompute()
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 501) {
      unsupported.value = true
    } else {
      error.value = getErrorMessage(err, 'Failed to load relation history')
    }
  } finally {
    loading.value = false
  }
}

// sideState resolves a comparison side to {content, properties}. 'current' maps
// to the newest version; any ordinal fetches that snapshot.
async function sideState(
  s: Side
): Promise<{ content: string; properties: Record<string, unknown> }> {
  const version = s === 'current' ? versions.value[versions.value.length - 1]?.version : s
  if (!version) return { content: '', properties: {} }
  const snap = await getRelationVersion(
    fromType.value,
    from.value,
    relType.value,
    to.value,
    version
  )
  return {
    content: snap.relation.content ?? '',
    properties: (snap.relation.meta ?? {}) as Record<string, unknown>,
  }
}

let recomputeSeq = 0
async function recompute() {
  const seq = ++recomputeSeq
  try {
    const [before, after] = await Promise.all([
      sideState(baseSel.value),
      sideState(targetSel.value),
    ])
    if (seq !== recomputeSeq) return
    contentDiff.value = lineDiff(before.content, after.content)
    propDiff.value = propertyDiff(before.properties, after.properties)
  } catch (err) {
    if (seq !== recomputeSeq) return
    contentDiff.value = []
    propDiff.value = []
    uiStore.showToast('error', getErrorMessage(err, 'Failed to compute diff'))
  }
}

function selectVersion(v: number) {
  baseSel.value = v
  void recompute()
}

function swapSides() {
  ;[baseSel.value, targetSel.value] = [targetSel.value, baseSel.value]
  void recompute()
}

async function restore(v: number) {
  if (restoring.value) return
  restoring.value = true
  try {
    await restoreRelationVersion(fromType.value, from.value, relType.value, to.value, v)
    uiStore.showToast('success', `Restored relation to version ${v}`)
    await load()
  } catch (err) {
    uiStore.showToast('error', getErrorMessage(err, 'Restore failed'))
  } finally {
    restoring.value = false
  }
}

function principalLabel(m: RelationVersionMeta): string {
  const user = m.principal.user || 'unknown'
  return m.principal.tool ? `${user} · ${m.principal.tool}` : user
}

function formatWhen(iso: string): string {
  const d = new Date(iso)
  return isNaN(d.getTime()) ? iso : d.toLocaleString()
}

function goBack() {
  router.push(`/entity/${fromType.value}/${from.value}`)
}

const hasContentChanges = computed(() => contentDiff.value.some((l) => l.op !== 'equal'))
const versionsNewestFirst = computed(() => [...versions.value].reverse())

onMounted(load)
</script>

<template>
  <div class="history-view">
    <div class="page-header">
      <div>
        <h2>Relation history</h2>
        <p>
          {{ from }} <span class="rel-arrow">—{{ relType }}→</span> {{ to }}
        </p>
      </div>
      <button class="btn btn-secondary" @click="goBack">Back to entity</button>
    </div>

    <div v-if="loading" class="loading-state">Loading relation history…</div>
    <div v-else-if="unsupported" class="loading-state">
      Relation version history is not available for this deployment.
    </div>
    <div v-else-if="error" class="error-state">{{ error }}</div>
    <div v-else-if="versions.length === 0" class="loading-state">No versions recorded yet.</div>

    <div v-else class="history-layout">
      <aside class="card timeline-card">
        <ul class="timeline">
          <li
            v-for="m in versions"
            :key="m.version"
            class="timeline-item"
            :class="{ selected: selectedVersion === m.version }"
            :data-version="m.version"
          >
            <button type="button" class="timeline-select" @click="selectVersion(m.version)">
              <span class="timeline-badge" :data-op="m.op">{{ m.op }}</span>
              <span class="timeline-ver">v{{ m.version }}</span>
              <span class="timeline-who">{{ principalLabel(m) }}</span>
              <span class="timeline-when">{{ formatWhen(m.created_at) }}</span>
              <span v-if="m.op === 'rename' && (m.prev_from || m.prev_to)" class="timeline-note">
                was {{ m.prev_from }}—{{ m.type }}→{{ m.prev_to }}
              </span>
              <span v-if="m.triggered_by" class="timeline-note">{{ m.triggered_by }}</span>
            </button>
            <button
              v-if="m.op !== 'delete'"
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="restoring"
              @click="restore(m.version)"
            >
              Restore
            </button>
          </li>
        </ul>
      </aside>

      <section class="card diff-card">
        <div class="compare-bar">
          <span class="compare-label">Compare</span>
          <select v-model="baseSel" class="compare-select" @change="recompute">
            <option value="current">latest</option>
            <option v-for="m in versionsNewestFirst" :key="m.version" :value="m.version">
              v{{ m.version }} · {{ m.op }}
            </option>
          </select>
          <button
            type="button"
            class="btn btn-ghost btn-sm compare-swap"
            title="Swap sides"
            @click="swapSides"
          >
            ⇄
          </button>
          <select v-model="targetSel" class="compare-select" @change="recompute">
            <option value="current">latest</option>
            <option v-for="m in versionsNewestFirst" :key="m.version" :value="m.version">
              v{{ m.version }} · {{ m.op }}
            </option>
          </select>
        </div>
        <p class="compare-caption">
          {{ sideLabel(baseSel) }} <span class="diff-arrow">→</span> {{ sideLabel(targetSel) }}
        </p>

        <div v-if="propDiff.length" class="prop-diff">
          <div v-for="c in propDiff" :key="c.key" class="prop-row">
            <span class="prop-label">{{ propertyLabel(c.key) }}</span>
            <div class="prop-values">
              <template v-if="c.op === 'add'">
                <span class="prop-tag prop-tag--add">added</span>
                <span class="prop-val">{{ displayValue(c.after) }}</span>
              </template>
              <template v-else-if="c.op === 'del'">
                <span class="prop-tag prop-tag--del">removed</span>
                <span class="prop-val prop-val--old">{{ displayValue(c.before) }}</span>
              </template>
              <template v-else>
                <span class="prop-val prop-val--old">{{ displayValue(c.before) }}</span>
                <span class="prop-arrow">→</span>
                <span class="prop-val">{{ displayValue(c.after) }}</span>
              </template>
            </div>
          </div>
        </div>

        <div v-if="hasContentChanges" class="content-diff-block">
          <div class="content-diff-label">Content</div>
          <pre class="content-diff"><code
          ><span v-for="(l, i) in contentDiff" :key="i" class="diff-line" :data-op="l.op">{{
            l.op === 'add' ? '+ ' : l.op === 'del' ? '- ' : '  '
          }}{{ l.text }}
</span></code></pre>
        </div>

        <p v-if="!propDiff.length && !hasContentChanges" class="loading-state">
          {{
            baseSel === targetSel
              ? 'Select two different sides to compare.'
              : 'These two are identical.'
          }}
        </p>
      </section>
    </div>
  </div>
</template>

<style scoped>
.history-view {
  padding: 24px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 20px;
}
.page-header h2 {
  margin: 0;
}
.page-header p {
  margin: 4px 0 0;
  color: var(--muted-text);
  font-size: 0.9em;
}

.loading-state,
.error-state {
  color: var(--muted-text);
  padding: 24px 0;
}
.error-state {
  color: var(--error-color);
}

.history-layout {
  display: grid;
  /* Timeline stays a readable fixed-ish width; the diff takes all remaining
     space so wide screens are used rather than letterboxed. */
  grid-template-columns: minmax(340px, 460px) 1fr;
  gap: 20px;
  align-items: start;
}
@media (max-width: 820px) {
  .history-layout {
    grid-template-columns: 1fr;
  }
}

.card {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 16px;
}

/* Timeline */
.timeline {
  list-style: none;
  margin: 0;
  padding: 0;
}
.timeline-item {
  display: flex;
  align-items: center;
  gap: 8px;
  border-radius: 6px;
  padding: 2px;
}
.timeline-item.selected {
  background: var(--hover-bg);
}
.timeline-select {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
  flex: 1;
  background: none;
  border: none;
  text-align: left;
  cursor: pointer;
  color: var(--text-color);
  padding: 8px;
  border-radius: 6px;
}
.timeline-select:hover {
  background: var(--hover-bg);
}
.timeline-badge {
  text-transform: uppercase;
  font-size: 0.65em;
  font-weight: 700;
  letter-spacing: 0.03em;
  color: var(--muted-text);
  min-width: 4em;
}
.timeline-badge[data-op='delete'] {
  color: var(--error-color);
}
.timeline-badge[data-op='rename'] {
  color: var(--warning-color);
}
.timeline-badge[data-op='create'] {
  color: var(--success-color);
}
.timeline-ver {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}
.timeline-who {
  flex: 1;
  min-width: 6em;
}
.timeline-when,
.timeline-note {
  color: var(--muted-text);
  font-size: 0.82em;
}

/* Diff */
.compare-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.compare-label {
  font-weight: 600;
}
.compare-select {
  background: var(--input-bg, var(--card-bg));
  color: var(--text-color);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 6px 8px;
  font: inherit;
  cursor: pointer;
}
.compare-swap {
  padding: 4px 8px;
  font-size: 1em;
  line-height: 1;
}
.compare-caption {
  color: var(--muted-text);
  font-size: 0.85em;
  margin: 6px 0 14px;
}
.diff-arrow {
  color: var(--muted-text);
  font-weight: 400;
}
.prop-diff {
  margin-bottom: 16px;
}
.prop-row {
  display: grid;
  grid-template-columns: 8em 1fr;
  gap: 12px;
  align-items: center;
  padding: 6px 0;
  border-bottom: 1px solid var(--border-color);
}
.prop-label {
  font-weight: 600;
  color: var(--muted-text);
  font-size: 0.85em;
}
.prop-values {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.prop-val {
  color: var(--text-color);
}
.prop-val--old {
  color: var(--muted-text);
  text-decoration: line-through;
}
.prop-arrow {
  color: var(--muted-text);
}
.prop-tag {
  font-size: 0.7em;
  font-weight: 700;
  text-transform: uppercase;
  padding: 1px 6px;
  border-radius: 4px;
}
.prop-tag--add {
  color: var(--success-color);
  border: 1px solid var(--success-color);
}
.prop-tag--del {
  color: var(--error-color);
  border: 1px solid var(--error-color);
}

.content-diff-label {
  font-weight: 600;
  margin-bottom: 6px;
}
.content-diff {
  background: var(--code-bg, var(--bg-color));
  border: 1px solid var(--border-color);
  padding: 12px;
  border-radius: 6px;
  overflow-x: auto;
  font-size: 0.85em;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 420px;
  overflow-y: auto;
  margin: 0;
}
.diff-line {
  display: block;
}
.diff-line[data-op='add'] {
  color: var(--success-color);
}
.diff-line[data-op='del'] {
  color: var(--error-color);
  text-decoration: line-through;
}
.rel-arrow {
  color: var(--muted-text);
  font-family: var(--mono-font, monospace);
}
</style>
