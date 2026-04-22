import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/** View at /conflicts — lists files with unresolved git merge conflicts. */
export class ConflictsPage extends BasePage {
  readonly view: Locator;
  readonly header: Locator;
  readonly emptyState: Locator;
  readonly backToDashboard: Locator;

  constructor(page: Page) {
    super(page);
    this.view = page.locator('.conflicts-view');
    this.header = page.locator('.page-header');
    this.emptyState = page.locator('.conflict-empty');
    this.backToDashboard = page.locator('.page-header .btn');
  }

  async navigate() {
    await this.navigateTo('/conflicts');
    await expect(this.view).toBeVisible();
  }

  async expectEmptyStateVisible() {
    await expect(this.emptyState).toBeVisible();
    await expect(this.emptyState.locator('h3')).toHaveText('No conflicts detected');
    await expect(this.emptyState.locator('p')).toHaveText('All entity and relation files are clean.');
  }

  async expectHeaderText() {
    await expect(this.header.locator('h2')).toHaveText('Merge Conflicts');
    await expect(this.header.locator('p')).toHaveText('Files with unresolved git conflicts');
  }

  async expectBackButton() {
    await expect(this.backToDashboard).toBeVisible();
    await expect(this.backToDashboard).toHaveText('Back to Dashboard');
  }
}
