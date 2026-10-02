import { test, expect } from './fixtures';
import { SEED } from './fixtures';
import { ListPage } from '../pages/list.page';

/**
 * The detail panel beside a list.
 *
 * A panel is not the canonical item view: `/entity/:type/:id` stays the
 * address of the entity, and `?selected=` addresses a row of THIS list. The
 * tests below are mostly about that distinction holding — that the param is
 * the only state, so a link carries the open panel, and that the ways out of
 * the panel land where they claim to.
 */
test.describe('Detail panel', () => {
  test('a row click opens the panel without leaving the list', async ({ appPage }) => {
    const listPage = new ListPage(appPage);

    await listPage.navigateToList('features');
    await listPage.openPanelForRow(SEED.features.authentication);

    // Still the list, now carrying the row in its query.
    await expect(appPage).toHaveURL(/\/list\/features\?.*selected=FEAT-001/);
    await listPage.expectPanelOnRow(SEED.features.authentication);
  });

  test('a shared link opens the panel on load', async ({ appPage }) => {
    const listPage = new ListPage(appPage);

    // The whole point of putting it in the URL: paste, and the panel is open.
    await listPage.navigateToList('features', `selected=${SEED.features.exportData}`);

    await expect(listPage.detailPanel).toBeVisible();
    await listPage.expectPanelOnRow(SEED.features.exportData);
  });

  test('closing the panel clears the param', async ({ appPage }) => {
    const listPage = new ListPage(appPage);

    await listPage.navigateToList('features', `selected=${SEED.features.authentication}`);
    await listPage.closePanel();

    await listPage.expectPanelClosed();
    await expect(appPage).not.toHaveURL(/selected=/);
  });

  test('expand leaves for the entity page', async ({ appPage }) => {
    const listPage = new ListPage(appPage);

    await listPage.navigateToList('features', `selected=${SEED.features.authentication}`);
    await listPage.expandPanel();

    // The canonical item view, not the list with a param.
    await expect(appPage).toHaveURL(/\/entity\/feature\/FEAT-001/);
  });

  test('a stale link leaves the list usable', async ({ appPage }) => {
    const listPage = new ListPage(appPage);

    // A link to a row this list does not hold: the list still renders and the
    // param is simply inert, rather than reporting an error nobody can act on.
    await listPage.navigateToList('features', 'selected=FEAT-DOES-NOT-EXIST');

    await expect(listPage.table).toBeVisible();
    await listPage.expectPanelClosed();
  });

  test('the keyboard cursor moves independently of the open panel', async ({ appPage }) => {
    const listPage = new ListPage(appPage);

    // The reason the cursor needs a marker of its own. Stepping down the list
    // must not drag the panel along, or `Enter` loses its meaning.
    await listPage.navigateToList('features', `selected=${SEED.features.authentication}`);

    await listPage.focusTable();
    await listPage.pressKey('ArrowDown');
    await listPage.pressKey('ArrowDown');

    await listPage.expectPanelOnRow(SEED.features.authentication);
    await expect(listPage.cursorRow).toBeVisible();
    await listPage.expectCursorOnRow(SEED.features.dashboardAnalytics);
  });
});
