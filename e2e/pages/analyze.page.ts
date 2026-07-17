import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/** View at /analyze — runs metamodel validation checks and shows issues. */
export class AnalyzePage extends BasePage {
  readonly heading: Locator;
  readonly loadingState: Locator;
  readonly checkCards: Locator;
  readonly checkTitles: Locator;
  readonly checkCounts: Locator;
  readonly checkDescriptions: Locator;
  readonly issueRows: Locator;
  readonly refreshButton: Locator;

  constructor(page: Page) {
    super(page);
    this.heading = page.locator('h1');
    this.loadingState = page.locator('.loading-state');
    this.checkCards = page.locator('.check-card');
    this.checkTitles = page.locator('.check-title');
    this.checkCounts = page.locator('.check-count');
    this.checkDescriptions = page.locator('.check-description');
    this.issueRows = page.locator('.issue-row');
    this.refreshButton = page.locator('button').filter({ hasText: 'Refresh' });
  }

  async navigate() {
    await this.navigateTo('/analyze');
    await this.waitForAnalysisToSettle();
  }

  async waitForAnalysisToSettle() {
    await expect(this.loadingState).toBeHidden({ timeout: 15000 });
  }

  async expectHeading() {
    await expect(this.heading).toHaveText('Analysis');
  }

  async expectCheckCardCount(count: number) {
    await expect(this.checkCards).toHaveCount(count);
  }

  async expectCheckTitles(expected: string[]) {
    await expect(this.checkTitles).toContainText(expected);
  }

  async expectFirstDescriptionVisible() {
    await expect(this.checkDescriptions.first()).toBeVisible();
  }

  async expectDescriptionText(text: string) {
    await expect(this.page.getByText(text)).toBeVisible();
  }

  async getIssueRowCount(): Promise<number> {
    return this.issueRows.count();
  }

  /**
   * Navigate to the entity of the first entity-linked issue. The row's
   * click targets are split (TKT-IL499B): the entity-title cell
   * navigates, while the message cell reveals detail. Navigation is
   * triggered by clicking the title, not the row — and only rows with a
   * real entity expose a clickable title (ID-gap / load-error rows do
   * not), so target the first clickable title rather than the first row.
   */
  async clickFirstIssueEntity() {
    await this.page.locator('.entity-title.clickable').first().click();
  }

  async clickRefresh() {
    await expect(this.refreshButton).toBeVisible();
    await this.refreshButton.click();
    await this.waitForAnalysisToSettle();
  }
}
