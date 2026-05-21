<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import { listEntities, updateEntity } from '@/api'
import type { Entity, KanbanConfig } from '@/types'
import Badge from '@/components/common/Badge.vue'
import BackButton from '@/components/common/BackButton.vue'
import { useBackTarget } from '@/composables/useBackTarget'

const props = defineProps<{
  id: string
}>()

const router = useRouter()
const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()

// Back affordance — renders when ?return_to= or ?from= is present.
const backTarget = useBackTarget()

// State
const loading = ref(true)
const entities = ref<Entity[]>([])
const filterValues = ref<Record<string, string>>({})
const draggedCard = ref<Entity | null>(null)
const collectionActions = ref<Record<string, boolean> | undefined>(undefined)

// Affordance gates: `_actions` from server. `false` → hide / disallow;
// anything else → allow.
function canCreate(): boolean {
  return collectionActions.value?.create !== false
}
function canUpdate(entity: Entity): boolean {
  return entity._actions?.update !== false
}

// Computed
const kanbanConfig = computed(() => schemaStore.getKanban(props.id) as KanbanConfig | undefined)

const entityType = computed(() => {
  if (!kanbanConfig.value) return undefined
  return schemaStore.getEntityType(kanbanConfig.value.entity)
})

const columns = computed(() => {
  if (!kanbanConfig.value) return []

  // Use defined columns or generate from unique values
  if (kanbanConfig.value.columns?.length) {
    return kanbanConfig.value.columns
  }

  // Fallback: extract unique values from entities
  const property = kanbanConfig.value.column_property
  const values = new Set<string>()
  for (const entity of entities.value) {
    const val = String(entity.properties[property] || '')
    if (val) values.add(val)
  }
  return Array.from(values).map((v) => ({ value: v, label: v }))
})

const filteredEntities = computed(() => {
  let result = [...entities.value]

  // Apply kanban config filters
  if (kanbanConfig.value?.filters) {
    for (const filter of kanbanConfig.value.filters) {
      result = result.filter((entity) => {
        const val = String(entity.properties[filter.property] || '')
        switch (filter.operator) {
          case '=':
          case '==':
            return val === filter.value
          case '!=':
            return val !== filter.value
          default:
            return true
        }
      })
    }
  }

  // Apply user filter controls
  for (const [prop, value] of Object.entries(filterValues.value)) {
    if (value) {
      result = result.filter((entity) => String(entity.properties[prop] || '') === value)
    }
  }

  return result
})

// Swimlanes (rows in 2D grid layout)
const swimlanes = computed(() => {
  if (!kanbanConfig.value?.swimlane_property) return []

  // Use defined swimlanes or generate from unique values
  if (kanbanConfig.value.swimlanes?.length) {
    return kanbanConfig.value.swimlanes
  }

  // Fallback: extract unique values from entities
  const property = kanbanConfig.value.swimlane_property
  const values = new Set<string>()
  for (const entity of entities.value) {
    const val = String(entity.properties[property] || '')
    if (val) values.add(val)
  }
  return Array.from(values).sort().map((v) => ({ value: v, label: v }))
})

const hasSwimmlanes = computed(() => swimlanes.value.length > 0)

const entitiesByColumn = computed(() => {
  const grouped: Record<string, Entity[]> = {}
  const property = kanbanConfig.value?.column_property || ''

  for (const column of columns.value) {
    grouped[column.value] = []
  }

  for (const entity of filteredEntities.value) {
    const val = String(entity.properties[property] || '')
    if (grouped[val]) {
      grouped[val].push(entity)
    }
  }

  return grouped
})

// 2D grouping for swimlane mode: entitiesByCell[column][swimlane] = entities
const entitiesByCell = computed(() => {
  if (!hasSwimmlanes.value) return {}

  const cells: Record<string, Record<string, Entity[]>> = {}
  const colProp = kanbanConfig.value?.column_property || ''
  const swimProp = kanbanConfig.value?.swimlane_property || ''

  // Initialize all cells
  for (const column of columns.value) {
    cells[column.value] = {}
    for (const swimlane of swimlanes.value) {
      cells[column.value][swimlane.value] = []
    }
  }

  // Group entities into cells
  for (const entity of filteredEntities.value) {
    const colVal = String(entity.properties[colProp] || '')
    const swimVal = String(entity.properties[swimProp] || '')
    if (cells[colVal] && cells[colVal][swimVal]) {
      cells[colVal][swimVal].push(entity)
    }
  }

  return cells
})

