import { test, expect } from './fixtures'

/**
 * E2E tests for Conflicts page
 *
 * Note: Conflicts require actual git merge conflicts in the project directory,
 * which is impractical to set up in e2e tests. These tests verify the page
 * loads correctly and handles the empty state.
 */

interface ConflictsResponse {
  conflicts: Array<{
    path: string
    entity_type?: string
    entity_id?: string
    marker_count: number
  }>
  count: number
}

test.describe('Conflicts Page', () => {
  test('conflicts page is accessible at /conflicts', async ({ apiPage }) => {
    await apiPage.goto('/conflicts')
    await expect(apiPage.locator('.conflicts-view')).toBeVisible()
  })

  test('conflicts page shows empty state when no conflicts exist', async ({ apiPage }) => {
    await apiPage.goto('/conflicts')

    // Should show the empty state with "No conflicts detected"
    const emptyState = apiPage.locator('.conflict-empty')
    await expect(emptyState).toBeVisible()
    await expect(emptyState.locator('h3')).toHaveText('No conflicts detected')
    await expect(emptyState.locator('p')).toHaveText('All entity and relation files are clean.')
  })

  test('conflicts page shows page header', async ({ apiPage }) => {
    await apiPage.goto('/conflicts')

    const header = apiPage.locator('.page-header')
    await expect(header.locator('h2')).toHaveText('Merge Conflicts')
    await expect(header.locator('p')).toHaveText('Files with unresolved git conflicts')
  })

  test('back to dashboard button is visible', async ({ apiPage }) => {
    await apiPage.goto('/conflicts')

    const backButton = apiPage.locator('.page-header .btn')
    await expect(backButton).toBeVisible()
    await expect(backButton).toHaveText('Back to Dashboard')
  })
})

test.describe('Conflicts API', () => {
  test('GET /api/v1/_conflicts returns valid response shape', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_conflicts`)
    expect(response.ok()).toBeTruthy()

    const result: ConflictsResponse = await response.json()
    expect(result).toHaveProperty('conflicts')
    expect(result).toHaveProperty('count')
    expect(Array.isArray(result.conflicts)).toBeTruthy()
    expect(result.count).toBe(result.conflicts.length)
  })

  test('GET /api/v1/_conflicts returns empty list for clean project', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_conflicts`)
    expect(response.ok()).toBeTruthy()

    const result: ConflictsResponse = await response.json()
    expect(result.conflicts).toHaveLength(0)
    expect(result.count).toBe(0)
  })
})
