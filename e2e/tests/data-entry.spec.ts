import { test, expect } from './fixtures';

/**
 * Basic navigation and smoke tests for the Data Entry SPA.
 * More comprehensive tests are in separate spec files:
 * - crud.spec.ts - Entity CRUD operations
 * - list.spec.ts - List view (sorting, filtering, pagination)
 * - kanban.spec.ts - Kanban board
 * - search.spec.ts - Search functionality
 * - settings.spec.ts - Settings page
 */
test.describe('Data Entry App - Navigation', () => {
  test('loads and shows navigation', async ({ appPage }) => {
    // Check for navigation items from our test project
    await expect(appPage.getByRole('link', { name: 'Features' })).toBeVisible();
    await expect(appPage.getByRole('link', { name: 'Bugs' })).toBeVisible();
    await expect(appPage.getByRole('link', { name: 'Tasks' })).toBeVisible();
  });

  test('shows dashboard by default', async ({ appPage }) => {
    // SPA redirects to /dashboard
    await expect(appPage).toHaveURL(/\/(dashboard)?/);
  });

  test('can navigate to Features list', async ({ appPage }) => {
    await appPage.click('text=Features');
    await expect(appPage.locator('h1').filter({ hasText: 'Features' })).toBeVisible();
    await expect(appPage).toHaveURL(/\/list\/features/);
  });

  test('can navigate to Bugs list', async ({ appPage }) => {
    await appPage.click('text=Bugs');
    await expect(appPage.locator('h1').filter({ hasText: 'Bugs' })).toBeVisible();
    await expect(appPage).toHaveURL(/\/list\/bugs/);
  });

  test('can navigate to Feature Board (kanban)', async ({ appPage }) => {
    await appPage.click('text=Feature Board');
    await expect(appPage.locator('h1').filter({ hasText: 'Feature Board' })).toBeVisible();
    await expect(appPage).toHaveURL(/\/kanban\/feature-board/);
  });

  test('can navigate to Search', async ({ appPage }) => {
    await appPage.click('text=Search');
    await expect(appPage.locator('h1').filter({ hasText: 'Search' })).toBeVisible();
    await expect(appPage).toHaveURL(/\/search/);
  });

  test('can navigate to Settings', async ({ appPage }) => {
    await appPage.click('text=Settings');
    await expect(appPage.locator('h1').filter({ hasText: 'Settings' })).toBeVisible();
    await expect(appPage).toHaveURL(/\/settings/);
  });

  test('can click entity row to navigate to entity detail', async ({ appPage, serverUrl }) => {
    // First create an entity via API (since pre-seeded entities may not load immediately)
    const response = await appPage.request.post(`${serverUrl}/api/v1/features`, {
      data: {
        properties: {
          title: 'Navigation Test Feature',
          status: 'draft',
          priority: 'high',
        },
      },
    });
    expect(response.ok()).toBeTruthy();
    const entity = await response.json();

    // Navigate to Features list
    await appPage.click('text=Features');
    await expect(appPage.locator('h1').filter({ hasText: 'Features' })).toBeVisible();

    // Wait for the entity to appear in the list
    await expect(appPage.getByText('Navigation Test Feature').first()).toBeVisible({ timeout: 3000 });

    // Click on the row to navigate to entity detail
    const featureRow = appPage.locator('tr.entity-row').filter({ hasText: 'Navigation Test Feature' });
    await featureRow.click();

    // Wait for navigation - should go to /entity/feature/FEAT-xxx
    await expect(appPage).toHaveURL(/\/entity\/feature\//, { timeout: 3000 });

    // Should show entity detail (either the title or the entity ID)
    await expect(appPage.locator('h1, h2, .entity-title').filter({ hasText: /Navigation Test Feature|FEAT-/ }).first()).toBeVisible({ timeout: 3000 });
  });
});

test.describe('Data Entry App - Basic CRUD', () => {
  test('can create a new feature via API and see it in list', async ({ appPage, serverUrl }) => {
    // Create a feature using the API
    // API v1 uses: /api/v1/{plural} - e.g. /api/v1/features for entity type "feature"
    const response = await appPage.request.post(`${serverUrl}/api/v1/features`, {
      data: {
        properties: {
          title: 'Test Feature from E2E',
          status: 'draft',
          priority: 'high',
        },
      },
    });
    if (!response.ok()) {
      console.log('API response status:', response.status());
      console.log('API response body:', await response.text());
    }
    expect(response.ok()).toBeTruthy();
    const entity = await response.json();
    expect(entity.id).toBeTruthy();

    // Navigate to the Features list
    await appPage.click('text=Features');
    await expect(appPage.locator('h1').filter({ hasText: 'Features' })).toBeVisible();

    // Verify the feature appears in the list
    await expect(appPage.getByText('Test Feature from E2E').first()).toBeVisible({ timeout: 3000 });
  });

  test('can create a new bug via API and see it in list', async ({ appPage, serverUrl }) => {
    // Create a bug using the API
    // API v1 uses: /api/v1/{plural} - e.g. /api/v1/bugs for entity type "bug"
    const response = await appPage.request.post(`${serverUrl}/api/v1/bugs`, {
      data: {
        properties: {
          title: 'Test Bug from E2E',
          severity: 'high',
          status: 'draft',
        },
      },
    });
    expect(response.ok()).toBeTruthy();

    // Navigate to the Bugs list
    await appPage.getByRole('link', { name: 'Bugs' }).first().click();
    await expect(appPage.locator('h1').filter({ hasText: 'Bugs' })).toBeVisible();

    // Verify the bug appears in the list
    await expect(appPage.getByText('Test Bug from E2E').first()).toBeVisible({ timeout: 3000 });
  });
});
