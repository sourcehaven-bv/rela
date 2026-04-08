<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'
import { isCancelledFetch } from '@/composables/usePageData'
import type { SidePanelSection, SidePanelEntity, SidePanelAddTarget } from '@/types'
import Badge from '@/components/common/Badge.vue'

const props = defineProps<{
  formId: string
  entityId?: string
}>()

const router = useRouter()

const sections = ref<SidePanelSection[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const collapsedSections = ref<Set<string>>(new Set())

const hasSections = computed(() => sections.value.length > 0)

async function loadSidePanel() {
  if (!props.entityId) {
    sections.value = []
    return
  }

  loading.value = true
  error.value = null

  try {
    const data = await api.get<SidePanelSection[]>(
      `/_sidepanel/${props.formId}/${props.entityId}`
    )
    sections.value = data
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    console.error('Failed to load side panel:', err)
    error.value = 'Failed to load side panel'
    sections.value = []
  } finally {
    loading.value = false
  }
}

function toggleSection(sectionId: string) {
  if (collapsedSections.value.has(sectionId)) {
    collapsedSections.value.delete(sectionId)
  } else {
    collapsedSections.value.add(sectionId)
  }
}

function isCollapsed(sectionId: string): boolean {
  return collapsedSections.value.has(sectionId)
}

function navigateToEntity(entity: SidePanelEntity) {
  if (entity.editFormId) {
    router.push(`/form/${entity.editFormId}/${entity.id}`)
  } else {
    router.push(`/entity/${entity.type}/${entity.id}`)
  }
}

function createNewForSection(section: SidePanelSection, target: SidePanelAddTarget) {
  if (!section.addInfo) return
  // Navigate to create form with relation context
  const query: Record<string, string> = {
    _relation: section.addInfo.relation,
    _linkAs: section.addInfo.linkAs,
    _peerId: section.addInfo.peerId,
  }
  router.push({ path: `/form/${target.formId}`, query })
}

watch(
  () => [props.formId, props.entityId],
  () => loadSidePanel(),
  { immediate: true }
)

onMounted(() => loadSidePanel())
</script>

<template>
  <aside v-if="hasSections || loading" class="side-panel">
    <div v-if="loading" class="loading-state">
      <div class="spinner"/>
    </div>

    <div v-else-if="error" class="error-state">
      {{ error }}
    </div>

    <template v-else>
      <div
        v-for="section in sections"
        :key="section.sectionId"
        class="panel-section"
        :class="{ collapsed: isCollapsed(section.sectionId) }"
      >
        <button
          class="section-header"
          @click="toggleSection(section.sectionId)"
        >
          <span class="section-title">{{ section.heading }}</span>
          <span class="collapse-icon">{{ isCollapsed(section.sectionId) ? '+' : '-' }}</span>
        </button>

        <div v-if="!isCollapsed(section.sectionId)" class="section-content">
          <!-- Empty state -->
          <div v-if="section.isEmpty" class="empty-state">
            {{ section.emptyMessage || 'No items' }}
          </div>

          <!-- Properties display -->
          <template v-else-if="section.display === 'properties'">
            <dl class="properties-list">
              <div v-for="field in section.fields" :key="field.label" class="property-item">
                <dt>{{ field.label }}</dt>
                <dd>{{ field.value || '-' }}</dd>
              </div>
            </dl>
          </template>

          <!-- List display -->
          <template v-else-if="section.display === 'list'">
            <ul class="entity-list">
              <li
                v-for="entity in section.entities"
                :key="entity.id"
                class="entity-list-item"
                @click="navigateToEntity(entity)"
              >
                <Badge :value="entity.title || entity.id" :property="entity.type" />
              </li>
            </ul>
          </template>

          <!-- Cards display -->
          <template v-else-if="section.display === 'cards'">
            <div class="entity-cards">
              <div
                v-for="entity in section.entities"
                :key="entity.id"
                class="entity-card"
                @click="navigateToEntity(entity)"
              >
                <div class="card-header">
                  <span class="card-id">{{ entity.id }}</span>
                </div>
                <div class="card-title">{{ entity.title || entity.id }}</div>
                <div v-if="entity.fields?.length" class="card-fields">
                  <div v-for="field in entity.fields" :key="field.label" class="card-field">
                    <span class="field-label">{{ field.label }}:</span>
                    <Badge
                      v-if="field.propType === 'enum'"
                      :value="field.value"
                      :property="field.label.toLowerCase()"
                    />
                    <span v-else class="field-value">{{ field.value }}</span>
                  </div>
                </div>
              </div>
            </div>
          </template>

          <!-- Add button -->
          <div v-if="section.addInfo?.targets?.length" class="section-actions">
            <template v-if="section.addInfo.targets.length === 1">
              <button
                class="btn-add"
                @click="createNewForSection(section, section.addInfo.targets[0])"
              >
                + {{ section.addInfo.targets[0].label }}
              </button>
            </template>
            <template v-else>
              <div class="btn-group">
                <button
                  v-for="target in section.addInfo.targets"
                  :key="target.entityType"
                  class="btn-add btn-add-sm"
                  @click="createNewForSection(section, target)"
                >
                  + {{ target.label }}
                </button>
              </div>
            </template>
          </div>
        </div>
      </div>
    </template>
  </aside>
</template>

<style scoped>
.side-panel {
  width: 280px;
  min-width: 280px;
  background: var(--bg-color);
  padding: 16px;
  overflow-y: auto;
  max-height: calc(100vh - 64px);
}

.loading-state {
  display: flex;
  justify-content: center;
  padding: 24px;
}

.spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.error-state {
  padding: 16px;
  color: var(--error-color, #ef4444);
  font-size: 14px;
}

.panel-section {
  margin-bottom: 16px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
}

.section-header {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-color);
  text-align: left;
  transition: background 0.15s;
}

.section-header:hover {
  background: var(--hover-bg);
}

.collapse-icon {
  font-size: 16px;
  color: var(--muted-text);
}

.section-content {
  padding: 0 16px 16px;
}

.empty-state {
  font-size: 13px;
  color: var(--muted-text);
  font-style: italic;
}

/* Properties display */
.properties-list {
  margin: 0;
}

.property-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-bottom: 12px;
}

