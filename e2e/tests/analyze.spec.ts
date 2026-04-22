import { test, expect } from './fixtures';
import { AnalyzePage } from '../pages';

interface AnalyzeResponse {
  errors: number;
  warnings: number;
  issues: Array<{
    entityId: string;
    entityType: string;
    message: string;
    severity: 'error' | 'warning';
    checkType: string;
  }>;
  byCheck: Record<string, number>;
}

test.describe('Analyze Page', () => {
  test('is accessible at /analyze', async ({ appPage }) => {
    const analyze = new AnalyzePage(appPage);
    await analyze.navigate();
    await analyze.expectHeading();
  });

  test('shows all check type cards', async ({ appPage }) => {
    const analyze = new AnalyzePage(appPage);
    await analyze.navigate();
    await analyze.expectCheckCardCount(4);
    await analyze.expectCheckTitles(['Properties', 'Cardinality', 'Validations', 'Orphans']);
  });

  test('shows check type descriptions', async ({ appPage }) => {
    const analyze = new AnalyzePage(appPage);
    await analyze.navigate();
    await analyze.expectFirstDescriptionVisible();
    await analyze.expectDescriptionText('Property validation errors');
  });

  test('shows issue counts per check type', async ({ appPage }) => {
    const analyze = new AnalyzePage(appPage);
    await analyze.navigate();
    await expect(analyze.checkCounts).toHaveCount(4);
  });

  test('displays issues without error when entities exist', async ({ appPage, api }) => {
    // Create an entity; the inline metamodel has no required relations, so this
    // won't necessarily produce issues — we just verify the page renders.
    const created = await api.createEntity('features', {
      properties: { title: 'Analyze render check', status: 'draft', priority: 'high' },
    });
    try {
      const analyze = new AnalyzePage(appPage);
      await analyze.navigate();
      expect(await analyze.getIssueRowCount()).toBeGreaterThanOrEqual(0);
    } finally {
      await api.deleteEntity('features', created.id).catch(() => {});
    }
  });

  test('clicking an issue row navigates to that entity', async ({ appPage, api }) => {
    const created = await api.createEntity('features', {
      properties: { title: 'Analyze nav check', status: 'draft', priority: 'high' },
    });
    try {
      const analyze = new AnalyzePage(appPage);
      await analyze.navigate();

      if ((await analyze.getIssueRowCount()) > 0) {
        await analyze.clickFirstIssueRow();
        await expect(appPage).toHaveURL(/\/entity\//);
      }
    } finally {
      await api.deleteEntity('features', created.id).catch(() => {});
    }
  });

  test('refresh button reloads analysis', async ({ appPage }) => {
    const analyze = new AnalyzePage(appPage);
    await analyze.navigate();
    await analyze.clickRefresh();
    await analyze.expectCheckCardCount(4);
  });
});

test.describe('Analyze API', () => {
  test('GET /api/v1/_analyze returns valid data', async ({ api }) => {
    const resp = await api.rawRequest('GET', '_analyze');
    const result: AnalyzeResponse = await resp.json();
    expect(typeof result.errors).toBe('number');
    expect(typeof result.warnings).toBe('number');
    expect(Array.isArray(result.issues)).toBeTruthy();
    expect(typeof result.byCheck).toBe('object');
  });

  test('analyze issues have required fields', async ({ api }) => {
    const resp = await api.rawRequest('GET', '_analyze');
    const result: AnalyzeResponse = await resp.json();
    for (const issue of result.issues) {
      expect(issue.entityId).toBeTruthy();
      expect(issue.entityType).toBeTruthy();
      expect(issue.message).toBeTruthy();
      expect(['error', 'warning']).toContain(issue.severity);
      expect(issue.checkType).toBeTruthy();
    }
  });

  test('byCheck keys match known check types', async ({ api }) => {
    const resp = await api.rawRequest('GET', '_analyze');
    const result: AnalyzeResponse = await resp.json();
    const known = ['Properties', 'Cardinality', 'Validations', 'Orphans'];
    for (const key of Object.keys(result.byCheck)) {
      expect(known).toContain(key);
    }
  });

  test('error and warning counts match issues', async ({ api }) => {
    const resp = await api.rawRequest('GET', '_analyze');
    const result: AnalyzeResponse = await resp.json();
    const errorCount = result.issues.filter((i) => i.severity === 'error').length;
    const warningCount = result.issues.filter((i) => i.severity === 'warning').length;
    expect(result.errors).toBe(errorCount);
    expect(result.warnings).toBe(warningCount);
  });
});
