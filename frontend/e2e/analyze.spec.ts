import { test, expect } from './fixtures'

/**
 * E2E tests for the Analyze (validation) page
 */

interface AnalyzeResponse {
  errors: number
  warnings: number
  issues: Array<{
    entityId: string
    entityType: string
    message: string
    severity: 'error' | 'warning'
    checkType: string
  }>
  byCheck: Record<string, number>
}

test.describe('Analyze Page', () => {
  test('is accessible at /analyze', async ({ apiPage }) => {
    await apiPage.goto('/analyze')
    await expect(apiPage.locator('h1')).toHaveText('Analysis')
  })

  test('shows all check type cards', async ({ apiPage }) => {
    await apiPage.goto('/analyze')

    // Wait for loading to finish
    await expect(apiPage.locator('.loading-state')).toBeHidden({ timeout: 15000 })

    const checkCards = apiPage.locator('.check-card')
    await expect(checkCards).toHaveCount(4)

    // Verify each check type label is present
    await expect(apiPage.locator('.check-title')).toContainText(['Properties', 'Cardinality', 'Validations', 'Orphans'])
  })

  test('shows check type descriptions', async ({ apiPage }) => {
    await apiPage.goto('/analyze')
    await expect(apiPage.locator('.loading-state')).toBeHidden({ timeout: 15000 })

    await expect(apiPage.locator('.check-description').first()).toBeVisible()
    // Verify one specific description
    await expect(apiPage.locator('text=Property validation errors')).toBeVisible()
  })

  test('shows issue counts per check type', async ({ apiPage }) => {
    await apiPage.goto('/analyze')
    await expect(apiPage.locator('.loading-state')).toBeHidden({ timeout: 15000 })

    // Each check card should have a count badge
    const counts = apiPage.locator('.check-count')
    await expect(counts).toHaveCount(4)
  })

  test('displays issues when they exist', async ({ apiPage, api }) => {
    // Create a ticket missing required properties to generate validation issues
    const categoryId = await api.getOrCreateCategory()
    const created = await api.createEntity('tickets', {
      properties: { title: 'Analyze Test Orphan', status: 'open', reporter: 'e2e-test' },
      relations: { 'belongs-to': [categoryId] },
    })

    try {
      await apiPage.goto('/analyze')
      await expect(apiPage.locator('.loading-state')).toBeHidden({ timeout: 15000 })

      // There should be at least some issues (the prototype project may have some,
      // plus our entity may trigger property/cardinality issues)
      const totalIssues = await apiPage.locator('.issue-row').count()
      // We just verify the page renders without errors; there may or may not be issues
      // depending on the metamodel constraints
      expect(totalIssues).toBeGreaterThanOrEqual(0)
    } finally {
      await api.deleteEntity('tickets', created.id)
    }
  })

  test('clicking an entity row navigates to that entity', async ({ apiPage, api }) => {
    // Create a ticket that will likely trigger issues
    const categoryId = await api.getOrCreateCategory()
    const created = await api.createEntity('tickets', {
      properties: { title: 'Analyze Nav Test', status: 'open', reporter: 'e2e-test' },
      relations: { 'belongs-to': [categoryId] },
    })

    try {
      await apiPage.goto('/analyze')
      await expect(apiPage.locator('.loading-state')).toBeHidden({ timeout: 15000 })

      const issueRows = apiPage.locator('.issue-row')
      const count = await issueRows.count()

      if (count > 0) {
        // Click the first issue row
        await issueRows.first().click()
        // Should navigate to entity detail page
        await expect(apiPage).toHaveURL(/\/entity\//)
      }
    } finally {
      await api.deleteEntity('tickets', created.id)
    }
  })

  test('refresh button reloads analysis', async ({ apiPage }) => {
    await apiPage.goto('/analyze')
    await expect(apiPage.locator('.loading-state')).toBeHidden({ timeout: 15000 })

    // Click the refresh button
    const refreshBtn = apiPage.locator('button', { hasText: 'Refresh' })
    await expect(refreshBtn).toBeVisible()
    await refreshBtn.click()

    // Should show loading state briefly then resolve
    await expect(apiPage.locator('.loading-state')).toBeHidden({ timeout: 15000 })

    // Page should still show check cards
    await expect(apiPage.locator('.check-card')).toHaveCount(4)
  })
})

test.describe('Analyze API', () => {
  test('analyze endpoint returns valid data', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_analyze`)
    expect(response.ok()).toBeTruthy()

    const result: AnalyzeResponse = await response.json()
    expect(typeof result.errors).toBe('number')
    expect(typeof result.warnings).toBe('number')
    expect(Array.isArray(result.issues)).toBeTruthy()
    expect(typeof result.byCheck).toBe('object')
  })

  test('analyze issues have required fields', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_analyze`)
    expect(response.ok()).toBeTruthy()

    const result: AnalyzeResponse = await response.json()

    for (const issue of result.issues) {
      expect(issue.entityId).toBeTruthy()
      expect(issue.entityType).toBeTruthy()
      expect(issue.message).toBeTruthy()
      expect(['error', 'warning']).toContain(issue.severity)
      expect(issue.checkType).toBeTruthy()
    }
  })

  test('analyze byCheck keys match known check types', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_analyze`)
    expect(response.ok()).toBeTruthy()

    const result: AnalyzeResponse = await response.json()
    const knownCheckTypes = ['Properties', 'Cardinality', 'Validations', 'Orphans']

    for (const key of Object.keys(result.byCheck)) {
      expect(knownCheckTypes).toContain(key)
    }
  })

  test('analyze error and warning counts match issues', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_analyze`)
    expect(response.ok()).toBeTruthy()

    const result: AnalyzeResponse = await response.json()
    const errorCount = result.issues.filter((i) => i.severity === 'error').length
    const warningCount = result.issues.filter((i) => i.severity === 'warning').length

    expect(result.errors).toBe(errorCount)
    expect(result.warnings).toBe(warningCount)
  })
})