// CSS grid style for swimlane board
const swimlaneGridStyle = computed(() => {
  const colCount = columns.value.length
  return {
    gridTemplateColumns: `auto repeat(${colCount}, minmax(240px, 1fr))`,
  }
})

const filterOptions = computed(() => {
  const options: Record<string, string[]> = {}

  if (!kanbanConfig.value?.filter_controls) return options

  for (const control of kanbanConfig.value.filter_controls) {
    if (control.property) {
      const values = new Set<string>()
      for (const entity of entities.value) {
        const val = String(entity.properties[control.property] || '')
        if (val) values.add(val)
      }
      options[control.property] = Array.from(values).sort()
    }
  }

  return options
})

// Methods
async function loadEntities() {
  if (!kanbanConfig.value) return

  loading.value = true
  try {
    const response = await listEntities(kanbanConfig.value.entity)
    entities.value = response.data
    collectionActions.value = response._actions
  } catch (err) {
    console.error('Kanban load error:', err)
  } finally {
    loading.value = false
  }
}

function getCardTitle(entity: Entity): string {
  if (!kanbanConfig.value) return entity.id
  return String(entity.properties[kanbanConfig.value.card.title] || entity.id)
}

function getCardFieldValue(entity: Entity, field: { property?: string }): string {
  if (!field.property) return ''
  return String(entity.properties[field.property] || '')
}

function getCardFieldLabel(field: { property?: string }): string {
  return field.property || ''
}

function isEnumField(field: { property?: string }): boolean {
  if (!field.property || !entityType.value) return false
  const propDef = entityType.value.properties[field.property]
  return propDef?.type === 'enum' || (propDef?.values?.length ?? 0) > 0
}

function onDragStart(event: DragEvent, entity: Entity) {
  draggedCard.value = entity
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', entity.id)
  }
}

function onDragOver(event: DragEvent) {
  event.preventDefault()
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'move'
  }
}

async function onDrop(event: DragEvent, columnValue: string, swimlaneValue?: string) {
  event.preventDefault()

  if (!draggedCard.value || !kanbanConfig.value) return

  const entity = draggedCard.value
  // Defence in depth: `:draggable="false"` prevents drag-from-Kanban
  // starting, but external drag sources (text drag from another tab,
  // file drag) can still trigger this handler. Early-return on a
  // denied entity so we don't fire an update the server will 403.
  if (!canUpdate(entity)) {
    draggedCard.value = null
    return
  }
  const colProp = kanbanConfig.value.column_property
  const swimProp = kanbanConfig.value.swimlane_property

  const currentCol = String(entity.properties[colProp] || '')
  const currentSwim = swimProp ? String(entity.properties[swimProp] || '') : undefined

  // Don't update if same position
  const sameColumn = currentCol === columnValue
  const sameSwimmlane = !swimProp || currentSwim === swimlaneValue
  if (sameColumn && sameSwimmlane) {
    draggedCard.value = null
    return
  }

  // Build update payload
  const updates: Record<string, string> = {}
  const oldValues: Record<string, unknown> = {}

  if (!sameColumn) {
    oldValues[colProp] = entity.properties[colProp]
    entity.properties[colProp] = columnValue
    updates[colProp] = columnValue
  }

  if (swimProp && swimlaneValue !== undefined && !sameSwimmlane) {
    oldValues[swimProp] = entity.properties[swimProp]
    entity.properties[swimProp] = swimlaneValue
    updates[swimProp] = swimlaneValue
  }

  try {
    await updateEntity(kanbanConfig.value.entity, entity.id, {
      properties: updates,
    })
  } catch (err) {
    // Revert on error
    for (const [prop, val] of Object.entries(oldValues)) {
      entity.properties[prop] = val
    }
    console.error('Failed to update entity:', err)
  }

  draggedCard.value = null
}

function onDragEnd() {
  draggedCard.value = null
}

function openCard(entity: Entity) {
  if (kanbanConfig.value?.edit_form) {
    router.push(`/form/${kanbanConfig.value.edit_form}/${entity.id}`)
  } else {
    router.push(`/entity/${entity.type}/${entity.id}`)
  }
}

