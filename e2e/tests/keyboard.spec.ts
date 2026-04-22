import { test, expect } from './fixtures';
import { AppShellPage, DashboardPage, SearchPage } from '../pages';

test.describe('Keyboard Shortcuts', () => {
  test('pressing / navigates to the search page', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await shell.blurFocus();
    await shell.pressKey('/');

    await expect(appPage).toHaveURL(/\/search/);
  });

  test('g then d navigates to the dashboard', async ({ appPage }) => {
    const search = new SearchPage(appPage);
    await search.navigateToSearch();

    const shell = new AppShellPage(appPage);
    await shell.blurFocus();
    await shell.pressKey('g');
    await shell.pressKey('d');

    await expect(appPage).toHaveURL(/\/(dashboard|$)/);
  });

  test('? opens the keyboard-shortcuts modal', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await shell.blurFocus();
    await shell.dispatchGlobalKey('?');

    await shell.expectShortcutsOverlayVisible();
  });
});
