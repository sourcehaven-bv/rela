<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import { getSettings, saveSettings, savePalette } from '@/api'
import type {
  SettingsData,
  SettingsPropertyDef,
  SettingsRelationDef,
  UserDefaults,
  DefaultOverride,
} from '@/api/settings'
import TagSelect from '@/components/ui/TagSelect.vue'
import {
  parsePalette,
  parseRelaPalette,
  assignPalette,
  deriveTheme,
  generateDark,
  generateDarkBadges,
  normalizeHex,
} from '@/utils/palette'
import { buildPalettePayload, loadPaletteState } from './SettingsView.palette'

// Per-role color text inputs accept hex with optional `#` and 3 or 6
// digits. Used for whitespace-trim + normalize on paste.
const HEX_INPUT_RE = /^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

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

// Per-column file pickers. Each Import button in the column header
// triggers its own picker so the imported palette only populates
// that column.
const lightFileInput = ref<HTMLInputElement | null>(null)
const darkFileInput = ref<HTMLInputElement | null>(null)

// Dark mode state. paletteMode controls the overall layout:
//   'regular'    — single column, dark mode is disabled (saves dark: false)
//   'light-dark' — two columns side-by-side for the 8 theme roles
//                  AND for the 7 badges. Empty dark slots inherit
//                  from the corresponding light slot on the backend.
const paletteDarkColors = ref<Record<string, string>>({})
const paletteDarkBadges = ref<Record<string, string>>({})
const paletteMode = ref<'regular' | 'light-dark'>('regular')
const showDeriveConfirm = ref(false)

const MAX_FILE_SIZE = 102400 // 100KB

/** Trim and normalize a per-role color input. Returns the value to
 *  store: the normalized #rrggbb form if it's a valid hex, otherwise
 *  the trimmed raw (so partial input while typing isn't lost). */
function normalizeColorInput(value: string): string {
  const trimmed = value.trim()
  if (HEX_INPUT_RE.test(trimmed)) return normalizeHex(trimmed)
  return trimmed
}

function setLightColor(key: string, value: string) {
  paletteColors.value[key] = normalizeColorInput(value)
}

function setLightBadge(key: string, value: string) {
  paletteBadges.value[key] = normalizeColorInput(value)
}

function setDarkColor(key: string, value: string) {
  paletteDarkColors.value[key] = normalizeColorInput(value)
}

function setDarkBadge(key: string, value: string) {
  paletteDarkBadges.value[key] = normalizeColorInput(value)
}

function clearDarkBadge(key: string) {
  delete paletteDarkBadges.value[key]
}

/** True if any dark color or badge slot has a non-empty value. */
function hasAnyDarkValues(): boolean {
  for (const v of Object.values(paletteDarkColors.value)) if (v) return true
  for (const v of Object.values(paletteDarkBadges.value)) if (v) return true
  return false
}

/** True if the light palette has at least one valid hex value that
 *  Derive can actually use as input. Used to gate the Derive button
 *  so a click on an empty light palette doesn't silently wipe the
 *  dark column with empty strings. */
const canDeriveDark = computed<boolean>(() => {
  for (const v of Object.values(paletteColors.value)) {
    if (v && /^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(v.trim())) return true
  }
  return false
})

/** Compute and apply derived dark values from the current light
 *  palette. Overwrites all 8 base color slots AND all dark badge
 *  slots, so a single click produces a complete dark palette ready
 *  for further hand-tweaking. */
function applyDeriveDark() {
  paletteDarkColors.value = generateDark(paletteColors.value)
  paletteDarkBadges.value = generateDarkBadges(paletteBadges.value)
  showDeriveConfirm.value = false
}

/** User clicked "Derive Dark from Light". If any dark values are
 *  already set, ask before overwriting; otherwise apply immediately. */
