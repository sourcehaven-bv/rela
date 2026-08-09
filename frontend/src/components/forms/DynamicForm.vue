<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute, onBeforeRouteLeave } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { isCancelledFetch } from '@/composables/usePageData'
import { readReturnTo } from '@/utils/returnPath'
import { actionAllowed } from '@/utils/affordancesWarning'
import {
  isFieldWritable,
  isPropertyRedacted,
  optionVerdictsFor as optionVerdictsForVerdict,
} from '@/utils/affordances'
import { isClearedForType } from '@/utils/formValue'
import { useEntityIDControls } from '@/composables/useEntityIDControls'
import { useConfirm } from '@/composables/useConfirm'
import { useHiddenFieldPolicy, clearWhenHiddenOf } from '@/composables/useHiddenFieldPolicy'
import type {
  Entity,
  PropertyDef,
  FormFieldOrRelation,
  Template,
  ModernRelationsField,
  FieldAffordance,
  RelationAffordance,
  AttachmentInfo,
  TransitionOption,
} from '@/types'
import { getTemplates, createRelation, dryRunCreateEntity, ApiError, getErrorMessage } from '@/api'
import type { RelationCardState } from './RelationCards.vue'
import type { RelationPickerIncomingState } from './RelationPicker.vue'
import {
  buildRelationsPatch,
  reshapeLegacyToModern,
  OUTGOING_SUFFIX,
  INCOMING_SUFFIX,
} from './relationsPatch'
import { useAutoSave } from '@/composables/useAutoSave'
import { useFormWizard } from '@/composables/useFormWizard'
import type { Bindings } from '@/utils/conditions'
import { registerForm } from './dirtyFormRegistry'
import { adoptLockedFieldValues } from './stagedEntity'
import AutoSaveIndicator from './AutoSaveIndicator.vue'
import FormFieldList from './FormFieldList.vue'
import MarkdownEditor from './MarkdownEditor.vue'
import SidePanel from './SidePanel.vue'
import HelpModal from '@/components/ui/HelpModal.vue'

const props = defineProps<{
  formId: string
  entityId?: string
  /**
   * Render as a nested form inside a host (the inline-create modal) rather
   * than as a page (TKT-OMUD56).
   *
   * A second mounted DynamicForm would otherwise fight the first over
   * process-global state: the document-level Cmd+Enter listener, the route
   * guard (whose `useConfirm` is a singleton that returns ONE user decision to
   * every concurrent caller), the `?step=` wizard query key, and the router
   * itself — a `router.push` on create unmounts the host form and destroys its
   * draft, which is the whole thing inline creation exists to avoid.
   *
   * So `embedded` suppresses every page-level side effect and replaces
   * navigation with `inline-created` / `inline-cancelled`. It is read at setup
   * time and never reactive afterwards.
   */
  embedded?: boolean
}>()

/**
 * Deliberately namespaced. Declaring emits removes these names from `$attrs`,
 * so a plain `created` / `cancel` would silently swallow a future mount's
 * native listener of the same name (RR-P3CO33).
 */
