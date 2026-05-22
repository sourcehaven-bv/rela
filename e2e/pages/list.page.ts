import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class ListPage extends BasePage {
  readonly table: Locator;
  readonly createButton: Locator;
  readonly filterBar: Locator;
  readonly pagination: Locator;
  readonly emptyState: Locator;

  constructor(page: Page) {
    super(page);
    this.table = page.locator('.entity-table, table');
    this.createButton = page.locator('a, button').filter({ hasText: /new|create|add/i }).first();
    this.filterBar = page.locator('.filter-bar');
    this.pagination = page.locator('.pagination');
    this.emptyState = page.locator('.empty-state');
  }

  async navigateToList(listId: string, query?: string) {
    const path = query ? `/list/${listId}?${query}` : `/list/${listId}`;
    await this.navigateTo(path);
    await this.waitForSpinnerToDisappear();
    // Wait for the list to render — either rows or the empty-state message.
    const anyRow = this.page.locator('.entity-row, tbody tr').first();
    const empty = this.page.locator('.empty-state');
    await expect(anyRow.or(empty)).toBeVisible();
  }

  /** Wait for at least one row cell to render. Covers both the desktop
   *  table layout (`td`) and the mobile card layout (`.mobile-card-title`).
   *  Use when navigating to a list page directly (without
   *  `navigateToList`, which already waits). */
  async waitForRowsRendered(timeoutMs = 10_000) {
    await this.page.locator('td, .mobile-card-title').first().waitFor({ timeout: timeoutMs });
  }

  /** Assert no `+ New …` link is rendered on the page. Used to verify
   *  the AC10 read-only payoff: collection `_actions.create=false`
   *  hides the create affordance. */
  async expectNoCreateAffordance() {
    await expect(this.page.getByRole('link', { name: /^\+ New/ })).toHaveCount(0);
  }

  /** Assert no row-level delete buttons are rendered. Used to verify
   *  the AC10 read-only payoff: per-entity `_actions.delete=false`
   *  hides the per-row x. */
  async expectNoRowDeleteButtons() {
    await expect(this.page.locator('button.delete-btn')).toHaveCount(0);
  }

  async expectListHeading(title: string) {
    await expect(this.page.locator('h1', { hasText: title })).toBeVisible();
  }

  async getRowCount(): Promise<number> {
    const rows = this.page.locator('.entity-row, tbody tr');
    return rows.count();
  }

  async clickRow(index: number) {
    const rows = this.page.locator('.entity-row, tbody tr');
    await rows.nth(index).click();
  }

  async clickRowById(id: string) {
    await this.page.locator(`.entity-row[data-entity-id="${id}"]`).click();
  }

  async openDeleteModalForFirstRow() {
    const firstRow = this.page.locator('.entity-row, tbody tr').first();
    await firstRow.locator('.delete-btn, button[title="Delete"]').click();
  }

  async cancelDeleteModal() {
    await this.page.locator('.modal button').filter({ hasText: /^Cancel$/ }).click();
  }

  async deleteRowByTitle(title: string) {
    const row = this.page.locator('.entity-row').filter({ hasText: title });
    const id = await row.getAttribute('data-entity-id');
    if (!id) throw new Error(`No data-entity-id on row with title ${title}`);
    await this.deleteRowById(id);
  }

  async expectCellInRow(id: string, cellText: string) {
    await expect(
      this.page.locator(`.entity-row[data-entity-id="${id}"]`).locator(`text=${cellText}`),
    ).toBeVisible();
  }

  /** Count of cells in the row marked as inaccessible (rendered with
   *  the lock indicator). Used by git-crypt.spec.ts to assert that
   *  encrypted entities lock every visible column. */
  async lockedCellsInRow(id: string): Promise<number> {
    return this.page
      .locator(`.entity-row[data-entity-id="${id}"] .inaccessible-cell`)
      .count();
  }

  async expectRowNotVisible(text: string) {
    await expect(
      this.page.locator('.entity-row, tbody tr').filter({ hasText: text }),
    ).not.toBeVisible();
  }

  async clickCreateButton() {
    await this.createButton.click();
    await this.page.waitForLoadState('domcontentloaded');
  }

  async deleteRow(index: number) {
    const rows = this.page.locator('.entity-row, tbody tr');
    const row = rows.nth(index);
    const deleteBtn = row.locator('.delete-btn, button[title="Delete"]');
    await this.confirmDialog();
    await deleteBtn.click();
  }

  async deleteRowById(id: string) {
    const row = this.page.locator(`.entity-row[data-entity-id="${id}"]`);
    const deleteBtn = row.locator('.delete-btn, button[title="Delete"]');
    await deleteBtn.click();
    // Confirm deletion in the modal
    await this.page.locator('.modal button').filter({ hasText: /^Delete$/ }).click();
  }

  async sortByColumn(columnName: string) {
    const header = this.page.locator('th').filter({ hasText: columnName });
    await header.click();
    await this.waitForSpinnerToDisappear();
  }

  async expectSortIndicator(columnName: string, direction: 'asc' | 'desc') {
    const header = this.page.locator('th').filter({ hasText: columnName });
    const indicator = direction === 'asc' ? '▲' : '▼';
    await expect(header).toContainText(indicator);
  }

  async setFilter(property: string, value: string) {
    const filterSelect = this.filterBar.locator(`select[data-property="${property}"], select`).filter({ hasText: property }).first();
    if (await filterSelect.isVisible()) {
      await filterSelect.selectOption(value);
    } else {
      // Try generic filter control
      const control = this.filterBar.locator('.filter-group').filter({ hasText: property });
      await control.locator('select').selectOption(value);
    }
    await this.waitForSpinnerToDisappear();
  }

  async goToPage(pageNumber: number) {
    await this.pagination.locator(`button:has-text("${pageNumber}")`).click();
    await this.waitForSpinnerToDisappear();
  }

  async nextPage() {
    await this.pagination.locator('button:has-text("Next"), button:has-text("→")').click();
    await this.waitForSpinnerToDisappear();
  }

  async prevPage() {
    await this.pagination.locator('button:has-text("Prev"), button:has-text("←")').click();
    await this.waitForSpinnerToDisappear();
  }

  async expectRowCount(count: number) {
    const rows = this.page.locator('.entity-row, tbody tr');
    await expect(rows).toHaveCount(count);
  }

  async expectRowContains(text: string) {
    const rowById = this.page.locator(`.entity-row[data-entity-id="${text}"]`);
    const rowByText = this.page.locator('.entity-row, tbody tr').filter({ hasText: text });
    await expect(rowById.or(rowByText).first()).toBeVisible();
  }

  async expectColumnHeader(name: string | RegExp) {
    const matcher = name instanceof RegExp ? name : new RegExp(name, 'i');
    await expect(this.page.locator('th').filter({ hasText: matcher })).toBeVisible();
  }

  /** Set the Nth filter in the filter bar to the given option value and wait
   *  for any resulting list refetch to settle. */
  async setFilterByIndex(index: number, value: string | { index: number }) {
    const select = this.filterBar.locator('select').nth(index);
    await select.selectOption(value as never);
    await this.waitForSpinnerToDisappear();
  }

  async filterControlCount(): Promise<number> {
    return this.filterBar.locator('select').count();
  }

  /** Wait for rows to be present before we try to issue keyboard commands
   *  against them. The ListView's keydown handler is attached to `document`,
   *  so we don't need to focus the table itself. */
  async focusTable() {
    const firstRow = this.page.locator('.entity-row, tbody tr').first();
    await expect(firstRow).toBeVisible();
  }

  async pressKey(key: string) {
    await this.page.keyboard.press(key);
  }

  get selectedRow(): Locator {
    return this.page.locator('.entity-row.selected, tr.selected');
  }

  async expectEmpty() {
    await expect(this.emptyState).toBeVisible();
  }

  async expectTotal(total: number) {
    // Check pagination or header for total count
    const totalText = this.page.locator('.results-count, .total-count, .pagination');
    await expect(totalText).toContainText(String(total));
  }
}
