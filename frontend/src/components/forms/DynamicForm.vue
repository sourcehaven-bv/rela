<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, onBeforeRouteLeave } from 'vue-router'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import type { PropertyDef, FormFieldOrRelation } from '@/types'
import FieldRenderer from './FieldRenderer.vue'
import RelationPicker from './RelationPicker.vue'

const props = defineProps<{
  formId: string
  entityId?: string
}>()

const router = useRouter()
const schemaStore = useSchemaStore()
const entitiesStore = useEntitiesStore()
const uiStore = useUIStore()

// State
const formData = ref<Record<string, unknown>>({})
const relations = ref<Record<string, string[]>>({})
const content = ref('')
const loading = ref(true)
const saving = ref(false)
const dirty = ref(false)
const errors = ref<Record<string, string>>({})
const originalData = ref<string>('')

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

// Methods
async function loadEntity() {
  if (!props.entityId || !formConfig.value) return

  try {
    const entity = await entitiesStore.fetchEntity(
      formConfig.value.entity,
      props.entityId
    )
    formData.value = { ...entity.properties }
    relations.value = { ...entity.relations } || {}
    content.value = entity.content || ''
    originalData.value = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
  } catch (err) {
    uiStore.error('Failed to load entity')
    console.error(err)
  }
}

function initializeDefaults() {
  if (!entityType.value || isEdit.value) return

  for (const [propName, propDef] of Object.entries(entityType.value.properties)) {
    if (propDef.default !== undefined) {
      formData.value[propName] = propDef.default
    }
  }

  // Also apply form-level defaults
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

  originalData.value = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
}

function validate(): boolean {
  errors.value = {}

  if (!entityType.value) return true

  for (const [propName, propDef] of Object.entries(entityType.value.properties)) {
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
    const payload = {
      properties: formData.value,
      relations: relations.value,
      content: content.value || undefined,
    }

    if (isEdit.value && props.entityId) {
      await entitiesStore.update(formConfig.value.entity, props.entityId, payload)
      uiStore.success('Entity updated successfully')
    } else {
      const entity = await entitiesStore.create(formConfig.value.entity, payload)
      uiStore.success('Entity created successfully')
      router.push(`/entity/${formConfig.value.entity}/${entity.id}`)
      return
    }

    dirty.value = false
    originalData.value = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
    router.back()
  } catch (err) {
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

function updateContent(value: string) {
  content.value = value
  checkDirty()
}

function checkDirty() {
  const currentData = JSON.stringify({ formData: formData.value, relations: relations.value, content: content.value })
  dirty.value = currentData !== originalData.value
}

function getPropertyDef(property: string): PropertyDef | undefined {
  return entityType.value?.properties[property]
}

// Lifecycle & Navigation Guards
onMounted(async () => {
  loading.value = true
  if (isEdit.value) {
    await loadEntity()
  } else {
    initializeDefaults()
  }
  loading.value = false
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

// Warn before browser close
function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (dirty.value) {
    e.preventDefault()
    e.returnValue = ''
  }
}

onMounted(() => {
  window.addEventListener('beforeunload', handleBeforeUnload)
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
})
</script>

<template>
  <div class="dynamic-form" v-if="formConfig">
    <header class="form-header">
      <h1>{{ title }}</h1>
    </header>

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
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
            <template v-for="field in section.fields" :key="field.property || field.relation">
              <FieldRenderer
                v-if="field.property && !field.hidden"
                :field="field"
                :property-def="getPropertyDef(field.property)"
                :value="formData[field.property]"
                :error="errors[field.property]"
                :readonly="field.readonly"
                @update="updateField(field.property!, $event)"
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
        <template v-for="field in fields" :key="field.property || field.relation">
          <FieldRenderer
            v-if="field.property && !field.hidden"
            :field="field"
            :property-def="getPropertyDef(field.property)"
            :value="formData[field.property]"
            :error="errors[field.property]"
            :readonly="field.readonly"
            @update="updateField(field.property!, $event)"
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
        <textarea
          id="content"
          v-model="content"
          @input="updateContent(($event.target as HTMLTextAreaElement).value)"
          rows="10"
          placeholder="Markdown content..."
        ></textarea>
      </div>

      <div class="form-actions">
        <button
          type="button"
          class="btn btn-secondary"
          @click="handleCancel"
          :disabled="saving"
        >
          Cancel
        </button>
        <button
          type="submit"
          class="btn btn-primary"
          :disabled="saving"
        >
          {{ saving ? 'Saving...' : (isEdit ? 'Save Changes' : 'Create') }}
        </button>
      </div>
    </form>
  </div>

  <div v-else class="error-state">
    <h2>Form not found</h2>
    <p>The form "{{ formId }}" does not exist in the configuration.</p>
  </div>
</template>

<style scoped>
.dynamic-form {
  max-width: 800px;
}

.form-header {
  margin-bottom: 24px;
}

.form-header h1 {
  margin: 0;
}

.loading-state {
  display: flex;
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

.form-section {
  background: white;
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
  color: #64748b;
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
  color: #374151;
}

.content-field {
  background: white;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  padding: 24px;
  margin-bottom: 24px;
}

.content-field textarea {
  width: 100%;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-family: monospace;
  font-size: 14px;
  resize: vertical;
}

.content-field textarea:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 24px;
}

.btn {
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--accent-color, #6366f1);
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #4f46e5;
}

.btn-secondary {
  background: var(--border-color, #e2e8f0);
  color: var(--text-color, #1e293b);
}

.btn-secondary:hover:not(:disabled) {
  background: #cbd5e1;
}

.error-state {
  padding: 48px;
  text-align: center;
  color: #64748b;
}

.error-state h2 {
  color: var(--error-color, #ef4444);
}
</style>
