import { test } from './fixtures';
import { ConflictsPage } from '../pages';

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

/* Conflicts-API shape tests (response envelope, empty-list-for-clean-project)
 * belong in Go handler unit tests on internal/dataentry — they repeat work at a
 * slower layer here. The Page tests above exercise the rendered surface. */
