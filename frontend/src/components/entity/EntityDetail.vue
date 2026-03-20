<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import type { Entity, Command, ListParams } from '@/types'
import { getEditFormId } from '@/types'
import { isInputFocused } from '@/utils/dom'
import { renderMarkdown, renderMermaidDiagrams } from '@/utils/markdown'
import { getCommands } from '@/api'
import Badge from '@/components/common/Badge.vue'
import DocumentsPanel from '@/components/entity/DocumentsPanel.vue'

const props = defineProps<{
  entityType: string
  entityId: string
}>()

const router = useRouter()
const route = useRoute()
const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()
const uiStore = useUIStore()

// Scope navigation types
interface ScopeNav {
  backUrl: string
  prevId: string | null
  nextId: string | null
  current: number
  total: number
  label: string
}

// Scope navigation state
const scopeNav = ref<ScopeNav | null>(null)

function handleKeydown(e: KeyboardEvent) {
  if (isInputFocused()) return
  if (e.key === 'e' || e.key === 'E') {
    e.preventDefault()
    editEntity()
  }
  // Scope navigation: j/k (left/right on keyboard) or arrow keys
  if ((e.key === 'j' || e.key === 'ArrowUp') && scopeNav.value?.prevId) {
    e.preventDefault()
    navigateScope('prev')
  }
  if ((e.key === 'k' || e.key === 'ArrowDown') && scopeNav.value?.nextId) {
    e.preventDefault()
    navigateScope('next')
  }
  // Escape to go back
  if (e.key === 'Escape' && scopeNav.value) {
    e.preventDefault()
    router.push(scopeNav.value.backUrl)
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})

// State
const entity = ref<Entity | null>(null)
const loading = ref(true)
const deleting = ref(false)
const showDeleteConfirm = ref(false)

// Commands state
const commands = ref<Command[]>([])
const showCommandModal = ref(false)
const activeCommand = ref<Command | null>(null)
const commandRunning = ref(false)
const commandOutput = ref<Array<{ type: 'text' | 'file'; text?: string; path?: string; label?: string }>>([])
const commandSuccess = ref<boolean | null>(null)

// Computed
const typeDef = computed(() => schemaStore.getEntityType(props.entityType))
const editFormId = computed(() => getEditFormId(schemaStore, props.entityType))

const properties = computed(() => {
  if (!entity.value || !typeDef.value) return []

  return Object.entries(typeDef.value.properties).map(([name, def]) => ({
    name,
    label: name.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase()),
    value: entity.value?.properties[name],
    type: def.type,
    values: def.values,
  }))
})

const relations = computed(() => {
  if (!entity.value?.relations) return []

  return Object.entries(entity.value.relations).map(([type, targets]) => ({
    type,
    targets,
    label:
      schemaStore.getRelationType(type)?.label ||
      type.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase()),
  }))
})

const contentRef = ref<HTMLElement | null>(null)

const renderedContent = computed(() => {
  if (!entity.value?.content) return ''
  return renderMarkdown(entity.value.content)
})

// Render mermaid diagrams after content is mounted
watch(renderedContent, async () => {
  await nextTick()
  if (contentRef.value) {
    await renderMermaidDiagrams(contentRef.value)
  }
})

// Methods
async function loadEntity() {
  loading.value = true
  try {
    entity.value = await entitiesStore.fetchEntity(props.entityType, props.entityId, true)
    await Promise.all([loadCommands(), loadScopeNav()])
  } catch (err) {
    uiStore.error(`Failed to load ${props.entityType}`)
    console.error(err)
  } finally {
    loading.value = false
  }
}

