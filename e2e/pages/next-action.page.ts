import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/** The page-level next-action card, mounted once above every routed page. */
export class NextActionPage extends BasePage {
  readonly card: Locator;
  readonly message: Locator;
  /** The routed view's main region, to check the card does not crowd it out. */
  readonly kanbanColumns: Locator;

  constructor(page: Page) {
    super(page);
    this.card = page.locator('.rela-na');
    this.message = page.locator('.rela-na-message');
    this.kanbanColumns = page.locator('.rl-board-column');
  }

  async expectShowing(text: string | RegExp) {
    await expect(this.message).toHaveText(text);
  }

  /**
   * Nothing is shown once the page has asked. Waits for the page's own
   * request so an absent card is an answer, not a race.
   */
  async expectNoneAfter(load: Promise<unknown>) {
    await load;
    await expect(this.card).toHaveCount(0);
  }

  /** Resolves when the SPA has asked the server for this page's suggestion. */
  waitForRequest(surface: string): Promise<unknown> {
    return this.page.waitForResponse(
      (r) => r.url().includes('/api/v1/_next_action') && r.url().includes(`surface=${surface}`),
    );
  }

  /** Resolves when the SPA has reported the suggestion as shown. */
  waitForImpression(): Promise<unknown> {
    return this.page.waitForResponse(
      (r) => r.url().includes('/api/v1/_next_action') && r.request().method() === 'POST',
    );
  }

  async entityId(): Promise<string | null> {
    return this.card.getAttribute('data-entity-id');
  }

  /** The card and the first board column are both on screen, card above. */
  async expectAboveBoard() {
    await expect(this.card).toBeInViewport();
    await expect(this.kanbanColumns.first()).toBeInViewport();
    const card = await this.card.boundingBox();
    const column = await this.kanbanColumns.first().boundingBox();
    expect(card && column && card.y + card.height <= column.y + 1).toBeTruthy();
  }
}
