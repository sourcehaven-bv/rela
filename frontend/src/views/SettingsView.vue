<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import { getSettings, saveSettings, savePalette } from '@/api'
import type {
  SettingsData,
  SettingsPropertyDef,
  SettingsRelationDef,
  UserDefaults,
  DefaultOverride,
  PaletteConfig,
} from '@/api/settings'
import TagSelect from '@/components/ui/TagSelect.vue'
import { parsePalette, parseRelaPalette, assignPalette, deriveTheme } from '@/utils/palette'

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

// Palette state
const paletteColors = ref<Record<string, string>>({})
const paletteBadges = ref<Record<string, string>>({})
const savingPalette = ref(false)

const paletteRoles = [
  { key: 'base', label: 'Base', description: 'Sidebar & navigation background' },
  { key: 'surface', label: 'Surface', description: 'Main background' },
  { key: 'accent', label: 'Accent', description: 'Primary action color' },
  { key: 'text', label: 'Text', description: 'Main text color' },
  { key: 'success', label: 'Success', description: 'Success indicators' },
  { key: 'error', label: 'Error', description: 'Error indicators' },
  { key: 'warning', label: 'Warning', description: 'Warning indicators' },
  { key: 'info', label: 'Info', description: 'Info indicators' },
]

const badgeNames = ['blue', 'purple', 'green', 'gray', 'red', 'orange', 'yellow']

// Import state
const importText = ref('')
const importedColors = ref<string[]>([])
const selectedRole = ref<{ type: 'color' | 'badge'; key: string } | null>(null)
const dragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

// Dark mode editing state
const paletteDarkColors = ref<Record<string, string>>({})
const paletteDarkBadges = ref<Record<string, string>>({})
const editingDark = ref(false)

const MAX_FILE_SIZE = 102400 // 100KB

// Computed: which palette refs to show based on light/dark toggle
const activeColors = computed(() => editingDark.value ? paletteDarkColors.value : paletteColors.value)
const activeBadges = computed(() => editingDark.value ? paletteDarkBadges.value : paletteBadges.value)

// Map role keys to CSS variable names for looking up auto-generated dark values
const roleToCSSVar: Record<string, string> = {
  base: '--sidebar-bg', surface: '--bg-color', accent: '--accent-color', text: '--text-color',
  success: '--success-color', error: '--error-color', warning: '--warning-color', info: '--info-color',
}
const badgeToCSSVar = (name: string) => `--badge-${name}`

/** Get the auto-generated dark value for a role (from resolved palette). */
function autoDarkColor(key: string): string {
  const cssVar = roleToCSSVar[key]
  return cssVar ? (schemaStore.paletteDark[cssVar] || '') : ''
}

function autoDarkBadge(name: string): string {
  return schemaStore.paletteDark[badgeToCSSVar(name)] || ''
}

function setColor(key: string, value: string) {
  if (editingDark.value) {
    paletteDarkColors.value[key] = value
  } else {
    paletteColors.value[key] = value
  }
}

function setBadge(key: string, value: string) {
  if (editingDark.value) {
    paletteDarkBadges.value[key] = value
  } else {
    paletteBadges.value[key] = value
  }
}

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

    // Load palette
    const p = data.userPalette
    if (p) {
      paletteColors.value = {}
      for (const role of paletteRoles) {
        const val = p[role.key as keyof PaletteConfig]
        if (typeof val === 'string') paletteColors.value[role.key] = val
      }
      paletteBadges.value = { ...(p.badges || {}) }
    } else {
      paletteColors.value = {}
      paletteBadges.value = {}
    }
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

