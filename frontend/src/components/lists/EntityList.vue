<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { useListKeyboard } from '@/composables/useListKeyboard'
import type { Entity, ListMeta, EntityType, ListParams } from '@/types'
import FilterBar from './FilterBar.vue'
import Pagination from './Pagination.vue'
import Badge from '@/components/common/Badge.vue'

const props = defineProps<{
  listId: string
}>()

const router = useRouter()
const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()
const uiStore = useUIStore()

// State
const entities = ref<Entity[]>([])
const meta = ref<ListMeta>({ total: 0, page: 1, per_page: 25, has_more: false })
const loading = ref(true)
const filters = ref<Record<string, string>>({})
const sortField = ref<string | null>(null)
const sortDesc = ref(false)

// Computed for keyboard navigation
const itemCount = computed(() => entities.value.length)
const hasPrevPage = computed(() => meta.value.page > 1)
const hasNextPage = computed(() => meta.value.has_more || meta.value.page * meta.value.per_page < meta.value.total)

// Keyboard navigation
const { selectedIndex, clearSelection } = useListKeyboard({
  itemCount,
  hasPrevPage,
  hasNextPage,
  onOpen: (index) => {
    const entity = entities.value[index]
    if (entity) navigateToEntity(entity)
  },
  onEdit: (index) => {
    const entity = entities.value[index]
    if (entity && listConfig.value?.edit_form) {
      router.push(`/form/${listConfig.value.edit_form}/${entity.id}`)
    }
  },
  onCreate: () => {
    if (listConfig.value?.create_form) {
      router.push(`/form/${listConfig.value.create_form}`)
    }
  },
  onPrevPage: () => {
    if (hasPrevPage.value) {
      handlePageChange(meta.value.page - 1)
    }
  },
  onNextPage: () => {
    if (hasNextPage.value) {
      handlePageChange(meta.value.page + 1)
    }
  },
})

// Computed
const listConfig = computed(() => schemaStore.getList(props.listId))
const entityType = computed(() => {
  if (!listConfig.value) return undefined
  return schemaStore.getEntityType(listConfig.value.entity)
})

// Build query params
const queryParams = computed((): ListParams => {
  const params: ListParams = {
    page: meta.value.page,
    per_page: listConfig.value?.page_size || 25,
  }

  // Add filters
  for (const [key, value] of Object.entries(filters.value)) {
    if (value) {
      (params as Record<string, string | number | undefined>)[`filter[${key}]`] = value
    }
  }

  // Add sorting
  if (sortField.value) {
    params.sort = sortDesc.value ? `-${sortField.value}` : sortField.value
  } else if (listConfig.value?.default_sort?.length) {
    const defaultSort = listConfig.value.default_sort
      .map((s) => (s.direction === 'desc' ? `-${s.field}` : s.field))
      .join(',')
    params.sort = defaultSort
  }

  return params
})

// Methods
async function loadEntities() {
  if (!listConfig.value) return

  loading.value = true
  try {
    const result = await entitiesStore.fetchList(
      listConfig.value.entity,
      queryParams.value
    )
    entities.value = result.data
    meta.value = result.meta
  } catch (err) {
    uiStore.error('Failed to load entities')
    console.error(err)
  } finally {
    loading.value = false
  }
}

function handleSort(field: string) {
  if (sortField.value === field) {
    sortDesc.value = !sortDesc.value
  } else {
    sortField.value = field
    sortDesc.value = false
  }
  meta.value.page = 1
  loadEntities()
}

function handleFilter(newFilters: Record<string, string>) {
  filters.value = newFilters
  meta.value.page = 1
  loadEntities()
}

function handlePageChange(page: number) {
  meta.value.page = page
  loadEntities()
}

function navigateToEntity(entity: Entity) {
  router.push(`/entity/${entity.type}/${entity.id}`)
}

function getCellValue(entity: Entity, column: { property?: string; relation?: string; direction?: string }): unknown {
  if (column.property) {
    if (column.property === 'id') return entity.id
    return entity.properties[column.property]
  }
  if (column.relation && entity.relations) {
    return entity.relations[column.relation] || []
  }
  return ''
}

function formatCellValue(value: unknown, column: { property?: string; relation?: string }, entityType?: EntityType): string {
  if (value === null || value === undefined) return ''

  if (Array.isArray(value)) {
    return value.join(', ')
  }

  if (column.property && entityType) {
    const propDef = entityType.properties[column.property]
    if (propDef?.type === 'date' && typeof value === 'string') {
      return new Date(value).toLocaleDateString()
    }
    if (propDef?.type === 'boolean') {
      return value ? 'Yes' : 'No'
    }
  }

  return String(value)
}

function isEnumProperty(column: { property?: string }, entityType?: EntityType): boolean {
  if (!column.property || !entityType) return false
  const propDef = entityType.properties[column.property]
  return propDef?.type === 'enum' || (propDef?.values?.length ?? 0) > 0
}

async function handleDelete(entity: Entity, event: Event) {
  event.stopPropagation()
  const confirmed = window.confirm(`Are you sure you want to delete "${entity.id}"?`)
  if (!confirmed) return

  try {
    await entitiesStore.remove(entity.type, entity.id)
    uiStore.success(`Deleted ${entity.id}`)
    loadEntities()
  } catch (err) {
    uiStore.error('Failed to delete entity')
    console.error(err)
  }
}

