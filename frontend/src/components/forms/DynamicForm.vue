<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute, onBeforeRouteLeave } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { isCancelledFetch } from '@/composables/usePageData'
import { readReturnTo } from '@/utils/returnPath'
import { actionAllowed } from '@/utils/affordancesWarning'
import { useEntityIDControls } from '@/composables/useEntityIDControls'
import { useConfirm } from '@/composables/useConfirm'
import type {
  PropertyDef,
  FormFieldOrRelation,
  Template,
  ModernRelationsField,
  FieldAffordance,
  RelationAffordance,
} from '@/types'
import { getTemplates, createRelation, dryRunCreateEntity } from '@/api'
import type { RelationCardState } from './RelationCards.vue'
import type { RelationPickerIncomingState } from './RelationPicker.vue'
import {
  buildRelationsPatch,
  reshapeLegacyToModern,
  OUTGOING_SUFFIX,
  INCOMING_SUFFIX,
} from './relationsPatch'
import { useAutoSave } from '@/composables/useAutoSave'
import { registerForm } from './dirtyFormRegistry'
import AutoSaveIndicator from './AutoSaveIndicator.vue'
import FieldRenderer from './FieldRenderer.vue'
import RelationPicker from './RelationPicker.vue'
import RelationCards from './RelationCards.vue'
import MarkdownEditor from './MarkdownEditor.vue'
import SidePanel from './SidePanel.vue'
import HelpModal from '@/components/ui/HelpModal.vue'

const props = defineProps<{
  formId: string
  entityId?: string
}>()

const router = useRouter()
const route = useRoute()
const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()
const uiStore = useUIStore()
const { confirm } = useConfirm()

// Link params for auto-linking after create (from custom views / side panels)
interface LinkParams {
  relation: string
  peer: string
  as: 'from' | 'to'
}
const linkParams = ref<LinkParams | null>(null)
const returnTo = ref<string | null>(null)

// State
const formData = ref<Record<string, unknown>>({})
const relations = ref<Record<string, string[]>>({})
// Per-entity field affordances from the server. Loaded together with
// the entity in edit mode; populated as `_fields` from the API. Drives
// readonly + option-filter rendering and the F1 hidden-field filter
// (TKT-G7N5).
const fieldAffordances = ref<Record<string, FieldAffordance>>({})
// Same for relation affordances. Drives RelationCards' +Add / x button
// visibility and meta-field disable.
const relationAffordances = ref<Record<string, RelationAffordance>>({})
// TKT-3I5U: in create mode the form models a staged `++new++` entity
// and re-derives affordances from the server's dry-run (no persist) as
// the user types. `stagedVisibleProps` holds the property keys the
// dry-run returned as visible (hidden fields are stripped server-side),
// so the field filter can hide policy-hidden fields in create mode the
// same way edit mode uses the loaded entity's `properties`. Empty until
// the first dry-run resolves.
const stagedVisibleProps = ref<Set<string>>(new Set())
// True once at least one create-mode dry-run has resolved, so the field
// filter switches from "render everything" (first paint, F19) to the
// affordance-filtered list. Stays false in edit mode.
const stagedAffordancesReady = ref(false)
// Property keys the user has explicitly edited via updateField in
// create mode. The commit-side filter always preserves these keys even
// when the dry-run hasn't reported them as visible-writable yet (debounce
// race, RR-2U2D), and they're authoritative over a stale metamodel
// default for "what was the user's intent" — the server's gate (BUG-Q60V)
// is the boundary that rejects a touched key the policy denies.
const userTouched = ref<Set<string>>(new Set())
// Per-relation `id -> entity type` map, fed by RelationPicker's
// `update:types` emit. Required by the unified PATCH builder to emit
// JSON:API §9 resource identifiers without guessing target types
// via `to[0]` (which is wrong for polymorphic relations).
const pickerTypes = ref<Record<string, Map<string, string>>>({})
const content = ref('')
const loading = ref(true)
// Set to true in edit mode when the loaded entity's `_actions.update`
// is explicitly false. The template renders an inline "not editable"
// message instead of the form. The EntityDetail Edit button already
// gates on the same verdict, so this branch only fires for direct-URL
// navigation (bookmark, paste) or when the policy tightened after the
// detail view loaded.
const notEditable = ref(false)
const saveGeneration = ref(0) // Incremented after save to reset RelationCards
const saving = ref(false)
const dirty = ref(false)
const errors = ref<Record<string, string>>({})
const originalData = ref<string>('')
const helpModalOpen = ref(false)
const templates = ref<Template[]>([])
const selectedTemplate = ref<string>('')

// Computed
const formConfig = computed(() => schemaStore.getForm(props.formId))
const entityType = computed(() => {
  if (!formConfig.value) return undefined
  return schemaStore.getEntityType(formConfig.value.entity)
})

const isEdit = computed(() => !!props.entityId)
const formMode = computed(() => (isEdit.value ? 'edit' : 'create') as 'create' | 'edit')

const idControls = useEntityIDControls(entityType, formMode)
const {
  showManualIDInput,
  showPrefixPicker,
  prefixOptions,
  manualId,
  selectedPrefix,
} = idControls

const showReadOnlyID = computed(
  () => isEdit.value && entityType.value?.id_type === 'manual'
)

const title = computed(() => {
  if (!formConfig.value) return ''
  const label = entityType.value?.label || formConfig.value.entity
  return isEdit.value ? `Edit ${label}` : `New ${label}`
})

const allFields = computed((): FormFieldOrRelation[] => {
  if (!formConfig.value) return []
  if (formConfig.value.sections?.length) {
    return formConfig.value.sections.flatMap((s) => s.fields) as FormFieldOrRelation[]
  }
  // Combine property fields and relation fields into a single list
  const propFields = (formConfig.value.fields || []) as FormFieldOrRelation[]
  const relFields = (formConfig.value.relations || []) as FormFieldOrRelation[]
  return [...propFields, ...relFields]
})

