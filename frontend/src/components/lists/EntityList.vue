<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { useListKeyboard } from '@/composables/useListKeyboard'
import { toApiOperator } from '@/utils/filters'
import { getCellValue, formatCellValue, isEnumPropertyDef } from '@/utils/format'
import type { Entity, ListMeta, ListParams } from '@/types'
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
const includedEntities = ref<Record<string, Entity>>({})

// Sort specs: array of { property, direction } for multi-field sorting
interface SortSpec {
  property: string
  direction: 'asc' | 'desc'
}
const sortSpecs = ref<SortSpec[]>([])

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

// Pre-configured filters from list config
const configuredFilters = computed(() => {
  return listConfig.value?.filters?.filter(f => f.operator && f.value) || []
})

// Check if any columns reference relations (need to include related entities)
const hasRelationColumns = computed(() => {
  return listConfig.value?.columns?.some(col => col.relation) || false
})

// Build query params
const queryParams = computed((): ListParams => {
  const params: ListParams = {
    page: meta.value.page,
    per_page: listConfig.value?.page_size || 25,
  }

  // Add pre-configured filters from list config
  for (const filter of configuredFilters.value) {
    const apiOp = toApiOperator(filter.operator)
    const key = `filter[${filter.property}][${apiOp}]`
    const filterValue = filter.value
    // Append to existing filter or create new
    const existing = (params as Record<string, string | number | undefined>)[key]
    if (existing) {
      (params as Record<string, string | number | undefined>)[key] = `${existing},${filterValue}`
    } else {
      (params as Record<string, string | number | undefined>)[key] = filterValue as string
    }
  }

  // Add user-selected filters
  for (const [key, value] of Object.entries(filters.value)) {
    if (value) {
      (params as Record<string, string | number | undefined>)[`filter[${key}]`] = value
    }
  }

  // Add sorting - supports multi-field sorting
  if (sortSpecs.value.length > 0) {
    params.sort = sortSpecs.value
      .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
      .join(',')
  } else if (listConfig.value?.default_sort?.length) {
    const defaultSort = listConfig.value.default_sort
      .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
      .join(',')
    params.sort = defaultSort
  }

  // Include related entities for relation columns
  if (hasRelationColumns.value) {
    params.include = '*'
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
    // Store included entities for relation column rendering
    includedEntities.value = result.included || {}
  } catch (err) {
    uiStore.error('Failed to load entities')
    console.error(err)
  } finally {
    loading.value = false
  }
}

function handleSort(field: string, event: MouseEvent) {
  const existingIndex = sortSpecs.value.findIndex((s) => s.property === field)

  if (event.shiftKey) {
    // Multi-sort: add/toggle field while keeping others
    if (existingIndex >= 0) {
      // Toggle direction if already in list
      const spec = sortSpecs.value[existingIndex]
      if (spec.direction === 'asc') {
        spec.direction = 'desc'
      } else {
        // Remove from sort if clicking desc again
        sortSpecs.value.splice(existingIndex, 1)
      }
    } else {
      // Add new sort field
      sortSpecs.value.push({ property: field, direction: 'asc' })
    }
  } else {
    // Single-sort: replace all with just this field
    if (existingIndex >= 0 && sortSpecs.value.length === 1) {
      // Toggle direction if already the only sort
      sortSpecs.value[0].direction = sortSpecs.value[0].direction === 'asc' ? 'desc' : 'asc'
    } else {
      // Replace all sorts with just this field
      sortSpecs.value = [{ property: field, direction: 'asc' }]
    }
  }

  meta.value.page = 1
  loadEntities()
}

// Helper to get sort index and direction for a field
function getSortInfo(field: string): { index: number; direction: 'asc' | 'desc' | null } {
  const idx = sortSpecs.value.findIndex((s) => s.property === field)
  if (idx < 0) return { index: -1, direction: null }
  return { index: idx, direction: sortSpecs.value[idx].direction }
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
  // Build query params to preserve navigation context
  const query: Record<string, string> = {
    from: props.listId,
    scope: `list:${props.listId}`,
  }

  // Include sort if active
  if (sortSpecs.value.length > 0) {
    query.sort = sortSpecs.value
      .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
      .join(',')
  } else if (listConfig.value?.default_sort?.length) {
    query.sort = listConfig.value.default_sort
      .map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property))
      .join(',')
  }

  // Include active filters
  for (const [key, value] of Object.entries(filters.value)) {
    if (value) {
      query[`filter_${key}`] = value
    }
  }

  router.push({
    path: `/entity/${entity.type}/${entity.id}`,
    query,
  })
}

