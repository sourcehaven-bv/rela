<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, onUnmounted, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { useScopeNavigation } from '@/composables'
import type { Entity, Command } from '@/types'
import { getEditFormId } from '@/types'
import { isInputFocused } from '@/utils/dom'
import { isAnyModalOpen } from '@/composables/modalStack'
import { renderMarkdown, renderMermaidDiagrams, getCheckboxStats } from '@/utils/markdown'
import { getCommands, toggleCheckbox } from '@/api'
import PropertyDisplay from '@/components/common/PropertyDisplay.vue'
import DocumentsPanel from '@/components/entity/DocumentsPanel.vue'
import CommandModal from '@/components/entity/CommandModal.vue'
import ConfirmModal from '@/components/ui/ConfirmModal.vue'

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
  // Don't handle shortcuts while any modal is open — the modal owns Escape,
  // and Delete/Backspace must not reopen a second confirmation.
  if (isAnyModalOpen()) return
  if (document.querySelector('.shortcuts-overlay')) return
  if (e.key === 'e' || e.key === 'E') {
    e.preventDefault()
    editEntity()
  }
  // Delete / Backspace: open delete confirmation modal. Only active once the
  // entity has loaded — otherwise Backspace during initial load would both
  // hijack browser back-nav and render a modal referencing a null entity.
  // preventDefault stops the browser back-nav side effect of Backspace.
  if ((e.key === 'Delete' || e.key === 'Backspace') && entity.value) {
    e.preventDefault()
    showDeleteConfirm.value = true
  }
  // Scope navigation: p/n for prev/next
  if (e.key === 'p' && scopeNav.value?.prevId) {
    e.preventDefault()
    navigateScope('prev')
  }
  if (e.key === 'n' && scopeNav.value?.nextId) {
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
const showOverflowMenu = ref(false)

// Close overflow menu on outside click
function closeOverflow() { showOverflowMenu.value = false }
onMounted(() => document.addEventListener('click', closeOverflow))
onUnmounted(() => document.removeEventListener('click', closeOverflow))

// Commands state
const commands = ref<Command[]>([])

// Computed
const typeDef = computed(() => schemaStore.getEntityType(props.entityType))
const editFormId = computed(() => getEditFormId(schemaStore, props.entityType))

const properties = computed(() => {
  if (!entity.value || !typeDef.value) return []

  // Get property order from edit form if available, otherwise use metamodel order
  const formId = editFormId.value
  const form = formId ? schemaStore.getForm(formId) : null
  const formFieldOrder = form?.fields?.map((f) => f.property).filter(Boolean) as string[] || []

  // Build properties list
  const props = Object.entries(typeDef.value.properties).map(([name, def]) => ({
    name,
    label: name.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase()),
    value: entity.value?.properties[name],
    type: def.type,
    values: def.values,
    isLongText: def.type === 'string' && String(entity.value?.properties[name] || '').length > 60,
  }))

  // Sort by form field order, then alphabetically for any not in form
  return props.sort((a, b) => {
    const aIdx = formFieldOrder.indexOf(a.name)
    const bIdx = formFieldOrder.indexOf(b.name)
    if (aIdx !== -1 && bIdx !== -1) return aIdx - bIdx
    if (aIdx !== -1) return -1
    if (bIdx !== -1) return 1
    return a.name.localeCompare(b.name)
  })
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
    // Close modal only on success. On error the modal stays open (with busy
    // cleared) so the user keeps context and can retry or cancel.
    showDeleteConfirm.value = false
    router.push(backTargetAfterDelete())
  } catch (err) {
    uiStore.error('Failed to delete entity')
    console.error(err)
  } finally {
    deleting.value = false
  }
}

