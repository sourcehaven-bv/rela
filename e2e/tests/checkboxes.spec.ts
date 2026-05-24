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

  test('clicking a checkbox persists the toggle on the server', async ({ appPage, api }) => {
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

    // ...and the SPA's rendered state must visibly reflect the new value.
    // Server-state alone passing would be the exact failure shape of the
    // original bug (API works, UI doesn't); assert end-to-end.
    await expect.poll(() => entity.contentCheckboxIsChecked(0), { timeout: 2000 }).toBe(true);
  });

  test('toggling a checkbox does not flicker the entity detail tree', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', SEED.features.checkboxBody);
    expect(await entity.contentCheckboxCount()).toBeGreaterThanOrEqual(2);

    // The pre-TKT-R7Q9 toggle path called loadView() after every click,
    // which flipped loading.value=true → v-if="loading" branch → entire
    // entity-detail tree torn down and rebuilt (visible flicker). The
    // PATCH-based reactive flow mutates only viewData.entry.content +
    // the entry-content section, so the EntityDetail's .loading-state
    // spinner must never appear during a toggle.
    //
    // Install a MutationObserver inside the page BEFORE the click so a
    // sub-frame loading flip can't slip between polls. Scoped to
    // `.entity-detail > .loading-state` (direct child) so we don't catch
    // unrelated DocumentsPanel / SidePanel / HelpModal spinners that share
    // the same CSS class.
    await appPage.evaluate(() => {
      const w = window as unknown as { __entityDetailLoadingSeen?: boolean };
      w.__entityDetailLoadingSeen = false;
      const observer = new MutationObserver(() => {
        if (document.querySelector('.entity-detail > .loading-state')) {
          w.__entityDetailLoadingSeen = true;
        }
      });
      observer.observe(document.body, { childList: true, subtree: true });
      // Also capture state at install time (in case it's already showing).
      if (document.querySelector('.entity-detail > .loading-state')) {
        w.__entityDetailLoadingSeen = true;
      }
    });

    await entity.clickContentCheckbox(0);
    await expect
      .poll(() => entity.contentCheckboxIsChecked(0), { timeout: 2000 })
      .toBe(true);

    const loadingSeen = await appPage.evaluate(
      () => (window as unknown as { __entityDetailLoadingSeen?: boolean }).__entityDetailLoadingSeen ?? false,
    );
    expect(loadingSeen, 'entity-detail loading spinner appeared during checkbox toggle').toBe(false);
  });
});
