<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import type { Entity } from '@/types'
import { getEditFormId } from '@/types'
import { isInputFocused } from '@/utils/dom'
import { renderMarkdown, renderMermaidDiagrams } from '@/utils/markdown'
import Badge from '@/components/common/Badge.vue'
import DocumentsPanel from '@/components/entity/DocumentsPanel.vue'

const props = defineProps<{
  entityType: string
  entityId: string
}>()

const router = useRouter()
const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()
const uiStore = useUIStore()

function handleKeydown(e: KeyboardEvent) {
  if (isInputFocused()) return
  if (e.key === 'e' || e.key === 'E') {
    e.preventDefault()
    editEntity()
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
  } catch (err) {
    uiStore.error(`Failed to load ${props.entityType}`)
    console.error(err)
  } finally {
    loading.value = false
  }
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

function navigateToRelation(targetId: string) {
  // Try to determine target type from ID prefix
  for (const [typeName, typeDef] of schemaStore.entityTypeList) {
    // Check if the ID starts with this entity type's prefix
    if (typeDef.id_prefix && targetId.startsWith(typeDef.id_prefix)) {
      router.push(`/entity/${typeName}/${targetId}`)
      return
    }
  }
  // Fallback for manual IDs without prefix: try matching type name
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
      <header class="detail-header">
        <div class="header-info">
          <span class="entity-type-badge">{{ typeDef?.label || entityType }}</span>
          <h1>{{ entity.properties.title || entity.id }}</h1>
        </div>
        <div class="header-actions">
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
                @click="navigateToRelation(target)"
              >
                {{ target }}
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

.content-body :deep(ul) {
  margin: 0 0 12px;
  padding-left: 24px;
}

.content-body :deep(li) {
  margin-bottom: 4px;
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
</style>