// Determine where to navigate after deleting this entity.
// Priority:
//   1. The list we came from (scope navigation)
//   2. A list configured for this entity type
//   3. The dashboard
function backTargetAfterDelete(): string {
  if (scopeNav.value?.backUrl) return scopeNav.value.backUrl
  const listId = schemaStore.findListIdForEntityType(props.entityType)
  if (listId) return `/list/${listId}`
  return '/'
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
          ← Prev <kbd>P</kbd>
        </button>
        <span v-else class="scope-nav-btn disabled">← Prev</span>
        <span class="scope-nav-progress">[{{ scopeNav.current }}/{{ scopeNav.total }}]</span>
        <span class="scope-nav-label">{{ scopeNav.label }}</span>
        <button
          v-if="scopeNav.nextId"
          class="scope-nav-btn"
          @click="navigateScope('next')"
        >
          Next → <kbd>N</kbd>
        </button>
        <span v-else class="scope-nav-btn disabled">Next →</span>
      </div>

      <header class="detail-header">
        <div class="header-info">
          <span class="entity-type-badge">{{ typeDef?.label || entityType }}</span>
          <h1>{{ entity.properties.title || entity.id }}</h1>
        </div>
        <!-- Desktop actions -->
        <div class="header-actions desktop-actions">
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
            Delete <kbd>Del</kbd>
          </button>
        </div>

        <!-- Mobile actions: Edit primary, delete icon, overflow menu for commands -->
        <div class="header-actions mobile-actions">
          <button v-if="editFormId" class="btn btn-secondary" @click="editEntity">
            Edit
          </button>
          <button class="btn btn-danger mobile-delete-btn" @click="showDeleteConfirm = true" aria-label="Delete">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="3 6 5 6 21 6"/>
              <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/>
            </svg>
          </button>
          <div v-if="commands.length" class="overflow-menu-wrapper">
            <button
              class="btn btn-secondary mobile-overflow-btn"
              aria-label="More actions"
              @click.stop="showOverflowMenu = !showOverflowMenu"
            >
              ⋯
            </button>
            <div v-if="showOverflowMenu" class="overflow-menu" @click="showOverflowMenu = false">
              <button
                v-for="cmd in commands"
                :key="cmd.id"
                class="overflow-menu-item"
                @click="runCommand(cmd)"
              >
                {{ cmd.label }}
              </button>
            </div>
          </div>
        </div>
      </header>

      <!-- Properties Section -->
      <section class="detail-section">
        <h2>Properties</h2>
        <PropertyDisplay :properties="properties" :entity-type="typeDef" />
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
      <ConfirmModal
        :open="showDeleteConfirm"
        title="Delete Entity?"
        confirm-label="Delete"
        :busy="deleting"
        danger
        @confirm="deleteEntity"
        @cancel="showDeleteConfirm = false"
      >
        Are you sure you want to delete <strong>{{ entity.id }}</strong>?
        This action cannot be undone.
      </ConfirmModal>

      <!-- Command Execution Modal -->
      <CommandModal ref="commandModalRef" :entity-id="entityId" />
    </template>

    <div v-else class="error-state">
      <h2>Entity not found</h2>
      <p>{{ entityType }} "{{ entityId }}" could not be found.</p>
      <router-link :to="backTargetAfterDelete()" class="btn btn-secondary">
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

/* Mobile actions — hidden on desktop */
.mobile-actions {
  display: none;
}

.mobile-delete-btn {
  padding: 8px 12px;
}

.overflow-menu-wrapper {
  position: relative;
}

.mobile-overflow-btn {
  font-size: 18px;
  letter-spacing: 2px;
  padding: 8px 14px;
}

.overflow-menu {
  position: absolute;
  right: 0;
  top: 100%;
  margin-top: 4px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  min-width: 180px;
  z-index: 20;
  overflow: hidden;
}

.overflow-menu-item {
  display: block;
  width: 100%;
  padding: 12px 16px;
  background: none;
  border: none;
  text-align: left;
  font-size: 14px;
  color: var(--text-color);
  cursor: pointer;
  font-family: inherit;
}

.overflow-menu-item:hover {
  background: var(--hover-bg);
}

.overflow-menu-item + .overflow-menu-item {
  border-top: 1px solid var(--border-color);
}

@media (max-width: 768px) {
  /* Native-style mobile nav bar — sits in the hamburger area */
  .scope-nav {
    position: sticky;
    top: -60px; /* pull up into main-content padding-top (60px) */
    z-index: 102; /* above hamburger (101) */
    background: var(--bg-color);
    margin: -60px -16px 12px -16px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border-color);
    gap: 0;
    flex-wrap: nowrap;
    justify-content: space-between;
  }

  .scope-nav-label {
    display: none;
  }

  .scope-nav-progress {
    font-size: 13px;
    color: var(--muted-text);
    flex: 1;
    text-align: center;
  }

  .scope-nav-btn {
    padding: 8px 12px;
    font-size: 14px;
    min-height: 36px;
    background: none;
    border: none;
    color: var(--accent-color);
    font-weight: 500;
  }

  .scope-nav-btn:hover:not(.disabled) {
    filter: none;
    border-color: transparent;
    opacity: 0.7;
  }

  .scope-nav-btn.disabled {
    color: var(--border-color);
    opacity: 0.4;
  }

  .detail-header {
    flex-direction: column;
    gap: 12px;
  }

  .desktop-actions {
    display: none;
  }

  .mobile-actions {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .mobile-actions .btn {
    min-height: 44px;
  }

  .mobile-actions .btn-secondary {
    flex: 1;
  }

  .header-info h1 {
    font-size: 22px;
  }

  .detail-section {
    padding: 16px;
    margin-bottom: 16px;
  }
}
</style>