function createNew() {
  if (kanbanConfig.value?.create_form) {
    router.push(`/form/${kanbanConfig.value.create_form}`)
  }
}

// Lifecycle
onMounted(() => {
  loadEntities()
})

watch(() => props.id, () => {
  loadEntities()
})

// Watch for SSE cache invalidation to reload entities
watch(() => entitiesStore.cacheVersion, () => {
  loadEntities()
})
</script>

<template>
  <div class="kanban-view">
    <header class="page-header">
      <div class="header-left">
        <BackButton v-if="backTarget" :target="backTarget" />
        <h1>{{ kanbanConfig?.title || props.id }}</h1>
      </div>
      <div class="header-actions">
        <button v-if="kanbanConfig?.create_form && canCreate()" class="btn btn-primary" @click="createNew">
          + New
        </button>
      </div>
    </header>

    <!-- Filter controls -->
    <div v-if="kanbanConfig?.filter_controls?.length" class="filter-bar">
      <div v-for="control in kanbanConfig.filter_controls" :key="control.property" class="filter-group">
        <label>{{ control.label || control.property }}</label>
        <select v-model="filterValues[control.property || '']">
          <option value="">All</option>
          <option
            v-for="opt in filterOptions[control.property || '']"
            :key="opt"
            :value="opt"
          >
            {{ opt }}
          </option>
        </select>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div class="spinner"/>
      <span>Loading board...</span>
    </div>

    <!-- Simple board (columns only) -->
    <div v-else-if="!hasSwimmlanes" class="kanban-board">
      <div
        v-for="column in columns"
        :key="column.value"
        class="kanban-column"
        @dragover="onDragOver"
        @drop="onDrop($event, column.value)"
      >
        <div class="column-header">
          <span class="column-title">{{ column.label || column.value }}</span>
          <span class="column-count">{{ entitiesByColumn[column.value]?.length || 0 }}</span>
        </div>

        <div class="column-cards">
          <div
            v-for="entity in entitiesByColumn[column.value]"
            :key="entity.id"
            class="kanban-card"
            :draggable="canUpdate(entity) ? 'true' : 'false'"
            @dragstart="onDragStart($event, entity)"
            @dragend="onDragEnd"
            @click="openCard(entity)"
          >
            <div class="card-id">{{ entity.id }}</div>
            <div class="card-title">{{ getCardTitle(entity) }}</div>
            <div v-if="kanbanConfig?.card.fields?.length" class="card-fields">
              <div
                v-for="field in kanbanConfig.card.fields"
                :key="field.property"
                class="card-field"
              >
                <span class="field-label">{{ getCardFieldLabel(field) }}:</span>
                <Badge
                  v-if="isEnumField(field)"
                  :value="getCardFieldValue(entity, field)"
                  :property="field.property"
                  :entity-type="entityType"
                />
                <span v-else class="field-value">{{ getCardFieldValue(entity, field) || '-' }}</span>
              </div>
            </div>
          </div>

          <div v-if="!entitiesByColumn[column.value]?.length" class="empty-column">
            No items
          </div>
        </div>
      </div>
    </div>

    <!-- Swimlane board (2D grid layout) -->
    <div v-else class="kanban-swimlane-board" :style="swimlaneGridStyle">
      <!-- Column headers -->
      <div class="swimlane-header-row">
        <div class="swimlane-label-cell" />
        <div
          v-for="column in columns"
          :key="column.value"
          class="swimlane-column-header"
        >
          <span class="column-title">{{ column.label || column.value }}</span>
        </div>
      </div>

      <!-- Swimlane rows -->
      <div
        v-for="swimlane in swimlanes"
        :key="swimlane.value"
        class="swimlane-row"
      >
        <div class="swimlane-label-cell">
          <span class="swimlane-label">{{ swimlane.label || swimlane.value }}</span>
        </div>
        <div
          v-for="column in columns"
          :key="column.value"
          class="swimlane-cell"
          @dragover="onDragOver"
          @drop="onDrop($event, column.value, swimlane.value)"
        >
          <div
            v-for="entity in entitiesByCell[column.value]?.[swimlane.value] || []"
            :key="entity.id"
            class="kanban-card"
            draggable="true"
            @dragstart="onDragStart($event, entity)"
            @dragend="onDragEnd"
            @click="openCard(entity)"
          >
            <div class="card-id">{{ entity.id }}</div>
            <div class="card-title">{{ getCardTitle(entity) }}</div>
            <div v-if="kanbanConfig?.card.fields?.length" class="card-fields">
              <div
                v-for="field in kanbanConfig.card.fields"
                :key="field.property"
                class="card-field"
              >
                <span class="field-label">{{ getCardFieldLabel(field) }}:</span>
                <Badge
                  v-if="isEnumField(field)"
                  :value="getCardFieldValue(entity, field)"
                  :property="field.property"
                  :entity-type="entityType"
                />
                <span v-else class="field-value">{{ getCardFieldValue(entity, field) || '-' }}</span>
              </div>
            </div>
          </div>
          <div v-if="!(entitiesByCell[column.value]?.[swimlane.value]?.length)" class="empty-cell">
            —
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.kanban-view {
  max-width: 100%;
  overflow-x: auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-actions {
  display: flex;
  gap: 12px;
}

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
  background: #4f46e5;
}