async function handleSavePalette() {
  savingPalette.value = true
  try {
    const palette: PaletteConfig = {}
    for (const [key, val] of Object.entries(paletteColors.value)) {
      if (val) (palette as Record<string, string>)[key] = val
    }
    const badges: Record<string, string> = {}
    for (const [key, val] of Object.entries(paletteBadges.value)) {
      if (val) badges[key] = val
    }
    if (Object.keys(badges).length > 0) palette.badges = badges

    // Include dark mode overrides if any are set
    const darkColors: Record<string, string> = {}
    for (const [key, val] of Object.entries(paletteDarkColors.value)) {
      if (val) darkColors[key] = val
    }
    for (const [key, val] of Object.entries(paletteDarkBadges.value)) {
      if (val) darkColors[key] = val
    }
    if (Object.keys(darkColors).length > 0) {
      (palette as Record<string, unknown>).dark = darkColors
    }

    await savePalette(palette)
    uiStore.success('Palette saved')
    await schemaStore.reload()
  } catch (err) {
    uiStore.error(err instanceof Error ? err.message : 'Failed to save palette')
  } finally {
    savingPalette.value = false
  }
}

async function handleResetPalette() {
  // Reload from disk — discard unsaved edits
  await loadSettings()
  // Reapply the saved palette from schema store
  const palette = uiStore.isDark ? schemaStore.paletteDark : schemaStore.paletteLight
  if (Object.keys(palette).length > 0) {
    uiStore.applyPalette(palette)
  } else {
    uiStore.clearPalette()
  }
  uiStore.info('Palette reset to saved state')
}

/** Build a full CSS variable map from current editing state for live preview.
 *  Uses deriveTheme to compute all 21 variables including derived ones. */
function buildPreviewPalette(): Record<string, string> {
  const src = editingDark.value ? paletteDarkColors.value : paletteColors.value
  const badgeSrc = editingDark.value ? paletteDarkBadges.value : paletteBadges.value

  // Only build preview if at least one color is set
  const hasColors = Object.values(src).some(Boolean)
  const hasBadges = Object.values(badgeSrc).some(Boolean)
  if (!hasColors && !hasBadges) return {}

  // Derive all 21 CSS variables (8 base + 6 computed + 7 badges)
  return deriveTheme(src, badgeSrc)
}

// Live preview: apply palette as user edits colors
const stopPreviewWatch = watch(
  [paletteColors, paletteBadges, paletteDarkColors, paletteDarkBadges, editingDark],
  () => {
    const preview = buildPreviewPalette()
    if (Object.keys(preview).length > 0) {
      uiStore.applyPalette(preview)
    }
  },
  { deep: true },
)

// On unmount, reapply the saved palette (discard unsaved preview)
onUnmounted(() => {
  stopPreviewWatch()
  const palette = uiStore.isDark ? schemaStore.paletteDark : schemaStore.paletteLight
  if (Object.keys(palette).length > 0) {
    uiStore.applyPalette(palette)
  } else {
    uiStore.clearPalette()
  }
})

function handleImport() {
  const colors = parsePalette(importText.value)
  if (colors.length === 0) {
    uiStore.warning('No valid colors found. Paste hex values or a GIMP Palette.')
    return
  }
  importedColors.value = colors
  const assignment = assignPalette(colors)

  // Apply assignment to palette state
  for (const [key, val] of Object.entries(assignment.colors)) {
    paletteColors.value[key] = val
  }
  for (const [key, val] of Object.entries(assignment.badges)) {
    paletteBadges.value[key] = val
  }

  uiStore.success(`Imported ${colors.length} colors and auto-assigned to roles`)
}

function clearImport() {
  importText.value = ''
  importedColors.value = []
  selectedRole.value = null
}

function handleFilePick() {
  fileInput.value?.click()
}

function handleFileSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) readFile(file)
  input.value = '' // reset so same file can be re-selected
}

function handleDragOver(event: DragEvent) {
  event.preventDefault()
  dragging.value = true
}

function handleDragLeave() {
  dragging.value = false
}

function handleDrop(event: DragEvent) {
  event.preventDefault()
  dragging.value = false
  const file = event.dataTransfer?.files[0]
  if (file) readFile(file)
}

