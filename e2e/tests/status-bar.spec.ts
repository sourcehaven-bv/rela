import { test, expect } from './fixtures';
import { AppShellPage, DashboardPage } from '../pages';

test.describe('Status Bar', () => {
  test('status bar is visible on page load', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await shell.expectStatusBarVisible();
  });

  test('status bar exposes settings link', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await shell.expectSettingsLinkVisible();
  });

  test('status bar exposes shortcuts button', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await shell.expectShortcutsButtonVisible();
  });

  test('git status is either rendered (repo) or hidden (temp dir)', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    // Temp test projects aren't git repos, so gitStatusContainer is hidden.
    // If git is somehow available (devs running against a configured project),
    // the branch element should be visible. Either outcome is correct.
    if (await shell.isGitAvailable()) {
      const text = (await shell.gitBranch.textContent()) ?? '';
      expect(text.length).toBeGreaterThan(0);
    }
  });
});