// TKT-G7N5 F1 / TKT-3I5U: filter the config-driven field list against
// the entity's affordances. A property field is rendered only if it is
// visible: either (a) it appears in `_fields` (server emitted a verdict,
// including the implicit "writable default") or (b) the server treated
// it as visible — in edit mode that's "present in `properties`"; in
// create mode the staged dry-run reports visible props in
// `stagedVisibleProps` (hidden fields are stripped server-side in both
// cases). Relations / non-property fields are never filtered here.
//
// F19 flicker prevention: render the unfiltered list until affordances
// are available — during initial `loading` (edit), and in create until
// the first dry-run resolves (`stagedAffordancesReady`). Otherwise a
// policy-hidden field would flash in then disappear. If a create-mode
// dry-run never resolves (fail-open, RR-HUQ3) the form stays unfiltered
// and usable; the commit gate is the real boundary.
const fields = computed((): FormFieldOrRelation[] => {
  const all = allFields.value
  if (loading.value) return all
  if (!isEdit.value && !stagedAffordancesReady.value) return all
  const visibleProps = isEdit.value ? formData.value : undefined
  return all.filter((f) => {
    if (!f.property) return true // relations / non-property fields untouched
    if (f.property in fieldAffordances.value) return true
    if (visibleProps && f.property in visibleProps) return true
    if (!isEdit.value && stagedVisibleProps.value.has(f.property)) return true
    return false
  })
})

// TKT-G7N5 readonly helper: the rendered field is readonly if either
// the config marks it so OR the server's _fields verdict reports
// writable=false. Both sources are honored — the server's affordance
// is the strongest signal but the config can still set its own
// readonly for static cases (e.g. ID display).
function isFieldReadonly(field: FormFieldOrRelation): boolean {
  if (field.readonly) return true
  if (!field.property) return false
  const verdict = fieldAffordances.value[field.property]
  return verdict?.writable === false
}

// TKT-G7N5 option-verdict helper: pulls per-option allowed-map from
// the server's _fields verdict. Undefined if no verdict for this
// field (all options allowed by default). Sparse: only false entries
// appear in the map.
function optionVerdictsFor(field: FormFieldOrRelation): Record<string, boolean> | undefined {
  if (!field.property) return undefined
  return fieldAffordances.value[field.property]?.options
}

// TKT-3I5U: build the create-commit property map, sending ONLY visible
// and writable keys. Hidden fields (stripped from the staged dry-run's
// visible set) and read-only fields (writable=false in `_fields`) are
// omitted so the server applies their defaults itself, downstream of
// the affordance gate (RR-SIA6). Without affordances (fail-open) we
// send everything — the commit gate is the boundary that rejects any
// denied write.
function visibleWritablePropertiesForCommit(): Record<string, unknown> {
  if (isEdit.value || !stagedAffordancesReady.value) {
    return { ...formData.value }
  }
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(formData.value)) {
    // RR-2U2D: never drop a key the user explicitly typed into — the
    // dry-run may not have resolved yet for that key (debounce in
    // flight) and silently dropping the value would lose user intent.
    // The server's affordance gate (BUG-Q60V) rejects denied writes,
    // so this is safe: an under-resolved touched key that the policy
    // actually denies still 403s at commit, with a clear rule_id.
    if (userTouched.value.has(key)) {
      out[key] = value
      continue
    }
    if (!stagedVisibleProps.value.has(key)) continue // hidden → omit
    if (fieldAffordances.value[key]?.writable === false) continue // read-only → omit
    out[key] = value
  }
  return out
}

// Helper to look up entity type from ID prefix (e.g., "TKT-001" -> "ticket")
function getTypeFromId(entityId: string): string | undefined {
  const prefix = entityId.split('-')[0]
  if (!prefix) return undefined

  for (const [typeName, typeDef] of schemaStore.entityTypes) {
    if (typeDef.id_prefix?.toUpperCase() === prefix.toUpperCase()) {
      return typeName
    }
  }
  return undefined
}

// Methods
async function loadEntity() {
  if (!props.entityId || !formConfig.value) return

  try {
    const entity = await entitiesStore.fetchEntity(
      formConfig.value.entity,
      props.entityId
    )
    // Route-guard: if the server says this entity is not updatable,
    // render an inline "not editable" message instead of the form.
    // The EntityDetail Edit button already hides for the same
    // verdict, so this branch fires only for direct-URL navigation.
    notEditable.value = !actionAllowed(entity, 'update')
    formData.value = { ...entity.properties }
    relations.value = entity.relations ? { ...entity.relations } : {}
    content.value = entity.content || ''
    // TKT-G7N5: per-entity affordances from the server. The wire keys
    // are always present on per-entity GET (possibly empty); we
    // default to empty maps so the filter / readonly / options paths
    // can treat absence as "default everything".
    fieldAffordances.value = entity._fields ?? {}
    relationAffordances.value = entity._relations ?? {}
    originalData.value = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    uiStore.error('Failed to load entity')
    console.error(err)
  }
}

// Read return_to from the query eagerly — needed in both create and
// edit modes. initializeDefaults below handles create-only pre-fills
// (prop.*, rel.*, link_*) and early-returns in edit mode, so return_to
// can't live in there if edit submits are to honour it too.
//
// readReturnTo from utils enforces the open-redirect guard and the
// array-valued-query case (vue-router yields string[] on duplicate keys).
function applyReturnToFromQuery() {
  const safe = readReturnTo(route.query)
  if (safe) returnTo.value = safe
}

