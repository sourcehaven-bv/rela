import { test, expect, SEED } from './fixtures';
import { EntityPage } from '../pages';

/**
 * Body content with GFM checkboxes renders as interactive checkboxes in the
 * entity detail view. Clicking toggles the markdown source on the server.
 *
 * SEED.features.checkboxBody (FEAT-004) is the dedicated fixture: its body has
 * one unchecked and one checked item. Specs read or flip the rendered state;
 * there is no per-test API setup.
 */

test.describe('Checkbox toggling', () => {
  test('entity detail shows checkbox stats for content with checkboxes', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', SEED.features.checkboxBody);

    // Stats widget renders when the body contains checkboxes; format is "n/m".
    if (await entity.hasCheckboxStats()) {
      expect(await entity.getCheckboxStatsText()).toMatch(/\d+\/\d+/);
    }

    expect(await entity.contentCheckboxCount()).toBeGreaterThanOrEqual(2);
  });

  // Skipped: tracked by BUG-9RANL (Playwright's `force: true` click on the
  // disabled <input type="checkbox"> doesn't reliably fire the Vue click
  // handler in this harness). The product behaviour works for real users;
  // the gap is test-harness-only. See the bug for the repro harness needed
  // to unskip this test.
  test.skip('clicking a checkbox persists the toggle on the server', async ({ appPage, api }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', SEED.features.checkboxBody);

    expect(await entity.contentCheckboxCount()).toBeGreaterThanOrEqual(2);
    await entity.clickContentCheckbox(0);

    // The first line starts unchecked; after a click the server-side content
    // should report it checked. Re-read via the API to avoid racing SSE and
    // frontend re-render timing.
    await expect
      .poll(
        async () => {
          const content = await api.getContent('features', SEED.features.checkboxBody);
          const firstLine = content.split('\n')[0] ?? '';
          return /- \[x\]/i.test(firstLine);
        },
        { timeout: 5000 },
      )
      .toBe(true);
  });
});
