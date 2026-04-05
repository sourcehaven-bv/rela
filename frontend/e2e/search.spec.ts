import { test, expect, type EntityResponse } from './fixtures'

/**
 * E2E tests for Search functionality
 */

interface SearchResponse {
  data: EntityResponse[]
  meta?: {
    total: number
    page: number
    per_page: number
    has_more: boolean
  }
}

test.describe('Search', () => {
  const createdTickets: string[] = []

  test.beforeEach(async ({ api }) => {
    const categoryId = await api.getOrCreateCategory()

    // Create test tickets with unique titles for search testing
    const tickets = [
      { title: 'SearchTest Alpha Unique', status: 'open', priority: 'high', description: 'Finding alpha items', reporter: 'e2e-test' },
      { title: 'SearchTest Beta Unique', status: 'in-progress', priority: 'medium', description: 'Finding beta items', reporter: 'e2e-test' },
      { title: 'SearchTest Gamma Unique', status: 'open', priority: 'low', description: 'Finding gamma items', reporter: 'e2e-test' },
    ]

    for (const ticket of tickets) {
      const created = await api.createEntity('tickets', {
        properties: ticket,
        relations: { 'belongs-to': [categoryId] },
      })
      createdTickets.push(created.id)
    }
  })

  test.afterEach(async ({ api }) => {
    for (const id of createdTickets) {
      await api.deleteEntity('tickets', id)
    }
    createdTickets.length = 0
  })

  test('search API returns matching results', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_search?q=SearchTest Alpha`)
    expect(response.ok()).toBeTruthy()

    const result: SearchResponse = await response.json()
    const results = result.data || result
    expect(Array.isArray(results)).toBeTruthy()
    expect(results.length).toBeGreaterThanOrEqual(1)

    // Should find our alpha ticket
    const alphaResult = results.find((r: EntityResponse) => r.properties.title === 'SearchTest Alpha Unique')
    expect(alphaResult).toBeTruthy()
  })

  test('search page is accessible', async ({ pages }) => {
    const searchPage = pages.search()
    await searchPage.visit()
  })

  test('search input is visible', async ({ pages }) => {
    const searchPage = pages.search()
    await searchPage.visit()

    await expect(searchPage.searchInput).toBeVisible()
  })

  test('can perform search and see results', async ({ pages }) => {
    const searchPage = pages.search()
    await searchPage.visit()

    await searchPage.search('SearchTest')

    // Should show search results
    expect(await searchPage.resultsContain('SearchTest')).toBeTruthy()
  })

  test('search results include ticket title', async ({ pages }) => {
    const searchPage = pages.search()
    await searchPage.visit()

    await searchPage.search('SearchTest Alpha Unique')

    expect(await searchPage.resultsContain('SearchTest Alpha Unique')).toBeTruthy()
  })

  test('clicking search result navigates to entity', async ({ pages, apiPage }) => {
    const searchPage = pages.search()
    await searchPage.visit()

    await searchPage.search('SearchTest Alpha')

    // Click on a search result
    await searchPage.clickResult('SearchTest')

    await expect(apiPage).toHaveURL(/\/entity\/ticket\/TKT-\d+/)
  })

  test('empty search shows appropriate message', async ({ pages }) => {
    const searchPage = pages.search()
    await searchPage.visit()

    await searchPage.search('ThisQueryShouldNotMatchAnything12345')

    // Should show no results message or empty state
    expect(await searchPage.isEmptyStateVisible()).toBeTruthy()
  })
})

test.describe('Search API', () => {
  test('search supports query parameter', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_search?q=ticket`)
    expect(response.ok()).toBeTruthy()
  })

  test('search supports type filter', async ({ request, backend }) => {
    const response = await request.get(`${backend.baseUrl}/api/v1/_search?q=test&type=ticket`)
    expect(response.ok()).toBeTruthy()

    const result: SearchResponse = await response.json()
    const results = result.data || result
    if (results.length > 0) {
      // All results should be tickets
      expect(results.every((r: EntityResponse) => r.type === 'ticket')).toBeTruthy()
    }
  })

  test('search returns entities with properties', async ({ request, backend, api }) => {
    const categoryId = await api.getOrCreateCategory()

    const created = await api.createEntity('tickets', {
      properties: { title: 'API Search Test Ticket', status: 'open', priority: 'medium', reporter: 'e2e-test' },
      relations: { 'belongs-to': [categoryId] },
    })

    try {
      const response = await request.get(`${backend.baseUrl}/api/v1/_search?q=API Search Test`)
      expect(response.ok()).toBeTruthy()

      const result: SearchResponse = await response.json()
      const results = result.data || result
      const found = results.find((r: EntityResponse) => r.id === created.id)
      expect(found).toBeTruthy()
      expect(found.properties.title).toBe('API Search Test Ticket')
    } finally {
      await api.deleteEntity('tickets', created.id)
    }
  })
})