const emit = defineEmits<{
  'inline-created': [entity: Entity]
  'inline-cancelled': []
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
// Property names the server withheld by field-level ACL (`_redacted`,
// DEC-T0XIWQ). This is the edit-mode hidden-field signal: previously the
// filter inferred hiding from a key's absence in `properties`, which also
// matches a property that was simply never set — so every unset property
// became permanently unreachable (BUG-MLT9DE). Empty = nothing redacted.
const redactedProps = ref<string[]>([])
// Same for relation affordances. Drives RelationCards' +Add / x button
// visibility and meta-field disable.
const relationAffordances = ref<Record<string, RelationAffordance>>({})
// Per-`file`-property attachment metadata from the loaded entity, passed
// to the file widget so it can show the current file + drive upload/remove.
const attachments = ref<Record<string, AttachmentInfo[]>>({})
// Per-state-machine-field transition verdicts from the loaded entity
// (`_transitions`, TKT-3G93B8). Present only for machine-typed fields; drives
// the StatusControl (only-allowed moves) instead of the plain enum select. A
// field absent here renders as an ordinary widget.
const transitions = ref<Record<string, TransitionOption[]>>({})
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
// loadState is a stable, test-visible signal of whether the edit entity
// actually loaded: 'pending' until the fetch settles, then 'loaded' or
// 'error'. Stamped as data-testid on the form root so an out-of-process
// consumer (the rela-docs screenshot harness) can unambiguously tell a
// rendered-with-data form from an empty schema-only shell after a failed load.
const loadState = ref<'pending' | 'loaded' | 'error'>('pending')
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
const { showManualIDInput, showPrefixPicker, prefixOptions, manualId, selectedPrefix } = idControls

const showReadOnlyID = computed(() => isEdit.value && entityType.value?.id_type === 'manual')

const title = computed(() => {
  if (!formConfig.value) return ''
  const label = entityType.value?.label || formConfig.value.entity
  return isEdit.value ? `Edit ${label}` : `New ${label}`
})

const allFields = computed((): FormFieldOrRelation[] => {
  if (!formConfig.value) return []
  // Wizard forms carry their fields under steps; flatten them so affordance
  // filtering, payload assembly, and load-time hydration see every field the
  // same way single-page forms do. Per-step visibility is applied separately
  // by the wizard layer at render/validate time.
  if (formConfig.value.steps?.length) {
    return formConfig.value.steps.flatMap((s) => [
      ...((s.fields || []) as FormFieldOrRelation[]),
      ...((s.relations || []) as FormFieldOrRelation[]),
    ])
  }
  // Combine property fields and relation fields into a single list
  const propFields = (formConfig.value.fields || []) as FormFieldOrRelation[]
  const relFields = (formConfig.value.relations || []) as FormFieldOrRelation[]
  return [...propFields, ...relFields]
})

// Live binding namespaces for condition evaluation. Supplied as a getter (not a
// computed) so the reactive read of formData happens inside each wizard
// computed's tracking scope — see useFormWizard. `entity`/`current_user` are
// reserved for future ACL/view use (form conditions reference `form.<field>`).
const conditionBindings = (): Bindings => ({
  form: formData.value,
  entity: {},
  current_user: {},
})

// An embedded wizard keeps its step in memory: `?step=` is a single global key
// that the host form already owns (see FormWizardOptions.syncUrl).
const wizard = useFormWizard(formConfig, conditionBindings, { syncUrl: !props.embedded })

// Field lookup by property, for reading per-field config (clear_when_hidden)
// without re-scanning allFields at every call site.
const fieldByProperty = computed(() => {
  const map = new Map<string, FormFieldOrRelation>()
  for (const f of allFields.value) {
    if (f.property) map.set(f.property, f)
  }
  return map
})

// BUG-FB0LN8: the fate of a condition-hidden field's STORED value. Default is
// to keep it — hiding is presentation, not a delete. See useHiddenFieldPolicy.
const hiddenPolicy = useHiddenFieldPolicy({
  policyFor: (property) => clearWhenHiddenOf(fieldByProperty.value.get(property)),
})

// Visible-step indices that currently have a validation error, so the stepper
// can flag them. Recomputes as `errors` changes, so a pill's flag clears the
// moment its field becomes valid.
const stepsWithErrors = computed<Set<number>>(() => {
  const flagged = new Set<number>()
  for (const property of Object.keys(errors.value)) {
    const idx = wizard.visibleStepIndexForProperty(property)
    if (idx >= 0) flagged.add(idx)
  }
  return flagged
})

const errorCount = computed(() => Object.keys(errors.value).length)

// React to a conditional field/step becoming hidden (its `visible_when` flipped
// false). Three effects, keyed on a property entering or leaving
// `activeProperties`:
//   1. Always: drop any standing validation error for a now-hidden field, so it
//      can't leave a phantom "N fields need attention" with no flagged,
//      reachable step (RR-U9ERK).
//   2. Hiding: RETAIN the value (out of formData) rather than unsetting it, then
//      apply the field's `clear_when_hidden` policy — which defaults to keeping
//      it. This used to unconditionally PATCH `properties_unset`, destroying
//      stored data as a side effect of a UI visibility change (BUG-FB0LN8).
//   3. Revealing: restore the retained value, so hide → reveal is lossless and
//      needs no server round-trip.
// Skips the initial hydration (`loading`); only touches wizard-governed
// (managed) fields, so a plain non-conditional field is never affected.
watch(
  () => wizard.activeProperties.value,
  (active, prevActive) => {
    if (loading.value || !prevActive) return
    const managed = wizard.managedProperties.value
    let nextErrors: Record<string, string> | null = null

    // Reveal: put back what we held while the branch was hidden.
    for (const prop of active) {
      if (prevActive.has(prop) || !managed.has(prop)) continue
      if (!hiddenPolicy.hasRetained(prop)) continue
      if (!isClearedForType(formData.value[prop], entityType.value?.properties?.[prop])) continue
      formData.value[prop] = hiddenPolicy.retainedValue(prop)
      hiddenPolicy.release(prop)
    }

    const hiding: string[] = []
    for (const prop of prevActive) {
      if (active.has(prop) || !managed.has(prop)) continue // still shown / not governed
      if (errors.value[prop]) {
        nextErrors ??= { ...errors.value }
        delete nextErrors[prop]
      }
      hiding.push(prop)
    }
    if (nextErrors) errors.value = nextErrors
    if (hiding.length) applyHidePolicy(hiding)
  }
)

// Apply `clear_when_hidden` to the fields that just hid.
//
// Retention happens unconditionally, so a reveal is lossless whatever the
// policy says; only the server-side clear is per-field. Synchronous on purpose:
// both policies are decided from state we already hold, so there is no window
// in which the form and the server can disagree. (An interactive `confirm`
// policy would introduce exactly that window — see useHiddenFieldPolicy for why
// it waits on the propose/commit refactor.)
function applyHidePolicy(hiding: string[]) {
  for (const prop of hiding) {
    hiddenPolicy.retain(prop, formData.value[prop])
  }
  if (!isEdit.value) return // create has no stored value to lose (RR-O4SRG owns that path)
  if (!autoSave.value) return // nothing can be written yet; retention already stands

  for (const prop of hiddenPolicy.clearOnHide(hiding)) {
    delete formData.value[prop]
    hiddenPolicy.release(prop)
    autoSave.value.scheduleUnset(prop)
  }
}

// TKT-G7N5 F1 / TKT-3I5U / DEC-T0XIWQ: filter the config-driven field list
// against the entity's affordances. Both render paths (this and the wizard's
// `visibleStepFields`) delegate to the single `affordanceVisible` predicate
// below — two hand-synced copies of this rule is how the wizard path silently
// carried BUG-MLT9DE alongside the flat one.
const fields = computed((): FormFieldOrRelation[] => allFields.value.filter(affordanceVisible))

// TKT-G7N5 readonly helper: the rendered field is readonly if either
// the config marks it so OR the server's _fields verdict reports
// writable=false. Both sources are honored — the server's affordance
// is the strongest signal but the config can still set its own
// readonly for static cases (e.g. ID display).
function isFieldReadonly(field: FormFieldOrRelation): boolean {
  if (!field.property) return field.readonly === true
  const verdict = fieldAffordances.value[field.property]
  return !isFieldWritable(verdict, field.readonly)
}

// TKT-G7N5 option-verdict helper: pulls per-option allowed-map from
// the server's _fields verdict. Undefined if no verdict for this
// field (all options allowed by default). Sparse: only false entries
// appear in the map.
function optionVerdictsFor(field: FormFieldOrRelation): Record<string, boolean> | undefined {
  if (!field.property) return undefined
  return optionVerdictsForVerdict(fieldAffordances.value[field.property])
}

// TKT-3G93B8 transition helper: the server-resolved outgoing transitions for a
// state-machine field (`_transitions`). Undefined for a non-machine field, so
// FieldRenderer renders the ordinary widget; a machine field routes to the
// StatusControl. Only present in edit mode (a create locks the field instead —
// see applyCreateLock on the backend).
function transitionsFor(field: FormFieldOrRelation): TransitionOption[] | undefined {
  if (!field.property) return undefined
  return transitions.value[field.property]
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
async function loadEntity(force = false) {
  if (!props.entityId || !formConfig.value) return

  try {
    const entity = await entitiesStore.fetchEntity(formConfig.value.entity, props.entityId, force)
    // Route-guard: if the server says this entity is not updatable,
    // render an inline "not editable" message instead of the form.
    // The EntityDetail Edit button already hides for the same
    // verdict, so this branch fires only for direct-URL navigation.
    notEditable.value = !actionAllowed(entity, 'update')
    // Retained hidden values belong to the form state we are about to replace.
    // Carrying them across a reload — or across an entity switch, since this
    // component is not re-keyed per entity — would restore one entity's value
    // onto another, or resurrect a value the server no longer has (BUG-FB0LN8).
    // The stored value is safe on the server, so dropping the cache is lossless:
    // a later reveal reads it back from `properties`.
    hiddenPolicy.releaseAll()
    formData.value = { ...entity.properties }
    relations.value = entity.relations ? { ...entity.relations } : {}
    content.value = entity.content || ''
    // TKT-G7N5: per-entity affordances from the server. The wire keys
    // are always present on per-entity GET (possibly empty); we
    // default to empty maps so the filter / readonly / options paths
    // can treat absence as "default everything".
    fieldAffordances.value = entity._fields ?? {}
    redactedProps.value = entity._redacted ?? []
    relationAffordances.value = entity._relations ?? {}
    attachments.value = entity._attachments ?? {}
    transitions.value = entity._transitions ?? {}
    originalData.value = JSON.stringify({
      formData: formData.value,
      relations: relations.value,
      content: content.value,
    })
    loadState.value = 'loaded'
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    loadState.value = 'error'
    uiStore.error('Failed to load entity')
    console.error(err)
  }
}

// onAttachmentChanged fires after a file widget uploads or removes an
// attachment (already persisted server-side via the attachment endpoint).
// Reload the entity so the stamped property value and _attachments
// refresh. Cache-bust the entity first so the SSE-less refetch is fresh.
async function onAttachmentChanged() {
  await loadEntity(true)
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

  // Parse query params for pre-filling (prop.*, rel.*, link_*).
  //
  // Embedded forms read an EMPTY query: they are mounted over whatever page
  // the host is on, so `route.query` belongs to that page. Honouring it would
  // pre-fill the nested entity from the host's parameters and — via link_* —
  // auto-link it to the host's peer, both silently wrong. Field defaults and
  // templates below still apply; only the URL-sourced overlay is dropped.
  const query = props.embedded ? {} : route.query
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

  originalData.value = JSON.stringify({
    formData: formData.value,
    relations: relations.value,
    content: content.value,
  })
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
      controller.signal
    )
    // A newer request superseded this one between await points — discard.
    if (controller !== stagedDryRunController) return
    // Component is gone (unmount fired between resolve and here) — bail
    // so we don't write to dead refs (RR-2PZB).
    if (stagedUnmounted) return

    fieldAffordances.value = candidate._fields ?? {}
    relationAffordances.value = candidate._relations ?? {}
    // Entry-locked create fields (TKT-3G93B8 / BUG-X1C7S): adopt the server's
    // authoritative value for every read-only field (a machine field the server
    // pinned to its initial value, or any policy-read-only field) so the locked
    // control DISPLAYS the server value, not stale user input. Scoped to read-only
    // fields, so it never clobbers an editable one. See adoptLockedFieldValues.
    adoptLockedFieldValues(candidate._fields, candidate.properties, formData.value)
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
  originalData.value = JSON.stringify({
    formData: formData.value,
    relations: relations.value,
    content: content.value,
  })
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

// Validate the form. `scopeFields` restricts validation to a subset (used for
// per-step validation on wizard "Next"); when omitted, all shown fields are
// validated (single-page submit, or wizard final submit over visible steps).
// `requiredProps` are property keys made required by a matching `required_when`
// condition, in addition to the metamodel's own `required` flag.
//
// Errors are updated **only for the scope's fields**: existing errors on fields
// OUTSIDE the scope are preserved. This is what lets a wizard's per-step Next
// re-check just the current step while leaving flags on other errored steps
// standing until they're actually resolved. Returns whether the SCOPE is valid.
function validate(scopeFields?: FormFieldOrRelation[], requiredProps?: Set<string>): boolean {
  if (!entityType.value) return true

  const scope = scopeFields ?? fields.value
  // Only validate properties that are shown in the form (not hidden)
  const formPropertyNames = new Set(
    scope
      .filter((f): f is typeof f & { property: string } => !!f.property && !f.hidden)
      .map((f) => f.property)
  )

  // Start from the existing errors, then clear this scope's entries so a
  // fixed field drops out while out-of-scope errors are untouched.
  const next: Record<string, string> = { ...errors.value }
  for (const prop of formPropertyNames) delete next[prop]
  let scopeValid = true

  for (const [propName, propDef] of Object.entries(entityType.value.properties)) {
    // Skip properties not in the form - backend will validate them
    if (!formPropertyNames.has(propName)) continue

    const value = formData.value[propName]

    // Required check (metamodel `required` OR a matching `required_when`)
    const isRequired = propDef.required || (requiredProps?.has(propName) ?? false)
    if (isRequired && (value === undefined || value === null || value === '')) {
      next[propName] = 'This field is required'
      scopeValid = false
      continue
    }

    // Type-specific validation
    if (value !== undefined && value !== null && value !== '') {
      if (propDef.type === 'integer' && typeof value === 'string') {
        const num = parseInt(value, 10)
        if (isNaN(num)) {
          next[propName] = 'Must be a valid number'
          scopeValid = false
        }
      }

      if (propDef.type === 'date' && typeof value === 'string') {
        const date = new Date(value)
        if (isNaN(date.getTime())) {
          next[propName] = 'Must be a valid date'
          scopeValid = false
        }
      }

      if (propDef.values?.length) {
        const allowed = propDef.values
        const items = propDef.list && Array.isArray(value) ? value : [value]
        const invalid = items.some((v) => !allowed.includes(String(v)))
        if (invalid) {
          next[propName] = `Must be one of: ${allowed.join(', ')}`
          scopeValid = false
        }
      }
    }
  }

  errors.value = next
  return scopeValid
}

// The single affordance filter for BOTH render paths — flat `fields` and the
// wizard's `visibleStepFields`. Decides whether a config-declared field is
// rendered at all (readonly/options are separate, see isFieldReadonly).
//
// Edit mode (DEC-T0XIWQ): a configured field renders unless the server
// positively names it in `_redacted`. It previously rendered only if the key
// was already in `properties`, which conflated "redacted" with "never set" and
// made every unset property permanently unfillable (BUG-MLT9DE) — the field
// could not be filled in because it did not render, and did not render because
// it had never been filled in.
//
// Create mode is unchanged: the staged dry-run's candidate carries metamodel
// defaults, so `stagedVisibleProps` is a genuine visible-set (it distinguishes
// hidden from unset by construction) rather than an inference from absence.
//
// F19 flicker prevention: render unfiltered until affordances are available —
// during initial `loading` (edit) and until the first dry-run resolves
// (create). Otherwise a policy-hidden field would flash in then disappear. If a
// create-mode dry-run never resolves (fail-open, RR-HUQ3) the form stays
// unfiltered and usable; the commit gate is the real boundary.
function affordanceVisible(f: FormFieldOrRelation): boolean {
  if (!f.property) return true // relations / non-property fields untouched
  if (loading.value) return true
  if (isEdit.value) return !isPropertyRedacted(f.property, redactedProps.value)
  if (!stagedAffordancesReady.value) return true
  if (f.property in fieldAffordances.value) return true
  return stagedVisibleProps.value.has(f.property)
}

// A wizard step's fields to render: per-field `visible_when` (wizard layer)
// intersected with the affordance filter (same rule as flat `fields`).
function visibleStepFields(step: import('@/types').FormStep): FormFieldOrRelation[] {
  return wizard.visibleFieldsOf(step).filter(affordanceVisible)
}

// Fields in scope for submit: the visible fields of the currently-visible
// steps (for a flat/one-step form that's just its fields). Applies the same
// affordance filter as rendering (visibleStepFields), so validation never
// demands a policy-hidden field the user can't see or fill.
function submitScopeFields(): FormFieldOrRelation[] {
  return wizard.visibleSteps.value.flatMap((s) => visibleStepFields(s))
}

// Drop property keys under a condition-hidden step/field so a revealed-then-
// hidden branch is not persisted. Applied by BOTH submit paths — including on
// top of the create path's affordance prune — so the two pruning systems
// reconcile in one place. For a flat form with no conditions this is a no-op
// (nothing is hidden).
function pruneWizardHidden(props: Record<string, unknown>): Record<string, unknown> {
  const active = wizard.activeProperties.value
  const managed = wizard.managedProperties.value
  // Drop a key only if the wizard governs it (named by some step) AND it is not
  // currently active (its step/field is condition-hidden). A key no step
  // mentions — e.g. a metamodel default seeded into form state — is left as-is,
  // matching how a single-page form submits it.
  return Object.fromEntries(
    Object.entries(props).filter(([key]) => !managed.has(key) || active.has(key))
  )
}

// Relation analogue of pruneWizardHidden: drop a relation's edges when it is
// wizard-governed but currently hidden (its step/field visible_when is false),
// so a revealed-then-hidden relation is not persisted. `rels` is keyed by
// relation name (as relations.value is).
function pruneWizardHiddenRelations(rels: Record<string, string[]>): Record<string, string[]> {
  const active = wizard.activeRelations.value
  const managed = wizard.managedRelations.value
  return Object.fromEntries(
    Object.entries(rels).filter(([rel]) => !managed.has(rel) || active.has(rel))
  )
}

// Property keys made required right now by a matching `required_when`.
function requiredWhenProps(scope: FormFieldOrRelation[]): Set<string> {
  const req = new Set<string>()
  for (const f of scope) {
    if (f.property && wizard.isFieldRequired(f)) req.add(f.property)
  }
  return req
}

// Wizard "Next": validate only the current step's visible fields; advance only
// when valid, so an invalid step blocks progression with per-field errors.
// `validate` is scope-local, so it re-checks just this step and leaves any
// flags on OTHER errored steps standing until they're resolved.
function handleNext() {
  const step = wizard.currentStepDef.value
  if (!step) return
  // Same scope as the step renders (visibleStepFields), so Next doesn't block
  // on a policy-hidden field the user can't see.
  const scope = visibleStepFields(step)
  if (!validate(scope, requiredWhenProps(scope))) return
  wizard.next()
}

function handleBack() {
  // Back never validates and never clears errors — navigating isn't fixing.
  wizard.back()
}

// Clicking a step pill jumps straight there. A deliberate jump is not gated by
// per-step validation (unlike Next): visible_when keeps unmet branches hidden
// and the final Submit re-validates every visible step, so a jump can't persist
// invalid data. Errors are left intact — a flag clears when its field is fixed,
// not when the user navigates away.
function handleStepClick(index: number) {
  if (index === wizard.currentStep.value) return
  wizard.goTo(index)
}

// On a failed submit, take the user to the first step that has an error and
// focus its first invalid field, so the fix is one glance + zero hunting. For a
// one-step form this just focuses the field on the only step.
function focusFirstError() {
  // First errored property in visible order.
  let firstStep = Infinity
  let firstProp: string | null = null
  for (const property of Object.keys(errors.value)) {
    const idx = wizard.visibleStepIndexForProperty(property)
    if (idx >= 0 && idx < firstStep) {
      firstStep = idx
      firstProp = property
    }
  }
  if (firstProp === null) return
  wizard.goTo(firstStep)
  // Focus after the step renders.
  const prop = firstProp
  nextTick(() => {
    const el = document.getElementById(`field-${prop}`)
    el?.scrollIntoView({ block: 'center', behavior: 'smooth' })
    el?.focus()
  })
}

async function handleSubmit() {
  if (!formConfig.value) return
  // Edit mode has no explicit submit — autosave persists per field. Guard
  // against an Enter-key form submit doing anything (there's no Save button).
  //
  // This early return is load-bearing beyond the Enter-key case: because edit
  // mode never bulk-submits, an untouched field can never reach a payload, so
  // a redacted field's withheld value cannot be overwritten with empty
  // (DEC-T0XIWQ). Everything below is therefore the CREATE path only. If you
  // ever add a bulk edit submit here, you must first prune properties the
  // server reported in `_redacted` that the user did not touch.
  if (isEdit.value) return
  const scope = submitScopeFields()
  if (!validate(scope, requiredWhenProps(scope))) {
    focusFirstError()
    return
  }

  saving.value = true
  try {
    // Card-managed relations are not put into the legacy
    // `filteredRelations` IDs-only map — they're delivered through
    // pendingCardChanges and the unified PATCH-with-relations shape.
    const cardRelations = new Set(
      fields.value.filter((f) => f.relation && f.widget === 'cards').map((f) => f.relation!)
    )
    // Drop relations under a condition-hidden branch (mirrors property pruning),
    // then exclude card-managed relations (delivered via pendingCardChanges).
    const prunedRelations = pruneWizardHiddenRelations(relations.value)
    const filteredRelations: Record<string, string[]> = {}
    for (const [rel, ids] of Object.entries(prunedRelations)) {
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
        'Some related entities have unknown types. Save aborted; reload the form and try again.'
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
      // For a wizard, drop values under a hidden step/field so a toggled-off
      // branch doesn't persist stale data (matches OpenVWR's isFieldEnabled).
      // Single-page forms submit formData as-is. The create path re-derives
      // this from the affordance-pruned set below (line ~734); this covers the
      // edit path.
      properties: pruneWizardHidden(formData.value),
      relations: relationsPayload,
      content: content.value || undefined,
    }

    // Create only — the isEdit early return above means this is never an
    // update. Cards never render in create mode (they require entityId), so
    // pendingCardChanges is empty and relationsPayload is composed entirely
    // from reshaped picker selections.
    //
    // TKT-3I5U: send only visible + writable property keys; the server
    // fills hidden / read-only defaults after the affordance gate. For a
    // wizard, also drop keys under a condition-hidden step/field — a
    // `visible_when=false` branch wins over the affordance filter's
    // userTouched-preserve rule, so a revealed-then-hidden field is not
    // persisted (TKT-CHLAJ).
    payload.properties = pruneWizardHidden(visibleWritablePropertiesForCommit())
    Object.assign(payload, idControls.buildPayloadFields())
    const entity = await entitiesStore.create(formConfig.value.entity, payload)
    // DEC-HWZHA soft conditions ride the 200 create response; surface them or
    // they are invisible. (Edit-mode warnings come through autosave's own
    // response handling — this is the create channel.)
    surfaceWarnings(entity.warnings)

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

    dirty.value = false

    // Embedded: hand the entity to the host and stop. No navigation (it would
    // unmount the form that opened us, taking its draft with it) and no toast
    // — the host reports the creation itself, next to the link step the user
    // still has to complete.
    if (props.embedded) {
      emit('inline-created', entity)
      return
    }

    uiStore.success('Entity created successfully')

    // Navigate to return_to or entity detail
    if (returnTo.value) {
      router.push(returnTo.value)
    } else {
      router.push(`/entity/${formConfig.value.entity}/${entity.id}`)
    }
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V). A save that was interrupted by navigation is
    // not a user-facing failure; the user clicked away before the
    // save completed, which is their choice.
    if (isCancelledFetch(err)) return
    const validationErrors = err instanceof ApiError ? err.validationErrors : []
    if (validationErrors.length > 0) {
      const next = { ...errors.value }
      for (const e of validationErrors) {
        if (e.field) {
          next[e.field] = e.message || e.detail || 'Invalid value'
        }
      }
      errors.value = next
      uiStore.error('Please fix the validation errors')
    } else {
      uiStore.error(getErrorMessage(err, 'Failed to save entity'))
    }
    console.error(err)
  } finally {
    saving.value = false
  }
}

function handleCancel() {
  if (props.embedded) {
    emit('inline-cancelled')
    return
  }
  router.back()
}

function updateField(property: string, value: unknown) {
  formData.value[property] = value
  // Clear a standing validation error for this field as the user edits it, so
  // the wizard's per-step error flags and summary update live. A full re-check
  // happens on the next Next/Submit; this just avoids a stale "needs attention"
  // marker after the user has addressed it.
  if (errors.value[property]) {
    const next = { ...errors.value }
    delete next[property]
    errors.value = next
  }
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
// Dirty-registry cleanup, assigned in onMounted (after awaits) and run
// from the top-level onBeforeUnmount.
let unregisterDirtyForm: (() => void) | null = null

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
  const reshaped = hasLegacy ? reshapeLegacyToModern(filteredRelations, pickerTypes.value) : {}
  if (reshaped === null) {
    // Pathological: a picker target without a known type. Surface
    // and skip — explicit Save in create mode handles this case;
    // autosave is best-effort.
    uiStore.error(
      'Some related entities have unknown types; relation changes were not saved. Reload the form and try again.'
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
function updateIncomingPicker(relation: string, state: RelationPickerIncomingState) {
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
  const currentData = JSON.stringify({
    formData: formData.value,
    relations: relations.value,
    content: content.value,
  })
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

// Cmd/Ctrl+Enter: on the last (or only) step it submits (create); on an earlier
// wizard step it advances — matching the visible Create-only-on-last-step
// affordance rather than submitting from the middle of a wizard. No-op in edit
// mode (autosave; handleSubmit early-returns).
function handleKeydown(e: KeyboardEvent) {
  if (!((e.metaKey || e.ctrlKey) && e.key === 'Enter')) return
  e.preventDefault()
  if (!isEdit.value && wizard.isMultiStep.value && !wizard.isLastStep.value) {
    handleNext()
  } else {
    handleSubmit()
  }
}

// Lifecycle & Navigation Guards
onMounted(async () => {
  // Setup event listeners
  window.addEventListener('beforeunload', handleBeforeUnload)
  // Embedded: the host modal owns Cmd+Enter. This listener is document-level
  // with no target check, so two mounted forms would BOTH act on one keypress
  // — the modal submitting while the page form behind it also submits (or
  // silently advances a wizard step). beforeunload above is left registered:
  // duplicate handlers there just mean the browser warns if either form is
  // dirty, which is correct for a nested draft.
  if (!props.embedded) {
    document.addEventListener('keydown', handleKeydown)
  }

  // return_to is honoured in both modes — read it eagerly. Skipped when
  // embedded: a nested form inherits the HOST page's query string, so it would
  // otherwise adopt the parent's return_to (and, below, its prop.*/rel.*/link_*
  // pre-fill) as if they were its own.
  if (!props.embedded) {
    applyReturnToFromQuery()
  }

  // Load form data
  loading.value = true
  if (isEdit.value) {
    await loadEntity()
  } else {
    initializeDefaults()
    await loadTemplates()
    loadState.value = 'loaded' // create mode: no entity to fetch
  }
  loading.value = false

  // Re-seed the wizard step from `?step=` now that entity/defaults are loaded:
  // a step whose `visible_when` needs loaded data is only in `visibleSteps`
  // after this point, so the construction-time seed may have clamped a
  // deep-link to the wrong step (RR-TXMU6).
  wizard.seedFromUrl()

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
        // A committed state-machine move changes which transitions are now
        // performable — the loaded `_transitions` reflect the PRE-move state
        // (RR-NI145G). The PATCH response's fresh `_transitions` isn't threaded
        // through the autosave merge, so reload the entity to refresh the
        // status control (mirrors onAttachmentChanged's reload for
        // `_attachments`). Fire-and-forget: this is a UI-hint refresh, and the
        // write already succeeded.
        if (property in transitions.value) {
          void loadEntity(true)
        }
      },
      applyServerContent: (c) => {
        content.value = c
      },
      onError: (msg) => uiStore.error(msg),
    })
    // Register with the dirty registry so SSE-driven re-fetches in
    // other forms on the same entity preserve this form's dirty state.
    // The cleanup runs from the top-level onBeforeUnmount below —
    // registering a lifecycle hook after an `await` has no active
    // instance, so Vue would silently drop it and leak the registration.
    unregisterDirtyForm = registerForm(
      props.entityId,
      (property) => _autoSaveInstance.value?.isDirty(property) ?? false
    )
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
            `Saving changes from this widget will fail until the metamodel is updated.`
        )
      }
    }
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
  document.removeEventListener('keydown', handleKeydown)
  unregisterDirtyForm?.()
  unregisterDirtyForm = null
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
//
// Registration is skipped entirely when embedded. Two guards on one route
// record both fire on a single navigation, and both call the SINGLETON
// `confirm()` — whose in-flight promise is shared, so one dialog's answer is
// returned to both callers. A "Leave" meant for the page would silently
// discard the nested draft too. `props.embedded` is never reactive after
// mount, so this setup-time conditional is sound; the host modal owns
// discard-confirmation for the nested form.
if (!props.embedded) {
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
}

/**
 * Surface for a host that renders this form inside its own chrome (the
 * inline-create modal).
 *
 * The host needs to know whether there is unsaved input and to trigger a
 * submit. Both must be ASKED FOR, not inferred: sniffing bubbling `input` /
 * `change` events off a wrapper element misses every non-native widget — a
 * relation-picker selection, a wizard step, and the CodeMirror-backed markdown
 * body all emit Vue events that do not bubble as DOM events, so a form with a
 * written body would read as pristine and be discarded without a prompt.
 */
defineExpose({
  /** True when the user has entered anything not yet persisted. */
  isDirty: () => dirty.value,
  /** True while a create request is in flight. */
  isSaving: () => saving.value,
  /** Submit the form, exactly as the Create button does. */
  submit: () => handleSubmit(),
})
</script>

<template>
  <div
    v-if="formConfig"
    class="form-layout"
    :class="{ 'with-sidepanel': isEdit }"
    :data-testid="`form-state-${loadState}`"
  >
    <div class="dynamic-form" :class="{ embedded }">
      <!-- The host modal supplies its own title and help affordance, so the
           page header would be a duplicate heading inside the dialog. -->
      <header v-if="!embedded" class="form-header mobile-topbar">
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
        <div class="spinner" />
        <span>Loading...</span>
      </div>

      <div v-else-if="notEditable" class="not-editable-state">
        <h2>This entity is not editable</h2>
        <p>
          Your current permissions don't allow updating
          <code>{{ entityId }}</code
          >. Return to the entity view to see available actions.
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

        <!-- Every form renders through the same step model. A single-page (flat)
             form is one implicit, title-less step, so the stepper bar only shows
             when there is more than one step. -->
        <ol v-if="wizard.isMultiStep.value" class="wizard-steps" aria-label="Form steps">
          <li v-for="(step, sIdx) in wizard.visibleSteps.value" :key="sIdx">
            <button
              type="button"
              class="wizard-step-pill"
              :class="{
                active: sIdx === wizard.currentStep.value,
                done: sIdx < wizard.currentStep.value,
                'has-errors': stepsWithErrors.has(sIdx),
              }"
              :aria-current="sIdx === wizard.currentStep.value ? 'step' : undefined"
              @click="handleStepClick(sIdx)"
            >
              <span class="wizard-step-num">{{ stepsWithErrors.has(sIdx) ? '!' : sIdx + 1 }}</span>
              <span class="wizard-step-title">{{ step.title }}</span>
            </button>
          </li>
        </ol>

        <div
          v-if="wizard.currentStepDef.value"
          class="form-section"
          :class="{ 'wizard-panel': wizard.isMultiStep.value }"
        >
          <h2 v-if="wizard.currentStepDef.value.title">
            {{ wizard.currentStepDef.value.title }}
          </h2>
          <p v-if="wizard.currentStepDef.value.description" class="section-description">
            {{ wizard.currentStepDef.value.description }}
          </p>
          <div class="form-fields">
            <FormFieldList
              :fields="visibleStepFields(wizard.currentStepDef.value)"
              :entity-type="formConfig.entity"
              :entity-id="entityId"
              :form-data="formData"
              :relations="relations"
              :errors="errors"
              :relation-affordances="relationAffordances"
              :attachments="attachments"
              :save-generation="saveGeneration"
              :get-property-def="getPropertyDef"
              :is-field-readonly="isFieldReadonly"
              :option-verdicts-for="optionVerdictsFor"
              :transitions-for="transitionsFor"
              @update-field="updateField"
              @attachment-changed="onAttachmentChanged"
              @update-relation="updateRelation"
              @update-relation-types="updateRelationTypes"
              @incoming-changed="updateIncomingPicker"
              @cards-changed="updateRelationCards"
            />
          </div>
        </div>

        <!-- Degenerate config: every step is conditionally hidden right now. -->
        <p v-else class="section-description">No fields to display.</p>

        <!-- Content field (markdown body). Shown on the final (or only) step. -->
        <div v-if="wizard.isLastStep.value" class="form-field content-field">
          <label for="content">Content</label>
          <MarkdownEditor
            :model-value="content"
            placeholder="Markdown content..."
            @update:model-value="updateContent"
          />
        </div>

        <!-- Submit-time validation summary (create only — edit autosaves and has
             no submit gate). Announced via role=alert. -->
        <p v-if="!isEdit && errorCount > 0" class="wizard-error-summary" role="alert">
          ⚠ {{ errorCount }}
          {{ errorCount === 1 ? 'field needs' : 'fields need' }} attention<template
            v-if="wizard.isMultiStep.value && stepsWithErrors.size > 0"
          >
            — see the flagged step{{ stepsWithErrors.size === 1 ? '' : 's' }} above</template
          >.
        </p>

        <!--
          Actions branch on MODE, not on wizard-ness, so a wizard and a flat form
          behave identically:
          - EDIT: autosave per field, no Save button (the route guard flushes
            pending edits on leave); step Back/Next only when multi-step.
          - CREATE: Cancel + Back/Next (when multi-step) + Create on the last step.
        -->
        <div v-if="wizard.currentStepDef.value" class="form-actions mobile-actionbar">
          <!-- Leave-the-form control (autosave Back in edit, Cancel in create). -->
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="!autoSave && saving"
            @click="handleCancel"
          >
            {{ autoSave ? 'Back' : 'Cancel' }} <kbd>Esc</kbd>
          </button>

          <!-- Step navigation (multi-step only). -->
          <template v-if="wizard.isMultiStep.value">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="wizard.isFirstStep.value"
              @click="handleBack"
            >
              ← Prev step
            </button>
            <button
              v-if="!wizard.isLastStep.value"
              type="button"
              class="btn btn-primary"
              @click="handleNext"
            >
              Next step →
            </button>
          </template>

          <!-- Create only: the commit button, on the last (or only) step. -->
          <button
            v-if="!isEdit && wizard.isLastStep.value"
            type="submit"
            class="btn btn-primary"
            :disabled="saving"
          >
            {{ saving ? 'Saving...' : 'Create' }} <kbd>&#8984;&#8629;</kbd>
          </button>

          <!-- Edit: ambient autosave status stands in for a Save button. -->
          <AutoSaveIndicator
            v-if="autoSave"
            :status="autoSave.status"
            :error="autoSave.lastError"
          />
        </div>
      </form>
    </div>

    <!-- Side panel for edit mode -->
    <SidePanel v-if="isEdit && entityId" :form-id="formId" :entity-id="entityId" />
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

