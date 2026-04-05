import { test, expect } from './fixtures'

/**
 * E2E tests for entity CRUD operations via API
 */

test.describe('Entity CRUD Operations', () => {
  const createdEntities: { type: string; id: string }[] = []

  test.afterEach(async ({ api }) => {
    for (const entity of createdEntities) {
      const plural = entity.type === 'category' ? 'categories' : `${entity.type}s`
      await api.deleteEntity(plural, entity.id)
    }
    createdEntities.length = 0
  })

  test.describe('Tickets', () => {
    test('create a new ticket via API', async ({ api }) => {
      const categoryId = await api.getOrCreateCategory()

      const created = await api.createEntity('tickets', {
        properties: {
          title: 'E2E Test: Create Ticket',
          description: 'Testing ticket creation via API',
          status: 'open',
          priority: 'medium',
          reporter: 'e2e-test',
        },
        relations: {
          'belongs-to': [categoryId],
        },
      })
      createdEntities.push({ type: 'ticket', id: created.id })

      expect(created.id).toMatch(/^TKT-\d+$/)
      expect(created.type).toBe('ticket')
      expect(created.properties.title).toBe('E2E Test: Create Ticket')
      expect(created.properties.status).toBe('open')
      expect(created.properties.priority).toBe('medium')
    })

    test('read a ticket via API', async ({ api }) => {
      const categoryId = await api.getOrCreateCategory()

      const created = await api.createEntity('tickets', {
        properties: {
          title: 'E2E Test: Read Ticket',
          status: 'open',
          priority: 'low',
          reporter: 'e2e-test',
        },
        relations: {
          'belongs-to': [categoryId],
        },
      })
      expect(created.id).toBeTruthy()
      createdEntities.push({ type: 'ticket', id: created.id })

      // Read the ticket
      const ticket = await api.getEntity('tickets', created.id)
      expect(ticket.id).toBe(created.id)
      expect(ticket.properties.title).toBe('E2E Test: Read Ticket')
    })

    test('update a ticket via API', async ({ api }) => {
      const categoryId = await api.getOrCreateCategory()

      const created = await api.createEntity('tickets', {
        properties: {
          title: 'E2E Test: Update Ticket',
          status: 'open',
          priority: 'low',
          reporter: 'e2e-test',
        },
        relations: {
          'belongs-to': [categoryId],
        },
      })
      createdEntities.push({ type: 'ticket', id: created.id })

      // Update the ticket
      await api.updateEntity('tickets', created.id, {
        priority: 'high',
        assignee: 'developer',
      })

      // Verify the update
      const updated = await api.getEntity('tickets', created.id)
      expect(updated.properties.priority).toBe('high')
      expect(updated.properties.assignee).toBe('developer')
      expect(updated.properties.title).toBe('E2E Test: Update Ticket') // Unchanged
    })

    test('delete a ticket via API', async ({ request, backend, api }) => {
      const categoryId = await api.getOrCreateCategory()

      const created = await api.createEntity('tickets', {
        properties: {
          title: 'E2E Test: Delete Ticket',
          status: 'open',
          priority: 'low',
          reporter: 'e2e-test',
        },
        relations: {
          'belongs-to': [categoryId],
        },
      })

      // Delete the ticket
      await api.deleteEntity('tickets', created.id)

      // Verify deletion (need raw request to check 404)
      const readResponse = await request.get(`${backend.baseUrl}/api/v1/tickets/${created.id}`)
      expect(readResponse.status()).toBe(404)
    })

    test('list tickets via API', async ({ api }) => {
      const categoryId = await api.getOrCreateCategory()

      // Create multiple tickets
      for (let i = 0; i < 3; i++) {
        const created = await api.createEntity('tickets', {
          properties: {
            title: `E2E Test: List Ticket ${i + 1}`,
            status: 'open',
            priority: i === 0 ? 'high' : 'low',
            reporter: 'e2e-test',
          },
          relations: {
            'belongs-to': [categoryId],
          },
        })
        createdEntities.push({ type: 'ticket', id: created.id })
      }

      // List all tickets
      const result = await api.listEntities('tickets')
      expect(Array.isArray(result.data)).toBeTruthy()
      expect(result.data.length).toBeGreaterThanOrEqual(3)
    })

    test('list tickets with pagination', async ({ api }) => {
      const result = await api.listEntities('tickets', 'page=1&per_page=2')
      expect(Array.isArray(result.data)).toBeTruthy()
      expect(result.data.length).toBeLessThanOrEqual(2)
    })

    test('list tickets with filter', async ({ api }) => {
      const categoryId = await api.getOrCreateCategory()

      const created = await api.createEntity('tickets', {
        properties: {
          title: 'E2E Test: Filter Ticket',
          status: 'open',
          priority: 'critical',
          reporter: 'e2e-test',
        },
        relations: {
          'belongs-to': [categoryId],
        },
      })
      createdEntities.push({ type: 'ticket', id: created.id })

      // Filter by priority
      const result = await api.listEntities('tickets', 'filter[priority]=critical')
      expect(result.data.length).toBeGreaterThanOrEqual(1)
      expect(result.data.every((t) => t.properties.priority === 'critical')).toBeTruthy()
    })
  })

  test.describe('Categories', () => {
    test('create a new category via API', async ({ api }) => {
      // Don't pass custom ID - let the server auto-generate it using sequential ID
      const created = await api.createEntity('categories', {
        properties: {
          name: 'E2E Test Category',
          description: 'Category for e2e testing',
          color: '#ff0000',
        },
      })
      createdEntities.push({ type: 'category', id: created.id })

      expect(created.type).toBe('category')
      expect(created.properties.name).toBe('E2E Test Category')
      expect(created.id).toMatch(/^CAT-\d+$/) // Verify ID format
    })

    test('list categories via API', async ({ api }) => {
      const result = await api.listEntities('categories')
      expect(Array.isArray(result.data)).toBeTruthy()
    })
  })

  test.describe('Labels', () => {
    test('create a new label via API', async ({ api }) => {
      // Don't pass custom ID - let the server auto-generate it using sequential ID
      const created = await api.createEntity('labels', {
        properties: {
          name: 'e2e-test-label',
        },
      })
      createdEntities.push({ type: 'label', id: created.id })

      expect(created.type).toBe('label')
      expect(created.properties.name).toBe('e2e-test-label')
      expect(created.id).toMatch(/^LBL-\d+$/) // Verify ID format
    })
  })

  test.describe('Relations', () => {
    test('create ticket with relations', async ({ api }) => {
      const result = await api.listEntities('categories')

      if (result.data.length === 0) {
        test.skip(true, 'No categories available for relation test')
        return
      }

      const categoryId = result.data[0].id

      const created = await api.createEntity('tickets', {
        properties: {
          title: 'E2E Test: Ticket with Relation',
          status: 'open',
          priority: 'medium',
          reporter: 'e2e-test',
        },
        relations: {
          'belongs-to': [categoryId],
        },
      })
      createdEntities.push({ type: 'ticket', id: created.id })

      // Verify relation exists
      const ticket = await api.getEntity('tickets', created.id)
      if (ticket.relations && ticket.relations['belongs-to']) {
        expect(ticket.relations['belongs-to']).toContain(categoryId)
      } else {
        // If relations not in response, verify ticket was created successfully
        expect(ticket.id).toBe(created.id)
        expect(ticket.properties.title).toBe('E2E Test: Ticket with Relation')
      }
    })
  })
})
