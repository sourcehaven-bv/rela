<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { useSchemaStore } from '@/stores'
import { useScopeNavigation } from '@/composables'
import { useBackTarget } from '@/composables/useBackTarget'
import BackButton from '@/components/common/BackButton.vue'
import { isCancelledFetch } from '@/composables/usePageData'
import { fetchView } from '@/api'
import type { ViewResponse } from '@/api'
import { getEditFormId } from '@/types'
import { isInputFocused } from '@/utils/dom'
import { renderMarkdown } from '@/utils/markdown'
import Badge from '@/components/common/Badge.vue'
import LinkExistingModal from '@/components/forms/LinkExistingModal.vue'
import PropertyDisplay from '@/components/common/PropertyDisplay.vue'
import type { PropertyItem } from '@/components/common/PropertyDisplay.vue'
import type { ViewSectionField } from '@/api'

const props = defineProps<{
  id: string
  entityId: string
}>()

const router = useRouter()
const schemaStore = useSchemaStore()

// Scope navigation (prev/next within a list) and back affordance
// (return_to/from precedence). See EntityDetail for the same pairing.
const entryType = computed(() => viewData.value?.entry?.type || '')
const { scopeNav, loadScopeNav, navigateScope } = useScopeNavigation(
  () => entryType.value,
  () => props.entityId
)
const backTarget = useBackTarget()

