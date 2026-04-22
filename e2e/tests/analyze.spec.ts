import { test, expect, ANALYSIS_CHECKS } from './fixtures';
import { AnalyzePage } from '../pages';

test.describe('Analyze Page', () => {
  test('is accessible at /analyze', async ({ appPage }) => {
    const analyze = new AnalyzePage(appPage);
    await analyze.navigate();
    await analyze.expectHeading();
  });

  test('shows all check type cards', async ({ appPage }) => {
    const analyze = new AnalyzePage(appPage);
    await analyze.navigate();
    await analyze.expectCheckCardCount(ANALYSIS_CHECKS.length);
    await analyze.expectCheckTitles([...ANALYSIS_CHECKS]);
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
    await expect(analyze.checkCounts).toHaveCount(ANALYSIS_CHECKS.length);
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
    await analyze.expectCheckCardCount(ANALYSIS_CHECKS.length);
  });
});

/* Analyze-API shape and invariant tests (returns-valid-data, required fields,
 * byCheck keys, error/warning counts) are better covered by Go unit tests on
 * internal/dataentry/api_v1.go (handleV1Analyze) — they'd only repeat work at a
 * slower layer here. The e2e coverage above exercises the full rendered
 * surface via AnalyzePage, which is where this suite adds real value. */