// Scope navigation
async function loadScopeNav() {
  const fromListId = route.query.from as string | undefined
  if (!fromListId) {
    scopeNav.value = null
    return
  }

  const listConfig = schemaStore.getList(fromListId)
  if (!listConfig) {
    scopeNav.value = null
    return
  }

  try {
    // Build query params matching what EntityList uses
    const params: ListParams = {
      per_page: 1000, // Fetch all to get accurate position
    }

    // Add sort from query params or list default
    const sort = route.query.sort as string | undefined
    if (sort) {
      params.sort = sort
    } else if (listConfig.default_sort?.length) {
      params.sort = listConfig.default_sort
        .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
        .join(',')
    }

    // Add pre-configured filters from list config
    const operatorMap: Record<string, string> = {
      '!=': 'ne',
      '=': 'eq',
      '>': 'gt',
      '>=': 'gte',
      '<': 'lt',
      '<=': 'lte',
      '~': 'contains',
    }
    for (const filter of listConfig.filters || []) {
      if (filter.operator && filter.value) {
        const apiOp = operatorMap[filter.operator] || 'eq'
        params[`filter[${filter.property}][${apiOp}]`] = filter.value
      }
    }

    // Add user-selected filters from query
    for (const [key, value] of Object.entries(route.query)) {
      if (key.startsWith('filter_') && value) {
        const prop = key.replace('filter_', '')
        params[`filter[${prop}]`] = value as string
      }
    }

    const result = await entitiesStore.fetchList(listConfig.entity, params)
    const ids = result.data.map((e) => e.id)
    const currentIndex = ids.indexOf(props.entityId)

    if (currentIndex === -1) {
      scopeNav.value = null
      return
    }

    scopeNav.value = {
      backUrl: `/list/${fromListId}`,
      prevId: currentIndex > 0 ? ids[currentIndex - 1] : null,
      nextId: currentIndex < ids.length - 1 ? ids[currentIndex + 1] : null,
      current: currentIndex + 1,
      total: ids.length,
      label: listConfig.title || fromListId,
    }
  } catch {
    scopeNav.value = null
  }
}

function navigateScope(direction: 'prev' | 'next') {
  if (!scopeNav.value) return

  const targetId = direction === 'prev' ? scopeNav.value.prevId : scopeNav.value.nextId
  if (!targetId) return

  // Preserve all query params for consistent navigation
  router.push({
    path: `/entity/${props.entityType}/${targetId}`,
    query: route.query,
  })
}

function editEntity() {
  if (editFormId.value) {
    router.push(`/form/${editFormId.value}/${props.entityId}`)
  } else {
    uiStore.error('No edit form configured for this entity type')
  }
}

async function deleteEntity() {
  if (!entity.value) return

  deleting.value = true
  try {
    await entitiesStore.remove(props.entityType, props.entityId)
    uiStore.success('Entity deleted successfully')
    router.push(`/list/${props.entityType}s`)
  } catch (err) {
    uiStore.error('Failed to delete entity')
    console.error(err)
  } finally {
    deleting.value = false
    showDeleteConfirm.value = false
  }
}

// Commands
async function loadCommands() {
  try {
    commands.value = await getCommands({
      pageType: 'entity',
      entityType: props.entityType,
    })
  } catch (err) {
    console.error('Failed to load commands:', err)
    commands.value = []
  }
}

async function runCommand(cmd: Command) {
  if (cmd.confirm && !confirm(cmd.confirm)) {
    return
  }

  activeCommand.value = cmd
  commandRunning.value = true
  commandOutput.value = []
  commandSuccess.value = null
  showCommandModal.value = true

  const params = new URLSearchParams()
  params.set('entity_id', props.entityId)

  const url = `/api/command/${cmd.id}?${params.toString()}`

  try {
    const response = await fetch(url)
    if (!response.ok) {
      const text = await response.text()
      throw new Error(text || response.statusText)
    }

    const reader = response.body?.getReader()
    if (!reader) throw new Error('No response body')

    const decoder = new TextDecoder()
    let buffer = ''
    let currentEvent = 'message'

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (line.startsWith('event: ')) {
          currentEvent = line.substring(7).trim()
        } else if (line.startsWith('data: ')) {
          const data = line.substring(6)
          processSSEEvent(currentEvent, data, cmd)
          currentEvent = 'message'
        }
      }
    }

    // Stream ended without done event
    if (commandSuccess.value === null) {
      commandSuccess.value = true
      commandRunning.value = false
    }
  } catch (err) {
    commandOutput.value.push({ type: 'text', text: `Error: ${err instanceof Error ? err.message : 'Connection failed'}` })
    commandSuccess.value = false
    commandRunning.value = false
  }
}

function processSSEEvent(eventType: string, rawData: string, cmd: Command) {
  try {
    const data = JSON.parse(rawData)
    switch (eventType) {
      case 'message':
        commandOutput.value.push({ type: 'text', text: data.text || '' })
        break
      case 'file':
        commandOutput.value.push({
          type: 'file',
          path: data.path,
          label: data.label || data.path.split('/').pop() || 'File',
        })
        if (cmd.auto_open !== false && data.path) {
          // Auto-open file via API
          fetch(`/api/open-file?path=${encodeURIComponent(data.path)}&action=open`, { method: 'POST' })
        }
        break
      case 'error':
        commandOutput.value.push({ type: 'text', text: `Error: ${data.text || 'Command error'}` })
        commandSuccess.value = false
        commandRunning.value = false
        break
      case 'done':
        commandSuccess.value = !!data.success
        commandRunning.value = false
        break
    }
  } catch {
    // Ignore parse errors
  }
}