function handleDeriveDark() {
  if (!canDeriveDark.value) {
    uiStore.warning('Set at least one Light color before deriving Dark.')
    return
  }
  if (hasAnyDarkValues()) {
    showDeriveConfirm.value = true
    return
  }
  applyDeriveDark()
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

    // Load palette. Pass `schemaStore.darkDisabled` so that a user
    // with no `dark` field in their palette overlay still sees
    // Light+Dark mode if the project ships a dark theme — otherwise
    // a naive Save would shadow the project's dark with `dark: false`.
    const state = loadPaletteState(
      data.userPalette,
      paletteRoles.map((r) => r.key),
      schemaStore.darkDisabled,
    )
    paletteMode.value = state.mode
    paletteColors.value = state.light
    paletteBadges.value = state.badges
    paletteDarkColors.value = state.dark
    paletteDarkBadges.value = state.darkBadges
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
    const payload = buildPalettePayload({
      mode: paletteMode.value,
      light: paletteColors.value,
      badges: paletteBadges.value,
      dark: paletteDarkColors.value,
      darkBadges: paletteDarkBadges.value,
    })
    await savePalette(payload)
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

/** Build the resolved CSS variable maps for the live preview swatch
 *  component. Returns one map per theme; the swatch component scopes
 *  these to its own subtree via inline styles, so the rest of the
 *  application is never affected.
 *
 *  In Regular mode, both light and dark render identically (the
 *  user has explicitly opted out of having a separate dark theme).
 *  In Light+Dark mode, empty dark slots inherit from light so the
 *  preview matches what the backend will produce after save.
 */
const previewVars = computed<{ light: Record<string, string>; dark: Record<string, string> }>(() => {
  const light = deriveTheme(paletteColors.value, paletteBadges.value)
  if (paletteMode.value === 'regular') {
    return { light, dark: light }
  }
  const mergedColors = { ...paletteColors.value, ...stripEmpty(paletteDarkColors.value) }
  const mergedBadges = { ...paletteBadges.value, ...stripEmpty(paletteDarkBadges.value) }
  const dark = deriveTheme(mergedColors, mergedBadges)
  return { light, dark }
})

function stripEmpty(o: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(o)) if (v) out[k] = v
  return out
}

/** Build an inline `style` attribute string from a CSS-var map. The
 *  swatch components apply this to their own root element so the
 *  preview is scoped to that subtree only — no global side effects. */
function previewStyleAttr(vars: Record<string, string>): string {
  return Object.entries(vars)
    .map(([k, v]) => `${k}: ${v}`)
    .join('; ')
}

type ImportColumn = 'light' | 'dark'

/** File input change handler shared by the Light and Dark Import
 *  buttons. The `column` argument determines whether the imported
 *  palette populates Light or Dark slots. YAML files are parsed
 *  structurally; .gpl/.hex/.txt files are parsed as a flat color
 *  list and auto-assigned by `assignPalette`. */
function handleColumnFile(event: Event, column: ImportColumn) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = '' // reset so the same file can be re-imported
  if (!file) return
  if (file.size > MAX_FILE_SIZE) {
    uiStore.warning('File too large (max 100KB)')
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    const text = reader.result as string
    importIntoColumn(text, file.name, column)
  }
  reader.readAsText(file)
}

