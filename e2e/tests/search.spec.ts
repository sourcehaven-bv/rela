import { test, expect } from './fixtures';
import { SearchPage } from '../pages/search.page';

test.describe('Search', () => {
  test.describe('Basic Search', () => {
    test('search page loads', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();
      await searchPage.expectHeading('Search');

      await expect(searchPage.searchInput).toBeVisible();
      await expect(searchPage.searchButton).toBeVisible();
    });

    test('can search for entities by title', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.search('Authentication');

      // Should find FEAT-001
      await searchPage.expectResultContains('Authentication');
      await searchPage.expectResultContains('FEAT-001');
    });

    test('can search with Enter key', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.searchAndEnter('Dashboard');

      await searchPage.expectResultContains('Dashboard');
    });

    test('shows no results message', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.search('xyznonexistent123');

      await searchPage.expectNoResults();
    });

    test('shows result count', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.search('User');

      await expect(searchPage.resultsCount).toBeVisible();
    });
  });

  test.describe('Filtered Search', () => {
    test('can open filter menu', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.openFilterMenu();

      await expect(searchPage.filterMenu).toBeVisible();
    });

    test('can filter by entity type', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      // Add type filter
      await searchPage.addFilter('Entity Type', 'feature');

      // Should show filter chip
      await searchPage.expectFilterActive('Entity Type');

      // Search for a common term that exists in features
      await searchPage.search('User');

      // The matching feature should appear in results
      await searchPage.expectResultContains('User Authentication');
      const resultCount = await searchPage.getResultCount();
      expect(resultCount).toBeGreaterThan(0);
    });

    test('can combine filters', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      // Add type filter first
      await searchPage.addFilter('Entity Type', 'feature');

      // Search for text
      await searchPage.search('User');

      // Should find matching features
      await searchPage.expectResultContains('User');
    });

    test('can remove individual filter', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.addFilter('Entity Type', 'feature');
      await searchPage.expectFilterActive('Entity Type');

      await searchPage.removeFilter('Entity Type');

      // Filter should be removed
      await expect(searchPage.activeFilters.locator('.filter-chip')).toHaveCount(0);
    });

    test('can clear all filters', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.addFilter('Entity Type', 'bug');

      await searchPage.clearAllFilters();

      await expect(searchPage.activeFilters.locator('.filter-chip')).toHaveCount(0);
    });
  });

  test.describe('Search Results', () => {
    test('clicking result navigates to entity', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.search('Authentication');

      await searchPage.clickResultById('FEAT-001');

      await expect(appPage).toHaveURL(/\/entity\/feature\/FEAT-001|\/form\/feature\/FEAT-001/);
    });

    test('results show entity type badge', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.search('Authentication');

      const result = searchPage.resultItems.first();
      await expect(result.locator('.result-type')).toBeVisible();
    });

    test('results show entity ID', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.search('Authentication');

      const result = searchPage.resultItems.first();
      await expect(result.locator('.result-id')).toContainText('FEAT-');
    });
  });

  test.describe('Keyboard Navigation', () => {
    test('can navigate results with arrow keys', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.searchAndEnter('User');

      // Focus the input then press ArrowDown to enter results mode
      await searchPage.searchInput.focus();
      await appPage.keyboard.press('ArrowDown');

      // First result should be selected
      await expect(searchPage.resultItems.first()).toHaveClass(/selected/);
    });

    test('can open result with Enter', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.searchAndEnter('Authentication');

      // Focus the input then ArrowDown enters results mode
      await searchPage.searchInput.focus();
      await appPage.keyboard.press('ArrowDown');

      // Press Enter to open
      await appPage.keyboard.press('Enter');

      await expect(appPage).not.toHaveURL(/\/search/);
    });

    test('F key opens filter menu', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      // Click outside input first
      await appPage.locator('body').click();

      await appPage.keyboard.press('f');

      await expect(searchPage.filterMenu).toBeVisible();
    });
  });

  test.describe('URL State', () => {
    test('search query is reflected in URL', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.search('dashboard');

      await expect(appPage).toHaveURL(/q=dashboard/);
    });

    test('can initialize from URL query', async ({ appPage, serverUrl }) => {
      // Navigate directly with query param
      await appPage.goto(`${serverUrl}/search?q=Authentication`);

      const searchPage = new SearchPage(appPage);

      // Results should be shown
      await searchPage.expectResultContains('Authentication');
    });
  });
});
