import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getSchema, getConfig } from '@/api/schema'
import type {
  EntityType,
  RelationType,
  CustomType,
  FormConfig,
  ListConfig,
  ViewConfig,
  KanbanConfig,
  DashboardConfig,
  NavigationEntry,
  AppConfig,
} from '@/types'

export const useSchemaStore = defineStore('schema', () => {
  // State
  const entityTypes = ref<Map<string, EntityType>>(new Map())
  const relationTypes = ref<Map<string, RelationType>>(new Map())
  const customTypes = ref<Map<string, CustomType>>(new Map())
  const forms = ref<Map<string, FormConfig>>(new Map())
  const lists = ref<Map<string, ListConfig>>(new Map())
  const views = ref<Map<string, ViewConfig>>(new Map())
  const kanbans = ref<Map<string, KanbanConfig>>(new Map())
  const dashboard = ref<DashboardConfig | undefined>(undefined)
  const navigation = ref<NavigationEntry[]>([])
  const app = ref<AppConfig>({ name: 'rela' })
  const loaded = ref(false)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Getters
  const getEntityType = computed(() => (name: string) => entityTypes.value.get(name))
  const getRelationType = computed(() => (name: string) => relationTypes.value.get(name))
  const getForm = computed(() => (id: string) => forms.value.get(id))
  const getList = computed(() => (id: string) => lists.value.get(id))
  const getView = computed(() => (id: string) => views.value.get(id))
  const getKanban = computed(() => (id: string) => kanbans.value.get(id))

  const entityTypeList = computed(() => Array.from(entityTypes.value.entries()))
  const relationTypeList = computed(() => Array.from(relationTypes.value.entries()))

  // Actions
  async function load() {
    if (loaded.value || loading.value) return

    loading.value = true
    error.value = null

    try {
      const [schemaData, configData] = await Promise.all([getSchema(), getConfig()])

      // Schema
      entityTypes.value = new Map(Object.entries(schemaData.entities || {}))
      relationTypes.value = new Map(Object.entries(schemaData.relations || {}))
      customTypes.value = new Map(Object.entries(schemaData.types || {}))

      // Config
      app.value = configData.app || { name: 'rela' }
      forms.value = new Map(Object.entries(configData.forms || {}))
      lists.value = new Map(Object.entries(configData.lists || {}))
      views.value = new Map(Object.entries(configData.views || {}))
      kanbans.value = new Map(Object.entries(configData.kanbans || {}))
      dashboard.value = configData.dashboard
      navigation.value = configData.navigation || []

      loaded.value = true
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load schema'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function reload() {
    loaded.value = false
    await load()
  }

  return {
    // State
    entityTypes,
    relationTypes,
    customTypes,
    forms,
    lists,
    views,
    kanbans,
    dashboard,
    navigation,
    app,
    loaded,
    loading,
    error,

    // Getters
    getEntityType,
    getRelationType,
    getForm,
    getList,
    getView,
    getKanban,
    entityTypeList,
    relationTypeList,

    // Actions
    load,
    reload,
  }
})
