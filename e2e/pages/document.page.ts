import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class DocumentPage extends BasePage {
  readonly body: Locator;
  readonly headerRight: Locator;

  constructor(page: Page) {
    super(page);
    this.body = page.locator('.document-body');
    this.headerRight = page.locator('.header-right');
  }

  async navigateToDocument(name: string, entityId: string) {
    await this.navigateTo(`/document/${name}/${entityId}`);
    await expect(this.body).toBeVisible({ timeout: 15000 });
  }

  // Scoped to .header-right so a configured label that collides with another
  // button on the page (e.g. "Refresh") still resolves to the Edit button.
  editButton(label: string): Locator {
    return this.headerRight.getByRole('button', { name: label });
  }
}
