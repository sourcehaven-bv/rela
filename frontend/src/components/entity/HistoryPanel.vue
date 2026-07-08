<script setup lang="ts">
import { ref, computed } from 'vue'
import { useUIStore } from '@/stores'
import { getErrorMessage, ApiError } from '@/api/errors'
import { listVersions, getVersion, restoreVersion, type VersionMeta } from '@/api/history'
import { lineDiff, propertyDiff, type DiffLine, type PropertyChange } from '@/utils/lineDiff'

// The panel diffs each historical version against the CURRENT entity, so it
// takes the current content + properties directly (rather than a full Entity),
// keeping it decoupled from the view/entity wire shape. canRestore is the
// server-computed update affordance — the panel never evaluates ACL itself.
const props = defineProps<{
  entityType: string
  entityId: string
  currentContent: string
  currentProperties: Record<string, unknown>
  canRestore: boolean
}>()

const emit = defineEmits<{ (e: 'restored'): void }>()

const uiStore = useUIStore()

const open = ref(false)
const loading = ref(false)
const loaded = ref(false)
const unsupported = ref(false)
const error = ref('')
const versions = ref<VersionMeta[]>([])

const selectedVersion = ref<number | null>(null)
const contentDiff = ref<DiffLine[]>([])
const propDiff = ref<PropertyChange[]>([])
const restoring = ref(false)

// Restore is authorized as an update (or create) — the parent passes the
// server-computed update affordance. false → hide the restore button.
const canRestore = computed(() => props.canRestore)

async function toggle() {
  open.value = !open.value
  if (open.value && !loaded.value) {
    await load()
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    versions.value = await listVersions(props.entityType, props.entityId)
    loaded.value = true
  } catch (err) {
    // 501 → backend has no versioning (e.g. filesystem deployment). Show a
    // muted "unavailable" note rather than an error banner.
    if (err instanceof ApiError && err.status === 501) {
      unsupported.value = true
    } else {
      error.value = getErrorMessage(err, 'Failed to load version history')
    }
  } finally {
    loading.value = false
  }
}

async function selectVersion(v: number) {
  selectedVersion.value = v
  error.value = ''
  try {
    const snap = await getVersion(props.entityType, props.entityId, v)
    // Diff the historical version (before) against the current entity (after),
    // so "add" reads as "present now, not in this version".
    contentDiff.value = lineDiff(snap.entity.content ?? '', props.currentContent)
    propDiff.value = propertyDiff(
      (snap.entity.properties ?? {}) as Record<string, unknown>,
      props.currentProperties,
    )
  } catch (err) {
    error.value = getErrorMessage(err, 'Failed to load version')
  }
}

async function restore(v: number) {
  if (restoring.value) return
  restoring.value = true
  error.value = ''
  try {
    await restoreVersion(props.entityType, props.entityId, v)
    uiStore.showToast('success', `Restored to version ${v}`)
    emit('restored')
    // Refresh the timeline — the restore itself creates a new version.
    await load()
    selectedVersion.value = null
  } catch (err) {
    uiStore.showToast('error', getErrorMessage(err, 'Restore failed'))
  } finally {
    restoring.value = false
  }
}

function principalLabel(m: VersionMeta): string {
  const user = m.principal.user || 'unknown'
  return m.principal.tool ? `${user} (${m.principal.tool})` : user
}

function formatWhen(iso: string): string {
  const d = new Date(iso)
  return isNaN(d.getTime()) ? iso : d.toLocaleString()
}
</script>

