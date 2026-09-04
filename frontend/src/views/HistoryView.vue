<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { RouterLink, useRoute, type RouteLocationRaw } from 'vue-router'
import { useUIStore, useSchemaStore } from '@/stores'
import { useWorld } from '@/composables/useWorld'
import { getErrorMessage, ApiError } from '@/api/errors'
import { listVersions, getVersion, restoreVersion, type VersionMeta } from '@/api/history'
import { getEntity as fetchEntity } from '@/api/entities'
import { lineDiff, propertyDiff, type DiffLine, type PropertyChange } from '@/utils/lineDiff'
import { useVersionSelectionSync, type Side } from '@/composables/useVersionSelectionSync'
import { isEnumPropertyDef } from '@/utils/format'
import Badge from '@/components/common/Badge.vue'
import type { Entity } from '@/types'

const route = useRoute()
const uiStore = useUIStore()
const schemaStore = useSchemaStore()

const entityType = computed(() => String(route.params.type))
const entityId = computed(() => String(route.params.id))

// The world rides the history request, so the timeline is the history OF THE
// FACE ON SCREEN (BUG-2). Versioning is per-face — `entity_versions` is keyed
// by content state — so a draft and its published face have genuinely
// different histories, and serving the default face's under a world-bound page
// is the wrong record presented as the right one.
const { world, worldParam, isWorldBound } = useWorld()

// The face this timeline describes, as the SERVER resolved it — read back
// rather than re-derived from the world, for the same reason every other
// surface in this epic reads it back: the store did the resolution.
const face = ref('')
// The world resolves no face for this entity, so there is no history in it.
const worldFaceAbsent = ref(false)

const loading = ref(true)
const unsupported = ref(false)
const error = ref('')

// pageState mirrors DynamicForm's `form-state-*` contract: a stable signal
// that this screen has finished resolving, so a screenshot{} capture can wait
// for it rather than hanging until its timeout.
const pageState = computed<'pending' | 'loaded' | 'error'>(() => {
  if (error.value) return 'error'
  return loading.value ? 'pending' : 'loaded'
})
const versions = ref<VersionMeta[]>([])
const current = ref<Entity | null>(null)

// A comparison side is either a version ordinal (number) or 'current' (the live
// entity). 'current' is the sentinel for the working state, so a user can diff
// any past version against another OR against the live entity. The pair is
// mirrored into `?base=`/`?target=` so a diff can be linked to — see
// useVersionSelectionSync.
const {
  base: baseSel,
  target: targetSel,
  seedFromUrl,
  resetToDefaults,
  select,
  swap: swapSides,
  publish: publishSelection,
} = useVersionSelectionSync({
  validVersions: () => versions.value.map((m) => m.version),
  defaults: () => defaultSelection(),
  onChange: () => void recompute(),
})

// Default: the most recent version → current, preserving the "what changed
// since this version" reading the screen opened with. Extracted (rather than
// inlined in `defaults`) so the post-restore reset uses the same expression and
// the two can't drift.
function defaultSelection(): { base: Side; target: Side } {
  if (!versions.value.length) return { base: 'current', target: 'current' }
  return { base: versions.value[versions.value.length - 1].version, target: 'current' }
}

const contentDiff = ref<DiffLine[]>([])
const propDiff = ref<PropertyChange[]>([])
const restoring = ref(false)

// The timeline highlights the base selection (the row the user is "on").
const selectedVersion = computed<number | null>(() =>
  typeof baseSel.value === 'number' ? baseSel.value : null
)

function sideLabel(s: Side): string {
  return s === 'current' ? 'current' : `v${s}`
}

// Whether the current entity is writable (server-computed update affordance),
// ANDed with the world: a restore is a write, and under a world the timeline
// is the RESOLVED face's while the restore would land on the default face —
// the same mismatch every other write affordance refuses on a world-bound page.
const canRestore = computed(
  () => current.value?._actions?.update !== false && !isWorldBound.value,
)

// The entity type definition, for resolving property labels + badge styling.
const typeDef = computed(() => schemaStore.getEntityType(entityType.value))

