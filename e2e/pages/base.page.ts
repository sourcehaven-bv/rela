import { type Page, type Locator, expect } from '@playwright/test';

export class BasePage {
  readonly page: Page;
  readonly sidebar: Locator;
  readonly toastContainer: Locator;

  constructor(page: Page) {
    this.page = page;
    this.sidebar = page.locator('.sidebar, nav');
    this.toastContainer = page.locator('.toast, [role="alert"]');
  }

  async navigateTo(path: string) {
    // SPA routes are served at the root path.
    const currentUrl = this.page.url();
    const baseUrl = new URL(currentUrl).origin;
    const fullPath = path.startsWith('/') ? path : `/${path}`;
    await this.page.goto(`${baseUrl}${fullPath}`);
    await this.page.waitForLoadState('domcontentloaded');
  }

  async clickNavLink(name: string) {
    await this.page.getByRole('link', { name }).click();
    await this.page.waitForLoadState('domcontentloaded');
  }

  async waitForToast(message?: string) {
    if (message) {
      await expect(this.page.getByText(message)).toBeVisible({ timeout: 3000 });
    } else {
      await expect(this.toastContainer.first()).toBeVisible({ timeout: 3000 });
    }
  }

  async expectHeading(text: string) {
    await expect(this.page.locator('h1, h2').filter({ hasText: text }).first()).toBeVisible();
  }

  async expectUrl(pattern: RegExp | string) {
    if (typeof pattern === 'string') {
      await expect(this.page).toHaveURL(new RegExp(pattern));
    } else {
      await expect(this.page).toHaveURL(pattern);
    }
  }

  async waitForSpinnerToDisappear() {
    const spinner = this.page.locator('.spinner, .loading-state');
    if (await spinner.isVisible({ timeout: 100 }).catch(() => false)) {
      await expect(spinner).not.toBeVisible({ timeout: 3000 });
    }
  }

  async confirmDialog() {
    this.page.once('dialog', dialog => dialog.accept());
  }

  async dismissDialog() {
    this.page.once('dialog', dialog => dialog.dismiss());
  }
}
