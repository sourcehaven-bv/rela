<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute, onBeforeRouteLeave } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import { isCancelledFetch } from '@/composables/usePageData'
import type { PropertyDef, FormFieldOrRelation, Template } from '@/types'
import { getTemplates, createRelation, updateRelationProperties, deleteRelation } from '@/api'
import type { RelationCardState } from './RelationCards.vue'
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
const content = ref('')
const loading = ref(true)
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

const title = computed(() => {
  if (!formConfig.value) return ''
  const label = entityType.value?.label || formConfig.value.entity
  return isEdit.value ? `Edit ${label}` : `New ${label}`
})

const fields = computed((): FormFieldOrRelation[] => {
  if (!formConfig.value) return []
  if (formConfig.value.sections?.length) {
    return formConfig.value.sections.flatMap((s) => s.fields) as FormFieldOrRelation[]
  }
  // Combine property fields and relation fields into a single list
  const propFields = (formConfig.value.fields || []) as FormFieldOrRelation[]
  const relFields = (formConfig.value.relations || []) as FormFieldOrRelation[]
  return [...propFields, ...relFields]
})

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
    formData.value = { ...entity.properties }
    relations.value = entity.relations ? { ...entity.relations } : {}
    content.value = entity.content || ''
    originalData.value = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
  } catch (err) {
    // Suppress cancellation errors from rapid navigation in Firefox
    // (see BUG-6C3V and src/composables/usePageData.ts).
    if (isCancelledFetch(err)) return
    uiStore.error('Failed to load entity')
    console.error(err)
  }
}

function initializeDefaults() {
  if (!entityType.value || isEdit.value) return

  // Parse query params for pre-filling (prop.*, rel.*, link_*, return_to)
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
    } else if (key === 'return_to') {
      returnTo.value = value
    }
  }

  // Apply metamodel defaults
  for (const [propName, propDef] of Object.entries(entityType.value.properties)) {
    if (propDef.default !== undefined) {
      formData.value[propName] = propDef.default
    }
  }

  // Apply form-level defaults
  for (const field of fields.value) {
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

      if (propDef.values?.length && !propDef.values.includes(String(value))) {
        errors.value[propName] = `Must be one of: ${propDef.values.join(', ')}`
      }
    }
  }

  return Object.keys(errors.value).length === 0
}

async function handleSubmit() {
  if (!validate() || !formConfig.value) return

  saving.value = true
  try {
    // Exclude card-managed relations from the entity update payload —
    // those are saved separately via savePendingRelationCards()
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

    const payload = {
      properties: formData.value,
      relations: filteredRelations,
      content: content.value || undefined,
    }

    if (isEdit.value && props.entityId) {
      await entitiesStore.update(formConfig.value.entity, props.entityId, payload)
      // Save any pending relation card changes (adds, removes, property edits)
      await savePendingRelationCards()
      uiStore.success('Entity updated successfully')
    } else {
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
      if (returnTo.value && returnTo.value.startsWith('/')) {
        router.push(returnTo.value)
      } else {
        router.push(`/entity/${formConfig.value.entity}/${entity.id}`)
      }
      return
    }

    dirty.value = false
    originalData.value = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })

    // Navigate to return_to or back
    if (returnTo.value && returnTo.value.startsWith('/')) {
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
    if (err && typeof err === 'object' && 'errors' in err) {
      const problemErrors = (err as { errors: Array<{ field?: string; message?: string; detail?: string }> }).errors
      for (const e of problemErrors) {
        if (e.field) {
          errors.value[e.field] = e.message || e.detail || 'Invalid value'
        }
      }
      uiStore.error('Please fix the validation errors')
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
}

function updateRelation(relation: string, value: string[]) {
  relations.value[relation] = value
  checkDirty()
}

// Pending relation card changes (for batch save)
const pendingCardChanges = ref<Map<string, RelationCardState>>(new Map())

function updateRelationCards(relation: string, state: RelationCardState) {
  pendingCardChanges.value.set(relation, state)
  checkDirty()
}

async function savePendingRelationCards() {
  const entity = formConfig.value!.entity
  const entityId = props.entityId!

  await Promise.all(
    Array.from(pendingCardChanges.value.entries()).map(async ([key, state]) => {
      const relation = key.replace(/-outgoing$|-incoming$/, '')
      const direction = key.endsWith('-incoming') ? 'incoming' : undefined
      for (const targetId of state.removed) {
        await deleteRelation(entity, entityId, relation, targetId, direction)
      }
      for (const add of state.added) {
        await createRelation(entity, entityId, relation, add.targetId, add.meta, direction)
      }
      for (const upd of state.updated) {
        await updateRelationProperties(entity, entityId, relation, upd.targetId, upd.meta, direction)
      }
    })
  )

  pendingCardChanges.value.clear()
  saveGeneration.value++
}

function updateContent(value: string) {
  content.value = value
  checkDirty()
}

function checkDirty() {
  const currentData = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
  const hasCardChanges = pendingCardChanges.value.size > 0
  dirty.value = currentData !== originalData.value || hasCardChanges
}

function getPropertyDef(property: string): PropertyDef | undefined {
  return entityType.value?.properties[property]
}

// Warn before browser close
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

  // Load form data
  loading.value = true
  if (isEdit.value) {
    await loadEntity()
  } else {
    initializeDefaults()
    await loadTemplates()
  }
  loading.value = false
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
  document.removeEventListener('keydown', handleKeydown)
})

