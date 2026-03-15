<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import { getSettings, saveSettings } from '@/api'
import type {
  SettingsData,
  SettingsPropertyDef,
  SettingsRelationDef,
  UserDefaults,
  DefaultOverride,
} from '@/api/settings'
import TagSelect from '@/components/ui/TagSelect.vue'

const schemaStore = useSchemaStore()
const uiStore = useUIStore()

// State
const loading = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)

const allProperties = ref<SettingsPropertyDef[]>([])
const allRelations = ref<SettingsRelationDef[]>([])
const entityTypes = ref<string[]>([])

// User defaults state
const propertyDefaults = ref<Record<string, string>>({})
const relationDefaults = ref<Record<string, string>>({})
const overrides = ref<DefaultOverride[]>([])

// UI state for adding new items
const selectedNewProperty = ref('')
const selectedNewRelation = ref('')

// Computed
const availableProperties = computed(() => {
  return allProperties.value.filter((p) => !(p.name in propertyDefaults.value))
})

const availableRelations = computed(() => {
  return allRelations.value.filter((r) => !(r.name in relationDefaults.value))
})

function getPropertyDef(name: string): SettingsPropertyDef | undefined {
  return allProperties.value.find((p) => p.name === name)
}

function getRelationDef(name: string): SettingsRelationDef | undefined {
  return allRelations.value.find((r) => r.name === name)
}