/* Embedded: the host dialog owns the width. The page min-width would
   otherwise force the modal wider than a narrow viewport allows. */
.dynamic-form.embedded {
  max-width: none;
  min-width: 0;
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

/* Wizard stepper */
.wizard-steps {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  list-style: none;
  margin: 0 0 20px;
  padding: 0;
}

.wizard-step-pill {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 999px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  color: var(--muted-text);
  font-size: 13px;
  font-family: inherit;
  cursor: pointer;
  transition:
    border-color 0.15s,
    color 0.15s;
}

.wizard-step-pill:hover {
  border-color: var(--accent-color);
  color: var(--text-color);
}

.wizard-step-pill:focus-visible {
  outline: 2px solid var(--accent-color);
  outline-offset: 2px;
}

/* "You are here" = a filled pill. This is the primary cue and is INDEPENDENT
   of the error color, so an active step still reads as active even when it also
   has an error (see .active.has-errors below). */
.wizard-step-pill.active {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: #fff;
  font-weight: 600;
}

.wizard-step-pill.done {
  color: var(--text-color);
}

.wizard-step-num {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--border-color);
  font-size: 12px;
}

.wizard-step-pill.done .wizard-step-num {
  background: var(--accent-color);
  color: #fff;
}

/* On the filled active pill, the number badge is inverted (light on accent). */
.wizard-step-pill.active .wizard-step-num {
  background: #fff;
  color: var(--accent-color);
}