function importIntoColumn(text: string, fileName: string, column: ImportColumn) {
  const isYaml = fileName.endsWith('.yaml') || fileName.endsWith('.yml')
  if (isYaml) {
    const result = parseRelaPalette(text)
    const colorTarget = column === 'light' ? paletteColors : paletteDarkColors
    const badgeTarget = column === 'light' ? paletteBadges : paletteDarkBadges
    let count = 0
    for (const [key, val] of Object.entries(result.colors)) {
      colorTarget.value[key] = val
      count++
    }
    for (const [key, val] of Object.entries(result.badges)) {
      badgeTarget.value[key] = val
      count++
    }
    if (column === 'dark') {
      // Importing into the Dark column always implies Light+Dark mode.
      paletteMode.value = 'light-dark'
    }
    if (count === 0) {
      uiStore.warning(`No valid colors found in ${fileName}.`)
      return
    }
    uiStore.success(`Imported ${count} colors from ${fileName} into ${column}`)
    return
  }

  // Non-YAML: flat hex list / GIMP palette → assignPalette
  // distributes colors to roles by HSL heuristics. When importing
  // into the Dark column we flip the lightness rules so the darkest
  // color becomes the surface (background) and the lightest becomes
  // text — otherwise the surface gets a light color which is wrong
  // for a dark theme.
  const colors = parsePalette(text)
  if (colors.length === 0) {
    uiStore.warning(`No valid colors found in ${fileName}.`)
    return
  }
  const assignment = assignPalette(colors, { darkTheme: column === 'dark' })
  const colorTarget = column === 'light' ? paletteColors : paletteDarkColors
  const badgeTarget = column === 'light' ? paletteBadges : paletteDarkBadges
  for (const [key, val] of Object.entries(assignment.colors)) {
    colorTarget.value[key] = val
  }
  for (const [key, val] of Object.entries(assignment.badges)) {
    badgeTarget.value[key] = val
  }
  if (column === 'dark') paletteMode.value = 'light-dark'
  uiStore.success(`Imported ${colors.length} colors from ${fileName} into ${column}`)
}

function clearLightColor(key: string) {
  delete paletteColors.value[key]
}

function clearLightBadge(key: string) {
  delete paletteBadges.value[key]
}

