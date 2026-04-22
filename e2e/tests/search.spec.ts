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
      await searchPage.expectNoActiveFilters();
    });

    test('can clear all filters', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.addFilter('Entity Type', 'bug');

      await searchPage.clearAllFilters();
      await searchPage.expectNoActiveFilters();
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
      await searchPage.expectFirstResultHasTypeBadge();
    });

    test('results show entity ID', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();
      await searchPage.search('Authentication');
      await searchPage.expectFirstResultIdContains('FEAT-');
    });
  });

  test.describe('Keyboard Navigation', () => {
    test('can navigate results with arrow keys', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.searchAndEnter('User');

      // Enters results mode and selects the first result
      await searchPage.focusFirstResult();
    });

    test('can open result with Enter', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();

      await searchPage.searchAndEnter('Authentication');

      // Enters results mode and selects the first result
      await searchPage.focusFirstResult();

      await searchPage.openSelectedResult();

      await expect(appPage).not.toHaveURL(/\/search/);
    });

    test('F key opens filter menu', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearch();
      await searchPage.pressFilterHotkey();

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

    test('can initialize from URL query', async ({ appPage }) => {
      const searchPage = new SearchPage(appPage);

      await searchPage.navigateToSearchWithQuery('Authentication');

      // Results should be shown
      await searchPage.expectResultContains('Authentication');
    });
  });
});

test.describe('Search (create → index → query integration)', () => {
  // Kept as an e2e test because it exercises the create→index→search pipeline
  // end-to-end. Pure shape/filter tests live in Go handler unit tests on
  // internal/dataentry/api_v1.go (handleV1Search).
  type SearchHit = { id: string; type: string; properties: Record<string, unknown> };
  interface SearchResultsEnvelope {
    data?: SearchHit[];
  }

  test('newly-created entity appears in search results with properties intact', async ({ api }) => {
    const created = await api.createEntity('features', {
      properties: { title: 'API Search Test Feature', status: 'draft', priority: 'medium' },
    });
    try {
      const resp = await api.rawRequest('GET', '_search?q=API Search Test');
      const body = (await resp.json()) as SearchResultsEnvelope;
      const results = body.data ?? [];
      const found = results.find((r) => r.id === created.id);
      expect(found).toBeTruthy();
      expect(found?.properties.title).toBe('API Search Test Feature');
    } finally {
      await api.deleteEntity('features', created.id).catch(() => {});
    }
  });
});
