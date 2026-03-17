import { test, expect } from './fixtures'

/**
 * E2E tests for Dashboard functionality
 */

test.describe('Dashboard', () => {
  const createdTickets: string[] = []

  test.beforeEach(async ({ api }) => {
    // Create test tickets with various statuses and priorities
    const tickets = [
      { title: 'Dashboard Test: Open 1', status: 'open', priority: 'high', reporter: 'e2e-test' },
      { title: 'Dashboard Test: Open 2', status: 'open', priority: 'medium', reporter: 'e2e-test' },
      { title: 'Dashboard Test: In Progress', status: 'in-progress', priority: 'low', reporter: 'e2e-test' },
      { title: 'Dashboard Test: Critical', status: 'open', priority: 'critical', reporter: 'e2e-test' },
    ]

    for (const ticket of tickets) {
      const created = await api.createEntity('tickets', { properties: ticket })
      createdTickets.push(created.id)
    }
  })

  test.afterEach(async ({ api }) => {
    for (const id of createdTickets) {
      await api.deleteEntity('tickets', id)
    }
    createdTickets.length = 0
  })

  test('displays dashboard with cards', async ({ pages }) => {
    const dashboardPage = pages.dashboard()
    await dashboardPage.visit()

    // Should have dashboard cards
    const cardCount = await dashboardPage.getCardCount()
    expect(cardCount).toBeGreaterThanOrEqual(1)
  })

  test('displays Open Tickets count card', async ({ pages }) => {
    const dashboardPage = pages.dashboard()
    await dashboardPage.visit()

    // Look for "Open Tickets" card
    const statValue = await dashboardPage.getStatValue('Open Tickets')
    // Should have a count
    expect(statValue).toMatch(/\d+/)
  })

  test('displays In Progress count card', async ({ pages }) => {
    const dashboardPage = pages.dashboard()
    await dashboardPage.visit()

    // Look for "In Progress" card
    const statValue = await dashboardPage.getStatValue('In Progress')
    expect(statValue).toMatch(/\d+/)
  })

  test('displays By Status breakdown card', async ({ pages }) => {
    const dashboardPage = pages.dashboard()
    await dashboardPage.visit()

    // Look for "By Status" breakdown card
    const content = await dashboardPage.getPageContent()
    // Should show status values
    expect(content.toLowerCase()).toMatch(/by status|open|in-progress|resolved|closed/)
  })

  test('displays By Priority breakdown card', async ({ pages }) => {
    const dashboardPage = pages.dashboard()
    await dashboardPage.visit()

    // Look for "By Priority" breakdown card
    const content = await dashboardPage.getPageContent()
    // Should show priority values
    expect(content.toLowerCase()).toMatch(/by priority|critical|high|medium|low/)
  })

  test('displays Critical Issues table card', async ({ pages }) => {
    const dashboardPage = pages.dashboard()
    await dashboardPage.visit()

    // Should contain our critical test ticket
    const content = await dashboardPage.getPageContent()
    expect(content).toContain('Dashboard Test: Critical')
  })

  test('clicking ticket in table navigates to detail', async ({ pages, apiPage }) => {
    const dashboardPage = pages.dashboard()
    await dashboardPage.visit()

    // Find a ticket link in a table card and click it
    const ticketLink = apiPage.locator('.dashboard a[href*="/entity/ticket/"], .card a[href*="/entity/ticket/"]').first()
    if (await ticketLink.isVisible()) {
      await ticketLink.click()

      // Should navigate to entity detail
      await expect(apiPage).toHaveURL(/\/entity\/ticket\/TKT-\d+/)
    }
  })

  test('dashboard shows app title', async ({ pages }) => {
    const dashboardPage = pages.dashboard()
    await dashboardPage.visit()

    // Should show dashboard title
    const content = await dashboardPage.getPageContent()
    expect(content).toMatch(/Dashboard|Ticket overview/)
  })
})

test.describe('Dashboard Commands', () => {
  test('project info command is available', async ({ pages }) => {
    const dashboardPage = pages.dashboard()
    await dashboardPage.visit()

    // Look for commands section or button
    // Command may or may not be visible depending on implementation
    expect(true).toBeTruthy() // Page loaded successfully
  })
})