// Watchers
watch(() => props.listId, () => {
  filters.value = {}
  sortField.value = null
  sortDesc.value = false
  meta.value.page = 1
  clearSelection()
  loadEntities()
})

// Clear selection when entities change
watch(entities, () => {
  clearSelection()
})

// Lifecycle
onMounted(() => {
  loadEntities()
})
</script>

<template>
  <div class="entity-list" v-if="listConfig">
    <header class="list-header">
      <h1>{{ listConfig.title || listConfig.entity }}</h1>
      <router-link
        v-if="listConfig.create_form"
        :to="`/form/${listConfig.create_form}`"
        class="btn btn-primary"
      >
        + New <kbd>N</kbd>
      </router-link>
    </header>

    <FilterBar
      v-if="listConfig.filters?.length"
      :config="listConfig"
      :entity-type="entityType"
      :filters="filters"
      @filter="handleFilter"
    />

    <div class="list-content">
      <div v-if="loading" class="loading-state">
        <div class="spinner"></div>
        <span>Loading...</span>
      </div>

      <div v-else-if="entities.length === 0" class="empty-state">
        <p>No {{ listConfig.entity }}s found.</p>
        <router-link v-if="listConfig.create_form" :to="`/form/${listConfig.create_form}`" class="btn btn-secondary">
          Create one
        </router-link>
      </div>

      <table v-else class="entity-table">
        <thead>
          <tr>
            <th
              v-for="column in listConfig.columns"
              :key="column.property || column.relation"
              :class="{
                sortable: column.sortable !== false && column.property,
                sorted: sortField === column.property,
                'sorted-desc': sortField === column.property && sortDesc,
              }"
              @click="column.sortable !== false && column.property && handleSort(column.property)"
            >
              {{ column.label || column.property || column.relation }}
              <span v-if="sortField === column.property" class="sort-indicator">
                {{ sortDesc ? '▼' : '▲' }}
              </span>
            </th>
            <th class="actions-column"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(entity, index) in entities"
            :key="entity.id"
            @click="navigateToEntity(entity)"
            class="entity-row"
            :class="{ selected: index === selectedIndex }"
          >
            <td
              v-for="column in listConfig.columns"
              :key="column.property || column.relation"
            >
              <Badge
                v-if="isEnumProperty(column, entityType)"
                :value="String(getCellValue(entity, column) || '')"
                :property="column.property"
                :entity-type="entityType"
              />
              <span v-else>
                {{ formatCellValue(getCellValue(entity, column), column, entityType) }}
              </span>
            </td>
            <td class="actions-cell">
              <button
                class="delete-btn"
                title="Delete"
                @click="handleDelete(entity, $event)"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="3 6 5 6 21 6"/>
                  <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/>
                </svg>
              </button>
            </td>
          </tr>
        </tbody>
      </table>

      <Pagination
        v-if="meta.total > meta.per_page"
        :meta="meta"
        @page-change="handlePageChange"
      />
    </div>
  </div>

  <div v-else class="error-state">
    <h2>List not found</h2>
    <p>The list "{{ listId }}" does not exist in the configuration.</p>
  </div>
</template>

<style scoped>
.entity-list {
  max-width: 1200px;
}

.list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.list-header h1 {
  margin: 0;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  text-decoration: none;
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

.btn-secondary {
  background: var(--border-color, #e2e8f0);
  color: var(--text-color, #1e293b);
}

.btn-secondary:hover {
  background: #cbd5e1;
}

.list-content {
  background: white;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  overflow: hidden;
}

.loading-state,
.empty-state {
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

.entity-table {
  width: 100%;
  border-collapse: collapse;
}

.entity-table th {
  text-align: left;
  padding: 12px 16px;
  background: #f8fafc;
  border-bottom: 1px solid var(--border-color);
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: #64748b;
}

.entity-table th.sortable {
  cursor: pointer;
  user-select: none;
}

.entity-table th.sortable:hover {
  background: #f1f5f9;
}

.entity-table th.sorted {
  color: var(--accent-color);
}

.sort-indicator {
  margin-left: 4px;
  font-size: 10px;
}

.entity-table td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
  font-size: 14px;
}

.entity-row {
  cursor: pointer;
  transition: background 0.15s;
}

.entity-row:hover {
  background: #f8fafc;
}

.entity-row.selected {
  background: #e0e7ff;
  outline: 2px solid var(--accent-color, #6366f1);
  outline-offset: -2px;
}

.entity-row.selected:hover {
  background: #c7d2fe;
}

.error-state {
  padding: 48px;
  text-align: center;
  color: #64748b;
}

.error-state h2 {
  color: var(--error-color, #ef4444);
}

.actions-column {
  width: 40px;
}

.actions-cell {
  width: 40px;
  white-space: nowrap;
}

.delete-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: #94a3b8;
  cursor: pointer;
  transition: all 0.15s;
}

.delete-btn:hover {
  background: #fee2e2;
  color: var(--error-color, #ef4444);
}
</style>