function initializeDefaults() {
  if (!entityType.value || isEdit.value) return

  idControls.reset()

  // Parse query params for pre-filling (prop.*, rel.*, link_*)
  const query = route.query
  const queryProps: Record<string, string> = {}
  const queryRels: Record<string, string[]> = {}

  for (const [key, value] of Object.entries(query)) {
    if (typeof value !== 'string') continue

    if (key.startsWith('prop.')) {
      const propName = key.slice(5) // Remove 'prop.' prefix
      queryProps[propName] = value
    } else if (key.startsWith('rel.')) {
      const relType = key.slice(4) // Remove 'rel.' prefix
      if (!queryRels[relType]) {
        queryRels[relType] = []
      }
      queryRels[relType].push(value)
    } else if (key === 'link_relation' && typeof query.link_peer === 'string') {
      linkParams.value = {
        relation: value,
        peer: query.link_peer,
        as: (query.link_as as 'from' | 'to') || 'to',
      }
    }
  }

  // Apply metamodel defaults
  for (const [propName, propDef] of Object.entries(entityType.value.properties)) {
    if (propDef.default !== undefined) {
      formData.value[propName] = propDef.default
    }
  }

  // Apply form-level defaults. Iterate `allFields.value` directly
  // (not the affordance-filtered `fields` computed): defaults must seed
  // from every configured field, independent of any incidental
  // affordance state. RR-00VT made this dependency explicit so a
  // future change to `fields`'s early-return can't silently drop
  // create-mode defaults.
  for (const field of allFields.value) {
    if (field.property && field.default !== undefined) {
      formData.value[field.property] = field.default
    }
    if (field.relation && field.default !== undefined) {
      const defaultValue = field.default
      if (Array.isArray(defaultValue)) {
        relations.value[field.relation] = defaultValue as string[]
      } else {
        relations.value[field.relation] = [defaultValue as string]
      }
    }
  }

  // Apply query param overrides (highest priority)
  for (const [propName, value] of Object.entries(queryProps)) {
    formData.value[propName] = value
  }
  for (const [relType, targets] of Object.entries(queryRels)) {
    if (!relations.value[relType]) {
      relations.value[relType] = []
    }
    for (const target of targets) {
      if (!relations.value[relType].includes(target)) {
        relations.value[relType].push(target)
      }
    }
  }

  // Pre-fill relation from link params (but this is usually auto-created, not shown)
  if (linkParams.value) {
    const rel = linkParams.value.relation
    if (!relations.value[rel]) {
      relations.value[rel] = []
    }
    if (!relations.value[rel].includes(linkParams.value.peer)) {
      relations.value[rel].push(linkParams.value.peer)
    }
  }

  originalData.value = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
}

async function loadTemplates() {
  if (!formConfig.value) return
  try {
    templates.value = await getTemplates(formConfig.value.entity)
    if (templates.value.length > 0) {
      // Select first template by default
      selectedTemplate.value = templates.value[0].name
      applyTemplate(templates.value[0])
    }
  } catch (err) {
    // Templates are optional, ignore errors
    console.warn('Failed to load templates:', err)
  }
}

// TKT-3I5U create-mode affordance machinery.
//
// The staged entity's verdicts depend on its current field VALUES
// (value-dependent predicates), so we re-derive them via the server's
// dry-run create (no persist) on mount and, debounced, as the user
// types. Verdicts are ADVISORY — the real create re-authorizes; on any
// dry-run failure we fail OPEN (leave the form unfiltered/usable),
// since a missing hint is a UX regression, not a security hole
// (RR-HUQ3). Only the latest request's result is applied (RR-ZKL2).
const STAGED_DRYRUN_DEBOUNCE_MS = 400
let stagedDryRunController: AbortController | null = null
let stagedDryRunTimer: ReturnType<typeof setTimeout> | null = null
// RR-2PZB: signals that the component is gone so a dry-run response
// arriving post-unmount doesn't write to refs of a destroyed component.
let stagedUnmounted = false

async function refreshStagedAffordances() {
  if (isEdit.value || !formConfig.value) return

  // Drop any in-flight request: only the newest form state matters.
  stagedDryRunController?.abort()
  const controller = new AbortController()
  stagedDryRunController = controller

  try {
    const candidate = await dryRunCreateEntity(
      formConfig.value.entity,
      { properties: { ...formData.value }, content: content.value || undefined },
      controller.signal,
    )
    // A newer request superseded this one between await points — discard.
    if (controller !== stagedDryRunController) return
    // Component is gone (unmount fired between resolve and here) — bail
    // so we don't write to dead refs (RR-2PZB).
    if (stagedUnmounted) return

    fieldAffordances.value = candidate._fields ?? {}
    relationAffordances.value = candidate._relations ?? {}
    // The dry-run strips hidden fields from `properties`; the remaining
    // keys are exactly the visible-by-default props the field filter
    // needs to render (since they won't appear in the sparse `_fields`).
    stagedVisibleProps.value = new Set(Object.keys(candidate.properties ?? {}))
    // Note: dry-run soft warnings are intentionally NOT surfaced as
    // toasts here — nothing is saved, and a per-keystroke "Saved with
    // warnings" toast would be noisy and misleading. Warnings still
    // surface on the real commit response. Inline per-field validation
    // feedback from the dry-run is a future enhancement.
    stagedAffordancesReady.value = true
  } catch (err) {
    if (isCancelledFetch(err)) return // superseded / unmounted — not an error
    if (stagedUnmounted) return // post-unmount; don't write to dead refs
    // Fail open: leave whatever affordances we have (possibly none) and
    // let the form render. The commit gate is the real boundary.
    console.warn('Dry-run affordance check failed; create form left unfiltered:', err)
    stagedAffordancesReady.value = true
  }
}

// scheduleStagedAffordances debounces refreshStagedAffordances so a
// burst of keystrokes collapses to one dry-run.
function scheduleStagedAffordances() {
  if (isEdit.value) return
  if (stagedDryRunTimer) clearTimeout(stagedDryRunTimer)
  stagedDryRunTimer = setTimeout(() => {
    void refreshStagedAffordances()
  }, STAGED_DRYRUN_DEBOUNCE_MS)
}

