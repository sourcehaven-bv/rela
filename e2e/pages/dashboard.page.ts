import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/** View at /dashboard — config-driven cards over search queries + analysis. */
export class DashboardPage extends BasePage {
  readonly view: Locator;
  readonly header: Locator;
  readonly loadingState: Locator;
  readonly cards: Locator;
  readonly grid: Locator;

  constructor(page: Page) {
    super(page);
    this.view = page.locator('.dashboard-view');
    this.header = page.locator('.dashboard-header');
    this.loadingState = page.locator('.dashboard-view .loading-state');
    this.cards = page.locator('.dashboard-card');
    this.grid = page.locator('.dashboard-grid');
  }

  async navigate() {
    await this.navigateTo('/dashboard');
    await expect(this.view).toBeVisible();
    await expect(this.loadingState).toBeHidden({ timeout: 10000 });
  }

  async getCardCount(): Promise<number> {
    return this.cards.count();
  }

  /** Returns the text content of the count-number in the named card. */
  async getStatValue(cardTitle: string): Promise<string> {
    const card = this.cards.filter({ hasText: cardTitle }).first();
    const value = card.locator('.count-number').first();
    await expect(value).toBeVisible();
    return (await value.textContent()) ?? '';
  }

  /** True if the page body contains any of the given strings (case-insensitive). */
  async pageContainsAny(patterns: RegExp | string[]): Promise<boolean> {
    const content = (await this.view.textContent()) ?? '';
    if (patterns instanceof RegExp) return patterns.test(content);
    const lowered = content.toLowerCase();
    return patterns.some((p) => lowered.includes(p.toLowerCase()));
  }

  async expectCardTextContains(cardTitle: string, text: string) {
    const card = this.cards.filter({ hasText: cardTitle }).first();
    await expect(card).toContainText(text);
  }

  async clickFirstEntityLinkInCard(cardTitle: string) {
    const card = this.cards.filter({ hasText: cardTitle }).first();
    const link = card.locator('a[href*="/entity/"]').first();
    await expect(link).toBeVisible();
    await link.click();
  }

  async hasEntityLinkInCard(cardTitle: string): Promise<boolean> {
    const card = this.cards.filter({ hasText: cardTitle }).first();
    return card.locator('a[href*="/entity/"]').first().isVisible();
  }
}