// Keyboard shortcuts
function handleKeydown(e: KeyboardEvent) {
  if (isInputFocused()) return
  if (e.key === 'e' || e.key === 'E') {
    e.preventDefault()
    editEntry()
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
  // Escape to go back — uses back-target precedence (return_to > from).
  if (e.key === 'Escape' && backTarget.value) {
    e.preventDefault()
    router.push(backTarget.value.to)
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
})

// State
const loading = ref(false)
const error = ref<string | null>(null)
const viewData = ref<ViewResponse | null>(null)

// Link existing modal state
const showLinkModal = ref(false)
const linkModalInfo = ref<{
  relation: string
  linkAs: 'from' | 'to'
  peerId: string
  entityTypes: string[]
  excludeIds: string[]
} | null>(null)

// Computed
const viewConfig = computed(() => schemaStore.getView(props.id))

const entryTitle = computed(() => {
  if (!viewData.value?.entry) return props.entityId
  return (viewData.value.entry.properties.title as string) || viewData.value.entry.id
})

// Load view data
async function loadView() {
  loading.value = true
  error.value = null

  try {
    viewData.value = await fetchView(props.id, props.entityId)
    // Load scope nav after we have the entry type
    await loadScopeNav()
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    error.value = err instanceof Error ? err.message : 'Failed to load view'
    console.error('Failed to load view:', err)
  } finally {
    loading.value = false
  }
}

// Edit entry
function editEntry() {
  if (!viewData.value?.entry) return
  const formId = getEditFormId(schemaStore, viewData.value.entry.type)
  if (formId) {
    navigateToEdit(formId, viewData.value.entry.id)
  }
}

// Navigation
function navigateToEntity(entityId: string) {
  router.push({ name: 'entity', params: { id: entityId } })
}

function navigateToEdit(formId: string, entityId: string) {
  router.push({ name: 'form-edit', params: { id: formId, entityId } })
}

function navigateToCreate(formId: string, relationInfo?: { relation: string; linkAs: string; peerId: string }) {
  const query: Record<string, string> = {}
  if (relationInfo) {
    query.linkRelation = relationInfo.relation
    query.linkAs = relationInfo.linkAs
    query.linkPeer = relationInfo.peerId
  }
  router.push({ name: 'form-create', params: { id: formId }, query })
}

// Link existing modal
function openLinkExisting(linkInfo: { relation: string; linkAs: 'from' | 'to'; peerId: string; entityTypes: string[] }, section: { entities?: Array<{ id: string }>, rows?: Array<{ entityId: string }> }) {
  // Collect already-linked IDs from this section to exclude them
  const excludeIds: string[] = []
  if (section.entities) {
    excludeIds.push(...section.entities.map((e) => e.id))
  }
  if (section.rows) {
    excludeIds.push(...section.rows.map((r) => r.entityId))
  }
  linkModalInfo.value = { ...linkInfo, excludeIds }
  showLinkModal.value = true
}

function handleLinked() {
  // Reload the view to reflect the new relation
  loadView()
}

// Map ViewSectionField[] to PropertyItem[] for PropertyDisplay
function mapFieldsToProperties(fields: ViewSectionField[] | undefined): PropertyItem[] {
  if (!fields) return []
  return fields.map((field) => ({
    name: field.label.toLowerCase().replace(/\s+/g, '_'),
    label: field.label,
    value: field.values ?? [],
    propType: field.propType,
  }))
}

// Check if value should use badge styling (used for tables/lists)
function shouldUseBadge(value: string, propType?: string): boolean {
  // Use badge for enum types or known styled values
  return !!propType && !!value
}

// Jump to section
function scrollToSection(sectionId: string) {
  const el = document.getElementById(sectionId)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

// Watch for route changes
watch(
  () => [props.id, props.entityId],
  () => loadView(),
  { immediate: false }
)

onMounted(() => loadView())
</script>

<template>
  <div class="custom-view">
    <!-- Loading state -->
    <div v-if="loading" class="loading">
      <div class="spinner"/>
      <p>Loading view...</p>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="error">
      <h2>Error</h2>
      <p>{{ error }}</p>
      <button class="btn btn-primary" @click="loadView">Retry</button>
    </div>

    <!-- View content -->
    <template v-else-if="viewData">
      <!-- Back affordance + optional scope (prev/next) navigation. Same
           pattern as EntityDetail — see TKT-JIEKC. -->
      <div v-if="backTarget || scopeNav" class="scope-nav">
        <BackButton v-if="backTarget" :target="backTarget" />
        <template v-if="scopeNav">
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
        </template>
      </div>

      <!-- Header -->
      <header class="view-header">
        <div class="header-content">
          <h1>{{ viewConfig?.title || props.id }}: {{ entryTitle }}</h1>
          <div class="header-actions">
            <button
              v-if="viewData.entry && getEditFormId(schemaStore, viewData.entry.type)"
              class="btn btn-secondary"
              @click="editEntry"
            >
              Edit <kbd>E</kbd>
            </button>
          </div>
        </div>
      </header>

      <!-- Jump bar -->
      <nav v-if="viewData.sections.length > 1" class="jump-bar">
        <button
          v-for="section in viewData.sections.filter(s => s.heading)"
          :key="section.sectionId"
          class="jump-link"
          @click="scrollToSection(section.sectionId)"
        >
          {{ section.heading }}
        </button>
      </nav>

      <!-- Sections -->
      <div class="sections">
        <section
          v-for="section in viewData.sections"
          :id="section.sectionId"
          :key="section.sectionId"
          class="view-section"
        >
          <h2 v-if="section.heading" class="section-heading">{{ section.heading }}</h2>

          <!-- Empty state -->
          <div v-if="section.isEmpty" class="section-empty">
            {{ section.emptyMessage || 'No items' }}
          </div>

          <!-- Properties display -->
          <PropertyDisplay
            v-else-if="section.display === 'properties'"
            :properties="mapFieldsToProperties(section.fields)"
          />

          <!-- Content display (single) -->
          <div v-else-if="section.display === 'content' && section.hasContent" class="content-block">
            <div class="markdown-content" v-html="renderMarkdown(section.content || '')"/>
          </div>

          <!-- Content display (collection) -->
          <div v-else-if="section.display === 'content' && section.entities?.length" class="content-cards">
            <article
              v-for="entity in section.entities"
              :key="entity.id"
              class="content-card"
            >
              <header class="card-header" @click="navigateToEntity(entity.id)">
                <span class="entity-type">{{ entity.type }}</span>
                <span class="entity-title">{{ entity.title }}</span>
              </header>
              <div v-if="entity.hasContent" class="markdown-content" v-html="renderMarkdown(entity.content || '')"/>
            </article>
          </div>

          <!-- Cards display -->
          <div v-else-if="section.display === 'cards'" class="cards-grid">
            <article
              v-for="entity in section.entities"
              :key="entity.id"
              class="entity-card"
              @click="navigateToEntity(entity.id)"
            >
              <header class="card-header">
                <span class="entity-type">{{ entity.type }}</span>
                <span class="entity-title">{{ entity.title }}</span>
                <button
                  v-if="entity.editFormId"
                  class="edit-btn"
                  title="Edit"
                  @click.stop="navigateToEdit(entity.editFormId, entity.id)"
                >
                  &times;
                </button>
              </header>
              <div v-if="entity.fields?.length" class="card-fields">
                <div v-for="field in entity.fields" :key="field.label" class="card-field">
                  <span class="field-label">{{ field.label }}:</span>
                  <div v-if="field.propType && field.values?.length" class="badge-row">
                    <Badge
                      v-for="v in field.values"
                      :key="v"
                      :value="v"
                      :property="field.propType"
                    />
                  </div>
                  <span v-else class="field-value">{{ field.values?.join(', ') || '-' }}</span>
                </div>
              </div>
            </article>
          </div>

          <!-- List display -->
          <ul v-else-if="section.display === 'list'" class="entity-list">
            <li
              v-for="entity in section.entities"
              :key="entity.id"
              class="list-item"
            >
              <a class="list-link" @click="navigateToEntity(entity.id)">
                <span class="entity-type">{{ entity.type }}</span>
                <span class="entity-title">{{ entity.title }}</span>
              </a>
              <span v-if="entity.fields?.length" class="list-fields">
                <template v-for="field in entity.fields" :key="field.label">
                  <Badge
                    v-for="v in field.values ?? []"
                    :key="`${field.label}-${v}`"
                    :value="v"
                    :property="field.propType"
                  />
                </template>
              </span>
            </li>
          </ul>

          <!-- Table display -->
          <div v-else-if="section.display === 'table'" class="table-wrapper">
            <!-- Grouped table -->
            <template v-if="section.isGrouped && section.groups?.length">
              <div v-for="group in section.groups" :key="group.groupName" class="table-group">
                <h3 class="group-heading">{{ group.groupName }}</h3>
                <table class="data-table">
                  <thead>
                    <tr>
                      <th v-for="col in section.columns" :key="col.property || col.relation">
                        {{ col.label || col.property || col.relation }}
                      </th>
                      <th class="actions-col"/>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="row in group.rows" :key="row.entityId">
                      <td v-for="(cell, idx) in row.cells" :key="idx">
                        <a
                          v-if="cell.link"
                          :href="cell.link"
                          @click.prevent="navigateToEntity(cell.entityId || row.entityId)"
                        >
                          <template v-for="(val, vidx) in cell.values" :key="vidx">
                            <Badge
                              v-if="shouldUseBadge(val, cell.propType)"
                              :value="val"
                              :property="cell.propType"
                            />
                            <span v-else>{{ val }}</span>
                            <span v-if="vidx < cell.values.length - 1">, </span>
                          </template>
                        </a>
                        <template v-else>
                          <template v-for="(val, vidx) in cell.values" :key="vidx">
                            <Badge
                              v-if="shouldUseBadge(val, cell.propType)"
                              :value="val"
                              :property="cell.propType"
                            />
                            <span v-else>{{ val }}</span>
                            <span v-if="vidx < cell.values.length - 1">, </span>
                          </template>
                        </template>
                      </td>
                      <td class="actions-cell">
                        <button
                          v-if="row.editFormId"
                          class="icon-btn"
                          title="Edit"
                          @click="navigateToEdit(row.editFormId, row.entityId)"
                        >
                          &#9998;
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </template>

            <!-- Non-grouped table -->
            <table v-else class="data-table">
              <thead>
                <tr>
                  <th v-for="col in section.columns" :key="col.property || col.relation">
                    {{ col.label || col.property || col.relation }}
                  </th>
                  <th class="actions-col"/>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in section.rows" :key="row.entityId">
                  <td v-for="(cell, idx) in row.cells" :key="idx">
                    <a
                      v-if="cell.link"
                      :href="cell.link"
                      @click.prevent="navigateToEntity(cell.entityId || row.entityId)"
                    >
                      <template v-for="(val, vidx) in cell.values" :key="vidx">
                        <Badge
                          v-if="shouldUseBadge(val, cell.propType)"
                          :value="val"
                          :property="cell.propType"
                        />
                        <span v-else>{{ val }}</span>
                        <span v-if="vidx < cell.values.length - 1">, </span>
                      </template>
                    </a>
                    <template v-else>
                      <template v-for="(val, vidx) in cell.values" :key="vidx">
                        <Badge
                          v-if="shouldUseBadge(val, cell.propType)"
                          :value="val"
                          :property="cell.propType"
                        />
                        <span v-else>{{ val }}</span>
                        <span v-if="vidx < cell.values.length - 1">, </span>
                      </template>
                    </template>
                  </td>
                  <td class="actions-cell">
                    <button
                      v-if="row.editFormId"
                      class="icon-btn"
                      title="Edit"
                      @click="navigateToEdit(row.editFormId, row.entityId)"
                    >
                      &#9998;
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Section actions (Add / Link existing) -->
          <div v-if="section.addInfo || section.linkInfo" class="section-actions">
            <template v-if="section.addInfo">
              <button
                v-for="target in section.addInfo.targets"
                :key="target.entityType"
                class="btn btn-add"
                @click="navigateToCreate(target.formId, {
                  relation: section.addInfo!.relation,
                  linkAs: section.addInfo!.linkAs,
                  peerId: section.addInfo!.peerId
                })"
              >
                + Add {{ target.label }}
              </button>
            </template>
            <button
              v-if="section.linkInfo"
              class="btn btn-link-existing"
              @click="openLinkExisting(section.linkInfo!, section)"
            >
              &#128279; Link Existing
            </button>
          </div>
        </section>
      </div>
    </template>

    <!-- Link existing modal -->
    <LinkExistingModal
      v-if="linkModalInfo"
      :show="showLinkModal"
      :relation="linkModalInfo.relation"
      :link-as="linkModalInfo.linkAs"
      :peer-id="linkModalInfo.peerId"
      :entity-types="linkModalInfo.entityTypes"
      :exclude-ids="linkModalInfo.excludeIds"
      @close="showLinkModal = false"
      @linked="handleLinked"
    />
  </div>
