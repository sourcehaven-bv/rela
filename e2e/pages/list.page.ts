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

  async navigateToList(listId: string) {
    await this.navigateTo(`/list/${listId}`);
    await this.waitForSpinnerToDisappear();
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
    await this.page.locator(`.entity-row:has-text("${id}"), tr:has-text("${id}")`).click();
  }

  async clickCreateButton() {
    await this.createButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  async deleteRow(index: number) {
    const rows = this.page.locator('.entity-row, tbody tr');
    const row = rows.nth(index);
    const deleteBtn = row.locator('.delete-btn, button[title="Delete"]');
    await this.confirmDialog();
    await deleteBtn.click();
  }

  async deleteRowById(id: string) {
    const row = this.page.locator(`.entity-row:has-text("${id}"), tr:has-text("${id}")`);
    const deleteBtn = row.locator('.delete-btn, button[title="Delete"]');
    await this.confirmDialog();
    await deleteBtn.click();
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
    await expect(this.page.locator('.entity-row, tbody tr').filter({ hasText: text })).toBeVisible();
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
