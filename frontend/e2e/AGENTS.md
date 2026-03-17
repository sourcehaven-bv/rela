# E2E Test Guidelines for AI Agents

This document provides instructions for AI agents working on e2e tests in this project.

## Page Object Pattern

All e2e tests **MUST** use the Page Object pattern. This pattern is based on
[Martin Fowler's Page Object](https://martinfowler.com/bliki/PageObject.html) design pattern.

### Core Principles

1. **Encapsulate UI structure** - Page objects hide the HTML structure from tests
2. **No assertions in page objects** - Page objects provide data; tests make assertions
3. **Methods return page objects** - Navigation methods return the destination page object
4. **Domain-specific API** - Public methods represent services the page offers to users

### Available Page Objects

Import page objects from the central index:

```typescript
import { SearchPage, EntityDetailPage, DashboardPage } from './page-objects'
```

| Page Object | View | Factory Function |
|-------------|------|------------------|
| `SearchPage` | Search view | `new SearchPage(page)` |
| `EntityDetailPage` | Entity detail view | `createEntityDetailPage(page, type, id)` |
| `FormPage` | Create/edit forms | `createFormPage(page, formName)` |
| `CreateTicketFormPage` | Ticket creation | `new CreateTicketFormPage(page)` |
| `ListPage` | List views | `createListPage(page, listName)` |
| `KanbanPage` | Kanban boards | `createKanbanPage(page, boardName)` |
| `DashboardPage` | Dashboard | `new DashboardPage(page)` |
| `GraphPage` | Graph explorer | `new GraphPage(page)` |

### Writing Tests with Page Objects

**Good Example:**

```typescript
import { test, expect } from './fixtures'
import { SearchPage } from './page-objects'

test('search finds tickets', async ({ apiPage, api }) => {
  // Setup test data via API
  const categoryId = await api.getOrCreateCategory()
  await api.createEntity('tickets', {
    properties: { title: 'SearchTest Ticket', status: 'open', priority: 'medium', reporter: 'e2e-test' },
    relations: { 'belongs-to': [categoryId] },
  })

  // Use page object for UI interaction
  const searchPage = new SearchPage(apiPage)
  await searchPage.visit()  // Combines goto() + waitForLoad()
  await searchPage.search('SearchTest')

  // Assertions in test, not page object
  expect(await searchPage.resultsContain('SearchTest Ticket')).toBeTruthy()
})
```

**Bad Example (avoid):**

```typescript
// DON'T: Raw selectors scattered in tests
test('search finds tickets', async ({ apiPage }) => {
  await apiPage.goto('/search')
  await apiPage.locator('input[type="search"]').fill('test')  // Fragile!
  await apiPage.locator('.search-result').click()  // Breaks when class changes!
})
```

### Creating New Page Objects

When adding a new view or page, create a page object following this structure:

```typescript
import { Page, Locator } from '@playwright/test'
import { BasePage } from './BasePage'

export class MyNewPage extends BasePage {
  constructor(page: Page) {
    super(page)
  }

  // Selectors - centralized, easy to update
  get myElement(): Locator {
    return this.page.locator('.my-element, [data-testid="my-element"]')
  }

  // Navigation
  async goto(): Promise<void> {
    await this.page.goto('/my-page')
  }

  async waitForLoad(): Promise<void> {
    await this.myElement.waitFor({ state: 'visible', timeout: 10000 })
  }

  // Actions - domain-specific methods
  async doSomething(): Promise<void> {
    await this.myElement.click()
  }

  // State queries - for tests to assert against
  async isSomethingVisible(): Promise<boolean> {
    return this.myElement.isVisible()
  }
}
```

### Selector Best Practices

1. **Use multiple fallback selectors** for resilience:
   ```typescript
   get submitButton(): Locator {
     return this.page.locator('button[type="submit"], .btn-submit, button:has-text("Save")').first()
   }
   ```

2. **Prefer semantic selectors** over implementation-specific ones:
   - Good: `button:has-text("Save")`, `input[name="title"]`
   - Avoid: `.btn-primary.mt-2`, `div > span:nth-child(3)`

3. **Use `data-testid` attributes** when available:
   ```typescript
   get searchResults(): Locator {
     return this.page.locator('[data-testid="search-results"]')
   }
   ```

## Test Fixtures

### Available Fixtures

| Fixture | Purpose |
|---------|---------|
| `backend` | Isolated backend server with temp project |
| `apiPage` | Page with API routing to test backend |
| `api` | API helpers for CRUD operations |

### API Helpers

Use the `api` fixture for test data setup/teardown:

```typescript
test('my test', async ({ api, apiPage }) => {
  // Create test data
  const categoryId = await api.getOrCreateCategory()
  const ticket = await api.createEntity('tickets', {
    properties: { title: 'Test', status: 'open', priority: 'low', reporter: 'e2e-test' },
    relations: { 'belongs-to': [categoryId] },
  })

  // Test UI with page objects...

  // Cleanup (or use afterEach)
  await api.deleteEntity('tickets', ticket.id)
})
```

### Required Fields

When creating entities, include ALL required fields per the metamodel:

**Tickets require:**
- `title` (required)
- `status` (required)
- `priority` (required)
- `reporter` (required) - Use `'e2e-test'` for test data
- `belongs-to` relation to a category

**Categories require:**
- `name` (required)
- `id` (required for manual ID types) - Use `\`e2e-cat-${Date.now()}\``

**Labels require:**
- `name` (required)
- `id` (required) - Use `\`e2e-label-${Date.now()}\``

## Waiting for Vue SPA

Since this is a Vue SPA, always wait for content to render:

```typescript
// Good: Wait for specific content
await searchPage.waitForLoad()
await page.getByText('Expected Title').waitFor({ state: 'visible', timeout: 10000 })

// Bad: Fixed timeout without waiting for content
await page.waitForTimeout(1000)  // Flaky!
```

## Test Organization

1. **Group related tests** with `test.describe()`
2. **Use beforeEach/afterEach** for setup/cleanup
3. **One assertion focus per test** when possible
4. **Descriptive test names** that explain the behavior being tested

## When to Update Page Objects

Update page objects when:
- UI structure changes (update selectors)
- New interactions are added (add new action methods)
- New assertions are needed (add state query methods)

The goal is to update selectors in ONE place (the page object) rather than in every test.
