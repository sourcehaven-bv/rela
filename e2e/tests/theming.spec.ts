import { test, expect } from './fixtures';
import { AppShellPage, DashboardPage } from '../pages';

/**
 * The theme picker is the library's three-way control (System / Light / Dark),
 * so these name the state they want rather than flipping an implied toggle.
 * "Click it twice and expect to be back" cannot express `system` at all, and
 * it passed against a control that ignored every second click.
 */
test.describe('Theme picker', () => {
  test('choosing Dark then Light switches the theme both ways', async ({ appPage }) => {
    // Navigate to dashboard and wait for the schema to load — the picker's
    // render depends on schemaStore.darkDisabled being resolved.
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await expect(shell.themeToggle).toBeVisible();

    await shell.chooseTheme('dark');
    expect(await shell.isDarkMode()).toBe(true);

    await shell.chooseTheme('light');
    expect(await shell.isDarkMode()).toBe(false);
  });

  test('dark mode applies the .dark class on documentElement', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await expect(shell.themeToggle).toBeVisible();

    await shell.chooseTheme('dark');
    expect(await shell.isDarkMode()).toBe(true);
  });

  test('System writes neither class, so the stylesheet resolves the OS itself', async ({
    appPage,
  }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();

    const shell = new AppShellPage(appPage);
    await expect(shell.themeToggle).toBeVisible();

    // Pin a choice first, so returning to System has something to clear.
    await shell.chooseTheme('dark');
    await shell.chooseTheme('system');

    const classes = await shell.themeClasses();
    expect(classes).not.toContain('dark');
    expect(classes).not.toContain('light');
  });
});
