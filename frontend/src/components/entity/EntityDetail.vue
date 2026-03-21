<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { useScopeNavigation } from '@/composables'
import type { Entity, Command } from '@/types'
import { getEditFormId } from '@/types'
import { isInputFocused } from '@/utils/dom'
import { renderMarkdown, renderMermaidDiagrams, getCheckboxStats } from '@/utils/markdown'
import { formatValue, isEnumProperty } from '@/utils/format'
import { getCommands, toggleCheckbox } from '@/api'
import Badge from '@/components/common/Badge.vue'
import DocumentsPanel from '@/components/entity/DocumentsPanel.vue'
import CommandModal from '@/components/entity/CommandModal.vue'

const props = defineProps<{
  entityType: string
  entityId: string
}>()

const router = useRouter()
const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()
const uiStore = useUIStore()

// Scope navigation
const { scopeNav, loadScopeNav, navigateScope, goBack } = useScopeNavigation(
  () => props.entityType,
  () => props.entityId
)

// Command modal ref
const commandModalRef = ref<InstanceType<typeof CommandModal> | null>(null)

function handleKeydown(e: KeyboardEvent) {
  if (isInputFocused()) return
  if (e.key === 'e' || e.key === 'E') {
    e.preventDefault()
    editEntity()
  }
  // Scope navigation: j/k or arrow keys
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
    goBack()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
})

// State
const entity = ref<Entity | null>(null)
const loading = ref(true)
const deleting = ref(false)
const showDeleteConfirm = ref(false)

// Commands state
const commands = ref<Command[]>([])

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

const checkboxStats = computed(() => {
  if (!entity.value?.content) return null
  return getCheckboxStats(entity.value.content)
})

// Render mermaid diagrams and setup checkbox handlers after content is mounted
watch(renderedContent, async () => {
  await nextTick()
  if (contentRef.value) {
    await renderMermaidDiagrams(contentRef.value)
    setupCheckboxHandlers()
  }
})

function setupCheckboxHandlers() {
  if (!contentRef.value) return

  const checkboxes = contentRef.value.querySelectorAll('input[type="checkbox"][data-cb-idx]')
  checkboxes.forEach((cb) => {
    const checkbox = cb as HTMLInputElement
    // Remove any existing handler
    checkbox.onclick = null
    // Add click handler
    checkbox.addEventListener('click', async (e) => {
      e.preventDefault()
      const idx = parseInt(checkbox.dataset.cbIdx || '0', 10)
      await handleCheckboxToggle(idx)
    })
  })
}

async function handleCheckboxToggle(index: number) {
  if (!entity.value) return

  try {
    await toggleCheckbox(entity.value.id, index)
    // Reload entity to get updated content
    entity.value = await entitiesStore.fetchEntity(props.entityType, props.entityId, true)
  } catch (err) {
    uiStore.error('Failed to toggle checkbox')
    console.error(err)
  }
}

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

function runCommand(cmd: Command) {
  commandModalRef.value?.runCommand(cmd)
}

function getRelationTitle(targetId: string): string {
  const included = entity.value?.included?.[targetId]
  if (included?._title && included._title !== targetId) {
    return `${included._title} (${targetId})`
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
        <h2>
          Content
          <span v-if="checkboxStats" class="cb-stats">({{ checkboxStats.checked }}/{{ checkboxStats.total }})</span>
        </h2>
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
      <CommandModal ref="commandModalRef" :entity-id="entityId" />
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

/* Uses global .loading-state, .error-state, .spinner from App.vue */

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
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted-text);
}

.header-info h1 {
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 8px;
}

/* Uses global .btn, .btn-secondary, .btn-danger from App.vue */

.detail-section {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 24px;
}

.detail-section h2 {
  margin: 0 0 16px;
  font-size: 18px;
  color: var(--text-color);
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
  color: var(--muted-text);
}

.property-item dd {
  margin: 0;
  font-size: 14px;
  color: var(--text-color);
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
  color: var(--muted-text);
  text-transform: capitalize;
}

.relation-targets {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.relation-link {
  padding: 6px 12px;
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 13px;
  font-family: monospace;
  color: var(--accent-color);
  cursor: pointer;
  transition: all 0.15s;
}

.relation-link:hover {
  filter: brightness(0.95);
  border-color: var(--accent-color);
}

.content-body {
  font-size: 15px;
  line-height: 1.7;
  color: var(--text-color);
}

.content-body :deep(h1),
.content-body :deep(h2),
.content-body :deep(h3) {
  margin: 24px 0 12px;
  color: var(--text-color);
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
  color: var(--muted-text);
  font-weight: 500;
}

.content-body :deep(code) {
  background: var(--hover-bg);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
  color: var(--text-color);
}

.content-body :deep(input[type="checkbox"]) {
  margin-right: 8px;
  cursor: pointer;
}

.cb-stats {
  font-size: 14px;
  font-weight: 500;
  color: var(--muted-text);
  margin-left: 8px;
}

/* Uses global .modal-overlay, .modal, .modal-actions from App.vue */

.btn-command {
  background: var(--accent-color, #3b82f6);
  color: white;
}

.btn-command:hover:not(:disabled) {
  filter: brightness(1.1);
}

/* Scope Navigation Bar */
.scope-nav {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.scope-nav-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 13px;
  color: var(--text-color);
  cursor: pointer;
  text-decoration: none;
  transition: all 0.15s;
}

.scope-nav-btn:hover:not(.disabled) {
  filter: brightness(0.95);
  border-color: var(--accent-color);
}

.scope-nav-btn.disabled {
  color: var(--muted-text);
  cursor: not-allowed;
  opacity: 0.6;
}

.scope-nav-btn kbd {
  padding: 2px 5px;
  font-size: 10px;
  background: var(--border-color);
  border-radius: 3px;
  font-family: monospace;
}

.scope-nav-progress {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-color);
  font-family: monospace;
}

.scope-nav-label {
  flex: 1;
  font-size: 13px;
  color: var(--muted-text);
}
</style>
