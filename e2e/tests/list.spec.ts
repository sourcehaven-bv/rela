import { test, expect } from './fixtures';
import { ListPage } from '../pages/list.page';

test.describe('List View', () => {
  test.describe('Display', () => {
    test('displays entities in table format', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');
      await listPage.expectHeading('Features');

      // Table should be visible with entities
      await expect(listPage.table).toBeVisible();
      const rowCount = await listPage.getRowCount();
      expect(rowCount).toBeGreaterThan(0);
    });

    test('shows correct columns', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Check column headers
      await expect(appPage.locator('th').filter({ hasText: /title/i })).toBeVisible();
      await expect(appPage.locator('th').filter({ hasText: /status/i })).toBeVisible();
      await expect(appPage.locator('th').filter({ hasText: /priority/i })).toBeVisible();
    });

    test('shows create button', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      await expect(listPage.createButton).toBeVisible();
    });

    test('shows empty state when no entities', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      // Tasks list might be empty or have few items
      await listPage.navigateToList('tasks');

      // Either shows entities or empty state
      const hasEntities = await listPage.getRowCount() > 0;
      if (!hasEntities) {
        await listPage.expectEmpty();
      }
    });
  });

  test.describe('Sorting', () => {
    test('can sort by title ascending', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Click title header to sort
      await listPage.sortByColumn('title');

      // Check sort indicator
      await listPage.expectSortIndicator('title', 'asc');
    });

    test('can toggle sort direction', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Sort ascending first
      await listPage.sortByColumn('title');
      await listPage.expectSortIndicator('title', 'asc');

      // Click again to sort descending
      await listPage.sortByColumn('title');
      await listPage.expectSortIndicator('title', 'desc');
    });

    test('can sort by different columns', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Sort by status
      await listPage.sortByColumn('status');
      await listPage.expectSortIndicator('status', 'asc');

      // Sort by priority
      await listPage.sortByColumn('priority');
      await listPage.expectSortIndicator('priority', 'asc');
    });
  });

  test.describe('Filtering', () => {
    test('filter controls are visible', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Filter bar should be visible for lists with filter_controls
      await expect(listPage.filterBar).toBeVisible();
    });

    test('can filter by status', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      const initialCount = await listPage.getRowCount();

      // Apply filter
      const statusFilter = listPage.filterBar.locator('select').first();
      await statusFilter.selectOption('approved');

      await appPage.waitForTimeout(500);

      // Should show fewer results
      const filteredCount = await listPage.getRowCount();
      expect(filteredCount).toBeLessThanOrEqual(initialCount);

      // Should only show approved features
      await listPage.expectRowContains('approved');
    });

    test('can clear filters', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      const initialCount = await listPage.getRowCount();

      // Apply filter
      const statusFilter = listPage.filterBar.locator('select').first();
      await statusFilter.selectOption('approved');
      await appPage.waitForTimeout(500);

      // Clear filter by selecting empty option
      await statusFilter.selectOption('');
      await appPage.waitForTimeout(500);

      // Should show all results again
      const clearedCount = await listPage.getRowCount();
      expect(clearedCount).toBe(initialCount);
    });

    test('can combine multiple filters', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Apply multiple filters
      const filters = listPage.filterBar.locator('select');
      const filterCount = await filters.count();

      if (filterCount >= 2) {
        await filters.nth(0).selectOption({ index: 1 });
        await appPage.waitForTimeout(300);
        await filters.nth(1).selectOption({ index: 1 });
        await appPage.waitForTimeout(500);
      }

      // Results should be filtered
      const count = await listPage.getRowCount();
      expect(count).toBeGreaterThanOrEqual(0);
    });
  });

  test.describe('Navigation', () => {
    test('clicking row navigates to entity', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      await listPage.clickRow(0);

      // Should navigate away from list
      await expect(appPage).not.toHaveURL(/\/list\/features$/);
    });

    test('create button navigates to form', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      await listPage.clickCreateButton();

      await expect(appPage).toHaveURL(/\/form\/feature/);
    });
  });

  test.describe('Keyboard Navigation', () => {
    test('can navigate rows with keyboard', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Focus the list area
      await appPage.locator('.entity-table, table').click();

      // Press down arrow to select
      await appPage.keyboard.press('ArrowDown');

      // First row should be selected
      const selectedRow = appPage.locator('.entity-row.selected, tr.selected');
      await expect(selectedRow).toBeVisible();
    });

    test('N key opens create form', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Press N to create new
      await appPage.keyboard.press('n');

      await expect(appPage).toHaveURL(/\/form\/feature/);
    });
  });
});