function applyTemplate(template: Template) {
  // Apply template properties
  for (const [key, value] of Object.entries(template.properties)) {
    formData.value[key] = value
  }
  // Apply template content
  content.value = template.content
  // Apply template relations
  for (const rel of template.relations) {
    if (!relations.value[rel.relation]) {
      relations.value[rel.relation] = []
    }
    if (!relations.value[rel.relation].includes(rel.target)) {
      relations.value[rel.relation].push(rel.target)
    }
  }
  originalData.value = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
}

function selectTemplate(name: string) {
  selectedTemplate.value = name
  const template = templates.value.find((t) => t.name === name)
  if (template) {
    // Reset to defaults first
    formData.value = {}
    relations.value = {}
    content.value = ''
    initializeDefaults()
    applyTemplate(template)
  }
}

function getTemplateLabel(name: string): string {
  if (name === '') return 'Default'
  // Capitalize first letter
  return name.charAt(0).toUpperCase() + name.slice(1)
}

function validate(): boolean {
  errors.value = {}

  if (!entityType.value) return true

  // Only validate properties that are shown in the form (not hidden)
  const formPropertyNames = new Set(
    fields.value
      .filter((f): f is typeof f & { property: string } => !!f.property && !f.hidden)
      .map((f) => f.property)
  )

  for (const [propName, propDef] of Object.entries(entityType.value.properties)) {
    // Skip properties not in the form - backend will validate them
    if (!formPropertyNames.has(propName)) continue

    const value = formData.value[propName]

    // Required check
    if (propDef.required && (value === undefined || value === null || value === '')) {
      errors.value[propName] = 'This field is required'
      continue
    }

    // Type-specific validation
    if (value !== undefined && value !== null && value !== '') {
      if (propDef.type === 'integer' && typeof value === 'string') {
        const num = parseInt(value, 10)
        if (isNaN(num)) {
          errors.value[propName] = 'Must be a valid number'
        }
      }

      if (propDef.type === 'date' && typeof value === 'string') {
        const date = new Date(value)
        if (isNaN(date.getTime())) {
          errors.value[propName] = 'Must be a valid date'
        }
      }

      if (propDef.values?.length) {
        const allowed = propDef.values
        const items = propDef.list && Array.isArray(value) ? value : [value]
        const invalid = items.some((v) => !allowed.includes(String(v)))
        if (invalid) {
          errors.value[propName] = `Must be one of: ${allowed.join(', ')}`
        }
      }
    }
  }

  return Object.keys(errors.value).length === 0
}

async function handleSubmit() {
  if (!validate() || !formConfig.value) return

  saving.value = true
  try {
    // Card-managed relations are not put into the legacy
    // `filteredRelations` IDs-only map — they're delivered through
    // pendingCardChanges and the unified PATCH-with-relations shape.
    const cardRelations = new Set(
      fields.value
        .filter((f) => f.relation && f.widget === 'cards')
        .map((f) => f.relation!)
    )
    const filteredRelations: Record<string, string[]> = {}
    for (const [rel, ids] of Object.entries(relations.value)) {
      if (!cardRelations.has(rel)) {
        filteredRelations[rel] = ids
      }
    }

    // Build the modern relations body. Picker selections (IDs-only
    // in memory) are reshaped to JSON:API §9 wrappers via pickerTypes;
    // card edits already carry per-edge meta. Incoming-suffix entries
    // become inverse-named body keys via the inverseByRelation lookup
    // (TKT-GFQK).
    const inverseByRelation = new Map<string, string>()
    for (const f of fields.value) {
      if (!f.relation) continue
      const inverse = schemaStore.getInverseName(f.relation)
      if (inverse) inverseByRelation.set(f.relation, inverse)
    }
    const modernRelations = buildRelationsPatch(pendingCardChanges.value, inverseByRelation)
    const reshapedPickers = reshapeLegacyToModern(filteredRelations, pickerTypes.value)
    if (!reshapedPickers) {
      // Pathological form — no resolved type for some picker target,
      // so we cannot emit a modern resource identifier. Abort the save
      // and tell the user to reload (the type comes from backend Step 0
      // and is normally always present).
      uiStore.error(
        'Some related entities have unknown types. Save aborted; reload the form and try again.',
      )
      // Drop the outgoing card-edit Map entries so they aren't
      // mistakenly cleared on success below.
      for (const key of Array.from(pendingCardChanges.value.keys())) {
        if (key.endsWith(OUTGOING_SUFFIX)) pendingCardChanges.value.delete(key)
      }
      saving.value = false
      return
    }
    const relationsPayload: ModernRelationsField = { ...reshapedPickers, ...modernRelations }

    const payload: {
      id?: string
      prefix?: string
      properties: Record<string, unknown>
      relations: ModernRelationsField
      content?: string
    } = {
      properties: formData.value,
      relations: relationsPayload,
      content: content.value || undefined,
    }

    if (isEdit.value && props.entityId) {
      const updated = await entitiesStore.update(formConfig.value.entity, props.entityId, payload)
      // After TKT-GFQK incoming-direction edits flow through the same
      // unified PATCH (remapped to inverse body keys), so no second
      // save channel is needed. Clear pending state.
      pendingCardChanges.value.clear()
      saveGeneration.value++
      surfaceWarnings(updated.warnings)
      uiStore.success('Entity updated successfully')
    } else {
      // Create path uses the same modern shape as edit. Cards never
      // render in create mode (they require entityId), so
      // pendingCardChanges is empty and relationsPayload is composed
      // entirely from reshaped picker selections.
      //
      // TKT-3I5U: send only visible + writable property keys; the server
      // fills hidden / read-only defaults after the affordance gate.
      payload.properties = visibleWritablePropertiesForCommit()
      Object.assign(payload, idControls.buildPayloadFields())
      const entity = await entitiesStore.create(formConfig.value.entity, payload)

      // Handle auto-linking from link_* params (e.g., from custom view "Add" buttons)
      // For link_as=to, the relation is already included in relations.value (pre-filled)
      // For link_as=from, we need to create the reverse relation: peer --relation--> new_entity
      if (linkParams.value && linkParams.value.as === 'from') {
        try {
          const { relation, peer } = linkParams.value
          // Look up peer type from ID prefix
          const peerType = getTypeFromId(peer)
          if (peerType) {
            await createRelation(peerType, peer, relation, entity.id)
          }
        } catch (linkErr) {
          console.warn('Auto-link failed:', linkErr)
          // Continue with navigation even if link fails
        }
      }

      uiStore.success('Entity created successfully')
      dirty.value = false

      // Navigate to return_to or entity detail
      if (returnTo.value) {
        router.push(returnTo.value)
      } else {
        router.push(`/entity/${formConfig.value.entity}/${entity.id}`)
      }
      return
    }

    dirty.value = false
    originalData.value = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })

    // Navigate to return_to or back
    if (returnTo.value) {
      router.push(returnTo.value)
    } else {
      router.back()
    }
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V). A save that was interrupted by navigation is
    // not a user-facing failure; the user clicked away before the
    // save completed, which is their choice.
    if (isCancelledFetch(err)) return
    if (err && typeof err === 'object' && 'errors' in err && Array.isArray((err as { errors: unknown }).errors)) {
      const problemErrors = (err as { errors: Array<{ field?: string; message?: string; detail?: string }> }).errors
      for (const e of problemErrors) {
        if (e.field) {
          errors.value[e.field] = e.message || e.detail || 'Invalid value'
        }
      }
      uiStore.error('Please fix the validation errors')
    } else if (err && typeof err === 'object' && ('detail' in err || 'title' in err)) {
      const problem = err as { detail?: string; title?: string }
      uiStore.error(problem.detail || problem.title || 'Failed to save entity')
    } else {
      uiStore.error('Failed to save entity')
    }
    console.error(err)
  } finally {
    saving.value = false
  }
}

