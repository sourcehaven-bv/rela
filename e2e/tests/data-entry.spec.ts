import { test, expect } from './fixtures';
import { BasePage, ListPage, ApiClient } from '../pages';

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
    const base = new BasePage(appPage);
    await base.expectNavLinkVisible('Features');
    await base.expectNavLinkVisible('Bugs');
    await base.expectNavLinkVisible('Tasks');
  });

  test('shows dashboard by default', async ({ appPage }) => {
    await expect(appPage).toHaveURL(/\/(dashboard)?/);
  });

  test('can navigate to Features list', async ({ appPage }) => {
    const base = new BasePage(appPage);
    await base.clickSidebarLink('Features');
    await expect(appPage).toHaveURL(/\/list\/features/);
  });

  test('can navigate to Bugs list', async ({ appPage }) => {
    const base = new BasePage(appPage);
    await base.clickSidebarLink('Bugs');
    await expect(appPage).toHaveURL(/\/list\/bugs/);
  });

  test('can navigate to Feature Board (kanban)', async ({ appPage }) => {
    const base = new BasePage(appPage);
    await base.clickSidebarLink('Feature Board');
    await expect(appPage).toHaveURL(/\/kanban\/feature-board/);
  });

  test('can navigate to Search', async ({ appPage }) => {
    const base = new BasePage(appPage);
    await base.clickSidebarLink('Search');
    await expect(appPage).toHaveURL(/\/search/);
  });

  test('can navigate to Settings', async ({ appPage }) => {
    const base = new BasePage(appPage);
    await base.clickSidebarLink('Settings');
    await expect(appPage).toHaveURL(/\/settings/);
  });

  test('can click entity row to navigate to entity detail', async ({ appPage, serverUrl }) => {
    const api = new ApiClient(appPage, serverUrl);
    const listPage = new ListPage(appPage);

    // Seed a known entity via the API
    const entity = await api.createEntity('features', {
      title: 'Navigation Test Feature',
      status: 'draft',
      priority: 'high',
    });

    await listPage.navigateToList('features');
    await listPage.expectRowContains(entity.id);
    await listPage.clickRowById(entity.id);

    await expect(appPage).toHaveURL(new RegExp(`/entity/feature/${entity.id}`));
  });
});

test.describe('Data Entry App - Basic CRUD', () => {
  test('can create a new feature via API and see it in list', async ({ appPage, serverUrl }) => {
    const api = new ApiClient(appPage, serverUrl);
    const listPage = new ListPage(appPage);

    const entity = await api.createEntity('features', {
      title: 'Test Feature from E2E',
      status: 'draft',
      priority: 'high',
    });

    await listPage.navigateToList('features');
    await listPage.expectRowContains(entity.id);
    await listPage.expectRowContains('Test Feature from E2E');
  });

  test('can create a new bug via API and see it in list', async ({ appPage, serverUrl }) => {
    const api = new ApiClient(appPage, serverUrl);
    const listPage = new ListPage(appPage);

    const entity = await api.createEntity('bugs', {
      title: 'Test Bug from E2E',
      severity: 'high',
      status: 'draft',
    });

    await listPage.navigateToList('bugs');
    await listPage.expectRowContains(entity.id);
    await listPage.expectRowContains('Test Bug from E2E');
  });
});
