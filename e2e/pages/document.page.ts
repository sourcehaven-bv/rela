import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class DocumentPage extends BasePage {
  readonly body: Locator;
  readonly title: Locator;
  readonly headerRight: Locator;
  readonly refreshButton: Locator;

  constructor(page: Page) {
    super(page);
    this.body = page.locator('.document-body');
    this.title = page.locator('.page-header h1');
    this.headerRight = page.locator('.page-header .header-right');
    this.refreshButton = this.headerRight.getByRole('button', { name: 'Refresh' });
  }

  async navigateToDocument(name: string, entityId: string) {
    await this.navigateTo(`/document/${name}/${entityId}`);
    await expect(this.body).toBeVisible({ timeout: 15000 });
  }

  /** Wait for the standalone document URL after a navigation that lands here. */
  async expectAtDocumentUrl(name: string, entityId: string) {
    await this.page.waitForURL(new RegExp(`/document/${name}/${entityId}`), { timeout: 10000 });
    await expect(this.body).toBeVisible({ timeout: 15000 });
  }

  // Scoped to .header-right so a configured label that collides with another
  // button on the page (e.g. "Refresh") still resolves to the Edit button.
  editButton(label: string): Locator {
    return this.headerRight.getByRole('button', { name: label });
  }
}