function isEnumColumn(column: { property?: string }): boolean {
  if (!column.property || !entityType.value) return false
  return isEnumPropertyDef(entityType.value.properties[column.property])
}

function getFormattedCellValue(entity: Entity, column: { property?: string; relation?: string }): string {
  // For relation columns, resolve IDs to titles using included entities
  if (column.relation) {
    const relationIds = entity.relations?.[column.relation] || []
    const titles = relationIds.map((id) => {
      const included = includedEntities.value[id]
      return included?._title || included?.properties?.title || id
    })
    return titles.join(', ')
  }

  const value = getCellValue(entity, column)
  return formatCellValue(value, column.property, entityType.value)
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
  sortSpecs.value = []
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
  <div v-if="listConfig" class="entity-list">
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

    <div v-if="configuredFilters.length" class="configured-filters">
      <span
        v-for="filter in configuredFilters"
        :key="`${filter.property}-${filter.operator}-${filter.value}`"
        class="filter-chip"
      >
        {{ filter.property }} {{ filter.operator }} {{ filter.value }}
      </span>
    </div>

    <div class="list-content">
      <FilterBar
        v-if="listConfig.filter_controls?.length"
        :config="listConfig"
        :entity-type="entityType"
        :filters="filters"
        @filter="handleFilter"
      />
      <div v-if="loading" class="loading-state">
        <div class="spinner"/>
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
                sorted: getSortInfo(column.property || '').index >= 0,
                'sorted-desc': getSortInfo(column.property || '').direction === 'desc',
              }"
              @click="column.sortable !== false && column.property && handleSort(column.property, $event)"
            >
              {{ column.label || column.property || column.relation }}
              <span v-if="getSortInfo(column.property || '').index >= 0" class="sort-indicator">
                <span v-if="sortSpecs.length > 1" class="sort-order">{{ getSortInfo(column.property || '').index + 1 }}</span>
                {{ getSortInfo(column.property || '').direction === 'desc' ? '▼' : '▲' }}
              </span>
            </th>
            <th class="actions-column"/>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(entity, index) in entities"
            :key="entity.id"
            class="entity-row"
            :class="{ selected: index === selectedIndex }"
            @click="navigateToEntity(entity)"
          >
            <td
              v-for="column in listConfig.columns"
              :key="column.property || column.relation"
            >
              <Badge
                v-if="isEnumColumn(column)"
                :value="String(getCellValue(entity, column) || '')"
                :property="column.property"
                :entity-type="entityType"
              />
              <span v-else>
                {{ getFormattedCellValue(entity, column) }}
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
  filter: brightness(0.9);
}

.btn-secondary {
  background: var(--border-color);
  color: var(--text-color);
}

.btn-secondary:hover {
  background: var(--hover-bg);
}

.configured-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
  margin-bottom: 20px;
}

.filter-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  background: var(--hover-bg);
  border: 1px solid var(--border-color);
  border-radius: 16px;
  font-size: 12px;
  color: var(--text-color);
}

.list-content {
  background: var(--card-bg);
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
  color: var(--muted-text);
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
  background: var(--hover-bg);
  border-bottom: 1px solid var(--border-color);
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted-text);
}

.entity-table th.sortable {
  cursor: pointer;
  user-select: none;
}

.entity-table th.sortable:hover {
  filter: brightness(0.95);
}

.entity-table th.sorted {
  color: var(--accent-color);
}

.sort-indicator {
  margin-left: 4px;
  font-size: 10px;
}

.sort-order {
  font-size: 9px;
  background: var(--accent-color);
  color: white;
  padding: 1px 4px;
  border-radius: 8px;
  margin-right: 2px;
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
  background: var(--hover-bg);
}

.entity-row.selected {
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  outline: 2px solid var(--accent-color);
  outline-offset: -2px;
}

.entity-row.selected:hover {
  background: color-mix(in srgb, var(--accent-color) 25%, transparent);
}

.error-state {
  padding: 48px;
  text-align: center;
  color: var(--muted-text);
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
  color: var(--muted-text);
  cursor: pointer;
  transition: all 0.15s;
}

.delete-btn:hover {
  background: color-mix(in srgb, var(--error-color) 15%, transparent);
  color: var(--error-color);
}
</style>
