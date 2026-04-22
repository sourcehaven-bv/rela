import { test, expect } from './fixtures';
import { AppShellPage, DashboardPage } from '../pages';

test.describe('Dark Mode Toggle', () => {
  test('theme toggle switches dark mode and back', async ({ appPage }) => {
    // Navigate to dashboard and wait for the schema to load — the toggle's
    // render depends on schemaStore.darkDisabled being resolved.
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await expect(shell.themeToggle).toBeVisible();

    const initial = await shell.isDarkMode();
    await shell.clickThemeToggle();
    expect(await shell.isDarkMode()).toBe(!initial);

    await shell.clickThemeToggle();
    expect(await shell.isDarkMode()).toBe(initial);
  });

  test('dark mode applies the .dark class on documentElement', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await expect(shell.themeToggle).toBeVisible();
    // Normalise to light first.
    if (await shell.isDarkMode()) {
      await shell.clickThemeToggle();
    }
    await shell.clickThemeToggle();
    expect(await shell.isDarkMode()).toBe(true);
  });
});