</template>

<style scoped>
.custom-view {
  max-width: 1200px;
  padding: 20px;
}

.loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding: 60px;
  color: var(--muted-text);
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.error {
  text-align: center;
  padding: 40px;
  color: var(--error-color, #ef4444);
}

.error h2 {
  margin-bottom: 12px;
}

.error p {
  margin-bottom: 20px;
}

/* Header */
.view-header {
  margin-bottom: 24px;
}

.header-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.view-header h1 {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
  color: var(--text-color);
}

.header-actions {
  display: flex;
  gap: 8px;
}

.header-actions kbd {
  padding: 2px 5px;
  font-size: 10px;
  background: var(--border-color);
  border-radius: 3px;
  font-family: monospace;
  margin-left: 4px;
}

/* Scope Navigation Bar
 * .scope-nav-btn styles live in src/styles/back-button.css — see
 * TKT-JIEKC. The bar layout (.scope-nav) + progress/label stay here. */
.scope-nav {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
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

/* Jump bar */
.jump-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 0;
  border-bottom: 1px solid var(--border-color);
  margin-bottom: 24px;
}

.jump-link {
  padding: 6px 12px;
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 13px;
  color: var(--text-color);
  cursor: pointer;
  transition: all 0.15s;
}

.jump-link:hover {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: white;
}

/* Sections */
.sections {
  display: flex;
  flex-direction: column;
  gap: 32px;
}

.view-section {
  scroll-margin-top: 20px;
}

.section-heading {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border-color);
  color: var(--text-color);
}