function handleCancel() {
  router.back()
}

function updateField(property: string, value: unknown) {
  formData.value[property] = value
  checkDirty()
  // TKT-3I5U: in create mode, re-derive affordances from the staged
  // entity's new values (value-dependent verdicts) — debounced. Also
  // track that the user explicitly touched this key (RR-2U2D) so the
  // commit-side filter never drops it even if the dry-run hasn't
  // resolved yet for it.
  if (!isEdit.value) {
    userTouched.value.add(property)
    scheduleStagedAffordances()
    return
  }
  if (!autoSave.value) return
  // TKT-E6094: clear semantics per type. For string/list properties an
  // empty value means "user cleared" → properties_unset. Boolean false
  // is a legitimate value, never an unset.
  const def = entityType.value?.properties[property]
  if (isClearedForType(value, def)) {
    autoSave.value.scheduleUnset(property)
  } else {
    autoSave.value.scheduleFieldSave(property, value)
  }
}

function isClearedForType(value: unknown, def: PropertyDef | undefined): boolean {
  if (def?.type === 'boolean') return false
  if (Array.isArray(value)) return value.length === 0
  return value === '' || value === null || value === undefined
}

function updateRelation(relation: string, value: string[]) {
  relations.value[relation] = value
  checkDirty()
  // Legacy IDs-only relation widget. Autosave routes this through the
  // pendingCardChanges map: any change triggers a relations PATCH.
  autoSave.value?.scheduleRelationsChange()
}

function updateRelationTypes(relation: string, types: Map<string, string>) {
  pickerTypes.value[relation] = types
}

// Pending relation card changes (for batch save)
const pendingCardChanges = ref<Map<string, RelationCardState>>(new Map())

// TKT-E6094: autosave is mounted only in edit mode. In create mode
// the user explicitly clicks Save; the form delays the entity into
// existence until then.
const autoSave = computed(() => {
  if (!isEdit.value || !props.entityId || !formConfig.value) return null
  return _autoSaveInstance.value
})
// Lazy holder so we construct the composable once per (entityId, formId).
const _autoSaveInstance = ref<ReturnType<typeof useAutoSave> | null>(null)

function buildAutoSaveRelationsBody(): ModernRelationsField | null {
  // Mirror handleSubmit's body assembly. Two sources of relation
  // edits flow through autosave:
  //   - card-managed widgets (`pendingCardChanges`) — modern shape
  //     via buildRelationsPatch (per-edge meta + content).
  //   - legacy IDs-only widgets (`relations`) — non-card pickers
  //     write IDs; reshapeLegacyToModern wraps them in {data:[{type,id}]}
  //     so they ride the same modern PATCH.
  //
  // Returns null when neither source is dirty.
  const inverseByRelation = new Map<string, string>()
  const cardRelations = new Set<string>()
  if (formConfig.value) {
    for (const f of fields.value) {
      if (!f.relation) continue
      const inv = schemaStore.getInverseName(f.relation)
      if (inv) inverseByRelation.set(f.relation, inv)
      if (f.widget === 'cards') cardRelations.add(f.relation)
    }
  }
  // Legacy picker edits — non-card relations from `relations.value`.
  const filteredRelations: Record<string, string[]> = {}
  for (const [rel, ids] of Object.entries(relations.value)) {
    if (cardRelations.has(rel)) continue
    filteredRelations[rel] = ids
  }
  const modernCards = buildRelationsPatch(pendingCardChanges.value, inverseByRelation)
  const hasModernCards = Object.keys(modernCards).length > 0
  const hasLegacy = Object.keys(filteredRelations).length > 0
  if (!hasModernCards && !hasLegacy) return null
  // Reshape legacy IDs to modern shape (autosave always uses modern;
  // shape_mixed 400 otherwise).
  const reshaped = hasLegacy
    ? reshapeLegacyToModern(filteredRelations, pickerTypes.value)
    : {}
  if (reshaped === null) {
    // Pathological: a picker target without a known type. Surface
    // and skip — explicit Save in create mode handles this case;
    // autosave is best-effort.
    uiStore.error(
      'Some related entities have unknown types; relation changes were not saved. Reload the form and try again.',
    )
    return null
  }
  return { ...reshaped, ...modernCards }
}