function readFile(file: File) {
  if (file.size > MAX_FILE_SIZE) {
    uiStore.warning('File too large (max 100KB)')
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    const text = reader.result as string
    importText.value = text

    // If it's a rela palette YAML, do structured import
    const isYaml = file.name.endsWith('.yaml') || file.name.endsWith('.yml')
    if (isYaml) {
      const result = parseRelaPalette(text)
      importedColors.value = result.allColors
      for (const [key, val] of Object.entries(result.colors)) {
        paletteColors.value[key] = val
      }
      for (const [key, val] of Object.entries(result.badges)) {
        paletteBadges.value[key] = val
      }
      if (result.dark) {
        for (const [key, val] of Object.entries(result.dark)) {
          paletteDarkColors.value[key] = val
        }
      }
      uiStore.success(`Imported palette from ${file.name}`)
    } else {
      handleImport()
    }
  }
  reader.readAsText(file)
}

function selectRole(type: 'color' | 'badge', key: string) {
  if (selectedRole.value?.type === type && selectedRole.value?.key === key) {
    selectedRole.value = null // deselect
  } else {
    selectedRole.value = { type, key }
  }
}

function assignSwatch(hex: string) {
  if (!selectedRole.value) return
  const colorsRef = editingDark.value ? paletteDarkColors : paletteColors
  const badgesRef = editingDark.value ? paletteDarkBadges : paletteBadges
  if (selectedRole.value.type === 'color') {
    colorsRef.value[selectedRole.value.key] = hex
  } else {
    badgesRef.value[selectedRole.value.key] = hex
  }
}

function isRoleSelected(type: 'color' | 'badge', key: string): boolean {
  return selectedRole.value?.type === type && selectedRole.value?.key === key
}

function clearPaletteColor(key: string) {
  if (editingDark.value) {
    delete paletteDarkColors.value[key]
  } else {
    delete paletteColors.value[key]
  }
}

