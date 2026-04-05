import { test, expect } from './fixtures'

/**
 * E2E tests for Settings page
 */

interface SettingsResponse {
  userDefaults: {
    defaults: Record<string, string>
    relationDefaults: Record<string, string>
    overrides: Array<{
      types: string[]
      defaults: Record<string, string>
      relationDefaults: Record<string, string>
    }>
  }
  allProperties: Array<{ name: string; type: string; values: string[] }>
  allRelations: Array<{ name: string; label: string; targetType: string }>
  entityTypes: string[]
}

test.describe('Settings Page', () => {
  test('settings page is accessible at /settings', async ({ apiPage }) => {
    await apiPage.goto('/settings')
    await apiPage.waitForSelector('.settings-view')
  })

  test('settings page shows property defaults section', async ({ apiPage }) => {
    await apiPage.goto('/settings')
    await apiPage.waitForSelector('.settings-view')

    // Wait for loading to finish
    await apiPage.waitForSelector('.settings-form')

    // Should show the Property Defaults card
    const propertyDefaultsCard = apiPage.locator('.settings-card').filter({ hasText: 'Property Defaults' })
    await expect(propertyDefaultsCard).toBeVisible()

    // Should show the description
    await expect(propertyDefaultsCard.locator('.description')).toContainText('Default values applied when creating any entity type')
  })

  test('settings page shows all sections', async ({ apiPage }) => {
    await apiPage.goto('/settings')
    await apiPage.waitForSelector('.settings-form')

    // Check all four settings cards are present
    const cards = apiPage.locator('.settings-card')
    await expect(cards).toHaveCount(4)

    // Verify card headings
    await expect(cards.nth(0).locator('h3')).toHaveText('Property Defaults')
    await expect(cards.nth(1).locator('h3')).toHaveText('Relation Defaults')
    await expect(cards.nth(2).locator('h3')).toHaveText('Overrides')
    await expect(cards.nth(3).locator('h3')).toHaveText('Application Info')
  })

  test('can add a property default and save', async ({ apiPage }) => {
    await apiPage.goto('/settings')
    await apiPage.waitForSelector('.settings-form')

    // Find the Property Defaults card
    const propertyCard = apiPage.locator('.settings-card').filter({ hasText: 'Property Defaults' })

    // Use the "Add property default..." dropdown to add a property
    const addSelect = propertyCard.locator('.add-row select')
    await expect(addSelect).toBeVisible()

    // Get the first available option (skip the placeholder)
    const options = addSelect.locator('option')
    const optionCount = await options.count()
    expect(optionCount).toBeGreaterThan(1) // at least placeholder + one option

    // Select the first real option
    const firstOptionValue = await options.nth(1).getAttribute('value')
    expect(firstOptionValue).toBeTruthy()
    await addSelect.selectOption(firstOptionValue!)

    // Should now have a settings row with that property
    const settingsRow = propertyCard.locator('.settings-row').filter({ hasText: firstOptionValue! })
    await expect(settingsRow).toBeVisible()

    // Click Save
    const saveBtn = apiPage.locator('.form-actions button[type="submit"]')
    await saveBtn.click()

    // Reload to verify persistence
    await apiPage.goto('/settings')
    await apiPage.waitForSelector('.settings-form')

    const propertyCardAfter = apiPage.locator('.settings-card').filter({ hasText: 'Property Defaults' })
    const savedRow = propertyCardAfter.locator('.settings-row').filter({ hasText: firstOptionValue! })
    await expect(savedRow).toBeVisible()
  })

  test('can remove a property default and save', async ({ apiPage, request, backend }) => {
    // First, add a property default via API
    const settingsResp = await request.get(`${backend.baseUrl}/api/v1/_settings`)
    const settings: SettingsResponse = await settingsResp.json()

    // Pick the first available property
    const propName = settings.allProperties[0]?.name
    expect(propName).toBeTruthy()

    // Save a default for it
    await request.put(`${backend.baseUrl}/api/v1/_settings`, {
      data: {
        defaults: { [propName]: 'test-value' },
        relationDefaults: {},
        overrides: [],
      },
    })

    // Navigate to settings
    await apiPage.goto('/settings')
    await apiPage.waitForSelector('.settings-form')

    // The property row should exist
    const propertyCard = apiPage.locator('.settings-card').filter({ hasText: 'Property Defaults' })
    const row = propertyCard.locator('.settings-row').filter({ hasText: propName })
    await expect(row).toBeVisible()

    // Click the remove button
    await row.locator('.remove-btn').click()

    // Row should be gone
    await expect(row).not.toBeVisible()

    // Save
    const saveBtn = apiPage.locator('.form-actions button[type="submit"]')
    await saveBtn.click()

    // Reload to verify it was removed
    await apiPage.goto('/settings')
    await apiPage.waitForSelector('.settings-form')

    const propertyCardAfter = apiPage.locator('.settings-card').filter({ hasText: 'Property Defaults' })
    const removedRow = propertyCardAfter.locator('.settings-row').filter({ hasText: propName })
    await expect(removedRow).not.toBeVisible()
  })

  test('shows application info', async ({ apiPage }) => {
    await apiPage.goto('/settings')
    await apiPage.waitForSelector('.settings-form')

    const infoCard = apiPage.locator('.settings-card').filter({ hasText: 'Application Info' })
    await expect(infoCard).toBeVisible()

    // Should show app metadata
    await expect(infoCard.locator('.info-row').filter({ hasText: 'App Name' })).toBeVisible()
    await expect(infoCard.locator('.info-row').filter({ hasText: 'Entity Types' })).toBeVisible()
  })
})

