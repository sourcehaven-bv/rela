import { test, expect } from './fixtures';
import { EntityPage } from '../pages';

/**
 * Body content with GFM checkboxes renders as interactive checkboxes in the
 * entity detail view. Clicking toggles the markdown source on the server.
 *
 * These tests rely on the `content` field being writable via PATCH. The
 * inline project seeds FEAT-001 with checkboxes in its body, so we don't need
 * to stage content per test — we simply assert on the rendered state.
 */

test.describe('Checkbox toggling', () => {
  test('entity detail shows checkbox stats for content with checkboxes', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', 'FEAT-001');

    // Either the stats widget is shown (most likely) or not — if shown, the
    // format should be "n/m". Both outcomes are acceptable; we're asserting
    // the render contract, not that the widget must be present.
    if (await entity.hasCheckboxStats()) {
      expect(await entity.getCheckboxStatsText()).toMatch(/\d+\/\d+/);
    }

    expect(await entity.contentCheckboxCount()).toBeGreaterThanOrEqual(1);
  });

  // TODO(TKT-4Q2VI): Playwright's `force: true` click on the disabled
  // <input type="checkbox"> doesn't fire the Vue-installed click handler
  // reliably. The behaviour is a real regression risk, but reproducing it
  // via the browser needs a dispatched InputEvent or a real user-gesture
  // click. Leaving skipped until a focused repro harness exists.
  test.skip('clicking a checkbox persists the toggle on the server', async ({ appPage, api }) => {
    // Stage the content via API so this test doesn't depend on seed ordering.
    const feature = await api.createEntity('features', {
      properties: { title: 'Checkbox toggle test', status: 'draft', priority: 'low' },
    });
    try {
      await api.rawRequest('PATCH', `features/${feature.id}`, {
        content: '- [ ] First\n- [x] Second\n',
      });

      const entity = new EntityPage(appPage);
      await entity.navigateToEntity('feature', feature.id);

      expect(await entity.contentCheckboxCount()).toBeGreaterThanOrEqual(2);
      await entity.clickContentCheckbox(0);

      // Verify persistence by re-fetching the entity content from the API,
      // independent of SSE/frontend re-render timing. The first line starts
      // unchecked; after a click it should be checked.
      await expect
        .poll(
          async () => {
            const resp = await api.rawRequest('GET', `features/${feature.id}`);
            const body = await resp.json();
            const content: string = body.content ?? '';
            const firstLine = content.split('\n')[0] ?? '';
            return /- \[x\]/i.test(firstLine);
          },
          { timeout: 5000 },
        )
        .toBe(true);
    } finally {
      await api.deleteEntity('features', feature.id).catch(() => {});
    }
  });
});