onBeforeRouteLeave((_to, _from, next) => {
  if (dirty.value) {
    const answer = window.confirm('You have unsaved changes. Are you sure you want to leave?')
    if (!answer) {
      next(false)
      return
    }
  }
  next()
})
</script>

<template>
  <div v-if="formConfig" class="form-layout" :class="{ 'with-sidepanel': isEdit }">
    <div class="dynamic-form">
      <header class="form-header">
        <button
          type="button"
          class="form-header-cancel"
          :disabled="saving"
          @click="handleCancel"
        >
          Cancel
        </button>
        <h1>{{ title }}</h1>
        <button
          type="button"
          class="help-btn"
          title="Show help for this entity type"
          @click="helpModalOpen = true"
        >
          ?
        </button>
        <button
          type="button"
          class="form-header-save"
          :disabled="saving"
          @click="handleSubmit"
        >
          {{ saving ? 'Saving...' : 'Save' }}
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

      <form v-else @submit.prevent="handleSubmit">
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
                  :readonly="field.readonly"
                  @update="updateField(field.property!, $event)"
                />
                <RelationCards
                  v-else-if="field.relation && field.widget === 'cards' && entityId"
                  :key="`cards-${field.relation}-${field.direction || 'outgoing'}-${saveGeneration}`"
                  :field="field"
                  :entity-type="formConfig.entity"
                  :entity-id="entityId"
                  @cards-changed="(state) => updateRelationCards(`${field.relation}-${field.direction || 'outgoing'}`, state)"
                />
                <RelationPicker
                  v-else-if="field.relation"
                  :field="field"
                  :entity-type="formConfig.entity"
                  :value="relations[field.relation] || []"
                  @update="updateRelation(field.relation!, $event)"
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
              :readonly="field.readonly"
              @update="updateField(field.property!, $event)"
            />
            <RelationCards
              v-else-if="field.relation && field.widget === 'cards' && entityId"
              :key="`cards-${field.relation}-${field.direction || 'outgoing'}-${saveGeneration}`"
              :field="field"
              :entity-type="formConfig.entity"
              :entity-id="entityId"
              @cards-changed="(state) => updateRelationCards(`${field.relation}-${field.direction || 'outgoing'}`, state)"
            />
            <RelationPicker
              v-else-if="field.relation"
              :field="field"
              :entity-type="formConfig.entity"
              :value="relations[field.relation] || []"
              @update="updateRelation(field.relation!, $event)"
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

        <div class="form-actions">
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
  flex: 1;
}

.form-header-cancel,
.form-header-save {
  display: none;
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

@media (max-width: 1024px) { /* BREAKPOINT:tablet */
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

  .form-header {
    position: sticky;
    top: 0;
    z-index: 102;
    background: var(--sidebar-bg, #1a1a2e);
    color: var(--sidebar-text, #e8e8e8);
    margin: -60px -16px 12px -16px;
    padding: 6px 12px;
    min-height: 44px;
    align-items: center;
  }

  .form-header h1 {
    font-size: 17px;
    font-weight: 600;
    color: var(--sidebar-text, #e8e8e8);
    line-height: 1;
  }

  .form-header .help-btn {
    color: var(--sidebar-text, #e8e8e8);
    background: none;
    border-color: rgba(255, 255, 255, 0.2);
  }

  .form-header-cancel {
    display: flex;
    align-items: center;
    align-self: center;
    background: none;
    border: none;
    color: var(--sidebar-text, #e8e8e8);
    font-size: 15px;
    line-height: 1;
    cursor: pointer;
    padding: 0;
  }

  .form-header-save {
    display: block;
    background: var(--accent-color, #6366f1);
    color: white;
    border: none;
    border-radius: 6px;
    font-size: 14px;
    font-weight: 600;
    padding: 6px 14px;
    cursor: pointer;
  }

  .form-header-save:disabled,
  .form-header-cancel:disabled {
    opacity: 0.5;
  }

  .form-actions {
    display: none;
  }

  .template-selector {
    flex-wrap: wrap;
    gap: 6px;
  }
}
</style>
