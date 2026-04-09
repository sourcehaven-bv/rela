<script setup lang="ts">
import { ref, computed, toRef, watch } from 'vue'
import { useSchemaStore } from '@/stores'
import { createEntity } from '@/api'
import { useModalStack } from '@/composables/modalStack'
import type { Entity, PropertyDef } from '@/types'

const props = defineProps<{
  show: boolean
  entityType: string
}>()

const emit = defineEmits<{
  close: []
  created: [entity: Entity]
}>()

const schemaStore = useSchemaStore()
useModalStack(toRef(props, 'show'))

// State
const loading = ref(false)
const error = ref<string | null>(null)
const formData = ref<Record<string, string | boolean>>({})
const manualId = ref('')

// Computed
const entityTypeDef = computed(() => schemaStore.getEntityType(props.entityType))

const requiresManualId = computed(() => {
  return entityTypeDef.value?.id_type === 'manual'
})

const fields = computed(() => {
  if (!entityTypeDef.value) return []

  const result: Array<{ name: string; def: PropertyDef }> = []
  for (const [name, def] of Object.entries(entityTypeDef.value.properties)) {
    // Skip internal properties
    if (name.startsWith('_')) continue
    result.push({ name, def })
  }
  return result
})

// Reset form when modal opens
watch(() => props.show, (show) => {
  if (show) {
    error.value = null
    formData.value = {}
    manualId.value = ''

    // Initialize with defaults
    for (const field of fields.value) {
      if (field.def.default) {
        formData.value[field.name] = field.def.default
      } else if (field.def.type === 'boolean') {
        formData.value[field.name] = false
      } else {
        formData.value[field.name] = ''
      }
    }
  }
})

// Get enum values for a property
function getEnumValues(def: PropertyDef): string[] {
  if (def.values?.length) return def.values

  // Check custom types
  const customType = schemaStore.customTypes.get(def.type)
  if (customType?.values?.length) return customType.values

  return []
}

// Determine input type
function getInputType(def: PropertyDef): string {
  if (def.type === 'date') return 'date'
  if (def.type === 'integer') return 'number'
  if (def.format === 'long' || def.format === 'multiline') return 'textarea'
  return 'text'
}

function isEnum(def: PropertyDef): boolean {
  return def.type === 'enum' || getEnumValues(def).length > 0
}

function isBoolean(def: PropertyDef): boolean {
  return def.type === 'boolean'
}

function isTextarea(def: PropertyDef): boolean {
  return def.format === 'long' || def.format === 'multiline'
}

// Format label
function formatLabel(name: string): string {
  return name
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

// Submit
async function handleSubmit() {
  if (loading.value) return

  loading.value = true
  error.value = null

  try {
    // Build properties, filtering out empty values
    const properties: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(formData.value)) {
      if (value !== '' && value !== false) {
        properties[key] = value
      }
    }

    const payload: { id?: string; properties: Record<string, unknown> } = { properties }
    if (requiresManualId.value && manualId.value) {
      payload.id = manualId.value
    }

    const entity = await createEntity(props.entityType, payload)
    emit('created', entity)
    emit('close')
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to create entity'
  } finally {
    loading.value = false
  }
}

function handleClose() {
  if (!loading.value) {
    emit('close')
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="modal-overlay" @click.self="handleClose">
      <div class="modal-content">
        <header class="modal-header">
          <h3>Create {{ entityTypeDef?.label || entityType }}</h3>
          <button type="button" class="close-btn" @click="handleClose">&times;</button>
        </header>

        <form @submit.prevent="handleSubmit">
          <div class="modal-body">
            <div v-if="error" class="error-message">{{ error }}</div>

            <!-- Manual ID field -->
            <div v-if="requiresManualId" class="form-field">
              <label>
                ID
                <span class="required">*</span>
              </label>
              <input
                v-model="manualId"
                type="text"
                required
                placeholder="Unique ID..."
              />
            </div>

            <!-- Dynamic fields from entity type -->
            <div v-for="field in fields" :key="field.name" class="form-field">
              <label>
                {{ formatLabel(field.name) }}
                <span v-if="field.def.required" class="required">*</span>
              </label>

              <!-- Boolean checkbox -->
              <template v-if="isBoolean(field.def)">
                <div class="checkbox-row">
                  <input
                    :id="`inline-${field.name}`"
                    v-model="formData[field.name]"
                    type="checkbox"
                  />
                  <label :for="`inline-${field.name}`">{{ formatLabel(field.name) }}</label>
                </div>
              </template>

              <!-- Enum select -->
              <template v-else-if="isEnum(field.def)">
                <select v-model="formData[field.name]" :required="field.def.required">
                  <option value="">Select...</option>
                  <option v-for="val in getEnumValues(field.def)" :key="val" :value="val">
                    {{ val }}
                  </option>
                </select>
              </template>

              <!-- Textarea -->
              <template v-else-if="isTextarea(field.def)">
                <textarea
                  v-model="formData[field.name] as string"
                  :required="field.def.required"
                  :placeholder="field.def.description || ''"
                  rows="3"
                />
              </template>

              <!-- Regular input -->
              <template v-else>
                <input
                  v-model="formData[field.name]"
                  :type="getInputType(field.def)"
                  :required="field.def.required"
                  :placeholder="field.def.description || ''"
                />
              </template>

              <p v-if="field.def.description && !isTextarea(field.def)" class="field-help">
                {{ field.def.description }}
              </p>
            </div>
          </div>

          <footer class="modal-footer">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="handleClose">
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" :disabled="loading">
              {{ loading ? 'Creating...' : 'Create' }}
            </button>
          </footer>
        </form>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: var(--card-bg);
  border-radius: 8px;
  width: 100%;
  max-width: 480px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.2);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color);
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  color: var(--text-color);
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--muted-text);
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color);
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 20px;
  border-top: 1px solid var(--border-color);
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-field > label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-color);
}

.form-field input[type="text"],
.form-field input[type="number"],
.form-field input[type="date"],
.form-field textarea,
.form-field select {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.form-field input:focus,
.form-field textarea:focus,
.form-field select:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.form-field textarea {
  resize: vertical;
  min-height: 80px;
}

.checkbox-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.checkbox-row input[type="checkbox"] {
  width: 18px;
  height: 18px;
}

.checkbox-row label {
  font-size: 14px;
  color: var(--text-color);
  cursor: pointer;
}

.field-help {
  font-size: 12px;
  color: var(--muted-text);
  margin: 0;
}

.required {
  color: var(--error-color, #ef4444);
  margin-left: 2px;
}

.error-message {
  padding: 10px 12px;
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid var(--error-color, #ef4444);
  border-radius: 6px;
  color: var(--error-color, #ef4444);
  font-size: 14px;
}

.btn {
  padding: 10px 16px;
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
  filter: brightness(1.1);
}

.btn-secondary {
  background: var(--border-color);
  color: var(--text-color);
}

.btn-secondary:hover:not(:disabled) {
  filter: brightness(0.95);
}
</style>
