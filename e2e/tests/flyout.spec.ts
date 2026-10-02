import { test, expect } from './fixtures';
import { SEED } from './fixtures';
import { FlyoutPage } from '../pages/flyout.page';

/**
 * A sidebar entry with `open: flyout` slides its list out over the page
 * instead of navigating, for a quick look. The fixture's "Triage" entry
 * opens the `bugs` list that way.
 */
test.describe('Sidebar flyout', () => {
  test('opens the list over the page without navigating', async ({ appPage }) => {
    const flyout = new FlyoutPage(appPage);
    await flyout.clickSidebarLink('Features');

    await flyout.open('Triage');

    await expect(appPage).toHaveURL(/\/list\/features$/);
    await expect(flyout.entry('Triage')).toHaveAttribute('aria-expanded', 'true');
    await expect(flyout.row('Triage', SEED.bugs.loginFormValidation)).toBeVisible();
  });

  test('a second click on the entry puts it away', async ({ appPage }) => {
    const flyout = new FlyoutPage(appPage);

    await flyout.open('Triage');
    await flyout.entry('Triage').click();

    await flyout.expectClosed();
    await expect(flyout.entry('Triage')).toHaveAttribute('aria-expanded', 'false');
  });

  test('a row opens its detail beside the list, and Escape closes one level', async ({ appPage }) => {
    const flyout = new FlyoutPage(appPage);

    await flyout.open('Triage');
    await flyout.openRow('Triage', SEED.bugs.loginFormValidation);
    await expect(flyout.panels).toHaveCount(2);

    await appPage.keyboard.press('Escape');
    await expect(flyout.panels).toHaveCount(1);
    await appPage.keyboard.press('Escape');
    await flyout.expectClosed();
  });

  test('navigating elsewhere closes it', async ({ appPage }) => {
    const flyout = new FlyoutPage(appPage);

    await flyout.open('Triage');
    await flyout.clickSidebarLink('Features');

    await flyout.expectClosed();
  });

  test('expand goes to the full list and closes the flyout', async ({ appPage }) => {
    const flyout = new FlyoutPage(appPage);

    await flyout.open('Triage');
    await flyout.expand('Triage');

    await expect(appPage).toHaveURL(/\/list\/bugs$/);
    await flyout.expectClosed();
  });

  test('expand on a row goes to its page', async ({ appPage }) => {
    const flyout = new FlyoutPage(appPage);

    await flyout.open('Triage');
    await flyout.openRow('Triage', SEED.bugs.loginFormValidation);
    await flyout.expandDetail();

    await expect(appPage).toHaveURL(new RegExp(`/${SEED.bugs.loginFormValidation}`));
    await flyout.expectClosed();
  });
});
