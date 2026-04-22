import { test, expect } from './fixtures';
import { ConflictsPage } from '../pages';

interface ConflictsResponse {
  conflicts: Array<{
    path: string;
    entity_type?: string;
    entity_id?: string;
    marker_count: number;
  }>;
  count: number;
}

test.describe('Conflicts Page', () => {
  test('conflicts page is accessible at /conflicts', async ({ appPage }) => {
    const page = new ConflictsPage(appPage);
    await page.navigate();
  });

  test('shows empty state when no conflicts exist', async ({ appPage }) => {
    const page = new ConflictsPage(appPage);
    await page.navigate();
    await page.expectEmptyStateVisible();
  });

  test('shows page header', async ({ appPage }) => {
    const page = new ConflictsPage(appPage);
    await page.navigate();
    await page.expectHeaderText();
  });

  test('back to dashboard button is visible', async ({ appPage }) => {
    const page = new ConflictsPage(appPage);
    await page.navigate();
    await page.expectBackButton();
  });
});

test.describe('Conflicts API', () => {
  test('GET /api/v1/_conflicts returns valid response shape', async ({ api }) => {
    const resp = await api.rawRequest('GET', '_conflicts');
    const result: ConflictsResponse = await resp.json();
    expect(result).toHaveProperty('conflicts');
    expect(result).toHaveProperty('count');
    expect(Array.isArray(result.conflicts)).toBeTruthy();
    expect(result.count).toBe(result.conflicts.length);
  });

  test('GET /api/v1/_conflicts returns empty list for clean project', async ({ api }) => {
    const resp = await api.rawRequest('GET', '_conflicts');
    const result: ConflictsResponse = await resp.json();
    expect(result.conflicts).toHaveLength(0);
    expect(result.count).toBe(0);
  });
});
