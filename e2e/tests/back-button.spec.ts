import { test, expect } from './fixtures';
import { EntityPage, ListPage } from '../pages';

/**
 * TKT-JIEKC AC7: the Back button renders on non-form screens when the
 * URL carries a valid `?return_to=`, and clicking it lands on the
 * source URL verbatim (including query string / fragment).
 *
 * This spec exercises the full round-trip:
 *   (1) Render a document panel on a feature detail page; the doc links
 *       to a bug detail page.
 *   (2) The server-side rewriter appends `?return_to=<source>` on that
 *       link because it targets a non-form internal route.
 *   (3) Clicking the link lands on the bug detail page; BackButton
 *       renders because `?return_to=` is present.
 *   (4) Clicking Back pushes via vue-router and lands on the original
 *       URL — assertion is on the landed URL, not the attribute value,
 *       so the test pins actual navigation behaviour.
 */

test.describe('Back button: return_to round-trip', () => {
  test('click through doc link and click Back lands on source URL', async ({ appPage }) => {
    const featurePage = new EntityPage(appPage);
    const bugPage = new EntityPage(appPage);

    // Step 1: Visit the feature detail page. The fixture ships a
    // feature-overview document that prints a link to BUG-001.
    await featurePage.navigateToEntity('feature', 'FEAT-001');
    await featurePage.waitForDocumentBody();
    await featurePage.selectDocument('feature-overview');
    await featurePage.expectDocumentBodyContains('FEAT-001');

    // Step 2: Snapshot the source URL (path + query we expect Back to
    // restore). The doc panel may have synced ?doc= into the URL via
    // router.replace; capture whatever is there now as the back target.
    const sourceURL = new URL(appPage.url());

    // Step 3: Click the bug link inside the doc body. Server rewriter
    // guarantees the anchor's href carries ?return_to=<sourcePath>.
    await featurePage.clickDocumentLink('BUG-001');
    await appPage.waitForURL(/\/entity\/bug\/BUG-001/);

    // Step 4: Back button is visible on the bug detail page because
    // ?return_to= was carried through the rewriter.
    await bugPage.expectBackButtonVisible();

    // Step 5: Click Back. Assert we land on the original URL path; the
    // ?doc= query survives the round-trip so the panel restores the
    // same document tab the user was viewing.
    await bugPage.clickBack();
    const sourcePathname = sourceURL.pathname;
    await appPage.waitForURL((url) => new URL(url).pathname === sourcePathname);
    await featurePage.expectDocumentBodyContains('FEAT-001');
    const landed = new URL(appPage.url());
    expect(landed.searchParams.get('doc')).toBe('feature-overview');
  });

  test('no Back button when no return_to / from is present', async ({ appPage }) => {
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features');
    await listPage.expectListHeading('Features');
    await listPage.expectNoBackButton();
  });

  test('Back button renders on list view when return_to is set', async ({ appPage }) => {
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features', 'return_to=%2Fdashboard');
    await listPage.expectListHeading('Features');
    await listPage.clickBack();
    await appPage.waitForURL(/\/dashboard/);
  });

  test('unsafe return_to is ignored (open-redirect guard)', async ({ appPage }) => {
    // ?return_to=//evil.com would be an open-redirect payload. The
    // client-side isSafeReturnPath guard rejects it; no Back button
    // should render (and the user isn't sent anywhere hostile).
    const listPage = new ListPage(appPage);
    await listPage.navigateToList(
      'features',
      `return_to=${encodeURIComponent('//evil.com')}`,
    );
    await listPage.expectListHeading('Features');
    await listPage.expectNoBackButton();
  });
});