// Methods
async function loadSettings() {
  loading.value = true
  error.value = null
  try {
    const data: SettingsData = await getSettings()
    allProperties.value = data.allProperties || []
    allRelations.value = data.allRelations || []
    entityTypes.value = data.entityTypes || []

    propertyDefaults.value = { ...(data.userDefaults.defaults || {}) }
    relationDefaults.value = { ...(data.userDefaults.relationDefaults || {}) }
    overrides.value = (data.userDefaults.overrides || []).map((o) => ({
      types: [...o.types],
      defaults: { ...o.defaults },
      relationDefaults: { ...o.relationDefaults },
    }))
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load settings'
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  error.value = null
  try {
    const userDefaults: UserDefaults = {
      defaults: { ...propertyDefaults.value },
      relationDefaults: { ...relationDefaults.value },
      overrides: overrides.value.map((o) => ({
        types: [...o.types],
        defaults: { ...o.defaults },
        relationDefaults: { ...o.relationDefaults },
      })),
    }
    await saveSettings(userDefaults)
    uiStore.success('Settings saved successfully')
  } catch (err) {
    uiStore.error('Failed to save settings')
    error.value = err instanceof Error ? err.message : 'Failed to save settings'
  } finally {
    saving.value = false
  }
}

function addPropertyDefault() {
  if (!selectedNewProperty.value) return
  propertyDefaults.value[selectedNewProperty.value] = ''
  selectedNewProperty.value = ''
}

function removePropertyDefault(name: string) {
  delete propertyDefaults.value[name]
}

function addRelationDefault() {
  if (!selectedNewRelation.value) return
  relationDefaults.value[selectedNewRelation.value] = ''
  selectedNewRelation.value = ''
}

function removeRelationDefault(name: string) {
  delete relationDefaults.value[name]
}

function addOverrideGroup() {
  overrides.value.push({
    types: [],
    defaults: {},
    relationDefaults: {},
  })
}

function removeOverrideGroup(index: number) {
  overrides.value.splice(index, 1)
}

function addOverrideProperty(overrideIndex: number, propName: string) {
  if (!propName) return
  overrides.value[overrideIndex].defaults[propName] = ''
}

function removeOverrideProperty(overrideIndex: number, propName: string) {
  delete overrides.value[overrideIndex].defaults[propName]
}

function addOverrideRelation(overrideIndex: number, relName: string) {
  if (!relName) return
  overrides.value[overrideIndex].relationDefaults[relName] = ''
}

function removeOverrideRelation(overrideIndex: number, relName: string) {
  delete overrides.value[overrideIndex].relationDefaults[relName]
}

function getAvailableOverrideProperties(overrideIndex: number): SettingsPropertyDef[] {
  const used = overrides.value[overrideIndex].defaults
  return allProperties.value.filter((p) => !(p.name in used))
}

function getAvailableOverrideRelations(overrideIndex: number): SettingsRelationDef[] {
  const used = overrides.value[overrideIndex].relationDefaults
  return allRelations.value.filter((r) => !(r.name in used))
}

onMounted(() => {
  loadSettings()
})
</script>

<template>
  <div class="settings-view">
    <div class="page-header">
      <div>
        <h1>Settings</h1>
        <p class="subtitle">Configure default values for new entities</p>
        <p class="file-path">.rela/user-defaults.yaml</p>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <span>Loading settings...</span>
    </div>

    <div v-else-if="error" class="error-state">
      {{ error }}
      <button class="btn btn-secondary btn-sm" @click="loadSettings">Retry</button>
    </div>

    <form v-else class="settings-form" @submit.prevent="handleSave">
      <!-- Property Defaults -->
      <div class="settings-card">
        <h3>Property Defaults</h3>
        <p class="description">Default values applied when creating any entity type.</p>

        <div class="settings-rows">
          <div
            v-for="(_, propName) in propertyDefaults"
            :key="propName"
            class="settings-row"
          >
            <span class="row-label">{{ propName }}</span>
            <div class="row-value">
              <template v-if="getPropertyDef(propName as string)">
                <select
                  v-if="getPropertyDef(propName as string)?.values?.length"
                  v-model="propertyDefaults[propName as string]"
                >
                  <option value="">-</option>
                  <option
                    v-for="val in getPropertyDef(propName as string)?.values"
                    :key="val"
                    :value="val"
                  >
                    {{ val }}
                  </option>
                </select>
                <select
                  v-else-if="getPropertyDef(propName as string)?.type === 'boolean'"
                  v-model="propertyDefaults[propName as string]"
                >
                  <option value="">-</option>
                  <option value="true">true</option>
                  <option value="false">false</option>
                </select>
                <input
                  v-else-if="getPropertyDef(propName as string)?.type === 'date'"
                  type="date"
                  v-model="propertyDefaults[propName as string]"
                />
                <input
                  v-else-if="getPropertyDef(propName as string)?.type === 'integer'"
                  type="number"
                  v-model="propertyDefaults[propName as string]"
                />
                <input
                  v-else
                  type="text"
                  v-model="propertyDefaults[propName as string]"
                />
              </template>
              <template v-else>
                <input type="text" v-model="propertyDefaults[propName as string]" />
                <span class="stale-badge">unknown</span>
              </template>
            </div>
            <button
              type="button"
              class="remove-btn"
              @click="removePropertyDefault(propName as string)"
            >
              &times;
            </button>
          </div>
        </div>

        <div class="add-row">
          <select v-model="selectedNewProperty" @change="addPropertyDefault">
            <option value="">Add property default...</option>
            <option v-for="prop in availableProperties" :key="prop.name" :value="prop.name">
              {{ prop.name }} ({{ prop.type }})
            </option>
          </select>
        </div>
      </div>

      <!-- Relation Defaults -->
      <div class="settings-card">
        <h3>Relation Defaults</h3>
        <p class="description">Default relations created when making a new entity.</p>

        <div class="settings-rows">
          <div
            v-for="(_, relName) in relationDefaults"
            :key="relName"
            class="settings-row"
          >
            <span class="row-label">{{ relName }}</span>
            <div class="row-value">
              <template v-if="getRelationDef(relName as string)">
                <select v-model="relationDefaults[relName as string]">
                  <option value="">-</option>
                  <option
                    v-for="target in getRelationDef(relName as string)?.targets"
                    :key="target.id"
                    :value="target.id"
                  >
                    {{ target.title }}
                  </option>
                </select>
              </template>
              <template v-else>
                <input type="text" v-model="relationDefaults[relName as string]" readonly />
                <span class="stale-badge">unknown</span>
              </template>
            </div>
            <button
              type="button"
              class="remove-btn"
              @click="removeRelationDefault(relName as string)"
            >
              &times;
            </button>
          </div>
        </div>

        <div class="add-row">
          <select v-model="selectedNewRelation" @change="addRelationDefault">
            <option value="">Add relation default...</option>
            <option v-for="rel in availableRelations" :key="rel.name" :value="rel.name">
              {{ rel.name }}{{ rel.targetType ? ` -> ${rel.targetType}` : '' }}
            </option>
          </select>
        </div>
      </div>

      <!-- Override Groups -->
      <div class="settings-card">
        <h3>Overrides</h3>
        <p class="description">
          Override defaults for specific entity types. First matching override takes precedence.
        </p>

        <div class="override-groups">
          <div
            v-for="(override, idx) in overrides"
            :key="idx"
            class="override-group"
          >
            <div class="override-header">
              <div class="override-types">
                <label>Entity Types</label>
                <TagSelect
                  v-model="override.types"
                  :options="entityTypes"
                  placeholder="Select entity types..."
                />
              </div>
              <button
                type="button"
                class="remove-btn large"
                @click="removeOverrideGroup(idx)"
              >
                &times;
              </button>
            </div>

            <div class="override-section">
              <label>Properties</label>
              <div class="settings-rows">
                <div
                  v-for="(_, propName) in override.defaults"
                  :key="propName"
                  class="settings-row"
                >
                  <span class="row-label">{{ propName }}</span>
                  <div class="row-value">
                    <template v-if="getPropertyDef(propName as string)">
                      <select
                        v-if="getPropertyDef(propName as string)?.values?.length"
                        v-model="override.defaults[propName as string]"
                      >
                        <option value="">-</option>
                        <option
                          v-for="val in getPropertyDef(propName as string)?.values"
                          :key="val"
                          :value="val"
                        >
                          {{ val }}
                        </option>
                      </select>
                      <select
                        v-else-if="getPropertyDef(propName as string)?.type === 'boolean'"
                        v-model="override.defaults[propName as string]"
                      >
                        <option value="">-</option>
                        <option value="true">true</option>
                        <option value="false">false</option>
                      </select>
                      <input
                        v-else-if="getPropertyDef(propName as string)?.type === 'date'"
                        type="date"
                        v-model="override.defaults[propName as string]"
                      />
                      <input
                        v-else-if="getPropertyDef(propName as string)?.type === 'integer'"
                        type="number"
                        v-model="override.defaults[propName as string]"
                      />
                      <input v-else type="text" v-model="override.defaults[propName as string]" />
                    </template>
                    <template v-else>
                      <input type="text" v-model="override.defaults[propName as string]" />
                      <span class="stale-badge">unknown</span>
                    </template>
                  </div>
                  <button
                    type="button"
                    class="remove-btn"
                    @click="removeOverrideProperty(idx, propName as string)"
                  >
                    &times;
                  </button>
                </div>
              </div>
              <select
                class="add-select"
                @change="(e) => { addOverrideProperty(idx, (e.target as HTMLSelectElement).value); (e.target as HTMLSelectElement).value = '' }"
              >
                <option value="">Add property...</option>
                <option
                  v-for="prop in getAvailableOverrideProperties(idx)"
                  :key="prop.name"
                  :value="prop.name"
                >
                  {{ prop.name }} ({{ prop.type }})
                </option>
              </select>
            </div>

            <div class="override-section">
              <label>Relations</label>
              <div class="settings-rows">
                <div
                  v-for="(_, relName) in override.relationDefaults"
                  :key="relName"
                  class="settings-row"
                >
                  <span class="row-label">{{ relName }}</span>
                  <div class="row-value">
                    <template v-if="getRelationDef(relName as string)">
                      <select v-model="override.relationDefaults[relName as string]">
                        <option value="">-</option>
                        <option
                          v-for="target in getRelationDef(relName as string)?.targets"
                          :key="target.id"
                          :value="target.id"
                        >
                          {{ target.title }}
                        </option>
                      </select>
                    </template>
                    <template v-else>
                      <input
                        type="text"
                        v-model="override.relationDefaults[relName as string]"
                        readonly
                      />
                      <span class="stale-badge">unknown</span>
                    </template>
                  </div>
                  <button
                    type="button"
                    class="remove-btn"
                    @click="removeOverrideRelation(idx, relName as string)"
                  >
                    &times;
                  </button>
                </div>
              </div>
              <select
                class="add-select"
                @change="(e) => { addOverrideRelation(idx, (e.target as HTMLSelectElement).value); (e.target as HTMLSelectElement).value = '' }"
              >
                <option value="">Add relation...</option>
                <option
                  v-for="rel in getAvailableOverrideRelations(idx)"
                  :key="rel.name"
                  :value="rel.name"
                >
                  {{ rel.name }}{{ rel.targetType ? ` -> ${rel.targetType}` : '' }}
                </option>
              </select>
            </div>
          </div>
        </div>

        <button type="button" class="btn btn-secondary btn-sm" @click="addOverrideGroup">
          + Add override group
        </button>
      </div>

      <!-- App Info -->
      <div class="settings-card">
        <h3>Application Info</h3>
        <div class="info-grid">
          <div class="info-row">
            <span class="info-label">App Name</span>
            <span class="info-value">{{ schemaStore.app.name }}</span>
          </div>
          <div class="info-row" v-if="schemaStore.app.description">
            <span class="info-label">Description</span>
            <span class="info-value">{{ schemaStore.app.description }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Entity Types</span>
            <span class="info-value">{{ schemaStore.entityTypes.size }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Relation Types</span>
            <span class="info-value">{{ schemaStore.relationTypes.size }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Forms</span>
            <span class="info-value">{{ schemaStore.forms.size }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">Lists</span>
            <span class="info-value">{{ schemaStore.lists.size }}</span>
          </div>
        </div>
      </div>

      <!-- Form Actions -->
      <div class="form-actions">
        <button type="submit" class="btn btn-primary" :disabled="saving">
          {{ saving ? 'Saving...' : 'Save' }}
        </button>
        <button type="button" class="btn btn-secondary" @click="loadSettings">Reset</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.settings-view {
  max-width: 720px;
}

.page-header {
  margin-bottom: 24px;
}

h1 {
  margin: 0 0 4px;
}

.subtitle {
  color: #64748b;
  font-size: 14px;
  margin: 0 0 4px;
}

.file-path {
  font-family: monospace;
  font-size: 12px;
  color: #94a3b8;
  margin: 0;
}

.settings-card {
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}

.settings-card h3 {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 600;
}

.description {
  color: #64748b;
  font-size: 13px;
  margin: 0 0 16px;
}

.settings-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.settings-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: #f8fafc;
  border-radius: 6px;
}

.row-label {
  min-width: 120px;
  font-size: 13px;
  font-weight: 500;
  color: #475569;
}

.row-value {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
}

.row-value select,
.row-value input {
  flex: 1;
  padding: 6px 10px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 13px;
}

.remove-btn {
  background: none;
  border: none;
  font-size: 18px;
  color: #94a3b8;
  cursor: pointer;
  padding: 0 4px;
  line-height: 1;
}

.remove-btn:hover {
  color: #ef4444;
}

.remove-btn.large {
  font-size: 24px;
  align-self: flex-start;
  margin-top: 20px;
}

.stale-badge {
  background: #fef3c7;
  color: #d97706;
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 500;
}

.add-row {
  margin-top: 12px;
}

.add-row select,
.add-select {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 13px;
  color: #64748b;
  background: white;
}

.override-groups {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 16px;
}

.override-group {
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  padding: 16px;
  background: #fafafa;
}

.override-header {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.override-types {
  flex: 1;
}

.override-types label {
  display: block;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: #64748b;
  margin-bottom: 6px;
}

.override-section {
  margin-top: 12px;
}

.override-section > label {
  display: block;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: #64748b;
  margin-bottom: 6px;
}

.override-section .add-select {
  margin-top: 8px;
}

.info-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  padding: 8px 12px;
  background: #f8fafc;
  border-radius: 6px;
}

.info-label {
  color: #64748b;
  font-size: 14px;
}

.info-value {
  font-size: 14px;
  font-weight: 500;
  color: #1e293b;
}

.form-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
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

.btn-primary {
  background: var(--accent-color, #6366f1);
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #4f46e5;
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-secondary {
  background: #f1f5f9;
  color: #475569;
  border: 1px solid var(--border-color, #e2e8f0);
}

.btn-secondary:hover {
  background: #e2e8f0;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 13px;
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 48px;
  color: #64748b;
}

.spinner {
  width: 24px;
  height: 24px;
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

.error-state {
  background: #fef2f2;
  color: #dc2626;
  padding: 16px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>
