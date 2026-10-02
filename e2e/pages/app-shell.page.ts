import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/** App-shell concerns that are cross-cutting: the sidebar's chrome footer,
 *  theme picker, keyboard shortcut modal, git status. Not scoped to any
 *  single view. */
export class AppShellPage extends BasePage {
  readonly chromeFooter: Locator;
  readonly themeToggle: Locator;
  readonly gitStatusContainer: Locator;
  readonly gitBranch: Locator;
  readonly shortcutsOverlay: Locator;

  constructor(page: Page) {
    super(page);
    // The app chrome used to be a bar pinned across the bottom of the
    // viewport; it is the sidebar's footer band now. Named for what it holds
    // rather than where it sits, so a later move does not rename this again.
    this.chromeFooter = page.locator('.sidebar-footer');
    // The picker is the library's three-way control (System / Light / Dark),
    // not rela's old two-state button.
    this.themeToggle = page.locator('.rl-theme-toggle');
    this.gitStatusContainer = page.locator('.git-status');
    this.gitBranch = page.locator('.git-branch');
    // The shortcuts dialog is an RlModal now, so it has neither of the two
    // classes this used to look for. Matched by its accessible name, which is
    // the part a user actually perceives.
    this.shortcutsOverlay = page.getByRole('dialog', { name: 'Keyboard Shortcuts' });
  }

  /**
   * Keyboard shortcut hints (RlKbd), which trail the controls they belong to.
   *
   * The key chrome is the library's, so these assert that a hint is present
   * and readable rather than how it is drawn — a restyle in rela-components
   * must not fail this.
   */
  get shortcutHints(): Locator {
    return this.page.locator('.rl-kbd');
  }

  /** A shortcut hint's accessible name, which is what a screen reader says.
   *  RlKbd builds it from the keys and their separator, so a combination
   *  reads as separate tokens rather than as one unpronounceable string. */
  async shortcutHintNames(): Promise<string[]> {
    return this.shortcutHints.evaluateAll((els) =>
      els.map((el) => el.getAttribute('aria-label') ?? ''),
    );
  }

  /** The account menu's trigger, which names the principal (TKT-MJTD12). */
  get accountMenu(): Locator {
    return this.page.getByTestId('account-menu');
  }

  /** An entry in the open account menu. */
  accountMenuItem(name: string): Locator {
    return this.page.getByRole('menuitem', { name });
  }

  /** The keyboard-operable handle between the sidebar and the main pane. */
  get sidebarResizeHandle(): Locator {
    return this.page.getByRole('separator', { name: 'Resize sidebar' });
  }

  /** The sidebar's rendered width, rounded to whole pixels. */
  async sidebarWidth(): Promise<number> {
    const box = await this.page.locator('.rl-sidebar').boundingBox();
    if (!box) throw new Error('sidebar has no bounding box');
    return Math.round(box.width);
  }

  async isDarkMode(): Promise<boolean> {
    return this.page.evaluate(() => document.documentElement.classList.contains('dark'));
  }

  /** The theme classes written to the document root. `system` is the absence
   *  of both, which only a list can express — `isDarkMode` alone reads a
   *  `light` pin and a cleared choice identically. */
  async themeClasses(): Promise<string[]> {
    return this.page.evaluate(() => Array.from(document.documentElement.classList));
  }

  /**
   * Chooses a theme explicitly.
   *
   * The control is a three-way radiogroup, so there is no "the other one" to
   * flip to — a caller has to say which state it wants. `system` is the
   * absence of both classes, which is why it is a valid choice here and not
   * just an initial value.
   */
  async chooseTheme(choice: 'system' | 'light' | 'dark') {
    await this.themeToggle.locator(`[data-value="${choice}"]`).click();
  }

  /** Blur focus so keyboard shortcuts land on document, not an input. */
  async blurFocus() {
    await this.page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  }

  async pressKey(key: string) {
    await this.page.keyboard.press(key);
  }

  async expectChromeFooterVisible() {
    await expect(this.chromeFooter).toBeVisible();
  }

  /** Settings is a button in the footer, not a link — RlSidebarFooterLink
   *  renders a <button> and routes on click. Matched by role so the assertion
   *  survives the library changing the element. */
  async expectSettingsControlVisible() {
    await expect(this.chromeFooter.getByRole('button', { name: 'Settings' })).toBeVisible();
  }

  async expectShortcutsButtonVisible() {
    await expect(this.chromeFooter.getByRole('button', { name: /Shortcuts/ })).toBeVisible();
  }

  async dispatchGlobalKey(key: string) {
    // Certain shortcuts (e.g. ?) need a synthesized keyboard event on document.
    await this.page.evaluate((k: string) => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }));
    }, key);
  }

  async expectShortcutsOverlayVisible() {
    await expect(this.shortcutsOverlay).toBeVisible();
  }

  async isGitAvailable(): Promise<boolean> {
    return this.gitStatusContainer.isVisible().catch(() => false);
  }
}
