<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUIStore, useSchemaStore } from '@/stores'
import { getErrorMessage, ApiError } from '@/api/errors'
import { listVersions, getVersion, restoreVersion, type VersionMeta } from '@/api/history'
import { getEntity as fetchEntity } from '@/api/entities'
import { lineDiff, propertyDiff, type DiffLine, type PropertyChange } from '@/utils/lineDiff'
import { isEnumPropertyDef } from '@/utils/format'
import Badge from '@/components/common/Badge.vue'
import type { Entity } from '@/types'

const route = useRoute()
const router = useRouter()
const uiStore = useUIStore()
const schemaStore = useSchemaStore()

const entityType = computed(() => String(route.params.type))
const entityId = computed(() => String(route.params.id))

const loading = ref(true)
const unsupported = ref(false)
const error = ref('')
const versions = ref<VersionMeta[]>([])
const current = ref<Entity | null>(null)

// A comparison side is either a version ordinal (number) or 'current' (the live
// entity). 'current' is the sentinel for the working state, so a user can diff
// any past version against another OR against the live entity.
type Side = number | 'current'

const baseSel = ref<Side>('current')
const targetSel = ref<Side>('current')
const contentDiff = ref<DiffLine[]>([])
const propDiff = ref<PropertyChange[]>([])
const restoring = ref(false)

// The timeline highlights the base selection (the row the user is "on").
const selectedVersion = computed<number | null>(() =>
  typeof baseSel.value === 'number' ? baseSel.value : null,
)

function sideLabel(s: Side): string {
  return s === 'current' ? 'current' : `v${s}`
}

// Whether the current entity is writable (server-computed update affordance).
const canRestore = computed(() => current.value?._actions?.update !== false)

// The entity type definition, for resolving property labels + badge styling.
const typeDef = computed(() => schemaStore.getEntityType(entityType.value))

// Property labels are not carried on PropertyDef (they live on form/view field
// config); for a raw property diff, humanize the property name — "display_name"
// → "Display name".
function propertyLabel(name: string): string {
  const spaced = name.replace(/[_-]+/g, ' ').trim()
  return spaced.charAt(0).toUpperCase() + spaced.slice(1)
}

// Whether a property should render as a Badge (enum types), reusing the same
// canonical detection the entity list and detail views use.
function isEnum(name: string): boolean {
  return isEnumPropertyDef(typeDef.value?.properties?.[name])
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
    const [vs, ent] = await Promise.all([
      listVersions(entityType.value, entityId.value),
      fetchEntity(entityType.value, entityId.value).catch(() => null),
    ])
    versions.value = vs
    current.value = ent
    // Default comparison: the most recent version → current, preserving the
    // "what changed since this version" reading the screen opened with.
    if (vs.length) {
      baseSel.value = vs[vs.length - 1].version
      targetSel.value = 'current'
      await recompute()
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 501) {
      unsupported.value = true
    } else {
      error.value = getErrorMessage(err, 'Failed to load version history')
    }
  } finally {
    loading.value = false
  }
}

// sideState resolves a comparison side to its {content, properties}: the live
// entity for 'current', or a fetched snapshot for a version ordinal.
async function sideState(s: Side): Promise<{ content: string; properties: Record<string, unknown> }> {
  if (s === 'current') {
    return {
      content: current.value?.content ?? '',
      properties: (current.value?.properties ?? {}) as Record<string, unknown>,
    }
  }
  const snap = await getVersion(entityType.value, entityId.value, s)
  return {
    content: snap.entity.content ?? '',
    properties: (snap.entity.properties ?? {}) as Record<string, unknown>,
  }
}

// recomputeSeq guards against a stale in-flight recompute clobbering a fresher
// one: rapidly changing base/target fires overlapping async diffs, and a slower
// earlier one could otherwise paint a diff that contradicts the current
// selections. Only the newest recompute commits its result.
let recomputeSeq = 0

// recompute diffs base → target (before → after). Called whenever either
// selection changes.
async function recompute() {
  const seq = ++recomputeSeq
  try {
    const [before, after] = await Promise.all([sideState(baseSel.value), sideState(targetSel.value)])
    if (seq !== recomputeSeq) return // a newer recompute superseded this one
    contentDiff.value = lineDiff(before.content, after.content)
    propDiff.value = propertyDiff(before.properties, after.properties)
  } catch (err) {
    if (seq !== recomputeSeq) return
    // Clear the stale diff so the pane never shows a comparison that no longer
    // matches the dropdowns.
    contentDiff.value = []
    propDiff.value = []
    uiStore.showToast('error', getErrorMessage(err, 'Failed to compute diff'))
  }
}

