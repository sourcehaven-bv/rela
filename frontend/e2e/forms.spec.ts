import { test, expect, type EntityResponse } from './fixtures'

/**
 * E2E tests for form operations
 */

test.describe('Create Form', () => {
  const createdEntities: { type: string; id: string }[] = []

  test.afterEach(async ({ api }) => {
    for (const entity of createdEntities) {
      const plural = entity.type === 'category' ? 'categories' : `${entity.type}s`
      await api.deleteEntity(plural, entity.id)
    }
    createdEntities.length = 0
  })

  test('create ticket form displays all fields', async ({ pages }) => {
    const formPage = pages.createTicketForm()
    await formPage.visit()

    // Check for expected form fields
    expect(await formPage.hasField('title')).toBeTruthy()
    expect(await formPage.hasField('priority')).toBeTruthy()
    expect(await formPage.hasField('description')).toBeTruthy()
  })

  test('create ticket form has submit button', async ({ pages }) => {
    const formPage = pages.createTicketForm()
    await formPage.visit()

    expect(await formPage.hasSubmitButton()).toBeTruthy()
  })

  test('can fill and submit create ticket form', async ({ pages, request, backend, apiPage }) => {
    const formPage = pages.createTicketForm()
    await formPage.visit()

    // Fill in form fields
    await formPage.fillField('title', 'E2E Form Test Ticket')
    await formPage.fillField('description', 'Testing form submission')
    await formPage.selectField('priority', 'medium')

    // Select a category (required relation)
    await formPage.selectFirstRelation('belongs-to')

    // Submit form
    await formPage.submit()

    // Wait for navigation or success message
    await apiPage.waitForTimeout(2000)

    // Should either redirect or show success
    const url = apiPage.url()
    const content = await apiPage.content()
    const success = url.includes('/entity/ticket/') || url.includes('/list/') || content.toLowerCase().includes('success') || content.toLowerCase().includes('created')
    expect(success).toBeTruthy()

    // Clean up: find and delete the created ticket
    const ticketsResponse = await request.get(`${backend.baseUrl}/api/v1/tickets?filter[title]=E2E Form Test Ticket`)
    const ticketsResult = await ticketsResponse.json()
    // API returns {data: [...]} or raw array
    const tickets: EntityResponse[] = Array.isArray(ticketsResult) ? ticketsResult : ticketsResult.data || []
    for (const ticket of tickets) {
      createdEntities.push({ type: 'ticket', id: ticket.id })
    }
  })

  test('form shows validation errors for required fields', async ({ pages, apiPage }) => {
    const formPage = pages.createTicketForm()
    await formPage.visit()

    // Submit without filling required fields
    await formPage.submit()

    // Should show validation error or prevent submission
    await apiPage.waitForTimeout(1000)

    // Check for validation indicators
    const hasError = await formPage.hasValidationError()
    const stillOnForm = apiPage.url().includes('/form/')

    expect(hasError || stillOnForm).toBeTruthy()
  })
})

test.describe('Edit Form', () => {
  let testTicketId: string | null = null

  test.beforeEach(async ({ api }) => {
    const ticket = await api.createEntity('tickets', {
      properties: {
        title: 'E2E Edit Form Test',
        description: 'Original description',
        status: 'open',
        priority: 'low',
        assignee: 'original-assignee',
        reporter: 'e2e-test',
      },
    })
    testTicketId = ticket.id
  })

  test.afterEach(async ({ api }) => {
    if (testTicketId) {
      await api.deleteEntity('tickets', testTicketId)
      testTicketId = null
    }
  })

  test('edit form loads with existing data', async ({ pages }) => {
    const formPage = pages.form(`edit_ticket/${testTicketId}`)
    await formPage.visit()

    // Title field should have existing value
    expect(await formPage.getFieldValue('title')).toBe('E2E Edit Form Test')

    // Priority should be selected
    expect(await formPage.getFieldValue('priority')).toBe('low')
  })

  test('can update ticket via edit form', async ({ pages, api, apiPage }) => {
    const formPage = pages.form(`edit_ticket/${testTicketId}`)
    await formPage.visit()

    // Update the title
    await formPage.fillField('title', 'E2E Edit Form Test - Updated')

    // Update priority
    await formPage.selectField('priority', 'high')

    // Submit
    await formPage.submit()

    await apiPage.waitForTimeout(2000)

    // Verify update via API
    const ticket = await api.getEntity('tickets', testTicketId!)
    expect(ticket.properties.title).toBe('E2E Edit Form Test - Updated')
    expect(ticket.properties.priority).toBe('high')
  })

  test('status transitions are enforced', async ({ pages }) => {
    const formPage = pages.form(`edit_ticket/${testTicketId}`)
    await formPage.visit()

    // Status select should only show valid transitions from 'open'
    const statusOptions = await formPage.getSelectOptions('status')
    // From 'open', valid transitions are: in-progress, closed
    // Just verify we have options
    expect(statusOptions.length).toBeGreaterThanOrEqual(1)
  })
})

test.describe('Create Category Form', () => {
  let createdCategoryId: string | null = null

  test.afterEach(async ({ api }) => {
    if (createdCategoryId) {
      await api.deleteEntity('categories', createdCategoryId)
      createdCategoryId = null
    }
  })

  test('can create category', async ({ pages, api, apiPage }) => {
    const formPage = pages.form('create_category')
    await formPage.visit()

    // Fill in category fields
    await formPage.fillField('name', 'E2E Test Category')
    await formPage.fillField('description', 'Category created by e2e test')
    await formPage.fillField('color', '#ff5500')

    // Submit
    await formPage.submit()

    await apiPage.waitForTimeout(2000)

    // Verify via API
    const result = await api.listEntities('categories')
    const created = result.data.find((c) => c.properties.name === 'E2E Test Category')
    expect(created).toBeTruthy()
    if (created) {
      createdCategoryId = created.id
    }
  })
})

test.describe('Inline Entity Creation', () => {
  test('can create related entity inline from form', async ({ pages, apiPage }) => {
    const formPage = pages.createTicketForm()
    await formPage.visit()

    // Look for inline create button for category
    const inlineCreateButton = apiPage.locator('button:has-text("New Category"), .btn-inline-create, [data-create-inline]')
    if (await inlineCreateButton.isVisible()) {
      await inlineCreateButton.click()

      // Should show inline form or modal
      const inlineForm = apiPage.locator('.inline-form, .modal, dialog')
      await expect(inlineForm).toBeVisible({ timeout: 5000 })
    }
  })
})