function clearBadgeColor(key: string) {
  if (editingDark.value) {
    delete paletteDarkBadges.value[key]
  } else {
    delete paletteBadges.value[key]
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
      <div class="spinner"/>
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
                  v-model="propertyDefaults[propName as string]"
                  type="date"
                />
                <input
                  v-else-if="getPropertyDef(propName as string)?.type === 'integer'"
                  v-model="propertyDefaults[propName as string]"
                  type="number"
                />
                <input
                  v-else
                  v-model="propertyDefaults[propName as string]"
                  type="text"
                />
              </template>
              <template v-else>
                <input v-model="propertyDefaults[propName as string]" type="text" />
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
                <input v-model="relationDefaults[relName as string]" type="text" readonly />
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
                        v-model="override.defaults[propName as string]"
                        type="date"
                      />
                      <input
                        v-else-if="getPropertyDef(propName as string)?.type === 'integer'"
                        v-model="override.defaults[propName as string]"
                        type="number"
                      />
                      <input v-else v-model="override.defaults[propName as string]" type="text" />
                    </template>
                    <template v-else>
                      <input v-model="override.defaults[propName as string]" type="text" />
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
                        v-model="override.relationDefaults[relName as string]"
                        type="text"
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

      <!-- Appearance / Palette -->
      <div class="settings-card">
        <h3>Appearance</h3>
        <p class="description">Customize the color palette. Empty fields use built-in defaults.</p>
        <p class="file-path">.rela/palette.yaml</p>

        <h4 class="section-subtitle">Import Palette</h4>
        <div
          class="import-section"
          :class="{ 'drop-active': dragging }"
          @dragover="handleDragOver"
          @dragleave="handleDragLeave"
          @drop="handleDrop"
        >
          <input
            ref="fileInput"
            type="file"
            accept=".gpl,.hex,.txt,.yaml,.yml"
            class="file-input-hidden"
            @change="handleFileSelected"
          />
          <textarea
            v-model="importText"
            class="import-textarea"
            placeholder="Paste hex colors, GIMP Palette (.gpl), or drag & drop a palette file"
            rows="4"
          />
          <div class="import-actions">
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="!importText.trim()"
              @click="handleImport"
            >Import</button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click="handleFilePick"
            >Browse File</button>
            <button
              v-if="importedColors.length"
              type="button"
              class="btn btn-secondary btn-sm"
              @click="clearImport"
            >Clear</button>
          </div>
          <div v-if="importedColors.length" class="swatch-section">
            <p class="swatch-hint">
              Click a role label below, then click a swatch to assign it.
            </p>
            <div class="swatch-grid">
              <button
                v-for="color in importedColors"
                :key="color"
                type="button"
                class="swatch"
                :style="{ backgroundColor: color }"
                :title="color"
                @click="assignSwatch(color)"
              />
            </div>
          </div>
        </div>

        <div class="mode-toggle">
          <button
            type="button"
            class="toggle-pill"
            :class="{ active: !editingDark }"
            @click="editingDark = false"
          >Light</button>
          <button
            type="button"
            class="toggle-pill"
            :class="{ active: editingDark }"
            @click="editingDark = true"
          >Dark</button>
        </div>

        <h4 class="section-subtitle">Theme Colors</h4>
        <div class="color-grid">
          <div
            v-for="role in paletteRoles"
            :key="role.key"
            class="color-row"
            :class="{ 'role-selected': isRoleSelected('color', role.key) }"
            @click="selectRole('color', role.key)"
          >
            <label class="color-label">
              <span class="color-name">{{ role.label }}</span>
              <span class="color-desc">{{ role.description }}</span>
            </label>
            <div class="color-input-group">
              <input
                type="color"
                :value="activeColors[role.key] || (editingDark ? autoDarkColor(role.key) : '') || '#808080'"
                class="color-picker"
                @input="setColor(role.key, ($event.target as HTMLInputElement).value)"
              />
              <input
                type="text"
                :value="activeColors[role.key] || ''"
                :placeholder="editingDark ? (autoDarkColor(role.key) || 'auto') : '#hex'"
                class="color-text"
                @input="setColor(role.key, ($event.target as HTMLInputElement).value)"
              />
              <button
                v-if="activeColors[role.key]"
                type="button"
                class="btn-icon btn-remove"
                title="Clear (use default)"
                @click="clearPaletteColor(role.key)"
              >&times;</button>
            </div>
          </div>
        </div>

        <h4 class="section-subtitle">Badge Colors</h4>
        <div class="color-grid">
          <div
            v-for="name in badgeNames"
            :key="name"
            class="color-row"
            :class="{ 'role-selected': isRoleSelected('badge', name) }"
            @click="selectRole('badge', name)"
          >
            <label class="color-label">
              <span class="color-name badge-label">{{ name }}</span>
            </label>
            <div class="color-input-group">
              <input
                type="color"
                :value="activeBadges[name] || (editingDark ? autoDarkBadge(name) : '') || '#808080'"
                class="color-picker"
                @input="setBadge(name, ($event.target as HTMLInputElement).value)"
              />
              <input
                type="text"
                :value="activeBadges[name] || ''"
                :placeholder="editingDark ? (autoDarkBadge(name) || 'auto') : '#hex'"
                class="color-text"
                @input="setBadge(name, ($event.target as HTMLInputElement).value)"
              />
              <button
                v-if="activeBadges[name]"
                type="button"
                class="btn-icon btn-remove"
                title="Clear (use default)"
                @click="clearBadgeColor(name)"
              >&times;</button>
            </div>
          </div>
        </div>

        <div class="palette-actions">
          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="savingPalette"
            @click="handleSavePalette"
          >
            {{ savingPalette ? 'Saving...' : 'Save Palette' }}
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            @click="handleResetPalette"
          >Reset</button>
        </div>
      </div>

      <!-- App Info -->
      <div class="settings-card">
        <h3>Application Info</h3>
        <div class="info-grid">
          <div class="info-row">
            <span class="info-label">App Name</span>
            <span class="info-value">{{ schemaStore.app.name }}</span>
          </div>
          <div v-if="schemaStore.app.description" class="info-row">
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
  color: var(--muted-text);
  font-size: 14px;
  margin: 0 0 4px;
}

.file-path {
  font-family: monospace;
  font-size: 12px;
  color: var(--muted-text);
  margin: 0;
}

.settings-card {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
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
  color: var(--muted-text);
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
  background: var(--hover-bg);
  border-radius: 6px;
}

.row-label {
  min-width: 120px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-color);
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
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 13px;
  background: var(--input-bg);
  color: var(--text-color);
}