.filter-bar {
  display: flex;
  gap: 16px;
  margin-bottom: 20px;
  padding: 12px 16px;
  background: var(--card-bg);
  border-radius: 8px;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.filter-group label {
  font-size: 12px;
  font-weight: 500;
  color: var(--muted-text);
  text-transform: uppercase;
}

.filter-group select {
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  min-width: 120px;
  background: var(--input-bg);
  color: var(--text-color);
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 48px;
  color: var(--muted-text);
}

.spinner {
  width: 24px;
  height: 24px;
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

.kanban-board {
  display: flex;
  gap: 16px;
  min-height: 500px;
  padding-bottom: 20px;
}

.kanban-column {
  flex: 1;
  min-width: 280px;
  max-width: 350px;
  background: var(--hover-bg);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
}

.column-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
}

.column-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-color);
}

.column-count {
  background: var(--border-color);
  color: var(--muted-text);
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
}

.column-cards {
  flex: 1;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow-y: auto;
}

.kanban-card {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 12px;
  cursor: grab;
  transition: all 0.15s;
}

.kanban-card:hover {
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.kanban-card:active {
  cursor: grabbing;
}

.card-id {
  font-family: monospace;
  font-size: 11px;
  color: var(--muted-text);
  margin-bottom: 4px;
}

.card-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-color);
  margin-bottom: 8px;
}

.card-fields {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.card-field {
  display: flex;
  gap: 4px;
  font-size: 12px;
}

.field-label {
  color: var(--muted-text);
}

.field-value {
  color: var(--text-color);
}

.empty-column {
  color: var(--muted-text);
  font-size: 13px;
  text-align: center;
  padding: 24px;
}

/* Swimlane board styles (2D grid layout) */
.kanban-swimlane-board {
  display: grid;
  /* grid-template-columns set via inline style */
  gap: 1px;
  background: var(--border-color);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
  min-height: 400px;
}

.swimlane-header-row {
  display: contents;
}

.swimlane-label-cell {
  background: var(--hover-bg);
  padding: 12px 16px;
  display: flex;
  align-items: center;
  min-width: 120px;
  max-width: 180px;
}

.swimlane-column-header {
  background: var(--hover-bg);
  padding: 12px 16px;
  text-align: center;
  font-weight: 600;
  font-size: 14px;
}

.swimlane-row {
  display: contents;
}

.swimlane-label {
  font-weight: 600;
  font-size: 13px;
  color: var(--text-color);
  writing-mode: horizontal-tb;
}

.swimlane-cell {
  background: var(--card-bg);
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 100px;
  overflow-y: auto;
}

.swimlane-cell:hover {
  background: var(--hover-bg);
}

.empty-cell {
  color: var(--muted-text);
  font-size: 12px;
  text-align: center;
  padding: 8px;
  opacity: 0.5;
}

@media (max-width: 768px) {
  .kanban-board {
    gap: 12px;
    min-height: 300px;
  }

  .kanban-column {
    min-width: 220px;
    max-width: 300px;
  }

  .column-header {
    padding: 10px 12px;
  }

  .column-cards {
    padding: 8px;
  }

  .kanban-card {
    padding: 10px;
  }

  .swimlane-label-cell {
    min-width: 100px;
    max-width: 140px;
  }
}
</style>
