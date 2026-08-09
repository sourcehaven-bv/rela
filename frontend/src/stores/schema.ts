import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getSchema, getConfig } from '@/api/schema'
import { getDashboard } from '@/api/dashboard'
import { registerEntityPlurals } from '@/api/entities'
import { getErrorMessage } from '@/api/errors'
import type {
  EntityType,
  PropertyDef,
  RelationType,
  CustomType,
  FormConfig,
  ListConfig,
  ViewConfig,
  KanbanConfig,
  DashboardResponse,
  NavigationEntry,
  AppConfig,
  AppEntry,
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
  const kanbans = ref<Map<string, KanbanConfig>>(new Map())
  const documents = ref<Map<string, DocumentConfig>>(new Map())
  const apps = ref<Map<string, AppEntry>>(new Map())
  const actions = ref<Map<string, ActionConfig>>(new Map())
  // The PER-PRINCIPAL dashboard from `/_dashboard`, not the verbatim
  // `dashboard:` block on `/_config` (TKT-53KICM). Cards the caller cannot use
  // are already omitted server-side; never re-derive visibility here.
  const dashboard = ref<DashboardResponse | undefined>(undefined)
  const navigation = ref<NavigationEntry[]>([])
  const app = ref<AppConfig>({ name: 'rela' })
  // The deployment description for the global "About" help (TKT-DUQBD0): the
  // data-entry.yaml app.description, falling back to the metamodel description.
  // Distinct from app.description so SettingsView's one-liner is unaffected.
  const aboutDescription = ref('')
  const styles = ref<Record<string, Record<string, string>>>({})
  const paletteLight = ref<Record<string, string>>({})
  const paletteDark = ref<Record<string, string>>({})
  const darkDisabled = ref(false)
  // Sidebar logo. Fed initially by Sidebar's `_sidebar` fetch on mount,
  // then mutated by SettingsView's upload/remove handlers so the
  // sidebar updates without a page reload.
  const logoUrl = ref<string | null>(null)
  // Entity type -> form id for inline creation, fed by Sidebar's
  // `_sidebar` fetch (the only principal-scoped boot payload). Empty
  // until that lands, which is why the offer simply does not render on
  // the first paint rather than flickering on.
  const inlineCreate = ref<Record<string, string>>({})
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
  // Look up a relation type's inverse name (e.g., "blocks" → "blockedBy").
  // Returns undefined when the relation has no declared inverse. Used by
  // the unified-PATCH builder to emit incoming-direction edits under the
  // inverse body key, so the backend's resolveDirection picks them up
  // as "path entity is target" writes.
  const getInverseName = computed(
    () => (name: string) => relationTypes.value.get(name)?.inverse?.id,
  )
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
  const getKanban = computed(() => (id: string) => kanbans.value.get(id))
  const getAction = computed(() => (id: string) => actions.value.get(id))

  const entityTypeList = computed(() => Array.from(entityTypes.value.entries()))
  const relationTypeList = computed(() => Array.from(relationTypes.value.entries()))

  // Resolve the enum labels map for a property. A property may either name a
  // custom type (labels live on the custom type) or declare an inline enum
  // (labels live on the property def). The `entityType` disambiguator may be a
  // type-name string OR a resolved EntityType object (callers hold one or the
  // other); when given we look up that type's property first.
  //
  // When no entityType is given (e.g. the cross-type search filter, which has
  // no single owning type — see AdHocFilterMenu's documented union) we scan
  // entity then relation types and return the FIRST matching property name that
  // has labels. That is a deliberate first-wins tie-break: if two types define
  // labels for the same property name with different text, the first-inserted
  // type wins. Labels are display-only, so this is acceptable for v1; callers
  // that know their type should pass it to avoid the ambiguity. (Note this is a
  // per-type-def scan, NOT the flat server-authored `styles` map — the two are
  // separate mechanisms that merely happen to both collapse by property name.)
  function labelsForProperty(
    property: string,
    entityType?: string | EntityType,
  ): Record<string, string> | undefined {
    return scanPropertyDefs(property, entityType, (def) => {
      if (!def) return undefined
      if (def.labels) return def.labels
      // Property references a custom type → labels live on that custom type.
      const ct = def.type ? customTypes.value.get(def.type) : undefined
      return ct?.labels
    })
  }

  // Shared resolution scaffold for the per-property lookups above/below:
  // walk the property defs named `property` (given entityType first, then
  // entity then relation types) and return the first def the extractor
  // accepts. A def the extractor rejects does NOT stop the scan — first-wins
  // is on "first def that yields a result", which is what makes the
  // labelsForProperty tie-break documented above hold identically for every
  // extractor sharing this walk.
  function scanPropertyDefs<T>(
    property: string,
    entityType: string | EntityType | undefined,
    fromDef: (def?: PropertyDef) => T | undefined,
  ): T | undefined {
    if (entityType) {
      const et = typeof entityType === 'string' ? entityTypes.value.get(entityType) : entityType
      const hit = fromDef(et?.properties?.[property])
      if (hit) return hit
    }
    for (const [, def] of entityTypes.value) {
      const hit = fromDef(def.properties?.[property])
      if (hit) return hit
    }
    for (const [, def] of relationTypes.value) {
      const hit = fromDef(def.properties?.[property])
      if (hit) return hit
    }
    return undefined
  }

  // Resolve the custom-type name a property refers to, or undefined when the
  // property is an inline enum / built-in type.
  function customTypeNameForProperty(
    property: string,
    entityType?: string | EntityType,
  ): string | undefined {
    return scanPropertyDefs(property, entityType, (def) =>
      def?.type && customTypes.value.has(def.type) ? def.type : undefined,
    )
  }

  // Resolve the badge style map (value → badge class) for a property. The
  // server keys `styles` by CUSTOM-TYPE name (buildStyleMap; validateStyles
  // rejects other keys), so resolve the property to its custom type first.
  // The direct-key fallback is load-bearing, not defensive: EntityDetail's
  // view sections pass the property's TYPE name as `property` (sections.go
  // populates PropType from the def), which is already the styles key and
  // matches no property def — the fallback is the only path that resolves
  // it. It also covers a property whose name coincides with its type.
  const stylesForProperty = computed(
    () =>
      (
        property: string,
        entityType?: string | EntityType,
      ): Record<string, string> | undefined => {
        const typeName = customTypeNameForProperty(property, entityType)
        return (typeName ? styles.value[typeName] : undefined) ?? styles.value[property]
      },
  )

  // Return the display label for an enum value, or undefined when no label is
  // configured (caller falls back to the raw value). Display-only: the value
  // stays the wire identity.
  const getEnumLabel = computed(
    () =>
      (
        value: string,
        property?: string,
        entityType?: string | EntityType,
      ): string | undefined => {
        if (!property) return undefined
        return labelsForProperty(property, entityType)?.[value]
      },
  )

  // Resolve the value→label map to feed an enum picker (select / multi-select /
  // filter dropdown). Single source of truth so the widgets and filter bars
  // don't each reimplement the inline-vs-custom-type resolution and drift. A
  // property def's own `labels` (populated by the server for both inline enums
  // and custom-type-backed properties) wins; otherwise fall back to a
  // schema-store lookup by (property, entityType) for callers that only hold a
  // property name. Values without a label are omitted (caller shows them raw).
  const resolveOptionLabels = computed(
    () =>
      (
        propertyDef: PropertyDef | undefined,
        property: string,
        entityType?: string | EntityType,
      ): Record<string, string> => {
        const inline = propertyDef?.labels
        if (inline && Object.keys(inline).length > 0) return inline
        const out: Record<string, string> = {}
        for (const value of propertyDef?.values ?? []) {
          const label = labelsForProperty(property, entityType)?.[value]
          if (label !== undefined) out[value] = label
        }
        return out
      },
  )

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
      // Fetched alongside schema/config rather than on dashboard entry, so the
      // per-principal card list costs no extra round-trip on the critical path
      // and repeat visits to /dashboard stay free.
      const [schemaData, configData, dashboardData] = await Promise.all([
        getSchema(),
        getConfig(),
        getDashboard(),
      ])

      // Schema
      entityTypes.value = new Map(Object.entries(schemaData.entities || {}))
      relationTypes.value = new Map(Object.entries(schemaData.relations || {}))
      customTypes.value = new Map(Object.entries(schemaData.types || {}))

      // Feed the API layer's plural registry so it doesn't have to import
      // this store (B1a). Mirror the server's GetPlural fallback (type+'s')
      // for entity types that don't declare an explicit plural.
      const plurals = new Map<string, string>()
      for (const [type, def] of entityTypes.value) {
        plurals.set(type, def.plural || `${type}s`)
      }
      registerEntityPlurals(plurals)

      // Config
      app.value = configData.app || { name: 'rela' }
      aboutDescription.value = configData.about_description || ''
      styles.value = configData.styles || {}
      forms.value = new Map(Object.entries(configData.forms || {}))
      lists.value = new Map(Object.entries(configData.lists || {}))
      views.value = new Map(Object.entries(configData.views || {}))
      kanbans.value = new Map(Object.entries(configData.kanbans || {}))
      documents.value = new Map(Object.entries(configData.documents || {}))
      apps.value = new Map(Object.entries(configData.apps || {}))
      actions.value = new Map(Object.entries(configData.actions || {}))
      // From /_dashboard, NOT configData.dashboard: the latter is the
      // unfiltered config block every principal receives.
      dashboard.value = dashboardData
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
      error.value = getErrorMessage(err, 'Failed to load schema')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function reload() {
    loaded.value = false
    await load()
  }

  function setLogoUrl(url: string | null) {
    logoUrl.value = url
  }

  function setInlineCreate(map: Record<string, string>) {
    inlineCreate.value = map
  }

  // The inline-create form id for an entity type, or undefined when the
  // type is not offered (no form, or no create permission). Presence in
  // the map IS the affordance — see SidebarData.inline_create.
  const inlineCreateFormFor = computed(
    () => (entityType: string) => inlineCreate.value[entityType]
  )

  return {
    // State
    entityTypes,
    relationTypes,
    customTypes,
    forms,
    lists,
    views,
    kanbans,
    documents,
    apps,
    actions,
    dashboard,
    navigation,
    app,
    aboutDescription,
    styles,
    paletteLight,
    paletteDark,
    darkDisabled,
    logoUrl,
    inlineCreate,
    loaded,
    loading,
    error,

    // Getters
    getEntityType,
    getRelationType,
    getInverseName,
    getForm,
    getList,
    findListIdForEntityType,
    getView,
    getKanban,
    getAction,
    getEnumLabel,
    resolveOptionLabels,
    stylesForProperty,
    entityTypeList,
    relationTypeList,
    inlineCreateFormFor,

    // Actions
    load,
    reload,
    setLogoUrl,
    setInlineCreate,
  }
})
