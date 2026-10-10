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
    this.columns = page.locator('.rl-board-column');
    this.cards = page.locator('.kanban-card');
    this.createButton = page.locator('button:has-text("+ New"), button:has-text("New")');
    this.filterBar = page.locator('.filter-bar');
  }

  /** The count shown on a card for a count field, found by its spoken label. */
  cardMetaCount(cardTitle: string, label: string): Locator {
    return this.cards.filter({ hasText: cardTitle }).locator('.rl-meta-item').filter({ hasText: label });
  }

  async expectCardMetaCount(cardTitle: string, label: string, count: number) {
    await expect(this.cardMetaCount(cardTitle, label)).toHaveText(new RegExp(`^${count}\\s`));
  }

  async expectNoCardMetaCount(cardTitle: string, label: string) {
    await expect(this.cards.filter({ hasText: cardTitle })).toBeVisible();
    await expect(this.cardMetaCount(cardTitle, label)).toHaveCount(0);
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

  getColumn(name: string): Locator {
    return this.columns.filter({ has: this.page.locator('.rl-section-heading').filter({ hasText: name }) });
  }

  /** The detail panel a plain card click opens over the board. */
  get detailPanel(): Locator {
    return this.page.locator('.entity-detail-panel');
  }

  /** The open panel's title heading. */
  get detailPanelHeading(): Locator {
    return this.detailPanel.getByRole('heading').first();
  }

  /** Open a card in the detail panel with a plain click. */
  async clickCard(cardTitle: string) {
    await this.cards.filter({ hasText: cardTitle }).click();
    await expect(this.detailPanel).toBeVisible();
  }

  /** Go to a card's own page: open the panel, then expand it. */
  async openCardPage(cardTitle: string) {
    await this.clickCard(cardTitle);
    await this.detailPanel.getByRole('button', { name: 'Expand' }).click();
    await this.page.waitForURL(/\/entity\/|\/form\//);
  }

  /** Modifier-click a kanban card and return the tab it opens (TKT-3CSZRG). */
  async openCardInNewTab(cardId: string, modifier: 'ControlOrMeta' = 'ControlOrMeta') {
    const popupPromise = this.page.context().waitForEvent('page');
    await this.cards.filter({ hasText: cardId }).click({ modifiers: [modifier] });
    return popupPromise;
  }

  /** Drag a card onto a column with the real pointer and wait for the
   *  resulting PATCH response. */
  private async dragCardToColumnLocator(card: Locator, column: Locator) {
    const patchPromise = this.page.waitForResponse(
      r => /\/api\/v1\/[^/]+\/[^/]+$/.test(r.url()) && r.request().method() === 'PATCH',
      { timeout: 5000 },
    ).catch(() => null);
    // Stepped moves rather than dragTo: the browser needs a few pointer moves
    // to start a native drag, and one jump to the target can race it.
    const from = (await card.boundingBox())!;
    const to = (await column.boundingBox())!;
    const start = { x: from.x + from.width / 2, y: from.y + from.height / 2 };
    const end = { x: to.x + to.width / 2, y: to.y + Math.min(to.height / 2, 120) };
    await this.page.mouse.move(start.x, start.y);
    await this.page.mouse.down();
    await this.page.mouse.move(start.x + 5, start.y + 5, { steps: 2 });
    await this.page.mouse.move(end.x, end.y, { steps: 10 });
    await this.page.mouse.up();
    await patchPromise;
  }

  async dragCardToColumn(cardTitle: string, targetColumnName: string) {
    const card = this.cards.filter({ hasText: cardTitle });
    const targetColumn = this.getColumn(targetColumnName);
    await this.dragCardToColumnLocator(card, targetColumn);
  }

  async dragCardByIdToColumn(cardId: string, targetColumnName: string) {
    const card = this.cards.filter({ hasText: cardId });
    const targetColumn = this.getColumn(targetColumnName);
    await this.dragCardToColumnLocator(card, targetColumn);
  }

  /** Move a card with the board's keyboard path: pick it up, arrow to the
   *  target column, drop. Waits for the resulting PATCH response. */
  async moveCardByKeyboard(cardTitle: string, targetColumnName: string) {
    const titles = await this.columns.locator('.rl-section-heading').allInnerTexts();
    const card = this.cards.filter({ hasText: cardTitle });
    const from = await this.columns.evaluateAll(
      (cols, el) => cols.findIndex((c) => c.contains(el)),
      await card.elementHandle(),
    );
    const to = titles.findIndex((t) => t.includes(targetColumnName));
    const patchPromise = this.page.waitForResponse(
      r => /\/api\/v1\/[^/]+\/[^/]+$/.test(r.url()) && r.request().method() === 'PATCH',
    );
    await card.locator('xpath=..').focus();
    await this.page.keyboard.press('Enter');
    const key = to > from ? 'ArrowRight' : 'ArrowLeft';
    for (let i = 0; i < Math.abs(to - from); i++) await this.page.keyboard.press(key);
    await this.page.keyboard.press('Enter');
    await patchPromise;
  }

  /** Pick a value in the filter control labelled `label`. The board renders
   *  the list's FilterBar, so it is found the way the list page finds it. */
  async setFilter(label: string, value: string) {
    const control = this.filterBar.locator('.filter-item').filter({ hasText: new RegExp(label, 'i') });
    await this.pickFilterOption(control, value);
    await this.waitForSpinnerToDisappear();
  }

  /** The create dialog New opens over the board. */
  get createDialog(): Locator {
    return this.page.getByRole('dialog');
  }

  async clickCreate() {
    await this.createButton.click();
    await expect(this.createDialog).toBeVisible();
  }

  /** Press the dialog's Create, not "Create & add another". */
  async submitCreateDialog() {
    await this.createDialog.getByRole('button', { name: /^Create(?! &)/ }).click();
  }

  async expectColumnCount(count: number) {
    await expect(this.columns).toHaveCount(count);
  }

  async expectColumnLabel(label: string) {
    await expect(this.page.locator('.rl-section-heading').filter({ hasText: label })).toBeVisible();
  }

  async expectFirstCardSeverityVisible() {
    await expect(this.cards.first().getByText(/high|critical|medium|low/)).toBeVisible();
  }

  async expectCardCount(count: number) {
    await expect(this.cards).toHaveCount(count);
  }

  async expectCardInColumn(cardTitle: string, columnName: string) {
    const column = this.getColumn(columnName);
    await expect(column.locator('.kanban-card').filter({ hasText: cardTitle })).toBeVisible();
  }

  async expectCardIdInColumn(cardId: string, columnName: string) {
    const column = this.getColumn(columnName);
    await expect(column.locator('.kanban-card').filter({ hasText: cardId })).toBeVisible();
  }

  async expectColumnCardCount(columnName: string, count: number) {
    const column = this.getColumn(columnName);
    const countBadge = column.locator('.rl-count');
    await expect(countBadge).toHaveText(String(count));
  }

  async expectColumnCountVisible(columnName: string) {
    const column = this.getColumn(columnName);
    await expect(column.locator('.rl-count')).toBeVisible();
  }

  /** Fold a column away with the collapse button on its heading. */
  async collapseColumn(name: string) {
    await this.page.getByRole('button', { name: `Collapse ${name}`, exact: true }).click();
  }

  /** Open a collapsed column by clicking its rail. */
  async expandColumn(name: string) {
    await this.collapsedRail(name).click();
  }

  /** A collapsed column's rail, which is its expand button. */
  collapsedRail(name: string): Locator {
    return this.page.getByRole('button', { name: `Expand ${name}`, exact: true });
  }

  /** Drag a card onto a collapsed column's rail with the real pointer. */
  async dragCardToCollapsedColumn(cardTitle: string, columnName: string) {
    await this.dragCardToColumnLocator(this.cards.filter({ hasText: cardTitle }), this.collapsedRail(columnName));
  }

  async expectColumnCollapsed(name: string, count: number) {
    await expect(this.collapsedRail(name)).toBeVisible();
    await expect(this.collapsedRail(name).locator('.rl-count')).toHaveText(String(count));
  }

  async expectColumnExpanded(name: string) {
    await expect(this.collapsedRail(name)).toHaveCount(0);
    await expect(this.page.getByRole('button', { name: `Collapse ${name}`, exact: true })).toBeVisible();
  }

  async expectEmptyColumn(columnName: string) {
    const column = this.getColumn(columnName);
    await expect(column.locator('.empty-column')).toBeVisible();
  }
}