function clearDarkColor(key: string) {
  delete paletteDarkColors.value[key]
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

        <!-- Hidden file inputs for the per-column Import buttons -->
        <input
          ref="lightFileInput"
          type="file"
          accept=".gpl,.hex,.txt,.yaml,.yml"
          class="file-input-hidden"
          @change="(e) => handleColumnFile(e, 'light')"
        />
        <input
          ref="darkFileInput"
          type="file"
          accept=".gpl,.hex,.txt,.yaml,.yml"
          class="file-input-hidden"
          @change="(e) => handleColumnFile(e, 'dark')"
        />

        <h4 class="section-subtitle">Mode</h4>
        <div class="mode-toggle">
          <button
            type="button"
            class="toggle-pill"
            :class="{ active: paletteMode === 'regular' }"
            @click="paletteMode = 'regular'"
          >Regular</button>
          <button
            type="button"
            class="toggle-pill"
            :class="{ active: paletteMode === 'light-dark' }"
            @click="paletteMode = 'light-dark'"
          >Light + Dark</button>
        </div>
        <p class="mode-hint">
          <template v-if="paletteMode === 'regular'">
            Single palette. Dark mode is disabled for this project.
          </template>
          <template v-else>
            Edit Light and Dark themes side by side. Empty Dark slots
            inherit from Light. Click <strong>Derive Dark from Light</strong>
            to auto-fill the Dark column from your Light values.
          </template>
        </p>

        <!-- Live preview swatch — scoped to its own DOM subtree via
             inline CSS variables, so the rest of the app keeps using
             the saved palette while the user edits. Renders both
             Light and Dark themes side-by-side in Light+Dark mode so
             the user can compare without toggling. -->
        <h4 class="section-subtitle">Preview</h4>
        <div class="palette-preview" :class="{ 'palette-preview--split': paletteMode === 'light-dark' }">
          <div class="palette-preview-pane" :style="previewStyleAttr(previewVars.light)">
            <span class="palette-preview-label">Light</span>
            <div class="palette-preview-frame">
              <div class="palette-preview-sidebar">
                <div class="palette-preview-sidebar-item">Nav</div>
              </div>
              <div class="palette-preview-body">
                <div class="palette-preview-card">
                  <div class="palette-preview-text">Sample text</div>
                  <div class="palette-preview-muted">Muted text</div>
                  <div class="palette-preview-buttons">
                    <button type="button" class="palette-preview-btn palette-preview-btn-accent">Action</button>
                    <span class="palette-preview-badge palette-preview-badge-success">ok</span>
                    <span class="palette-preview-badge palette-preview-badge-error">err</span>
                    <span class="palette-preview-badge palette-preview-badge-warning">warn</span>
                    <span class="palette-preview-badge palette-preview-badge-info">info</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div
            v-if="paletteMode === 'light-dark'"
            class="palette-preview-pane"
            :style="previewStyleAttr(previewVars.dark)"
          >
            <span class="palette-preview-label">Dark</span>
            <div class="palette-preview-frame">
              <div class="palette-preview-sidebar">
                <div class="palette-preview-sidebar-item">Nav</div>
              </div>
              <div class="palette-preview-body">
                <div class="palette-preview-card">
                  <div class="palette-preview-text">Sample text</div>
                  <div class="palette-preview-muted">Muted text</div>
                  <div class="palette-preview-buttons">
                    <button type="button" class="palette-preview-btn palette-preview-btn-accent">Action</button>
                    <span class="palette-preview-badge palette-preview-badge-success">ok</span>
                    <span class="palette-preview-badge palette-preview-badge-error">err</span>
                    <span class="palette-preview-badge palette-preview-badge-warning">warn</span>
                    <span class="palette-preview-badge palette-preview-badge-info">info</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <h4 class="section-subtitle">Theme Colors</h4>

        <!-- Column header band (Light+Dark mode) -->
        <div v-if="paletteMode === 'light-dark'" class="palette-grid palette-grid--split palette-header">
          <span /> <!-- label column spacer -->
          <div class="palette-column-header">
            <span>Light</span>
            <button
              type="button"
              class="btn btn-secondary btn-xs"
              title="Import a palette file into the Light column"
              @click="lightFileInput?.click()"
            >Import</button>
          </div>
          <div class="palette-column-header">
            <span>Dark</span>
            <div class="palette-column-actions">
              <button
                type="button"
                class="btn btn-secondary btn-xs"
                :disabled="!canDeriveDark"
                :title="canDeriveDark ? 'Auto-fill the Dark column from the Light palette' : 'Set at least one Light color first'"
                @click="handleDeriveDark"
              >Derive from Light</button>
              <button
                type="button"
                class="btn btn-secondary btn-xs"
                title="Import a palette file into the Dark column"
                @click="darkFileInput?.click()"
              >Import</button>
            </div>
          </div>
        </div>

        <!-- Single-column header band (Regular mode) -->
        <div v-else class="palette-header palette-header--single">
          <button
            type="button"
            class="btn btn-secondary btn-xs"
            title="Import a palette file"
            @click="lightFileInput?.click()"
          >Import</button>
        </div>

        <!-- Inline overwrite confirm appears right below the column
             header so it's always visible after the user clicks
             Derive (RR-finding #5: confirm was rendered far below the
             color grid and felt like the button did nothing). -->
        <div v-if="showDeriveConfirm" class="derive-confirm">
          <p>Overwrite all dark colors with values derived from the
            current light palette?</p>
          <div class="derive-confirm-actions">
            <button
              type="button"
              class="btn btn-primary btn-sm"
              @click="applyDeriveDark"
            >Overwrite</button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click="showDeriveConfirm = false"
            >Cancel</button>
          </div>
        </div>

        <!-- The 8 role rows. CSS grid keeps the label / light input /
             dark input columns aligned across all rows. -->
        <div class="palette-grid" :class="{ 'palette-grid--split': paletteMode === 'light-dark' }">
          <template v-for="role in paletteRoles" :key="role.key">
            <label class="palette-cell palette-label">
              <span class="palette-name">{{ role.label }}</span>
              <span class="palette-desc">{{ role.description }}</span>
            </label>
            <div class="palette-cell palette-input-cell">
              <input
                type="color"
                :value="paletteColors[role.key] || '#808080'"
                class="color-picker"
                @input="setLightColor(role.key, ($event.target as HTMLInputElement).value)"
              />
              <input
                type="text"
                :value="paletteColors[role.key] || ''"
                placeholder="#hex"
                class="color-text"
                @input="setLightColor(role.key, ($event.target as HTMLInputElement).value)"
              />
              <button
                v-if="paletteColors[role.key]"
                type="button"
                class="btn-icon btn-remove"
                title="Clear (use default)"
                @click="clearLightColor(role.key)"
              >&times;</button>
            </div>
            <div v-if="paletteMode === 'light-dark'" class="palette-cell palette-input-cell">
              <input
                type="color"
                :value="paletteDarkColors[role.key] || '#808080'"
                class="color-picker"
                @input="setDarkColor(role.key, ($event.target as HTMLInputElement).value)"
              />
              <input
                type="text"
                :value="paletteDarkColors[role.key] || ''"
                placeholder="(inherits)"
                class="color-text"
                @input="setDarkColor(role.key, ($event.target as HTMLInputElement).value)"
              />
              <button
                v-if="paletteDarkColors[role.key]"
                type="button"
                class="btn-icon btn-remove"
                title="Clear (inherit from light)"
                @click="clearDarkColor(role.key)"
              >&times;</button>
            </div>
          </template>
        </div>

        <h4 class="section-subtitle">Badge Colors</h4>
        <div class="palette-grid" :class="{ 'palette-grid--split': paletteMode === 'light-dark' }">
          <template v-for="name in badgeNames" :key="name">
            <label class="palette-cell palette-label">
              <span class="palette-name badge-label">{{ name }}</span>
            </label>
            <div class="palette-cell palette-input-cell">
              <input
                type="color"
                :value="paletteBadges[name] || '#808080'"
                class="color-picker"
                @input="setLightBadge(name, ($event.target as HTMLInputElement).value)"
              />
              <input
                type="text"
                :value="paletteBadges[name] || ''"
                placeholder="#hex"
                class="color-text"
                @input="setLightBadge(name, ($event.target as HTMLInputElement).value)"
              />
              <button
                v-if="paletteBadges[name]"
                type="button"
                class="btn-icon btn-remove"
                title="Clear (use default)"
                @click="clearLightBadge(name)"
              >&times;</button>
            </div>
            <div v-if="paletteMode === 'light-dark'" class="palette-cell palette-input-cell">
              <input
                type="color"
                :value="paletteDarkBadges[name] || '#808080'"
                class="color-picker"
                @input="setDarkBadge(name, ($event.target as HTMLInputElement).value)"
              />
              <input
                type="text"
                :value="paletteDarkBadges[name] || ''"
                placeholder="(inherits)"
                class="color-text"
                @input="setDarkBadge(name, ($event.target as HTMLInputElement).value)"
              />
              <button
                v-if="paletteDarkBadges[name]"
                type="button"
                class="btn-icon btn-remove"
                title="Clear (inherit from light)"
                @click="clearDarkBadge(name)"
              >&times;</button>
            </div>
          </template>
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

.badge-label {
  text-transform: capitalize;
}

.color-picker {
  width: 32px;
  height: 32px;
  padding: 2px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  cursor: pointer;
  background: var(--input-bg);
  flex-shrink: 0;
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

.btn-xs {
  font-size: 11px;
  padding: 4px 10px;
}

.derive-confirm {
  margin-top: 12px;
  padding: 12px 14px;
  background: var(--card-bg);
  border: 1px solid var(--warning-color);
  border-radius: 6px;
  font-size: 13px;
}

.derive-confirm p {
  margin: 0 0 10px;
}

.derive-confirm-actions {
  display: flex;
  gap: 8px;
}

.mode-hint {
  margin: 4px 0 12px;
  font-size: 12px;
  color: var(--muted-text);
}

/* --- Live preview swatch --- */
/*
  Each .palette-preview-pane has inline `style` injected with the
  full CSS-variable map for either Light or Dark. Descendants read
  via `var(--xxx)` so the preview is fully scoped to the pane and
  cannot affect anything outside.
*/

.palette-preview {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 16px;
}

.palette-preview--split {
  flex-direction: row;
}

.palette-preview-pane {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.palette-preview-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted-text);
}

.palette-preview-frame {
  display: flex;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  overflow: hidden;
  min-height: 110px;
  background: var(--bg-color);
  color: var(--text-color);
}

.palette-preview-sidebar {
  width: 60px;
  background: var(--sidebar-bg);
  color: var(--sidebar-text);
  padding: 10px 8px;
  font-size: 11px;
}

.palette-preview-sidebar-item {
  padding: 4px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.08);
}