function openFile(path: string) {
  fetch(`/api/open-file?path=${encodeURIComponent(path)}&action=open`, { method: 'POST' })
}

function revealFile(path: string) {
  fetch(`/api/open-file?path=${encodeURIComponent(path)}&action=reveal`, { method: 'POST' })
}

function closeCommandModal() {
  showCommandModal.value = false
  activeCommand.value = null
  commandRunning.value = false
}

function getRelationTitle(targetId: string): string {
  const included = entity.value?.included?.[targetId]
  if (included) {
    // Use title property if available, otherwise fall back to ID
    const title = included.properties?.title
    if (title && typeof title === 'string') {
      return `${title} (${targetId})`
    }
  }
  return targetId
}

function navigateToRelation(relationType: string, targetId: string) {
  // First, try to use the relation type definition to determine target type
  const relDef = schemaStore.getRelationType(relationType)
  if (relDef && relDef.to.length > 0) {
    // If the relation type specifies a single target type, use it
    if (relDef.to.length === 1) {
      router.push(`/entity/${relDef.to[0]}/${targetId}`)
      return
    }
    // Multiple possible target types - try to match by ID prefix within those types
    for (const typeName of relDef.to) {
      const typeDef = schemaStore.getEntityType(typeName)
      if (typeDef?.id_prefix && targetId.startsWith(typeDef.id_prefix)) {
        router.push(`/entity/${typeName}/${targetId}`)
        return
      }
    }
    // No prefix match - just use the first valid target type
    router.push(`/entity/${relDef.to[0]}/${targetId}`)
    return
  }

  // Fallback: try to determine target type from ID prefix (for unknown relation types)
  for (const [typeName, typeDef] of schemaStore.entityTypeList) {
    if (typeDef.id_prefix && targetId.startsWith(typeDef.id_prefix)) {
      router.push(`/entity/${typeName}/${targetId}`)
      return
    }
  }
  // Last resort: try matching type name from ID prefix
  const prefix = targetId.split('-')[0].toUpperCase()
  for (const [typeName] of schemaStore.entityTypeList) {
    if (typeName.toUpperCase().startsWith(prefix)) {
      router.push(`/entity/${typeName}/${targetId}`)
      return
    }
  }
  uiStore.warning(`Could not determine entity type for ${targetId}`)
}

function formatValue(value: unknown, type: string): string {
  if (value === null || value === undefined) return '-'
  if (type === 'date' && typeof value === 'string') {
    return new Date(value).toLocaleDateString()
  }
  if (type === 'boolean') {
    return value ? 'Yes' : 'No'
  }
  if (Array.isArray(value)) {
    return value.join(', ')
  }
  return String(value)
}

function isEnumProperty(prop: { type: string; values?: string[] }): boolean {
  return prop.type === 'enum' || (prop.values?.length ?? 0) > 0
}

// Watchers
watch(
  () => [props.entityType, props.entityId],
  () => loadEntity()
)

// Lifecycle
onMounted(() => loadEntity())
</script>