test.describe('Settings API', () => {
  test('GET /api/v1/_settings returns valid data', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_settings`)
    expect(response.ok()).toBeTruthy()

    const data: SettingsResponse = await response.json()

    // Should have the expected structure
    expect(data.userDefaults).toBeDefined()
    expect(data.allProperties).toBeDefined()
    expect(Array.isArray(data.allProperties)).toBeTruthy()
    expect(data.allRelations).toBeDefined()
    expect(Array.isArray(data.allRelations)).toBeTruthy()
    expect(data.entityTypes).toBeDefined()
    expect(Array.isArray(data.entityTypes)).toBeTruthy()

    // Should have some properties and entity types from the prototype project
    expect(data.allProperties.length).toBeGreaterThan(0)
    expect(data.entityTypes.length).toBeGreaterThan(0)
  })

  test('PUT /api/v1/_settings saves and persists defaults', async ({ request, backend }) => {
    const defaults = {
      defaults: { status: 'open' },
      relationDefaults: {},
      overrides: [],
    }

    // Save
    const putResponse = await request.put(`${backend.baseUrl}/api/v1/_settings`, { data: defaults })
    expect(putResponse.ok()).toBeTruthy()

    // Read back
    const getResponse = await request.get(`${backend.baseUrl}/api/v1/_settings`)
    expect(getResponse.ok()).toBeTruthy()

    const data: SettingsResponse = await getResponse.json()
    expect(data.userDefaults.defaults.status).toBe('open')
  })

  test('PUT /api/v1/_settings handles overrides', async ({ request, backend }) => {
    const defaults = {
      defaults: {},
      relationDefaults: {},
      overrides: [
        {
          types: ['ticket'],
          defaults: { priority: 'high' },
          relationDefaults: {},
        },
      ],
    }

    const putResponse = await request.put(`${backend.baseUrl}/api/v1/_settings`, { data: defaults })
    expect(putResponse.ok()).toBeTruthy()

    const getResponse = await request.get(`${backend.baseUrl}/api/v1/_settings`)
    const data: SettingsResponse = await getResponse.json()
    expect(data.userDefaults.overrides).toHaveLength(1)
    expect(data.userDefaults.overrides[0].types).toContain('ticket')
    expect(data.userDefaults.overrides[0].defaults.priority).toBe('high')
  })
})
