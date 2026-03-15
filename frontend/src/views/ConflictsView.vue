<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { getConflicts, getConflictDetail, resolveConflict, type ConflictItem, type ConflictDetail } from '@/api'
import { useGitStore } from '@/stores'

const router = useRouter()
const gitStore = useGitStore()

// List view state
const conflicts = ref<ConflictItem[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

// Detail view state
const selectedPath = ref<string | null>(null)
const detail = ref<ConflictDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref<string | null>(null)
const resolving = ref(false)

// Form state for resolution
const propertyChoices = ref<Record<string, 'ours' | 'theirs'>>({})
const contentChoice = ref<'ours' | 'theirs' | 'manual'>('ours')
const manualContent = ref('')

const hasConflicts = computed(() => conflicts.value.length > 0)
const showingDetail = computed(() => selectedPath.value !== null)

async function loadConflicts() {
  loading.value = true
  error.value = null
  try {
    const result = await getConflicts()
    conflicts.value = result.conflicts
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load conflicts'
  } finally {
    loading.value = false
  }
}

async function selectConflict(path: string) {
  selectedPath.value = path
  detailLoading.value = true
  detailError.value = null

  // Reset form state
  propertyChoices.value = {}
  contentChoice.value = 'ours'
  manualContent.value = ''

  try {
    detail.value = await getConflictDetail(path)

    // Initialize property choices - default to 'ours'
    for (const diff of detail.value.property_diffs) {
      if (!diff.is_same) {
        propertyChoices.value[diff.property] = 'ours'
      }
    }

    // Initialize manual content
    if (!detail.value.content_same && detail.value.content_ours) {
      manualContent.value = detail.value.content_ours
    }
  } catch (err) {
    detailError.value = err instanceof Error ? err.message : 'Failed to load conflict details'
  } finally {
    detailLoading.value = false
  }
}

function backToList() {
  selectedPath.value = null
  detail.value = null
}

function selectAllSide(side: 'ours' | 'theirs') {
  if (!detail.value) return
  for (const diff of detail.value.property_diffs) {
    if (!diff.is_same) {
      propertyChoices.value[diff.property] = side
    }
  }
  contentChoice.value = side
}

async function applyResolution() {
  if (!selectedPath.value) return

  resolving.value = true
  try {
    await resolveConflict({
      path: selectedPath.value,
      property_choices: propertyChoices.value,
      content_choice: contentChoice.value,
      manual_content: contentChoice.value === 'manual' ? manualContent.value : undefined,
    })

    // Refresh the list and git status
    await Promise.all([
      loadConflicts(),
      gitStore.fetchStatus(),
    ])

    // Go back to list if no more conflicts
    backToList()
  } catch (err) {
    detailError.value = err instanceof Error ? err.message : 'Failed to resolve conflict'
  } finally {
    resolving.value = false
  }
}

onMounted(() => {
  loadConflicts()
})
</script>

<template>
  <div class="conflicts-view">
    <!-- List View -->
    <template v-if="!showingDetail">
      <div class="page-header">
        <div>
          <h2>Merge Conflicts</h2>
          <p>Files with unresolved git conflicts</p>
        </div>
        <button class="btn btn-secondary" @click="router.push('/')">
          Back to Dashboard
        </button>
      </div>

      <div v-if="loading" class="loading-state">
        Loading conflicts...
      </div>

      <div v-else-if="error" class="error-state">
        {{ error }}
        <button @click="loadConflicts">Retry</button>
      </div>

      <template v-else-if="hasConflicts">
        <div class="conflict-summary">
          <span class="conflict-chip">
            {{ conflicts.length }} file{{ conflicts.length !== 1 ? 's' : '' }} with conflicts
          </span>
        </div>

        <div class="card">
          <table class="conflicts-table">
            <thead>
              <tr>
                <th>File</th>
                <th>Type</th>
                <th>Conflicts</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="conflict in conflicts"
                :key="conflict.path"
                class="conflict-row"
                @click="selectConflict(conflict.path)"
              >
                <td>
                  <div class="conflict-path">{{ conflict.path }}</div>
                  <div v-if="conflict.entity_id" class="conflict-id">{{ conflict.entity_id }}</div>
                </td>
                <td>
                  <span v-if="conflict.entity_type" class="badge badge-gray">
                    {{ conflict.entity_type }}
                  </span>
                  <span v-else class="badge badge-purple">relation</span>
                </td>
                <td>
                  <span class="badge badge-orange">{{ conflict.marker_count }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <div v-else class="conflict-empty">
        <div class="conflict-empty-icon">OK</div>
        <h3>No conflicts detected</h3>
        <p>All entity and relation files are clean.</p>
      </div>
    </template>

    <!-- Detail View -->
    <template v-else>
      <div class="page-header">
        <div>
          <h2>Resolve Conflict</h2>
          <p>{{ selectedPath }}</p>
        </div>
        <button class="btn btn-secondary" @click="backToList">
          Back to Conflicts
        </button>
      </div>

      <div v-if="detailLoading" class="loading-state">
        Loading conflict details...
      </div>

      <div v-else-if="detailError" class="error-state">
        {{ detailError }}
        <button @click="selectConflict(selectedPath!)">Retry</button>
      </div>

      <template v-else-if="detail">
        <div class="resolve-actions-top">
          <button class="btn btn-secondary" @click="selectAllSide('ours')">
            Select All Ours <kbd>O</kbd>
          </button>
          <button class="btn btn-secondary" @click="selectAllSide('theirs')">
            Select All Theirs <kbd>T</kbd>
          </button>
        </div>

        <div class="card resolve-card">
          <h3>Properties</h3>
          <table class="resolve-table">
            <thead>
              <tr>
                <th>Property</th>
                <th>Ours (HEAD)</th>
                <th>Theirs</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="diff in detail.property_diffs"
                :key="diff.property"
                :class="diff.is_same ? 'resolve-same' : 'resolve-diff'"
              >
                <td class="resolve-prop-name">{{ diff.property }}</td>
                <td
                  v-if="diff.is_same"
                  class="resolve-value resolve-value-same"
                  colspan="2"
                >
                  {{ diff.ours_value || 'empty' }}
                </td>
                <template v-else>
                  <td
                    class="resolve-value resolve-value-selectable"
                    :class="{ 'resolve-value-selected': propertyChoices[diff.property] === 'ours' }"
                    @click="propertyChoices[diff.property] = 'ours'"
                  >
                    <span class="resolve-value-text">{{ diff.ours_value || 'empty' }}</span>
                  </td>
                  <td
                    class="resolve-value resolve-value-selectable"
                    :class="{ 'resolve-value-selected': propertyChoices[diff.property] === 'theirs' }"
                    @click="propertyChoices[diff.property] = 'theirs'"
                  >
                    <span class="resolve-value-text">{{ diff.theirs_value || 'empty' }}</span>
                  </td>
                </template>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="!detail.content_same" class="card resolve-card">
          <h3>Content</h3>
          <div class="resolve-content-choice">
            <label>
              <input v-model="contentChoice" type="radio" value="ours" />
              Use Ours
            </label>
            <label>
              <input v-model="contentChoice" type="radio" value="theirs" />
              Use Theirs
            </label>
            <label>
              <input v-model="contentChoice" type="radio" value="manual" />
              Edit Manually
            </label>
          </div>
          <div class="resolve-content-compare">
            <div class="resolve-content-side">
              <div class="resolve-content-label">Ours (HEAD)</div>
              <pre class="resolve-content-pre">{{ detail.content_ours }}</pre>
            </div>
            <div class="resolve-content-side">
              <div class="resolve-content-label">Theirs</div>
              <pre class="resolve-content-pre">{{ detail.content_theirs }}</pre>
            </div>
          </div>
          <div v-if="contentChoice === 'manual'" class="resolve-manual-edit">
            <label>Manual Content</label>
            <textarea v-model="manualContent" rows="10"/>
          </div>
        </div>

        <div class="resolve-actions-bottom">
          <button
            class="btn btn-primary"
            :disabled="resolving"
            @click="applyResolution"
          >
            {{ resolving ? 'Applying...' : 'Apply Resolution' }}
          </button>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.conflicts-view {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 24px;
}

.page-header h2 {
  margin: 0 0 4px 0;
}

.page-header p {
  margin: 0;
  opacity: 0.7;
}

.loading-state,
.error-state {
  padding: 40px;
  text-align: center;
  background: var(--card-bg, #fff);
  border-radius: 8px;
  border: 1px solid var(--border-color, #e5e7eb);
}

.error-state button {
  margin-top: 12px;
}

.conflict-summary {
  margin-bottom: 16px;
}

.conflict-chip {
  display: inline-block;
  padding: 6px 12px;
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
  border-radius: 16px;
  font-weight: 500;
}

.card {
  background: var(--card-bg, #fff);
  border-radius: 8px;
  border: 1px solid var(--border-color, #e5e7eb);
  overflow: hidden;
}

.conflicts-table {
  width: 100%;
  border-collapse: collapse;
}

.conflicts-table th,
.conflicts-table td {
  padding: 12px 16px;
  text-align: left;
  border-bottom: 1px solid var(--border-color, #e5e7eb);
}

.conflicts-table th {
  background: var(--header-bg, #f9fafb);
  font-weight: 500;
}

.conflict-row {
  cursor: pointer;
  transition: background 0.15s ease;
}

.conflict-row:hover {
  background: var(--hover-bg, #f3f4f6);
}

.conflict-path {
  font-family: monospace;
}

.conflict-id {
  font-size: 12px;
  opacity: 0.6;
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.badge-gray {
  background: #6b7280;
  color: white;
}

.badge-purple {
  background: #8b5cf6;
  color: white;
}

.badge-orange {
  background: #f59e0b;
  color: white;
}

.conflict-empty {
  text-align: center;
  padding: 60px 24px;
  background: var(--card-bg, #fff);
  border-radius: 8px;
  border: 1px solid var(--border-color, #e5e7eb);
}

.conflict-empty-icon {
  width: 64px;
  height: 64px;
  margin: 0 auto 16px;
  background: #10b981;
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  font-weight: bold;
}

.conflict-empty h3 {
  margin: 0 0 8px 0;
}

.conflict-empty p {
  margin: 0;
  opacity: 0.7;
}

/* Resolution UI */
.resolve-actions-top {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.resolve-card {
  margin-bottom: 16px;
  padding: 16px;
}

.resolve-card h3 {
  margin: 0 0 16px 0;
  font-size: 16px;
}

.resolve-table {
  width: 100%;
  border-collapse: collapse;
}

.resolve-table th,
.resolve-table td {
  padding: 8px 12px;
  text-align: left;
  border: 1px solid var(--border-color, #e5e7eb);
}

.resolve-table th {
  background: var(--header-bg, #f9fafb);
}

.resolve-prop-name {
  font-family: monospace;
  font-weight: 500;
}

.resolve-same {
  opacity: 0.6;
}

.resolve-value-same {
  font-style: italic;
}

.resolve-value-selectable {
  cursor: pointer;
  transition: all 0.15s ease;
}

.resolve-value-selectable:hover {
  background: rgba(99, 102, 241, 0.1);
}

.resolve-value-selected {
  background: rgba(99, 102, 241, 0.2);
  border-color: #6366f1;
}

.resolve-content-choice {
  display: flex;
  gap: 24px;
  margin-bottom: 16px;
}

.resolve-content-choice label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.resolve-content-compare {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.resolve-content-side {
  min-width: 0;
}

.resolve-content-label {
  font-weight: 500;
  margin-bottom: 8px;
}

.resolve-content-pre {
  background: var(--code-bg, #f3f4f6);
  padding: 12px;
  border-radius: 4px;
  overflow-x: auto;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 300px;
  overflow-y: auto;
}

.resolve-manual-edit {
  margin-top: 16px;
}

.resolve-manual-edit label {
  display: block;
  font-weight: 500;
  margin-bottom: 8px;
}

.resolve-manual-edit textarea {
  width: 100%;
  font-family: monospace;
  padding: 12px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 4px;
  resize: vertical;
}

.resolve-actions-bottom {
  margin-top: 24px;
  display: flex;
  justify-content: flex-end;
}

.btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s ease;
}

.btn-primary {
  background: #6366f1;
  color: white;
}

.btn-primary:hover {
  background: #5558e6;
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-secondary {
  background: var(--card-bg, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  color: inherit;
}

.btn-secondary:hover {
  background: var(--hover-bg, #f3f4f6);
}

kbd {
  background: rgba(0, 0, 0, 0.1);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 11px;
  margin-left: 6px;
}
</style>