<template>
  <section class="history-panel">
    <button type="button" class="history-toggle" :aria-expanded="open" @click="toggle">
      <span>Version history</span>
      <span class="chevron" :class="{ open }">▸</span>
    </button>

    <div v-if="open" class="history-body">
      <p v-if="loading" class="history-muted">Loading…</p>
      <p v-else-if="unsupported" class="history-muted">
        Version history is not available for this deployment.
      </p>
      <p v-else-if="error" class="history-error">{{ error }}</p>
      <p v-else-if="loaded && versions.length === 0" class="history-muted">
        No versions recorded yet.
      </p>

      <ul v-else class="history-list">
        <li
          v-for="m in versions"
          :key="m.version"
          class="history-item"
          :class="{ selected: selectedVersion === m.version }"
        >
          <button type="button" class="history-item-btn" @click="selectVersion(m.version)">
            <span class="history-op" :data-op="m.op">{{ m.op }}</span>
            <span class="history-ver">v{{ m.version }}</span>
            <span class="history-who">{{ principalLabel(m) }}</span>
            <span class="history-when">{{ formatWhen(m.created_at) }}</span>
            <span v-if="m.prev_id" class="history-note">was {{ m.prev_id }}</span>
          </button>
          <button
            v-if="canRestore && m.op !== 'delete'"
            type="button"
            class="history-restore"
            :disabled="restoring"
            @click="restore(m.version)"
          >
            Restore
          </button>
        </li>
      </ul>

      <div v-if="selectedVersion !== null" class="history-diff">
        <h4>Changes since v{{ selectedVersion }} (→ current)</h4>

        <div v-if="propDiff.length" class="prop-diff">
          <div v-for="c in propDiff" :key="c.key" class="prop-row" :data-op="c.op">
            <code>{{ c.key }}</code>
            <span v-if="c.op === 'add'">added: {{ JSON.stringify(c.after) }}</span>
            <span v-else-if="c.op === 'del'">removed: {{ JSON.stringify(c.before) }}</span>
            <span v-else>{{ JSON.stringify(c.before) }} → {{ JSON.stringify(c.after) }}</span>
          </div>
        </div>

        <pre v-if="contentDiff.length" class="content-diff"><code
        ><span v-for="(l, i) in contentDiff" :key="i" class="diff-line" :data-op="l.op">{{
          l.op === 'add' ? '+ ' : l.op === 'del' ? '- ' : '  '
        }}{{ l.text }}
</span></code></pre>

        <p v-if="!propDiff.length && !contentDiff.some((l) => l.op !== 'equal')" class="history-muted">
          No differences from the current version.
        </p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.history-panel {
  margin-top: 1rem;
  border-top: 1px solid var(--border-color, #e2e2e2);
  padding-top: 0.5rem;
}
.history-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  background: none;
  border: none;
  cursor: pointer;
  font-weight: 600;
  padding: 0.4rem 0;
}
.chevron {
  transition: transform 0.15s;
}
.chevron.open {
  transform: rotate(90deg);
}
.history-muted {
  color: var(--text-muted, #888);
  font-size: 0.9em;
}
.history-error {
  color: var(--color-danger, #c0392b);
  font-size: 0.9em;
}
.history-list {
  list-style: none;
  padding: 0;
  margin: 0.5rem 0;
}
.history-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.history-item.selected {
  background: var(--surface-selected, #f0f6ff);
}
.history-item-btn {
  display: flex;
  gap: 0.6rem;
  align-items: baseline;
  flex: 1;
  background: none;
  border: none;
  text-align: left;
  cursor: pointer;
  padding: 0.3rem 0.2rem;
  font-size: 0.9em;
}
.history-op {
  text-transform: uppercase;
  font-size: 0.7em;
  font-weight: 700;
  color: var(--text-muted, #888);
  min-width: 3.5em;
}
.history-op[data-op='delete'] {
  color: var(--color-danger, #c0392b);
}
.history-op[data-op='rename'] {
  color: var(--color-warning, #b8860b);
}
.history-ver {
  font-variant-numeric: tabular-nums;
}
.history-who {
  flex: 1;
}
.history-when,
.history-note {
  color: var(--text-muted, #888);
  font-size: 0.85em;
}
.history-restore {
  font-size: 0.8em;
  cursor: pointer;
}
.history-diff {
  margin-top: 0.75rem;
  border-top: 1px dashed var(--border-color, #e2e2e2);
  padding-top: 0.5rem;
}
.prop-diff {
  margin-bottom: 0.5rem;
  font-size: 0.85em;
}
.prop-row {
  display: flex;
  gap: 0.5rem;
  padding: 0.1rem 0;
}
.prop-row[data-op='add'] {
  color: var(--color-success, #1e7e34);
}
.prop-row[data-op='del'] {
  color: var(--color-danger, #c0392b);
}
.content-diff {
  overflow-x: auto;
  background: var(--surface-code, #f7f7f7);
  padding: 0.5rem;
  border-radius: 4px;
  font-size: 0.85em;
}
.diff-line[data-op='add'] {
  background: var(--diff-add-bg, #e6ffed);
  color: var(--color-success, #1e7e34);
  display: block;
}
.diff-line[data-op='del'] {
  background: var(--diff-del-bg, #ffeef0);
  color: var(--color-danger, #c0392b);
  display: block;
}
</style>