.section-empty {
  padding: 24px;
  text-align: center;
  color: var(--muted-text);
  background: var(--hover-bg);
  border-radius: 6px;
  font-style: italic;
}

/* Content block */
.content-block {
  padding: 16px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
}

.markdown-content {
  line-height: 1.6;
}

/* Content cards */
.content-cards {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.content-card {
  padding: 16px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
}

.content-card .card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  cursor: pointer;
}

.content-card .card-header:hover .entity-title {
  color: var(--accent-color);
}

/* Cards grid */
.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.entity-card {
  padding: 16px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  cursor: pointer;
  transition: border-color 0.15s;
}

.entity-card:hover {
  border-color: var(--accent-color);
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.entity-type {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--muted-text);
  background: var(--border-color);
  padding: 2px 4px;
  border-radius: 2px;
}

.entity-title {
  font-weight: 500;
  color: var(--text-color);
  flex: 1;
}

.edit-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  font-size: 16px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
}

.edit-btn:hover {
  background: var(--hover-bg);
  color: var(--text-color);
}

.card-fields {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.card-field {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.field-label {
  color: var(--muted-text);
}

.field-value {
  color: var(--text-color);
}

/* Entity list */
.entity-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.list-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 4px;
}

.list-link {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  flex: 1;
}

.list-link:hover .entity-title {
  color: var(--accent-color);
}

.list-fields {
  display: flex;
  gap: 6px;
}

/* Table */
.table-wrapper {
  overflow-x: auto;
}

.table-group {
  margin-bottom: 24px;
}

.group-heading {
  font-size: 14px;
  font-weight: 600;
  color: var(--muted-text);
  margin: 0 0 8px 0;
  padding: 4px 0;
  border-bottom: 1px solid var(--border-color);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}

.data-table th,
.data-table td {
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-color);
}

