import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/** App-shell concerns that are cross-cutting: status bar, theme toggle,
 *  keyboard shortcut modal, git status. Not scoped to any single view. */
export class AppShellPage extends BasePage {
  readonly statusBar: Locator;
  readonly themeToggle: Locator;
  readonly gitStatusContainer: Locator;
  readonly gitBranch: Locator;
  readonly shortcutsOverlay: Locator;

  constructor(page: Page) {
    super(page);
    this.statusBar = page.locator('.status-bar');
    this.themeToggle = page.locator('.theme-toggle');
    this.gitStatusContainer = page.locator('.git-status');
    this.gitBranch = page.locator('.git-branch');
    this.shortcutsOverlay = page.locator('.shortcuts-overlay, .shortcuts-modal').first();
  }

  async isDarkMode(): Promise<boolean> {
    return this.page.evaluate(() => document.documentElement.classList.contains('dark'));
  }

  async clickThemeToggle() {
    await this.themeToggle.click();
  }

  /** Blur focus so keyboard shortcuts land on document, not an input. */
  async blurFocus() {
    await this.page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  }

  async pressKey(key: string) {
    await this.page.keyboard.press(key);
  }

  async expectStatusBarVisible() {
    await expect(this.statusBar).toBeVisible();
  }

  async expectSettingsLinkVisible() {
    await expect(this.statusBar.locator('a', { hasText: 'Settings' })).toBeVisible();
  }

  async expectShortcutsButtonVisible() {
    await expect(this.statusBar.locator('button', { hasText: 'Shortcuts' })).toBeVisible();
  }

  async dispatchGlobalKey(key: string) {
    // Certain shortcuts (e.g. ?) need a synthesized keyboard event on document.
    await this.page.evaluate((k: string) => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }));
    }, key);
  }

  async expectShortcutsOverlayVisible() {
    await expect(this.shortcutsOverlay).toBeVisible({ timeout: 5000 });
  }

  async isGitAvailable(): Promise<boolean> {
    return this.gitStatusContainer.isVisible().catch(() => false);
  }
}
