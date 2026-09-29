import { test, expect } from './fixtures';
import { AppShellPage, DashboardPage } from '../pages';

/**
 * The app's chrome band: git status, Settings, About, Shortcuts.
 *
 * It used to be a 24px bar pinned across the bottom of the viewport. It is the
 * sidebar's footer now, which is where the app shell puts chrome — the old bar
 * painted over the shell's own bottom edge, and duplicated the theme picker
 * and Settings link the sidebar already rendered.
 */
test.describe('Chrome footer', () => {
  test('the footer is visible on page load', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await shell.expectChromeFooterVisible();
  });

  test('the footer exposes settings', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await shell.expectSettingsControlVisible();
  });

  test('the footer exposes shortcuts', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await shell.expectShortcutsButtonVisible();
  });

  test('exactly one theme picker is on screen', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    // The defect this pins: the old bar carried its own theme control while
    // RlSidebar rendered the library's, so a user saw two.
    await expect(shell.themeToggle).toHaveCount(1);
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
