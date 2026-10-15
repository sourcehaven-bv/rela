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
    this.loadingState = page.locator('.dashboard-view .rl-status-region--pending');
    this.cards = page.locator('.dashboard-card');
    this.grid = page.locator('.dashboard-grid');
  }

  async navigate() {
    await this.navigateTo('/dashboard');
    await expect(this.view).toBeVisible();
    await expect(this.loadingState).toBeHidden({ timeout: 10000 });
  }

  /**
   * Open a named dashboard (TKT-5LQNLY) at /dashboard/<name>, or under a
   * space prefix when `space` is given.
   */
  async navigateNamed(name: string, space?: string) {
    await this.navigateTo(`${space ? `/s/${space}` : ''}/dashboard/${name}`);
    await expect(this.view).toBeVisible();
    await expect(this.loadingState).toBeHidden({ timeout: 10000 });
  }

  async expectTitle(title: string) {
    await expect(this.header.locator('h1')).toHaveText(title);
  }

  async expectCardTitles(titles: string[]) {
    await expect(this.cards.locator('h3')).toHaveText(titles);
  }

  async expectNotFound(name: string) {
    const state = this.page.getByTestId('dashboard-not-found');
    await expect(state).toBeVisible();
    await expect(state).toContainText(name);
  }

  /** A sidebar navigation link, by its exact label. */
  sidebarLink(label: string): Locator {
    return this.page.locator('#main-sidebar').getByRole('link', { name: label, exact: true });
  }

  async expectSidebarActive(label: string) {
    await expect(this.sidebarLink(label)).toHaveAttribute('aria-current', 'page');
  }

  async expectSidebarInactive(label: string) {
    await expect(this.sidebarLink(label)).not.toHaveAttribute('aria-current', 'page');
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