/* An errored step: red outline + red number badge, but NOT active. */
.wizard-step-pill.has-errors:not(.active) {
  border-color: var(--error-color);
  color: var(--error-color);
}

.wizard-step-pill.has-errors:not(.active) .wizard-step-num {
  background: var(--error-color);
  color: #fff;
}

/* The active step that also has an error: filled with the error color, so it
   still reads as "you are here" while signalling the problem. */
.wizard-step-pill.active.has-errors {
  background: var(--error-color);
  border-color: var(--error-color);
  color: #fff;
}

.wizard-step-pill.active.has-errors .wizard-step-num {
  background: #fff;
  color: var(--error-color);
}

.wizard-error-summary {
  color: var(--error-color);
  font-size: 13px;
  font-weight: 600;
  margin: 0 0 12px;
}

/* Same 12-column layout grid as the detail page (TKT-5V8704), so `span:` in
 * data-entry.yaml means the same thing on a form as in a view section. Forms
 * were already single-column via flex-column; the grid preserves that for
 * unspanned fields while letting an author group related ones onto a row. */
.form-fields {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 20px var(--space-xl);
  /* Top-align, don't stretch. Grid items default to `stretch`, which makes
     every field in a row as tall as the tallest — so one field with a
     transitions panel or a long help text under it leaves its neighbours
     with a void beneath their input. Aligning to the start keeps each
     control tight to its own label. */
  align-items: start;
}

/* EVERY direct child spans the full 12 by default, not just `.form-field`.
 * FormFieldList also emits RelationCards / RelationPicker, which carry their
 * own root class — without this they'd become auto-width grid items and the
 * whole form would collapse into narrow columns.
 *
 * The var() fallback (not a bare `span 12`) is load-bearing: `.form-fields > *`
 * and `.form-field` have equal specificity, so a plain `span 12` here would win
 * on source order and silently swallow every authored span. Reading the same
 * custom property means both rules agree, and the ONE default lives in the
 * fallback. */
.form-fields > * {
  grid-column: span var(--field-span, 12);
  min-width: 0;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  /* Grid items floor at min-content without this; a long placeholder or
     option label would push its track wider than its share. */
  min-width: 0;
}

@media (max-width: 640px) {
  .form-fields {
    grid-template-columns: minmax(0, 1fr);
  }

  .form-fields > *,
  .form-field {
    grid-column: span 1;
  }
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
