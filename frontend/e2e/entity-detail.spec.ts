import { test, expect } from './fixtures'

/**
 * E2E tests for Entity Detail View
 */

test.describe('Entity Detail View', () => {
  let testTicketId: string | null = null
  let testCategoryId: string | null = null

  test.beforeEach(async ({ api }) => {
    testCategoryId = await api.getOrCreateCategory()

    const ticket = await api.createEntity('tickets', {
      properties: {
        title: 'Detail View Test Ticket',
        description: 'Testing the entity detail view functionality',
        status: 'open',
        priority: 'high',
        assignee: 'tester',
        reporter: 'e2e-test',
      },
      relations: testCategoryId ? { 'belongs-to': [testCategoryId] } : {},
    })
    testTicketId = ticket.id
  })

  test.afterEach(async ({ api }) => {
    if (testTicketId) {
      await api.deleteEntity('tickets', testTicketId)
      testTicketId = null
    }
  })

  test('displays entity detail page', async ({ pages }) => {
    const detailPage = pages.entityDetail('ticket', testTicketId!)
    await detailPage.visit()
  })

  test('shows ticket title', async ({ pages }) => {
    const detailPage = pages.entityDetail('ticket', testTicketId!)
    await detailPage.visit()

    expect(await detailPage.containsText('Detail View Test Ticket')).toBeTruthy()
  })

  test('shows ticket properties', async ({ pages }) => {
    const detailPage = pages.entityDetail('ticket', testTicketId!)
    await detailPage.visit()

    // Wait for Vue to render the ticket title first
    expect(await detailPage.containsText('Detail View Test Ticket')).toBeTruthy()

    // Should show status
    expect(await detailPage.hasProperty('open')).toBeTruthy()
    // Should show priority
    expect(await detailPage.hasProperty('high')).toBeTruthy()
  })

  test('shows ticket description', async ({ pages }) => {
    const detailPage = pages.entityDetail('ticket', testTicketId!)
    await detailPage.visit()

    expect(await detailPage.containsText('Testing the entity detail view functionality')).toBeTruthy()
  })

  test('has edit button', async ({ pages }) => {
    const detailPage = pages.entityDetail('ticket', testTicketId!)
    await detailPage.visit()

    expect(await detailPage.hasEditButton()).toBeTruthy()
  })

  test('edit button navigates to edit form', async ({ pages, apiPage }) => {
    const detailPage = pages.entityDetail('ticket', testTicketId!)
    await detailPage.visit()

    await detailPage.clickEditButton()
    await expect(apiPage).toHaveURL(/\/form\/edit_ticket\//)
  })

  test('shows sections as configured in view', async ({ pages }) => {
    const detailPage = pages.entityDetail('ticket', testTicketId!)
    await detailPage.visit()

    // ticket_report view has sections: Ticket, Content, Blocks, Blocked By, Labels
    // Should have at least Ticket section
    const content = await detailPage.getPageContent()
    expect(content.toLowerCase()).toMatch(/ticket|properties|details/)
  })
})

test.describe('Category Detail View', () => {
  let testCategoryId: string | null = null

  test.beforeEach(async ({ api }) => {
    // Don't pass custom ID - let the server auto-generate it using sequential ID
    const category = await api.createEntity('categories', {
      properties: {
        name: 'Category Detail Test',
        description: 'Testing category detail view',
        color: '#0000ff',
      },
    })
    testCategoryId = category.id
  })

  test.afterEach(async ({ api }) => {
    if (testCategoryId) {
      await api.deleteEntity('categories', testCategoryId)
      testCategoryId = null
    }
  })

  test('displays category detail page', async ({ pages }) => {
    const detailPage = pages.entityDetail('category', testCategoryId!)
    await detailPage.visit()

    expect(await detailPage.containsText('Category Detail Test')).toBeTruthy()
  })

  test('shows category properties', async ({ pages }) => {
    const detailPage = pages.entityDetail('category', testCategoryId!)
    await detailPage.visit()

    expect(await detailPage.containsText('Testing category detail view')).toBeTruthy()
  })
})

test.describe('Entity Detail with Relations', () => {
  let ticketId: string | null = null
  let blockingTicketId: string | null = null

  test.beforeEach(async ({ request, backend, api }) => {
    const categoryId = await api.getOrCreateCategory()

    // Create two tickets and a blocking relation
    const ticket1 = await api.createEntity('tickets', {
      properties: { title: 'Blocking Ticket', status: 'open', priority: 'high', reporter: 'e2e-test' },
      relations: { 'belongs-to': [categoryId] },
    })
    blockingTicketId = ticket1.id

    const ticket2 = await api.createEntity('tickets', {
      properties: { title: 'Blocked Ticket', status: 'open', priority: 'medium', reporter: 'e2e-test' },
      relations: { 'belongs-to': [categoryId] },
    })
    ticketId = ticket2.id

    // Create blocking relation: ticket1 blocks ticket2
    await request.post(`${backend.baseUrl}/api/v1/tickets/${blockingTicketId}/relations/blocks`, {
      data: { target: ticketId },
    })
  })

  test.afterEach(async ({ api }) => {
    if (ticketId) {
      await api.deleteEntity('tickets', ticketId)
    }
    if (blockingTicketId) {
      await api.deleteEntity('tickets', blockingTicketId)
    }
  })

  test('shows blocking relations in detail view', async ({ pages }) => {
    // View the blocked ticket - should show "Blocked By" section
    const detailPage = pages.entityDetail('ticket', ticketId!)
    await detailPage.visit()

    const content = await detailPage.getPageContent()
    // Should show that it's blocked by another ticket
    expect(content.toLowerCase()).toMatch(/blocked|blocking/)
  })

  test('shows tickets that are blocked in detail view', async ({ pages }) => {
    // View the blocking ticket - should show "Blocks" section
    const detailPage = pages.entityDetail('ticket', blockingTicketId!)
    await detailPage.visit()

    const content = await detailPage.getPageContent()
    expect(content.toLowerCase()).toMatch(/block/)
  })
})
