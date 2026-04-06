export interface SettingsPropertyDef {
  name: string
  type: string
  values: string[]
}

export interface SettingsRelationTarget {
  id: string
  title: string
}

export interface SettingsRelationDef {
  name: string
  label: string
  targetType: string
  targets: SettingsRelationTarget[]
}

export interface DefaultOverride {
  types: string[]
  defaults: Record<string, string>
  relationDefaults: Record<string, string>
}

export interface UserDefaults {
  defaults: Record<string, string>
  relationDefaults: Record<string, string>
  overrides: DefaultOverride[]
}

export interface PaletteColors {
  base?: string
  surface?: string
  accent?: string
  text?: string
  success?: string
  error?: string
  warning?: string
  info?: string
}

export interface PaletteConfig {
  base?: string
  surface?: string
  accent?: string
  text?: string
  success?: string
  error?: string
  warning?: string
  info?: string
  badges?: Record<string, string>
}

export interface SettingsData {
  userDefaults: UserDefaults
  userPalette?: PaletteConfig
  allProperties: SettingsPropertyDef[]
  allRelations: SettingsRelationDef[]
  entityTypes: string[]
}

export async function getSettings(): Promise<SettingsData> {
  const response = await fetch('/api/v1/_settings')
  if (!response.ok) {
    throw new Error('Failed to load settings')
  }
  return response.json()
}

export async function saveSettings(userDefaults: UserDefaults): Promise<void> {
  const response = await fetch('/api/v1/_settings', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(userDefaults),
  })
  if (!response.ok) {
    throw new Error('Failed to save settings')
  }
}

export async function getPalette(): Promise<PaletteConfig> {
  const response = await fetch('/api/v1/_palette')
  if (!response.ok) {
    throw new Error('Failed to load palette')
  }
  return response.json()
}

export async function savePalette(palette: PaletteConfig): Promise<void> {
  const response = await fetch('/api/v1/_palette', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(palette),
  })
  if (!response.ok) {
    const data = await response.json().catch(() => ({ error: 'Unknown error' }))
    throw new Error(data.error || 'Failed to save palette')
  }
}
