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
  WorldInfo,
  FormConfig,
  ListConfig,
  ViewConfig,
  CalendarConfig,
  GanttConfig,
  KanbanConfig,
  DashboardResponse,
  NextActionBand,
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
  // Declared worlds and, per world, whether THIS caller may select it
  // (`/_schema`.worlds). Empty on a server too old to serve it — which is
  // why `worldReadable` below treats an unknown world as readable rather
  // than hiding an affordance against a map that was never populated.
  const worlds = ref<Map<string, WorldInfo>>(new Map())
  // The operator's browsing default: the world a request lands in when the
  // URL names none. '' means the raw default faces. See AppConfig.DefaultWorld.
  const defaultWorld = ref<string>('')
  // Whether this deployment can serve version history at all (postgres-only;
  // see AppConfig.history_enabled). Drives whether the History affordance
  // renders, so it defaults to FALSE — an unset flag hides a button rather
  // than shipping one that can only 501.
  const historyEnabled = ref<boolean>(false)
  const forms = ref<Map<string, FormConfig>>(new Map())
  const lists = ref<Map<string, ListConfig>>(new Map())
  const views = ref<Map<string, ViewConfig>>(new Map())
  const kanbans = ref<Map<string, KanbanConfig>>(new Map())
  const calendars = ref<Map<string, CalendarConfig>>(new Map())
  const gantts = ref<Map<string, GanttConfig>>(new Map())
  const documents = ref<Map<string, DocumentConfig>>(new Map())
  const apps = ref<Map<string, AppEntry>>(new Map())
  const actions = ref<Map<string, ActionConfig>>(new Map())
  // The PER-PRINCIPAL dashboard from `/_dashboard`, not the verbatim
  // `dashboard:` block on `/_config` (TKT-53KICM). Cards the caller cannot use
  // are already omitted server-side; never re-derive visibility here.
  const dashboard = ref<DashboardResponse | undefined>(undefined)
  // Operator-declared priority tiers for next-action suggestions, so the UI can
  // label a band rather than echo a raw id. The SOURCES are deliberately not
  // served: a suggestion arrives fully resolved, and shipping the rules would
  // invite a client-side re-implementation of the engine.
  const nextActionBands = ref<NextActionBand[]>([])
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
  // Whether this caller may select the named world (`''`/`'default'` = the
  // default world). Drives affordances that NAVIGATE to a world, so it must
  // not manufacture a denial the server would not make.
  //
  // Unknown world → TRUE. A world absent from the map means either a server
  // too old to serve `worlds` or a schema not yet loaded, and in both cases
  // the request would in fact be served. Defaulting to false would hide a
  // working affordance and read as a permission problem — a wrong answer in
  // the direction nobody can debug. Ignoring a real denial merely reproduces
  // what the URL bar already does: the server re-checks on every request and
  // renders a denial as an empty result.
  // worldForFace maps a stored POINTER to the world that serves it for a given
  // entity type — the input a face-switcher needs, because `?world=` is the
  // read-selection grammar this API has and a bare face is not a world.
  //
  // A world resolves the WHOLE graph consistently (relations included, RULING
  // 12), so jumping by face would show a Dutch body wrapped in English
  // links. Matching on the world's chain is what keeps the switch coherent.
  //
  // Match rules, in order:
  //   - a per-type `overrides` chain wins over the world's own `select`, since
  //     that is what the resolver does;
  //   - the face must be the chain's HEAD, not merely present: `site-nl` is
  //     [nl, en], so `en` appearing as a fallback does not make it the world
  //     that serves English;
  //   - "" (the default face) is the default world, which needs no parameter.
  //
  // When SEVERAL worlds head the same face, the operator breaks the tie with
  // `primary_for:` on the world (TKT-MFVH03). The server refuses at load both
  // an undeclared tie and a claim on a face the world does not head, so by the
  // time this runs a tie has exactly one claimant — but this does not ASSUME
  // that: it iterates deterministically and returns undefined if the schema
  // somehow arrives ambiguous, because the old behaviour (return whichever the
  // map yielded first) was the bug.
  //
  // Returns undefined when no declared world heads that face, or when a tie is
  // unresolved — the caller should then omit the affordance rather than invent
  // a parameter the server will reject with `unknown_world`.
  const worldForFace = computed(() => (entityType: string, face: string) => {
    if (!face) return ''
    const heads: string[] = []
    for (const [name, info] of worlds.value) {
      if (name === 'default') continue
      const chain = info.overrides?.[entityType] ?? info.select ?? []
      if (chain[0] === face) heads.push(name)
    }
    if (heads.length === 0) return undefined
    if (heads.length === 1) return heads[0]

    // Sorted so the answer cannot depend on map insertion order.
    heads.sort()

    // Several worlds lead this face. Two shapes reach here, and both resolve
    // the same way:
    //
    //   - INDISTINGUISHABLE worlds (same head, same `otherwise:`). The server
    //     refuses these unless one claims the face, so a claimant normally
    //     exists.
    //   - DISTINGUISHABLE worlds (same head, different `otherwise:`) — a
    //     `published` world where absence is the publication bit beside a
    //     lenient sibling that substitutes instead. The server accepts that
    //     pair without a declaration, because `otherwise:` already answers a
    //     different question; but it does not say which one a face-SWITCH
    //     means, and neither do the chains.
    //
    // So: one claimant wins, anything else omits the affordance. Returning a
    // world here on a hunch is what this whole ticket removed.
    const claimants = heads.filter((name) => worlds.value.get(name)?.primary_for?.includes(face))
    return claimants.length === 1 ? claimants[0] : undefined
  })

  const worldReadable = computed(() => (name: string) => {
    const key = !name ? 'default' : name
    const w = worlds.value.get(key)
    return w ? w.readable : true
  })

  // faceLabel is the display text for one face of a type, from the STORED
  // coordinate a caller has in hand. Mirrors metamodel.FaceLabel in Go, and
  // the two must agree: the server labels the faces an entity HAS (`_faces`),
  // this labels a face the client only knows from the TYPE — the return-to-
  // default button has to say "Go to English" before fetching anything.
  //
  // The zero coordinate resolves through the `default: true` face, which is
  // the case worth stating: a naive `faces['']` lookup finds nothing, so
  // the default face would render unlabelled while every sibling is labelled.
  //
  // Returns '' when the type declares no name for that coordinate — a type
  // with no `faces:` at all, or one that names no `bare_face`. The
  // caller supplies its own last-resort wording; inventing "default" here
  // would put a UI word in a schema lookup.
  const faceLabel = computed(() => (entityType: string, face: string) => {
    const def = entityTypes.value.get(entityType)
    const faces = def?.faces
    if (!faces) return face
    // The empty coordinate is the bare-id row, and the type says which
    // declared name refers to it. This used to scan for a `default` flag on
    // each face, which meant the answer depended on key order if two ever
    // carried it; `bare_face` is a single field and cannot.
    const declared = face || def?.bare_face || ''
    if (!declared) return ''
    return faces[declared]?.label || declared
  })
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
  const getCalendar = computed(() => (id: string) => calendars.value.get(id))
  const getGantt = computed(() => (id: string) => gantts.value.get(id))
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

  // The DECLARED values of an enum property, in declaration order — the
  // client-side mirror of dataentryconfig.GetValidEnumValues: inline `values:`
  // first, else the values of the named custom type the property references.
  //
  // Declaration ORDER is the point. It is the operator's statement of the
  // workflow, so a consumer defaulting a column/swimlane list to it renders the
  // workflow rather than whatever the current rows happen to contain
  // (TKT-R7H6G1).
  function enumValuesForProperty(
    property: string,
    entityType?: string | EntityType,
  ): string[] | undefined {
    return scanPropertyDefs(property, entityType, (def) => {
      if (!def) return undefined
      if (def.values?.length) return def.values
      const ct = def.type ? customTypes.value.get(def.type) : undefined
      return ct?.values?.length ? ct.values : undefined
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
      //
      // It is explicitly NOT a boot dependency: a rejection here is swallowed
      // to undefined, so the dashboard degrades to its empty state while the
      // sidebar, lists and forms load normally. Letting it reject would take
      // the WHOLE app to App.vue's error screen — doLoad re-throws — which is
      // a catastrophic failure mode for a UX filter most deployments (no
      // acl.yaml) never exercise. A newer SPA against an older server, where
      // this route 404s, is the concrete case.
      const [schemaData, configData, dashboardData] = await Promise.all([
        getSchema(),
        getConfig(),
        getDashboard().catch(() => undefined),
      ])

      // Schema
      entityTypes.value = new Map(Object.entries(schemaData.entities || {}))
      relationTypes.value = new Map(Object.entries(schemaData.relations || {}))
      customTypes.value = new Map(Object.entries(schemaData.types || {}))
      worlds.value = new Map(Object.entries(schemaData.worlds || {}))

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
      defaultWorld.value = configData.app?.default_world || ''
      historyEnabled.value = configData.app?.history_enabled === true
      aboutDescription.value = configData.about_description || ''
      styles.value = configData.styles || {}
      forms.value = new Map(Object.entries(configData.forms || {}))
      lists.value = new Map(Object.entries(configData.lists || {}))
      views.value = new Map(Object.entries(configData.views || {}))
      kanbans.value = new Map(Object.entries(configData.kanbans || {}))
      calendars.value = new Map(Object.entries(configData.calendars || {}))
      gantts.value = new Map(Object.entries(configData.gantts || {}))
      documents.value = new Map(Object.entries(configData.documents || {}))
      apps.value = new Map(Object.entries(configData.apps || {}))
      actions.value = new Map(Object.entries(configData.actions || {}))
      // From /_dashboard, NOT configData.dashboard: the latter is the
      // unfiltered config block every principal receives.
      dashboard.value = dashboardData
      nextActionBands.value = configData.next_action_bands || []
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
    worlds,
    forms,
    lists,
    views,
    kanbans,
    calendars,
    gantts,
    documents,
    apps,
    actions,
    dashboard,
    nextActionBands,
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
    worldReadable,
    defaultWorld,
    worldForFace,
    faceLabel,
    historyEnabled,
    getInverseName,
    getForm,
    getList,
    findListIdForEntityType,
    getView,
    getKanban,
    getCalendar,
    getGantt,
    getAction,
    getEnumLabel,
    enumValuesForProperty,
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
