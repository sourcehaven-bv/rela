import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class EntityPage extends BasePage {
  readonly heading: Locator;
  readonly editButton: Locator;
  readonly deleteButton: Locator;
  readonly typeBadge: Locator;

  constructor(page: Page) {
    super(page);
    this.heading = page.locator('main h1').first();
    this.editButton = page.locator('a:has-text("Edit"), button:has-text("Edit")').first();
    this.deleteButton = page.locator('button:has-text("Delete")').first();
    this.typeBadge = page.locator('.entity-type-badge');
  }

  async navigateToEntity(entityType: string, id: string) {
    await this.navigateTo(`/entity/${entityType}/${id}`);
    await this.waitForSpinnerToDisappear();
    await expect(this.heading).toBeVisible();
  }

  async expectHeadingText(text: string | RegExp) {
    const matcher = text instanceof RegExp ? text : new RegExp(text);
    await expect(this.heading.filter({ hasText: matcher })).toBeVisible();
  }

  async expectNoErrorState() {
    await expect(this.page.getByText('Entity not found')).not.toBeVisible();
    await expect(this.page.getByText('Failed to load')).not.toBeVisible();
  }

  async expectPropertyValue(value: string | RegExp) {
    const matcher = value instanceof RegExp ? value : new RegExp(value);
    await expect(this.page.locator('main').getByText(matcher).first()).toBeVisible();
  }

  /** Click the Edit action on the entity view to navigate to the edit form. */
  async clickEdit() {
    await expect(this.editButton).toBeVisible();
    await this.editButton.click();
    await this.page.waitForURL(/\/form\//);
  }

  async clickRelationLink(targetId: string) {
    const link = this.page.locator('button.relation-link').filter({ hasText: targetId });
    await expect(link).toBeVisible();
    await link.click();
  }

  async expectTypeBadge(type: string | RegExp) {
    const matcher = type instanceof RegExp ? type : new RegExp(type, 'i');
    await expect(this.typeBadge.filter({ hasText: matcher }).first()).toBeVisible();
  }
}