function updateRelationCards(relation: string, state: RelationCardState) {
  pendingCardChanges.value.set(relation, state)
  checkDirty()
  autoSave.value?.scheduleRelationsChange()
}

// Bridge incoming-direction RelationPicker changes into the pending-
// changes map under an `-incoming` suffix. After TKT-GFQK these flow
// through the SAME unified PATCH as outgoing — buildRelationsPatch
// remaps the suffix to the relation's inverse body key, and the
// backend's resolveDirection treats it as a "path entity is target"
// write. RelationPicker emits enough state (loadedEntries +
// currentEntries) for us to build a proper RelationCardState the
// builder can consume.
function updateIncomingPicker(
  relation: string,
  state: RelationPickerIncomingState,
) {
  pendingCardChanges.value.set(`${relation}${INCOMING_SUFFIX}`, {
    entries: state.currentEntries,
    added: state.added,
    removed: state.removed,
    updated: [],
  })
  checkDirty()
  autoSave.value?.scheduleRelationsChange()
}

// Surface soft validation warnings from a mutation response as a
// non-blocking toast. Per DEC-HWZHA, soft conditions (target type
// mismatch, unknown meta key, required-meta unset, etc.) ride on the
// 200 response rather than failing it. Without this, the conditions
// would be invisible to the user.
function surfaceWarnings(warnings: { code: string; path: string; detail: string }[] | undefined) {
  if (!warnings || warnings.length === 0) return
  const codes = [...new Set(warnings.map((w) => w.code))].join(', ')
  uiStore.warning(`Saved with ${warnings.length} warning(s): ${codes}`)
}

function updateContent(value: string) {
  content.value = value
  checkDirty()
  autoSave.value?.scheduleContentSave(value)
}

function checkDirty() {
  const currentData = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
  const hasCardChanges = pendingCardChanges.value.size > 0
  dirty.value = currentData !== originalData.value || hasCardChanges
}

function getPropertyDef(property: string): PropertyDef | undefined {
  return entityType.value?.properties[property]
}

// Warn before browser tab close / hard reload / external navigation. Browsers
// require this to be the native dialog — they ignore custom UI here — so this
// stays as-is even though the in-app navigation guard below uses ConfirmModal.
function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (dirty.value) {
    e.preventDefault()
    e.returnValue = ''
  }
}

// Cmd/Ctrl+Enter to submit
function handleKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    handleSubmit()
  }
}