<template>
  <div class="entity-detail">
    <div v-if="loading" class="loading-state">
      <div class="spinner"/>
      <span>Loading...</span>
    </div>

    <template v-else-if="entity">
      <!-- Scope Navigation Bar -->
      <div v-if="scopeNav" class="scope-nav">
        <router-link :to="scopeNav.backUrl" class="scope-nav-btn">
          Back <kbd>Esc</kbd>
        </router-link>
        <button
          v-if="scopeNav.prevId"
          class="scope-nav-btn"
          @click="navigateScope('prev')"
        >
          ← Prev <kbd>J</kbd>
        </button>
        <span v-else class="scope-nav-btn disabled">← Prev</span>
        <span class="scope-nav-progress">[{{ scopeNav.current }}/{{ scopeNav.total }}]</span>
        <span class="scope-nav-label">{{ scopeNav.label }}</span>
        <button
          v-if="scopeNav.nextId"
          class="scope-nav-btn"
          @click="navigateScope('next')"
        >
          Next → <kbd>K</kbd>
        </button>
        <span v-else class="scope-nav-btn disabled">Next →</span>
      </div>

      <header class="detail-header">
        <div class="header-info">
          <span class="entity-type-badge">{{ typeDef?.label || entityType }}</span>
          <h1>{{ entity.properties.title || entity.id }}</h1>
        </div>
        <div class="header-actions">
          <button
            v-for="cmd in commands"
            :key="cmd.id"
            class="btn btn-command"
            @click="runCommand(cmd)"
          >
            {{ cmd.label }}
          </button>
          <button v-if="editFormId" class="btn btn-secondary" @click="editEntity">
            Edit <kbd>E</kbd>
          </button>
          <button class="btn btn-danger" @click="showDeleteConfirm = true">
            Delete
          </button>
        </div>
      </header>

      <!-- Properties Section -->
      <section class="detail-section">
        <h2>Properties</h2>
        <div class="properties-grid">
          <div v-for="prop in properties" :key="prop.name" class="property-item">
            <dt>{{ prop.label }}</dt>
            <dd>
              <Badge
                v-if="isEnumProperty(prop)"
                :value="String(prop.value || '')"
                :property="prop.name"
                :entity-type="typeDef"
              />
              <span v-else>{{ formatValue(prop.value, prop.type) }}</span>
            </dd>
          </div>
        </div>
      </section>

      <!-- Relations Section -->
      <section v-if="relations.length" class="detail-section">
        <h2>Relations</h2>
        <div class="relations-list">
          <div v-for="rel in relations" :key="rel.type" class="relation-group">
            <h3>{{ rel.label }}</h3>
            <div class="relation-targets">
              <button
                v-for="target in rel.targets"
                :key="target"
                class="relation-link"
                @click="navigateToRelation(rel.type, target)"
              >
                {{ getRelationTitle(target) }}
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- Documents Section -->
      <DocumentsPanel :entity-type="entityType" :entity-id="entityId" />

      <!-- Content Section -->
      <section v-if="entity.content" class="detail-section">
        <h2>Content</h2>
        <div ref="contentRef" class="content-body" v-html="renderedContent"/>
      </section>

      <!-- Delete Confirmation Modal -->
      <div v-if="showDeleteConfirm" class="modal-overlay" @click.self="showDeleteConfirm = false">
        <div class="modal">
          <h3>Delete Entity?</h3>
          <p>
            Are you sure you want to delete <strong>{{ entity.id }}</strong>?
            This action cannot be undone.
          </p>
          <div class="modal-actions">
            <button
              class="btn btn-secondary"
              :disabled="deleting"
              @click="showDeleteConfirm = false"
            >
              Cancel
            </button>
            <button
              class="btn btn-danger"
              :disabled="deleting"
              @click="deleteEntity"
            >
              {{ deleting ? 'Deleting...' : 'Delete' }}
            </button>
          </div>
        </div>
      </div>

      <!-- Command Execution Modal -->
      <div v-if="showCommandModal" class="modal-overlay" @click.self="!commandRunning && closeCommandModal()">
        <div class="modal command-modal">
          <div class="command-header">
            <h3>{{ activeCommand?.label }}</h3>
            <span v-if="commandRunning" class="command-status running">Running...</span>
            <span v-else-if="commandSuccess === true" class="command-status success">Completed</span>
            <span v-else-if="commandSuccess === false" class="command-status error">Failed</span>
          </div>
          <div class="command-output">
            <template v-if="commandOutput.length === 0">
              <div class="output-line">Starting...</div>
            </template>
            <template v-for="(item, idx) in commandOutput" :key="idx">
              <div v-if="item.type === 'text'" class="output-line">{{ item.text }}</div>
              <div v-else-if="item.type === 'file'" class="output-file">
                <span class="file-icon">📄</span>
                <span class="file-label">{{ item.label }}</span>
                <button class="file-btn" @click="openFile(item.path!)">Open</button>
                <button class="file-btn" @click="revealFile(item.path!)">Reveal</button>
              </div>
            </template>
          </div>
          <div class="modal-actions">
            <button
              class="btn btn-secondary"
              :disabled="commandRunning"
              @click="closeCommandModal"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </template>

    <div v-else class="error-state">
      <h2>Entity not found</h2>
      <p>{{ entityType }} "{{ entityId }}" could not be found.</p>
      <router-link :to="`/list/${entityType}s`" class="btn btn-secondary">
        Back to list
      </router-link>
    </div>
  </div>
</template>

<style scoped>
.entity-detail {
  max-width: 900px;
}

.loading-state,
.error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  gap: 16px;
  color: #64748b;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 24px;
}

.header-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.entity-type-badge {
  display: inline-block;
  padding: 4px 10px;
  background: var(--accent-color);
  color: white;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
}

