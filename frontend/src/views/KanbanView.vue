<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useSchemaStore } from '@/stores'
import { listEntities, updateEntity } from '@/api'
import type { Entity, KanbanConfig } from '@/types'
import Badge from '@/components/common/Badge.vue'

const props = defineProps<{
  id: string
}>()

const router = useRouter()
const schemaStore = useSchemaStore()

// State
const loading = ref(true)
const entities = ref<Entity[]>([])
const filterValues = ref<Record<string, string>>({})
const draggedCard = ref<Entity | null>(null)

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

async function onDrop(event: DragEvent, columnValue: string) {
  event.preventDefault()

  if (!draggedCard.value || !kanbanConfig.value) return

  const entity = draggedCard.value
  const property = kanbanConfig.value.column_property

  // Don't update if same column
  if (String(entity.properties[property] || '') === columnValue) {
    draggedCard.value = null
    return
  }

  // Optimistic update
  const oldValue = entity.properties[property]
  entity.properties[property] = columnValue

  try {
    await updateEntity(kanbanConfig.value.entity, entity.id, {
      properties: { [property]: columnValue },
    })
  } catch (err) {
    // Revert on error
    entity.properties[property] = oldValue
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
</script>

<template>
  <div class="kanban-view">
    <header class="page-header">
      <h1>{{ kanbanConfig?.title || props.id }}</h1>
      <div class="header-actions">
        <button v-if="kanbanConfig?.create_form" class="btn btn-primary" @click="createNew">
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
      <div class="spinner"></div>
      <span>Loading board...</span>
    </div>

    <div v-else class="kanban-board">
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

          <div v-if="!entitiesByColumn[column.value]?.length" class="empty-column">
            No items
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
  background: #f8fafc;
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
  color: #64748b;
  text-transform: uppercase;
}

.filter-group select {
  padding: 6px 10px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 14px;
  min-width: 120px;
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 48px;
  color: #64748b;
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
  background: #f1f5f9;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
}

.column-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #e2e8f0;
}

.column-title {
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
}

.column-count {
  background: #e2e8f0;
  color: #64748b;
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
  background: white;
  border: 1px solid #e2e8f0;
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
  color: #64748b;
  margin-bottom: 4px;
}

.card-title {
  font-size: 14px;
  font-weight: 500;
  color: #1e293b;
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
  color: #64748b;
}

.field-value {
  color: #475569;
}

.empty-column {
  color: #94a3b8;
  font-size: 13px;
  text-align: center;
  padding: 24px;
}
</style>