.palette-preview-body {
  flex: 1;
  padding: 10px;
  background: var(--bg-color);
}

.palette-preview-card {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.palette-preview-text {
  color: var(--text-color);
  font-size: 12px;
  font-weight: 500;
}

.palette-preview-muted {
  color: var(--muted-text);
  font-size: 11px;
}

.palette-preview-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.palette-preview-btn {
  font-size: 11px;
  padding: 4px 10px;
  border: none;
  border-radius: 4px;
  cursor: default;
  font-weight: 500;
}

.palette-preview-btn-accent {
  background: var(--accent-color);
  color: white;
}

.palette-preview-badge {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 10px;
  color: white;
  font-weight: 500;
}

.palette-preview-badge-success {
  background: var(--success-color);
}

.palette-preview-badge-error {
  background: var(--error-color);
}

.palette-preview-badge-warning {
  background: var(--warning-color);
}

.palette-preview-badge-info {
  background: var(--info-color);
}

.palette-actions {
  margin-top: 16px;
  display: flex;
  gap: 12px;
}

.file-input-hidden {
  display: none;
}

/* --- Palette grid (CSS grid; replaces the older flex .color-row) --- */

.palette-grid {
  display: grid;
  grid-template-columns: minmax(160px, max-content) 1fr;
  gap: 6px 12px;
  align-items: center;
  margin-bottom: 16px;
}

.palette-grid--split {
  grid-template-columns: minmax(160px, max-content) 1fr 1fr;
}

.palette-cell {
  padding: 6px 12px;
  background: var(--hover-bg);
  border-radius: 6px;
  display: flex;
  align-items: center;
}

.palette-label {
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  background: transparent;
  padding-left: 0;
}

.palette-name {
  font-size: 13px;
  font-weight: 500;
  text-transform: capitalize;
}

.palette-desc {
  font-size: 11px;
  color: var(--muted-text);
}

.palette-input-cell {
  gap: 8px;
  min-width: 0; /* allow shrinking inside the grid */
}

.palette-input-cell .color-text {
  flex: 1;
  min-width: 0;
}

.palette-header {
  display: grid;
  grid-template-columns: minmax(160px, max-content) 1fr;
  gap: 6px 12px;
  align-items: center;
  margin: 16px 0 8px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted-text);
}

.palette-header.palette-grid--split {
  grid-template-columns: minmax(160px, max-content) 1fr 1fr;
}

.palette-header--single {
  display: flex;
  justify-content: flex-end;
}

.palette-column-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 12px;
}

.palette-column-actions {
  display: flex;
  gap: 6px;
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

@media (max-width: 768px) {
  .settings-row {
    flex-wrap: wrap;
  }

  .row-label {
    min-width: 0;
    flex-basis: 100%;
  }

  .override-header {
    flex-wrap: wrap;
  }

  .palette-grid {
    grid-template-columns: 1fr;
  }

  .palette-grid--split {
    grid-template-columns: 1fr;
  }

  .palette-preview--split {
    flex-direction: column;
  }

  .color-text {
    width: 70px;
  }
}
</style>
