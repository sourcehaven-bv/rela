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
  EntityViewConfig,
  KanbanConfig,
  DashboardConfig,
  NavigationEntry,
  AppConfig,
  DocumentConfig,
  ActionConfig,
} from '@/types'

export const useSchemaStore = defineStore('schema', () => {
  // State
  const entityTypes = ref<Map<string, EntityType>>(new Map())
  const relationTypes = ref<Map<string, RelationType>>(new Map())
  const customTypes = ref<Map<string, CustomType>>(new Map())
  const forms = ref<Map<string, FormConfig>>(new Map())
  const lists = ref<Map<string, ListConfig>>(new Map())
  const views = ref<Map<string, ViewConfig>>(new Map())
  // entityViewConfigs is the data-entry config's entity_views section: a
  // per-type binding of "the canonical detail view for entities of this type".
  // Distinct from `entityTypes` above, which is the metamodel definition.
  const entityViewConfigs = ref<Map<string, EntityViewConfig>>(new Map())
  const kanbans = ref<Map<string, KanbanConfig>>(new Map())
  const documents = ref<Map<string, DocumentConfig>>(new Map())
  const actions = ref<Map<string, ActionConfig>>(new Map())
  const dashboard = ref<DashboardConfig | undefined>(undefined)
  const navigation = ref<NavigationEntry[]>([])
  const app = ref<AppConfig>({ name: 'rela' })
  const styles = ref<Record<string, Record<string, string>>>({})
  const paletteLight = ref<Record<string, string>>({})
  const paletteDark = ref<Record<string, string>>({})
  const darkDisabled = ref(false)
  const loaded = ref(false)
  const loading = ref(false)
  const error = ref<string | null>(null)
  // In-flight promise shared between concurrent callers of load().
  // Without this, a second call to load() while the first is still
  // awaiting its fetch would see `loading === true`, return immediately
  // with `loaded === false`, and its caller would proceed without a
  // schema — leaving the SPA stuck on the Loading... spinner. See the
  // fuzzer findings around rapid navigation.
  let loadPromise: Promise<void> | null = null

  // Getters
  const getEntityType = computed(() => (name: string) => entityTypes.value.get(name))
  const getRelationType = computed(() => (name: string) => relationTypes.value.get(name))
  const getForm = computed(() => (id: string) => forms.value.get(id))
  const getList = computed(() => (id: string) => lists.value.get(id))
  // Find the first list ID that shows entities of the given type.
  // Returns undefined if no list is configured for that type.
  const findListIdForEntityType = computed(() => (entityType: string) => {
    for (const [id, cfg] of lists.value.entries()) {
      if (cfg.entity === entityType) return id
    }
    return undefined
  })
  const getView = computed(() => (id: string) => views.value.get(id))
  // getEntityDetailView returns the canonical detail view id for an entity
  // type, or undefined if none is configured. Consumers building entity
  // links should fall back to /entity/:type/:id when undefined.
  const getEntityDetailView = computed(
    () => (type: string) => entityViewConfigs.value.get(type)?.detail_view
  )
  const getKanban = computed(() => (id: string) => kanbans.value.get(id))
  const getAction = computed(() => (id: string) => actions.value.get(id))

  const entityTypeList = computed(() => Array.from(entityTypes.value.entries()))
  const relationTypeList = computed(() => Array.from(relationTypes.value.entries()))

  // Actions
  async function load(): Promise<void> {
    if (loaded.value) return
    // Share one in-flight promise across concurrent callers. The old
    // guard `if (loading.value) return` returned an already-resolved
    // undefined to the second caller, which then proceeded as if the
    // load had completed.
    if (loadPromise) return loadPromise
    loadPromise = doLoad().finally(() => {
      loadPromise = null
    })
    return loadPromise
  }

  async function doLoad(): Promise<void> {
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
      styles.value = configData.styles || {}
      forms.value = new Map(Object.entries(configData.forms || {}))
      lists.value = new Map(Object.entries(configData.lists || {}))
      views.value = new Map(Object.entries(configData.views || {}))
      entityViewConfigs.value = new Map(Object.entries(configData.entity_views || {}))
      kanbans.value = new Map(Object.entries(configData.kanbans || {}))
      documents.value = new Map(Object.entries(configData.documents || {}))
      actions.value = new Map(Object.entries(configData.actions || {}))
      dashboard.value = configData.dashboard
      navigation.value = configData.navigation || []

      // Apply palette if present
      if (configData.palette) {
        paletteLight.value = configData.palette.light || {}
        paletteDark.value = configData.palette.dark || {}
        darkDisabled.value = configData.palette.darkDisabled || false
      } else {
        paletteLight.value = {}
        paletteDark.value = {}
        darkDisabled.value = false
      }

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
    entityViewConfigs,
    kanbans,
    documents,
    actions,
    dashboard,
    navigation,
    app,
    styles,
    paletteLight,
    paletteDark,
    darkDisabled,
    loaded,
    loading,
    error,

    // Getters
    getEntityType,
    getRelationType,
    getForm,
    getList,
    findListIdForEntityType,
    getView,
    getEntityDetailView,
    getKanban,
    getAction,
    entityTypeList,
    relationTypeList,

    // Actions
    load,
    reload,
  }
})