.property-item:last-child {
  margin-bottom: 0;
}

.property-item dt {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--muted-text);
  letter-spacing: 0.5px;
}

.property-item dd {
  margin: 0;
  font-size: 14px;
  color: var(--text-color);
}

/* List display */
.entity-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.entity-list-item {
  cursor: pointer;
  transition: transform 0.1s;
}

.entity-list-item:hover {
  transform: scale(1.05);
}

/* Cards display */
.entity-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.entity-card {
  padding: 12px;
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s;
}

.entity-card:hover {
  border-color: var(--accent-color);
  filter: brightness(0.95);
}

.card-header {
  margin-bottom: 4px;
}

.card-id {
  font-size: 11px;
  font-family: monospace;
  color: var(--muted-text);
}

.card-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-color);
  margin-bottom: 8px;
}

.card-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.card-field {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
}

.field-label {
  color: var(--muted-text);
}

.field-value {
  color: var(--text-color);
}

/* Section actions */
.section-actions {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--border-color);
}

.btn-add {
  width: 100%;
  padding: 8px 12px;
  background: var(--hover-bg);
  border: 1px dashed var(--border-color);
  border-radius: 6px;
  color: var(--muted-text);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-add:hover {
  background: var(--card-bg);
  border-color: var(--accent-color);
  color: var(--accent-color);
}

.btn-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.btn-add-sm {
  width: auto;
  flex: 1;
  min-width: 80px;
}

/* Responsive: mobile overlay */
@media (max-width: 1024px) {
  .side-panel {
    position: fixed;
    right: 0;
    top: 0;
    height: 100vh;
    max-height: 100vh;
    z-index: 100;
    transform: translateX(100%);
    transition: transform 0.2s ease;
  }

  .side-panel.open {
    transform: translateX(0);
  }
}
</style>
