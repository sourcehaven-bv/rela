import { test, expect } from './fixtures';
import { SEED } from './fixtures';
import { ListPage } from '../pages/list.page';

/**
 * The page header band.
 *
 * The header is rendered by the app shell rather than by the view, which is
 * what lets it span the list AND the detail panel beside it. Left in the view
 * it would sit inside the content pane and stop at the panel's left edge, so
 * opening a panel would cut the page title in half.
 *
 * These are geometry assertions on purpose: a unit test cannot distinguish a
 * header that spans both panes from one that does not, because happy-dom has
 * no layout engine and reports the same box for either.
 */
test.describe('Page header', () => {
  test('carries the list title, its search box and its create action', async ({ appPage }) => {
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features');

    // All three used to live in the view's own header. They are in the band
    // now, and still driven by the view's state.
    await expect(listPage.headerTitle).toHaveText(/feature/i);
    await expect(listPage.headerSearch).toBeVisible();
    await expect(listPage.headerCreateLink).toBeVisible();
  });

  test('the search box in the band still filters the list', async ({ appPage }) => {
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features');

    // The control moved two levels up the tree; the wiring has to survive the
    // move, so assert the filtering, not just that the box renders.
    await listPage.headerSearch.fill('authentication');
    await expect(appPage).toHaveURL(/[?&]q=authentication/);
    await expect(listPage.rowById(SEED.features.authentication)).toBeVisible();
  });

  test('spans the panel too, so opening one does not narrow it', async ({ appPage }) => {
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features');

    const widthWithoutPanel = await listPage.widthOf(listPage.header);

    await listPage.openPanelForRow(SEED.features.authentication);

    // The guarantee: the band keeps its full width and reaches past the
    // panel's left edge, rather than shrinking to the content pane.
    const widthWithPanel = await listPage.widthOf(listPage.header);
    expect(Math.abs(widthWithPanel - widthWithoutPanel)).toBeLessThan(2);

    const headerRight = await listPage.rightEdgeOf(listPage.header);
    const panelRight = await listPage.rightEdgeOf(listPage.panelPane);
    expect(headerRight).toBeGreaterThanOrEqual(panelRight - 2);
  });

  test('sits above the panel, not beside it', async ({ appPage }) => {
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features', `selected=${SEED.features.authentication}`);
    await expect(listPage.detailPanel).toBeVisible();

    const header = await listPage.header.boundingBox();
    const panel = await listPage.panelPane.boundingBox();
    if (!header || !panel) throw new Error('missing box');

    // The band's bottom edge is above the panel's top edge. Asserted because
    // "full width" alone would also hold for a header overlapping the panel.
    expect(header.y + header.height).toBeLessThanOrEqual(panel.y + 2);
  });

  test('a screen with no hoisted header renders no empty band', async ({ appPage }) => {
    const listPage = new ListPage(appPage);

    // Settings has not been migrated, so it still draws its own header. The
    // band must be absent rather than present and blank, and the previous
    // screen's title must not be stranded above it.
    await listPage.navigateToList('features');
    await expect(listPage.headerTitle).toHaveText(/feature/i);

    await listPage.navigateTo('/settings');
    await expect(listPage.header).toHaveCount(0);
  });
});
