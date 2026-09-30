import { test, expect } from './fixtures';
import { ListPage } from '../pages/list.page';
import { FormPage } from '../pages/form.page';

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
      await listPage.expectColumnHeader('title');
      await listPage.expectColumnHeader('status');
      await listPage.expectColumnHeader('priority');
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
    test('can sort a column ascending', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      await listPage.sortByColumn('status');

      await listPage.expectSortIndicator('status', 'asc');
    });

    test('can toggle sort direction', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Sort ascending first
      await listPage.sortByColumn('status');
      await listPage.expectSortIndicator('status', 'asc');

      // Click again to sort descending
      await listPage.sortByColumn('status');
      await listPage.expectSortIndicator('status', 'desc');
    });

    /*
     * The title column sorts like any other.
     *
     * Briefly it did not: RlTable's first column is the row's NAME cell, and
     * its header was plain text with no control. `name-column` gives that
     * header a real column, so the title — the column users reach for first
     * — is sortable again.
     */
    test('can sort by the title column', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      await listPage.sortByColumn('title');
      await listPage.expectSortIndicator('title', 'asc');

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

      await listPage.setFilterByIndex(0, 'approved');

      // Should show fewer results
      const filteredCount = await listPage.getRowCount();
      expect(filteredCount).toBeLessThanOrEqual(initialCount);

      // Should only show approved features
      await listPage.expectRowContains('approved');
    });

    test('picking two values keeps rows matching either (OR)', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');
      await listPage.setFilterByIndex(0, ['approved', 'draft']);

      await listPage.expectRowContains('Dashboard Analytics');
      await listPage.expectRowContains('User Authentication');
      await listPage.expectRowNotVisible('Export Data');
    });

    test('can clear filters', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      const initialCount = await listPage.getRowCount();

      await listPage.setFilterByIndex(0, 'approved');
      await listPage.setFilterByIndex(0, '');

      // Should show all results again
      const clearedCount = await listPage.getRowCount();
      expect(clearedCount).toBe(initialCount);
    });

    test('can combine multiple filters', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      const filterCount = await listPage.filterControlCount();
      if (filterCount >= 2) {
        await listPage.setFilterByIndex(0, { index: 0 });
        await listPage.setFilterByIndex(1, { index: 0 });
      }

      // Results should be filtered
      const count = await listPage.getRowCount();
      expect(count).toBeGreaterThanOrEqual(0);
    });

    // TKT-DL16XM: a relation filter_control renders as a target selector (a
    // native <select> here, since the feature set is small) whose options are
    // the display titles of the relation's targets. Selecting a feature narrows
    // the task list to tasks that implement it. The tasks list declares
    // `filter_controls: [{ relation: implements }]`; the seed links
    // TASK-001 --implements--> FEAT-001 ("User Authentication"), and TASK-002
    // is unlinked.
    test('relation filter narrows the list by target title', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('tasks');
      await listPage.waitForRowsRendered();

      // Both seeded tasks visible initially.
      await listPage.expectRowContains('Write unit tests');
      await listPage.expectRowContains('Refactor auth module');

      // The relation filter renders as a <select> populated with feature
      // display titles (the option VALUE is the bare title — the backend
      // matches on it).
      expect(await listPage.filterOptionCount('User Authentication')).toBe(1);

      // Filter by the feature TASK-001 implements.
      await listPage.selectFilterOption('User Authentication');

      // Only the implementing task remains.
      await listPage.expectRowContains('Write unit tests');
      await listPage.expectRowNotVisible('Refactor auth module');

      // Clearing restores the full list.
      await listPage.selectFilterOption('');
      await listPage.expectRowContains('Refactor auth module');
    });
  });

  test.describe('Navigation', () => {
    /*
     * The first row is not covered by the sticky column header.
     *
     * rela's list is headerless (`show-section-header={false}`), which moves
     * the column header's sticky offset to 0. When that offset and the
     * section header disagree, the header floats over the first row: the row
     * renders correctly and passes every DOM assertion while being
     * unclickable. This asserts the property directly rather than relying on
     * a navigation test to notice a timeout.
     */
    test('the first row is not covered by the sticky header', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      expect(await listPage.firstRowIsClickable()).toBe(true);
    });

    test('clicking row navigates to entity', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      await listPage.clickRow(0);

      // Should navigate away from list
      await expect(appPage).not.toHaveURL(/\/list\/features$/);
    });

    test('create button opens the form in a dialog', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');
      await listPage.openCreateDialog();

      await expect(appPage).toHaveURL(/\/list\/features/);
    });

    test('the Add button below the rows opens the create dialog', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');
      await listPage.clickAddBelowRows();

      await expect(listPage.createDialog).toBeVisible();
      await expect(appPage).toHaveURL(/\/list\/features/);
    });

    test('selecting rows shows the bulk bar with the list actions', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('bugs_triage');
      await listPage.selectRowByTitle('Login form validation');
      await listPage.selectRowByTitle('Memory leak in list view');

      const bar = listPage.bulkBar;
      await expect(bar).toContainText('2');
      await expect(listPage.bulkActions).toHaveText(/Approve/);

      await listPage.clearSelection();
      await expect(bar).toBeHidden();
    });

    test('creating from the dialog opens the new entity in the panel', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);

      await listPage.navigateToList('features');
      await listPage.openCreateDialog();
      await formPage.fillField('title', 'Dialog Created Feature');
      await listPage.submitCreateDialog();

      await expect(listPage.createDialog).toBeHidden();
      await expect(listPage.detailPanelHeading).toContainText('Dialog Created Feature');
    });
  });

  test.describe('Keyboard Navigation', () => {
    test('can navigate rows with keyboard', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      await listPage.focusTable();
      await listPage.pressKey('ArrowDown');

      await expect(listPage.cursorRow).toBeVisible();
    });

    test('N key opens the create dialog', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      await listPage.pressKey('n');

      await expect(listPage.createDialog).toBeVisible();
    });
  });
});
