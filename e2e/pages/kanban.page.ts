import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class KanbanPage extends BasePage {
  readonly board: Locator;
  readonly columns: Locator;
  readonly cards: Locator;
  readonly createButton: Locator;
  readonly filterBar: Locator;

  constructor(page: Page) {
    super(page);
    this.board = page.locator('.kanban-board');
    this.columns = page.locator('.kanban-column');
    this.cards = page.locator('.kanban-card');
    this.createButton = page.locator('button:has-text("+ New"), button:has-text("New")');
    this.filterBar = page.locator('.filter-bar');
  }

  async navigateToKanban(kanbanId: string) {
    await this.navigateTo(`/kanban/${kanbanId}`);
    await this.waitForSpinnerToDisappear();
  }

  async getColumnCount(): Promise<number> {
    return this.columns.count();
  }

  async getCardCount(): Promise<number> {
    return this.cards.count();
  }

  async getColumnCardCount(columnName: string): Promise<number> {
    const column = this.columns.filter({ hasText: columnName });
    return column.locator('.kanban-card').count();
  }

  async getColumn(name: string): Locator {
    return this.columns.filter({ has: this.page.locator('.column-title, .column-header').filter({ hasText: name }) });
  }

  async clickCard(cardTitle: string) {
    await this.cards.filter({ hasText: cardTitle }).click();
    await this.page.waitForLoadState('domcontentloaded');
  }

  async clickCardById(cardId: string) {
    await this.cards.filter({ hasText: cardId }).click();
    await this.page.waitForLoadState('domcontentloaded');
  }

  async dragCardToColumn(cardTitle: string, targetColumnName: string) {
    const card = this.cards.filter({ hasText: cardTitle });
    const targetColumn = await this.getColumn(targetColumnName);
    const columnCards = targetColumn.locator('.column-cards');

    // Perform drag and drop
    await card.dragTo(columnCards);

    // Wait for update
    await this.page.waitForTimeout(500);
  }

  async dragCardByIdToColumn(cardId: string, targetColumnName: string) {
    const card = this.cards.filter({ hasText: cardId });
    const targetColumn = await this.getColumn(targetColumnName);
    const columnCards = targetColumn.locator('.column-cards');

    await card.dragTo(columnCards);
    await this.page.waitForTimeout(500);
  }

  async setFilter(property: string, value: string) {
    const filterGroup = this.filterBar.locator('.filter-group').filter({ hasText: property });
    await filterGroup.locator('select').selectOption(value);
    await this.waitForSpinnerToDisappear();
  }

  async clickCreate() {
    await this.createButton.click();
    await this.page.waitForLoadState('domcontentloaded');
  }

  async expectColumnCount(count: number) {
    await expect(this.columns).toHaveCount(count);
  }

  async expectCardCount(count: number) {
    await expect(this.cards).toHaveCount(count);
  }

  async expectCardInColumn(cardTitle: string, columnName: string) {
    const column = await this.getColumn(columnName);
    await expect(column.locator('.kanban-card').filter({ hasText: cardTitle })).toBeVisible();
  }

  async expectCardIdInColumn(cardId: string, columnName: string) {
    const column = await this.getColumn(columnName);
    await expect(column.locator('.kanban-card').filter({ hasText: cardId })).toBeVisible();
  }

  async expectColumnCardCount(columnName: string, count: number) {
    const column = await this.getColumn(columnName);
    const countBadge = column.locator('.column-count');
    await expect(countBadge).toHaveText(String(count));
  }

  async expectEmptyColumn(columnName: string) {
    const column = await this.getColumn(columnName);
    await expect(column.locator('.empty-column')).toBeVisible();
  }
}