.remove-btn {
  background: none;
  border: none;
  font-size: 18px;
  color: var(--muted-text);
  cursor: pointer;
  padding: 0 4px;
  line-height: 1;
}

.remove-btn:hover {
  color: var(--error-color);
}

.remove-btn.large {
  font-size: 24px;
  align-self: flex-start;
  margin-top: 20px;
}

.stale-badge {
  background: color-mix(in srgb, var(--warning-color) 15%, transparent);
  color: var(--warning-color);
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
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 13px;
  color: var(--muted-text);
  background: var(--input-bg);
}

.override-groups {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 16px;
}

.override-group {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 16px;
  background: var(--hover-bg);
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
  color: var(--muted-text);
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
  color: var(--muted-text);
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
  background: var(--hover-bg);
  border-radius: 6px;
}

.info-label {
  color: var(--muted-text);
  font-size: 14px;
}

.info-value {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-color);
}

.form-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
}

.section-subtitle {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-color);
  margin: 16px 0 8px;
}

.section-subtitle:first-of-type {
  margin-top: 0;
}

.color-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.color-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 12px;
  background: var(--hover-bg);
  border-radius: 6px;
}

.color-label {
  min-width: 140px;
  display: flex;
  flex-direction: column;
}

.color-name {
  font-size: 13px;
  font-weight: 500;
}

.color-desc {
  font-size: 11px;
  color: var(--muted-text);
}

.badge-label {
  text-transform: capitalize;
}

.color-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
}

.color-picker {
  width: 32px;
  height: 32px;
  padding: 2px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  cursor: pointer;
  background: var(--input-bg);
}

.color-text {
  width: 90px;
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 13px;
  font-family: monospace;
  background: var(--input-bg);
  color: var(--text-color);
}

.btn-icon {
  background: none;
  border: none;
  font-size: 18px;
  color: var(--muted-text);
  cursor: pointer;
  padding: 0 4px;
  line-height: 1;
}

.btn-icon:hover {
  color: var(--error-color);
}

.palette-actions {
  margin-top: 16px;
  display: flex;
  gap: 12px;
}

.import-section {
  margin-bottom: 16px;
}

.import-textarea {
  width: 100%;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 13px;
  font-family: monospace;
  background: var(--input-bg);
  color: var(--text-color);
  resize: vertical;
}

.import-actions {
  margin-top: 8px;
  display: flex;
  gap: 8px;
}

.swatch-section {
  margin-top: 12px;
}

.swatch-hint {
  font-size: 12px;
  color: var(--muted-text);
  margin: 0 0 8px;
}

.swatch-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.swatch {
  width: 28px;
  height: 28px;
  border: 2px solid var(--border-color);
  border-radius: 4px;
  cursor: pointer;
  padding: 0;
  transition: transform 0.1s;
}

.swatch:hover {
  transform: scale(1.15);
  border-color: var(--accent-color);
}

.role-selected {
  outline: 2px solid var(--accent-color);
  outline-offset: -2px;
  border-radius: 6px;
}

.file-input-hidden {
  display: none;
}

.drop-active {
  outline: 2px dashed var(--accent-color);
  outline-offset: 2px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--accent-color) 5%, transparent);
}

.mode-toggle {
  display: flex;
  gap: 0;
  margin: 16px 0 8px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  overflow: hidden;
  width: fit-content;
}

.toggle-pill {
  padding: 6px 16px;
  font-size: 13px;
  font-weight: 500;
  border: none;
  background: var(--input-bg);
  color: var(--muted-text);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.toggle-pill.active {
  background: var(--accent-color);
  color: white;
}

.toggle-pill:hover:not(.active) {
  background: var(--hover-bg);
}

/* Uses global .btn, .btn-primary, .btn-secondary, .btn-sm, .loading-state, .spinner, .error-state from App.vue */
</style>