// Property labels are not carried on PropertyDef (they live on form/view field
// config), so a raw property diff shows the property name as-is. Humanizing it
// here would be a derived label, which DEC-6C1NAA rules out.
function propertyLabel(name: string): string {
  return name
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

// `fromUrl` re-reads `?base=`/`?target=` now that the version list is known and
// params can be validated against real ordinals. A reload after a RESTORE
// passes false: the version list has changed underneath, so re-seeding would
// resurrect a pair the user chose against the old list.
async function load(fromUrl = true) {
  loading.value = true
  error.value = ''
  try {
    const [timeline, ent] = await Promise.all([
      listVersions(entityType.value, entityId.value, worldParam.value),
      // The live entity is the diff's 'current' side, so it must be the SAME
      // face the timeline describes — otherwise every diff against 'current'
      // compares two different faces and reports changes nobody made.
      fetchEntity(entityType.value, entityId.value, {
        ...(worldParam.value ? { world: worldParam.value } : {}),
      }).catch(() => null),
    ])
    const vs = timeline.versions
    versions.value = vs
    face.value = timeline.face
    worldFaceAbsent.value = timeline.worldFaceAbsent
    current.value = ent
    if (fromUrl) {
      seedFromUrl()
    } else {
      resetToDefaults()
    }
    if (vs.length) await recompute()
    // Publish the resolved pair so a bare URL becomes an explicit, shareable one
    // (and so a post-restore reset is reflected in the address bar). Runs even
    // with NO versions: that is exactly when the URL is most likely to carry a
    // stale ordinal, and leaving it there would let a bookmark re-apply it once
    // the sweep captures versions later.
    publishSelection()
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
async function sideState(
  s: Side
): Promise<{ content: string; properties: Record<string, unknown> }> {
  if (s === 'current') {
    return {
      content: current.value?.content ?? '',
      properties: (current.value?.properties ?? {}) as Record<string, unknown>,
    }
  }
  const snap = await getVersion(entityType.value, entityId.value, s, worldParam.value)
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
    const [before, after] = await Promise.all([
      sideState(baseSel.value),
      sideState(targetSel.value),
    ])
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

// Clicking a timeline row sets the BASE (before) side.
function selectVersion(v: number) {
  select({ base: v })
}

// The dropdowns bind v-model directly, so `select` re-publishes the value the
// ref already holds; passing it explicitly keeps one write path for all four
// mutation sources (dropdown, timeline row, swap, external nav).
function onBaseChange() {
  select({ base: baseSel.value })
}

function onTargetChange() {
  select({ target: targetSel.value })
}

async function restore(v: number) {
  if (restoring.value) return
  restoring.value = true
  try {
    await restoreVersion(entityType.value, entityId.value, v)
    uiStore.showToast('success', `Restored to version ${v}`)
    await load(false)
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

// The provenance note for a mechanism-produced version, or '' for a direct
// edit (TKT-VQHPFK). Same spelling as `rela history`:
//
//   copy from POL-1@draft (publish)
//
// Three things this deliberately does NOT do:
//
//   - It does not replace the op badge beside it. A copy genuinely IS a create
//     or an update; the provenance is an additional annotation on that op.
//   - It returns '' when there is no origin, so a hand edit renders nothing
//     extra. There is no `kind: 'manual'` to fall back to, and inventing one
//     here would mark every row and make the copy marker meaningless — the
//     absence is the signal, and `timeline-who` already names who typed it.
//   - It omits the source when the server withheld it, with no placeholder and
//     no error styling. A withheld source is a normal answer for a reader
//     whose verdict does not cover the source entity; the copy fact and the
//     definition name are still worth stating.
function originLabel(o: VersionMeta['origin']): string {
  if (!o?.kind) return ''
  let s = o.kind
  if (o.source) s += ` from ${o.source}`
  if (o.definition) s += ` (${o.definition})`
  return s
}

function formatWhen(iso: string): string {
  const d = new Date(iso)
  return isNaN(d.getTime()) ? iso : d.toLocaleString()
}

// Pure navigation, so it renders as a real link and supports cmd/middle-click.
// The target keeps the world, so the round trip lands on the face the reader
// came from rather than silently switching them to the default one.
const backTarget = computed<RouteLocationRaw>(() => ({
  path: `/entity/${entityType.value}/${entityId.value}`,
  query: worldParam.value ? { world: worldParam.value } : {},
}))

// The face label shown in the header. A timeline that does not name its subject
// invites the reader to assume the obvious one — which is how the default
// face's history passed for a published page's.
const faceLabel = computed(() => {
  if (!world.value) return ''
  return face.value || 'default'
})

const hasContentChanges = computed(() => contentDiff.value.some((l) => l.op !== 'equal'))

// Newest-first for the compare dropdowns (the timeline itself is oldest-first).
const versionsNewestFirst = computed(() => [...versions.value].reverse())

onMounted(load)
</script>

<template>
  <div class="history-view" :data-testid="`page-state-${pageState}`">
    <div class="page-header">
      <div>
        <h2>Version history</h2>
        <p>
          {{ entityType }} · {{ entityId }}
          <!--
            Named only under a world. In the default world there is one face and
            labelling it would be noise; under a world the label is the whole
            point, because the record on screen is face-specific.
          -->
          <span v-if="faceLabel" class="history-face"> · {{ faceLabel }} face </span>
        </p>
      </div>
      <RouterLink class="btn btn-secondary" :to="backTarget">Back to entity</RouterLink>
    </div>

    <div v-if="loading" class="loading-state">Loading version history…</div>
    <div v-else-if="unsupported" class="loading-state">
      Version history is not available for this deployment.
    </div>
    <div v-else-if="error" class="error-state">{{ error }}</div>
    <div v-else-if="worldFaceAbsent" class="loading-state">
      This entity has no {{ world }} face, so it has no history in that world.
    </div>
    <div v-else-if="versions.length === 0" class="loading-state">No versions recorded yet.</div>

    <div v-else class="history-layout">
      <!-- Timeline -->
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
              <span v-if="m.prev_id" class="timeline-note">renamed from {{ m.prev_id }}</span>
              <span v-if="m.triggered_by" class="timeline-note">{{ m.triggered_by }}</span>
              <!--
                Provenance sits BESIDE the op badge, never in place of it: a
                copy is still a create or an update. Rendered only when the
                server sent an origin — a direct edit gets nothing here.
              -->
              <span
                v-if="originLabel(m.origin)"
                class="timeline-origin"
                :data-origin-kind="m.origin?.kind"
                >{{ originLabel(m.origin) }}</span
              >
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
          <select v-model="baseSel" class="compare-select" @change="onBaseChange">
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
          <select v-model="targetSel" class="compare-select" @change="onTargetChange">
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
/* The face label is a quiet qualifier on the subtitle, not a badge: it names
   which record is on screen without competing with the entity id. */
.history-face {
  color: var(--muted-text);
}

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
/* Provenance reads as a quiet chip rather than plain prose, so the eye can
   find the copied rows down a long timeline without the row shouting. It is
   an annotation on the op badge, not a second op, so it is deliberately not
   coloured like one. */
.timeline-origin {
  color: var(--muted-text);
  font-size: 0.75em;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm, 4px);
  padding: 1px 6px;
  white-space: nowrap;
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