// Lifecycle & Navigation Guards
onMounted(async () => {
  // Setup event listeners
  window.addEventListener('beforeunload', handleBeforeUnload)
  document.addEventListener('keydown', handleKeydown)

  // return_to is honoured in both modes — read it eagerly.
  applyReturnToFromQuery()

  // Load form data
  loading.value = true
  if (isEdit.value) {
    await loadEntity()
  } else {
    initializeDefaults()
    await loadTemplates()
  }
  loading.value = false

  // TKT-3I5U: derive the staged entity's initial affordances from the
  // dry-run so the first affordance-filtered paint reflects defaults +
  // template values. Awaited so `stagedAffordancesReady` flips before
  // the user can interact, avoiding a hidden-field flash (F19).
  if (!isEdit.value) {
    await refreshStagedAffordances()
  }

  // TKT-E6094: mount the autosave composable in edit mode. The save
  // path replaces handleSubmit's Save button for edit forms; create
  // forms keep the explicit submit.
  if (isEdit.value && props.entityId && formConfig.value) {
    const inverseToCanonical = new Map<string, string>()
    for (const f of fields.value) {
      if (!f.relation) continue
      const inv = schemaStore.getInverseName(f.relation)
      if (inv) inverseToCanonical.set(inv, f.relation)
    }
    _autoSaveInstance.value = useAutoSave({
      getEntityType: () => formConfig.value!.entity,
      getEntityId: () => props.entityId!,
      formData,
      contentRef: content,
      inverseToCanonical,
      buildRelationsBody: () => buildAutoSaveRelationsBody(),
      applyServerProperty: (property, value) => {
        if (value === undefined) {
          delete formData.value[property]
        } else {
          formData.value[property] = value
        }
      },
      applyServerContent: (c) => { content.value = c },
      onError: (msg) => uiStore.error(msg),
    })
    // Register with the dirty registry so SSE-driven re-fetches in
    // other forms on the same entity preserve this form's dirty state.
    const unregister = registerForm(
      props.entityId,
      (property) => _autoSaveInstance.value?.isDirty(property) ?? false,
    )
    onBeforeUnmount(unregister)
  }

  // TKT-GFQK pre-flight: a `direction: incoming` widget on a relation
  // without an `inverse:` declared in the metamodel can't be saved
  // through the unified PATCH. Warn the user at form-load time so the
  // failure surfaces before edits accumulate. The widget still renders
  // (display path is direction-aware and works), but save will throw
  // a clear error from buildRelationsPatch if they try.
  for (const f of fields.value) {
    if (f.relation && f.direction === 'incoming') {
      const inverse = schemaStore.getInverseName(f.relation)
      if (!inverse) {
        uiStore.warning(
          `Relation '${f.relation}' has no 'inverse:' declared in the metamodel. ` +
            `Saving changes from this widget will fail until the metamodel is updated.`,
        )
      }
    }
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
  document.removeEventListener('keydown', handleKeydown)
  // TKT-3I5U: cancel any pending / in-flight staged dry-run, and mark
  // the component as gone so a response that has already arrived (but
  // is awaiting the microtask queue) doesn't write to dead refs
  // (RR-2PZB).
  stagedUnmounted = true
  if (stagedDryRunTimer) clearTimeout(stagedDryRunTimer)
  stagedDryRunController?.abort()
})

// Returning a promise from the guard preserves the original navigation's
// push/replace semantics and popstate cursor — `next(false) + router.push(...)`
// would corrupt history for back/forward and any internal `router.replace`.
//
// dirty.value=false is set before returning ok. This is safe in this app
// because there are no global beforeResolve guards that could cancel the
// navigation downstream — if one were added, the assignment should move into
// a router.afterEach hook gated on success.
onBeforeRouteLeave(async () => {
  // TKT-E6094: in edit mode, flush autosave before navigating away.
  // On clean commit we proceed silently; on error or timeout we
  // prompt the user to confirm.
  if (autoSave.value) {
    const result = await autoSave.value.commitImmediately()
    if (result.settled && !result.error) {
      dirty.value = false
      return true
    }
    return await confirm({
      title: 'Unsaved changes',
      message: result.error ?? 'Some changes are still saving.',
      confirmLabel: 'Leave anyway',
      danger: true,
    })
  }
  // Create-mode / no autosave: original prompt.
  if (!dirty.value) return true
  const ok = await confirm({
    title: 'Unsaved changes',
    message: 'You have unsaved changes. Are you sure you want to leave?',
    confirmLabel: 'Leave',
    danger: true,
  })
  if (ok) dirty.value = false
  return ok
})
</script>

<template>
  <div v-if="formConfig" class="form-layout" :class="{ 'with-sidepanel': isEdit }">
    <div class="dynamic-form">
      <header class="form-header mobile-topbar">
        <h1>{{ title }}</h1>
        <button
          type="button"
          class="help-btn"
          title="Show help for this entity type"
          @click="helpModalOpen = true"
        >
          ?
        </button>
      </header>

      <!-- Template selector (create mode only) -->
      <div v-if="!isEdit && templates.length > 1" class="template-selector">
        <button
          v-for="tpl in templates"
          :key="tpl.name"
          type="button"
          class="template-pill"
          :class="{ active: selectedTemplate === tpl.name }"
          @click="selectTemplate(tpl.name)"
        >
          {{ getTemplateLabel(tpl.name) }}
        </button>
      </div>

      <div v-if="loading" class="loading-state">
        <div class="spinner"/>
        <span>Loading...</span>
      </div>

      <div v-else-if="notEditable" class="not-editable-state">
        <h2>This entity is not editable</h2>
        <p>
          Your current permissions don't allow updating
          <code>{{ entityId }}</code>. Return to the entity view to see
          available actions.
        </p>
        <router-link
          v-if="formConfig && entityId"
          :to="`/entity/${formConfig.entity}/${entityId}`"
          class="btn btn-secondary"
        >
          ← Back to entity
        </router-link>
      </div>

      <form v-else @submit.prevent="handleSubmit">
        <div v-if="showReadOnlyID" class="form-field id-field">
          <label>ID</label>
          <div class="id-display">{{ entityId }}</div>
          <p class="field-help">IDs cannot be changed here; use rename.</p>
        </div>
        <div v-if="showManualIDInput" class="form-field id-field">
          <label>ID <span class="required">*</span></label>
          <input v-model="manualId" type="text" required placeholder="Unique ID..." />
        </div>
        <div v-if="showPrefixPicker" class="form-field id-field">
          <label>Prefix <span class="required">*</span></label>
          <select v-model="selectedPrefix" required>
            <option v-for="p in prefixOptions" :key="p" :value="p">{{ p }}</option>
          </select>
        </div>

        <template v-if="formConfig.sections?.length">
          <div
            v-for="section in formConfig.sections"
            :key="section.title"
            class="form-section"
          >
            <h2 v-if="section.title">{{ section.title }}</h2>
            <p v-if="section.description" class="section-description">
              {{ section.description }}
            </p>

            <div class="form-fields">
              <template v-for="(field, fieldIdx) in section.fields" :key="`${fieldIdx}-${field.property || field.relation}`">
                <FieldRenderer
                  v-if="field.property && !field.hidden"
                  :field="field"
                  :property-def="getPropertyDef(field.property)"
                  :value="formData[field.property]"
                  :error="errors[field.property]"
                  :readonly="isFieldReadonly(field)"
                  :option-verdicts="optionVerdictsFor(field)"
                  @update="updateField(field.property!, $event)"
                />
                <RelationCards
                  v-else-if="field.relation && field.widget === 'cards' && entityId"
                  :key="`cards-${field.relation}-${field.direction || 'outgoing'}-${saveGeneration}`"
                  :field="field"
                  :entity-type="formConfig.entity"
                  :entity-id="entityId"
                  :verdict="relationAffordances[field.relation!]"
                  @cards-changed="(state) => updateRelationCards(`${field.relation}-${field.direction || 'outgoing'}`, state)"
                />
                <RelationPicker
                  v-else-if="field.relation"
                  :key="`picker-${field.relation}-${field.direction || 'outgoing'}-${saveGeneration}`"
                  :field="field"
                  :entity-type="formConfig.entity"
                  :entity-id="entityId"
                  :value="relations[field.relation] || []"
                  :verdict="relationAffordances[field.relation!]"
                  @update="updateRelation(field.relation!, $event)"
                  @update:types="(types) => updateRelationTypes(field.relation!, types)"
                  @incoming-changed="(state) => updateIncomingPicker(field.relation!, state)"
                />
              </template>
            </div>
          </div>
        </template>

        <div v-else class="form-fields">
          <template v-for="(field, fieldIdx) in fields" :key="`${fieldIdx}-${field.property || field.relation}`">
            <FieldRenderer
              v-if="field.property && !field.hidden"
              :field="field"
              :property-def="getPropertyDef(field.property)"
              :value="formData[field.property]"
              :error="errors[field.property]"
              :readonly="isFieldReadonly(field)"
              :option-verdicts="optionVerdictsFor(field)"
              @update="updateField(field.property!, $event)"
            />
            <RelationCards
              v-else-if="field.relation && field.widget === 'cards' && entityId"
              :key="`cards-${field.relation}-${field.direction || 'outgoing'}-${saveGeneration}`"
              :field="field"
              :entity-type="formConfig.entity"
              :entity-id="entityId"
              :verdict="relationAffordances[field.relation!]"
              @cards-changed="(state) => updateRelationCards(`${field.relation}-${field.direction || 'outgoing'}`, state)"
            />
            <RelationPicker
              v-else-if="field.relation"
              :key="`picker-${field.relation}-${field.direction || 'outgoing'}-${saveGeneration}`"
              :field="field"
              :entity-type="formConfig.entity"
              :entity-id="entityId"
              :value="relations[field.relation] || []"
              :verdict="relationAffordances[field.relation!]"
              @update="updateRelation(field.relation!, $event)"
              @update:types="(types) => updateRelationTypes(field.relation!, types)"
              @incoming-changed="(state) => updateIncomingPicker(field.relation!, state)"
            />
          </template>
        </div>

        <!-- Content field (markdown body) -->
        <div class="form-field content-field">
          <label for="content">Content</label>
          <MarkdownEditor
            :model-value="content"
            placeholder="Markdown content..."
            @update:model-value="updateContent"
          />
        </div>

        <div class="form-actions mobile-actionbar">
          <!-- Edit mode: ambient autosave indicator replaces the
               explicit Save button. Cancel is repurposed as a Back
               button to navigate away (with the autosave-flushing
               route guard catching any pending edits). -->
          <template v-if="autoSave">
            <button
              type="button"
              class="btn btn-secondary"
              @click="handleCancel"
            >
              Back <kbd>Esc</kbd>
            </button>
            <AutoSaveIndicator
              :status="autoSave.status"
              :error="autoSave.lastError"
            />
          </template>
          <template v-else>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="saving"
              @click="handleCancel"
            >
              Cancel <kbd>Esc</kbd>
            </button>
            <button
              type="submit"
              class="btn btn-primary"
              :disabled="saving"
            >
              {{ saving ? 'Saving...' : (isEdit ? 'Save Changes' : 'Create') }} <kbd>&#8984;&#8629;</kbd>
            </button>
          </template>
        </div>
      </form>
    </div>

    <!-- Side panel for edit mode -->
    <SidePanel
      v-if="isEdit && entityId"
      :form-id="formId"
      :entity-id="entityId"
    />
  </div>

  <div v-else class="error-state">
    <h2>Form not found</h2>
    <p>The form "{{ formId }}" does not exist in the configuration.</p>
  </div>

  <!-- Help Modal -->
  <HelpModal
    v-if="formConfig"
    :open="helpModalOpen"
    :entity-type="formConfig.entity"
    :entity-label="entityType?.label"
    @close="helpModalOpen = false"
  />
</template>

<style scoped>
.form-layout {
  display: flex;
  gap: 24px;
}

.form-layout.with-sidepanel .dynamic-form {
  flex: 1;
  min-width: 0;
}

.dynamic-form {
  max-width: 800px;
  min-width: 500px;
  width: 100%;
}

.form-header {
  margin-bottom: 24px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.form-header h1 {
  margin: 0;
}

.help-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: var(--bg-color);
  border: 1px solid var(--border-color);
  border-radius: 50%;
  font-size: 14px;
  font-weight: 600;
  color: var(--muted-text);
  cursor: pointer;
  transition: all 0.15s;
}

.help-btn:hover {
  background: var(--accent-color, #6366f1);
  border-color: var(--accent-color, #6366f1);
  color: white;
}

/* Uses global .loading-state and .spinner from App.vue */

.form-section {
  background: var(--card-bg);
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  padding: 24px;
  margin-bottom: 24px;
}

.form-section h2 {
  margin: 0 0 8px;
  font-size: 18px;
}

.section-description {
  color: var(--muted-text);
  margin-bottom: 24px;
}

.form-fields {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-field label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-color);
}

.id-field {
  margin-bottom: 16px;
}

.id-field input,
.id-field select {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.id-display {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--muted-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.required {
  color: var(--error-color, #ef4444);
  margin-left: 2px;
}

.field-help {
  font-size: 12px;
  color: var(--muted-text);
  margin: 0;
}

.content-field {
  margin-top: 16px;
  margin-bottom: 24px;
}


.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 24px;
}

/* Uses global .btn, .btn-primary, .btn-secondary from App.vue */

.error-state {
  padding: 48px;
  text-align: center;
  color: var(--muted-text);
}

.error-state h2 {
  color: var(--error-color, #ef4444);
}

.template-selector {
  display: flex;
  gap: 8px;
  margin-bottom: 20px;
}

.template-pill {
  padding: 8px 16px;
  border-radius: 20px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid var(--border-color, #e2e8f0);
  background: var(--bg-color, #f8fafc);
  color: var(--text-color, #1e293b);
  transition: all 0.15s;
}

.template-pill:hover {
  border-color: var(--accent-color, #6366f1);
  background: var(--card-bg);
}

.template-pill.active {
  background: var(--accent-color, #6366f1);
  border-color: var(--accent-color, #6366f1);
  color: white;
}

@media (max-width: 768px) {
  .form-layout {
    flex-direction: column;
    gap: 12px;
  }

  .dynamic-form {
    min-width: 0;
    max-width: none;
  }

  .form-section {
    padding: 0;
    margin-bottom: 16px;
    border: none;
    box-shadow: none;
    background: none;
  }

  .content-field {
    padding: 0;
    margin-top: 8px;
    margin-bottom: 12px;
  }

  /* Sticky topbar styles live in src/styles/mobile-bars.css
     (.mobile-topbar). Override only the bottom margin here — the form
     wants 12px between bar and first field instead of the default 16. */
  .form-header {
    margin-bottom: 12px;
  }

  .form-header h1 {
    font-size: 20px;
  }

  /* .form-actions uses .mobile-actionbar from mobile-bars.css. */

  .form-actions .btn {
    flex: 1;
    justify-content: center;
    min-height: 44px;
  }

  .template-selector {
    flex-wrap: wrap;
    gap: 6px;
  }
}

</style>
