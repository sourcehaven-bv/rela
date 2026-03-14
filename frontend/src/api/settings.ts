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

export interface SettingsData {
  userDefaults: UserDefaults
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