.header-info h1 {
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s;
  text-decoration: none;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-secondary {
  background: var(--border-color, #e2e8f0);
  color: var(--text-color, #1e293b);
}

.btn-secondary:hover:not(:disabled) {
  background: #cbd5e1;
}

.btn-danger {
  background: var(--error-color, #ef4444);
  color: white;
}

.btn-danger:hover:not(:disabled) {
  background: #dc2626;
}

.detail-section {
  background: white;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  padding: 24px;
  margin-bottom: 24px;
}

.detail-section h2 {
  margin: 0 0 16px;
  font-size: 18px;
  color: #374151;
}

.properties-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
}

.property-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.property-item dt {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  color: #64748b;
}

.property-item dd {
  margin: 0;
  font-size: 14px;
  color: #1e293b;
}

.relations-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.relation-group h3 {
  margin: 0 0 8px;
  font-size: 14px;
  font-weight: 600;
  color: #64748b;
  text-transform: capitalize;
}

.relation-targets {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.relation-link {
  padding: 6px 12px;
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
  border-radius: 4px;
  font-size: 13px;
  font-family: monospace;
  color: var(--accent-color);
  cursor: pointer;
  transition: all 0.15s;
}

.relation-link:hover {
  background: #e2e8f0;
  border-color: var(--accent-color);
}

.content-body {
  font-size: 15px;
  line-height: 1.7;
  color: #374151;
}

.content-body :deep(h1),
.content-body :deep(h2),
.content-body :deep(h3) {
  margin: 24px 0 12px;
  color: #1e293b;
}

.content-body :deep(h1) {
  font-size: 24px;
}

.content-body :deep(h2) {
  font-size: 20px;
}

.content-body :deep(h3) {
  font-size: 16px;
}

.content-body :deep(p) {
  margin: 0 0 12px;
}

.content-body :deep(ul),
.content-body :deep(ol) {
  margin: 0 0 16px;
  padding-left: 28px;
}

.content-body :deep(ol) {
  list-style-type: decimal;
}

.content-body :deep(li) {
  margin-bottom: 6px;
  line-height: 1.6;
}

.content-body :deep(li::marker) {
  color: #64748b;
  font-weight: 500;
}

.content-body :deep(code) {
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}

.content-body :deep(input[type="checkbox"]) {
  margin-right: 8px;
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: white;
  border-radius: 12px;
  padding: 24px;
  max-width: 400px;
  width: 90%;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1);
}

.modal h3 {
  margin: 0 0 12px;
}

.modal p {
  margin: 0 0 24px;
  color: #64748b;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.btn-command {
  background: var(--accent-color, #3b82f6);
  color: white;
}

.btn-command:hover:not(:disabled) {
  background: #2563eb;
}

.command-modal {
  max-width: 600px;
  width: 90%;
}

.command-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.command-header h3 {
  margin: 0;
  flex: 1;
}

.command-status {
  font-size: 12px;
  font-weight: 600;
  padding: 4px 8px;
  border-radius: 4px;
}

.command-status.running {
  background: #fef3c7;
  color: #92400e;
}

.command-status.success {
  background: #d1fae5;
  color: #065f46;
}

.command-status.error {
  background: #fee2e2;
  color: #991b1b;
}

.command-output {
  background: #1e293b;
  border-radius: 6px;
  padding: 16px;
  max-height: 400px;
  overflow: auto;
  margin-bottom: 16px;
}

.command-output pre {
  margin: 0;
}

.command-output code {
  color: #e2e8f0;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.output-line {
  color: #e2e8f0;
  font-size: 13px;
  line-height: 1.6;
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-word;
}

.output-file {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  margin: 4px 0;
  background: #334155;
  border-radius: 4px;
}

.file-icon {
  font-size: 14px;
}

.file-label {
  flex: 1;
  color: #e2e8f0;
  font-size: 13px;
  font-family: monospace;
}

.file-btn {
  padding: 4px 10px;
  background: #475569;
  color: #e2e8f0;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  transition: background 0.15s;
}

.file-btn:hover {
  background: #64748b;
}

/* Scope Navigation Bar */
.scope-nav {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: #f8fafc;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  margin-bottom: 20px;
}

.scope-nav-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 13px;
  color: var(--text-color, #1e293b);
  cursor: pointer;
  text-decoration: none;
  transition: all 0.15s;
}

.scope-nav-btn:hover:not(.disabled) {
  background: #f1f5f9;
  border-color: var(--accent-color, #6366f1);
}

.scope-nav-btn.disabled {
  color: #94a3b8;
  cursor: not-allowed;
  background: #f8fafc;
}

.scope-nav-btn kbd {
  padding: 2px 5px;
  font-size: 10px;
  background: #e2e8f0;
  border-radius: 3px;
  font-family: monospace;
}

.scope-nav-progress {
  font-size: 13px;
  font-weight: 600;
  color: #475569;
  font-family: monospace;
}

.scope-nav-label {
  flex: 1;
  font-size: 13px;
  color: #64748b;
}
</style>