.data-table th {
  font-weight: 500;
  color: var(--muted-text);
  background: var(--hover-bg);
}

.data-table td {
  color: var(--text-color);
}

.data-table tbody tr:hover {
  background: var(--hover-bg);
}

.data-table a {
  color: var(--accent-color);
  text-decoration: none;
}

.data-table a:hover {
  text-decoration: underline;
}

.actions-col {
  width: 60px;
}

.actions-cell {
  text-align: center;
}

.icon-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  cursor: pointer;
  padding: 4px 8px;
  font-size: 14px;
  border-radius: 4px;
}

.icon-btn:hover {
  background: var(--hover-bg);
  color: var(--text-color);
}

/* Section actions */
.section-actions {
  margin-top: 16px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.btn-add {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 8px 14px;
  background: var(--hover-bg);
  border: 1px dashed var(--border-color);
  border-radius: 4px;
  color: var(--accent-color);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-add:hover {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: white;
}

.btn-link-existing {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 8px 14px;
  background: var(--hover-bg);
  border: 1px dashed var(--border-color);
  border-radius: 4px;
  color: var(--accent-color);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-link-existing:hover {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: white;
}

/* Buttons */
.btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s;
}

.btn-primary {
  background: var(--accent-color, #6366f1);
  color: white;
}

.btn-primary:hover {
  filter: brightness(1.1);
}

.btn-secondary {
  background: var(--border-color);
  color: var(--text-color);
}

.btn-secondary:hover {
  filter: brightness(0.95);
}
</style>