// Clicking a timeline row sets the BASE (before) side and recomputes.
function selectVersion(v: number) {
  baseSel.value = v
  void recompute()
}

// Swap the two sides (reverse the diff direction).
function swapSides() {
  ;[baseSel.value, targetSel.value] = [targetSel.value, baseSel.value]
  void recompute()
}

async function restore(v: number) {
  if (restoring.value) return
  restoring.value = true
  try {
    await restoreVersion(entityType.value, entityId.value, v)
    uiStore.showToast('success', `Restored to version ${v}`)
    await load()
  } catch (err) {
    uiStore.showToast('error', getErrorMessage(err, 'Restore failed'))
  } finally {
    restoring.value = false
  }
}

function principalLabel(m: VersionMeta): string {
  const user = m.principal.user || 'unknown'
  return m.principal.tool ? `${user} · ${m.principal.tool}` : user
}

function formatWhen(iso: string): string {
  const d = new Date(iso)
  return isNaN(d.getTime()) ? iso : d.toLocaleString()
}

function goBack() {
  router.push(`/entity/${entityType.value}/${entityId.value}`)
}

const hasContentChanges = computed(() => contentDiff.value.some((l) => l.op !== 'equal'))

// Newest-first for the compare dropdowns (the timeline itself is oldest-first).
const versionsNewestFirst = computed(() => [...versions.value].reverse())

onMounted(load)
</script>

<template>
  <div class="history-view">
    <div class="page-header">
      <div>
        <h2>Version history</h2>
        <p>{{ entityType }} · {{ entityId }}</p>
      </div>
      <button class="btn btn-secondary" @click="goBack">Back to entity</button>
    </div>

    <div v-if="loading" class="loading-state">Loading version history…</div>
    <div v-else-if="unsupported" class="loading-state">
      Version history is not available for this deployment.
    </div>
    <div v-else-if="error" class="error-state">{{ error }}</div>
    <div v-else-if="versions.length === 0" class="loading-state">
      No versions recorded yet.
    </div>

    <div v-else class="history-layout">
      <!-- Timeline -->
      <aside class="card timeline-card">
        <ul class="timeline">
          <li
            v-for="m in versions"
            :key="m.version"
            class="timeline-item"
            :class="{ selected: selectedVersion === m.version }"
          >
            <button type="button" class="timeline-select" @click="selectVersion(m.version)">
              <span class="timeline-badge" :data-op="m.op">{{ m.op }}</span>
              <span class="timeline-ver">v{{ m.version }}</span>
              <span class="timeline-who">{{ principalLabel(m) }}</span>
              <span class="timeline-when">{{ formatWhen(m.created_at) }}</span>
              <span v-if="m.prev_id" class="timeline-note">renamed from {{ m.prev_id }}</span>
              <span v-if="m.triggered_by" class="timeline-note">{{ m.triggered_by }}</span>
            </button>
            <button
              v-if="canRestore && m.op !== 'delete'"
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

      <!-- Diff between two chosen sides (any version or the live entity). -->
      <section class="card diff-card">
        <div class="compare-bar">
          <span class="compare-label">Compare</span>
          <select v-model="baseSel" class="compare-select" @change="recompute">
            <option value="current">current</option>
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
            <option value="current">current</option>
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
                <Badge v-if="isEnum(c.key)" :value="displayValue(c.after)" :property="c.key" />
                <span v-else class="prop-val">{{ displayValue(c.after) }}</span>
              </template>
              <template v-else-if="c.op === 'del'">
                <span class="prop-tag prop-tag--del">removed</span>
                <Badge v-if="isEnum(c.key)" :value="displayValue(c.before)" :property="c.key" />
                <span v-else class="prop-val prop-val--old">{{ displayValue(c.before) }}</span>
              </template>
              <template v-else>
                <template v-if="isEnum(c.key)">
                  <Badge :value="displayValue(c.before)" :property="c.key" />
                  <span class="prop-arrow">→</span>
                  <Badge :value="displayValue(c.after)" :property="c.key" />
                </template>
                <template v-else>
                  <span class="prop-val prop-val--old">{{ displayValue(c.before) }}</span>
                  <span class="prop-arrow">→</span>
                  <span class="prop-val">{{ displayValue(c.after) }}</span>
                </template>
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
          {{ baseSel === targetSel ? 'Select two different sides to compare.' : 'These two are identical.' }}
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
</style>
