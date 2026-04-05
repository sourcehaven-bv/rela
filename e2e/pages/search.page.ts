import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class SearchPage extends BasePage {
  readonly searchInput: Locator;
  readonly searchButton: Locator;
  readonly filterButton: Locator;
  readonly filterMenu: Locator;
  readonly results: Locator;
  readonly resultItems: Locator;
  readonly activeFilters: Locator;
  readonly emptyState: Locator;
  readonly resultsCount: Locator;

  constructor(page: Page) {
    super(page);
    this.searchInput = page.locator('.search-input, input[type="text"]').first();
    this.searchButton = page.locator('button:has-text("Search")');
    this.filterButton = page.locator('button:has-text("+ Filter"), button:has-text("Filter")');
    this.filterMenu = page.locator('.filter-menu');
    this.results = page.locator('.search-results');
    this.resultItems = page.locator('.result-item');
    this.activeFilters = page.locator('.active-filters');
    this.emptyState = page.locator('.empty-state');
    this.resultsCount = page.locator('.results-count');
  }

  async navigateToSearch() {
    await this.navigateTo('/search');
    await this.waitForSpinnerToDisappear();
  }

  async search(query: string) {
    await this.searchInput.fill(query);
    await this.searchButton.click();
    await this.waitForSpinnerToDisappear();
  }

  async searchAndEnter(query: string) {
    await this.searchInput.fill(query);
    await this.searchInput.press('Enter');
    await this.waitForSpinnerToDisappear();
  }

  async openFilterMenu() {
    await this.filterButton.click();
    await expect(this.filterMenu).toBeVisible();
  }

  async selectFilterProperty(propertyName: string) {
    await this.filterMenu.locator('.filter-option').filter({ hasText: propertyName }).click();
  }

  async selectFilterValue(value: string) {
    await this.filterMenu.locator('.filter-option').filter({ hasText: value }).click();
  }

  async addFilter(propertyName: string, value: string) {
    await this.openFilterMenu();
    await this.selectFilterProperty(propertyName);
    await this.selectFilterValue(value);
    await this.waitForSpinnerToDisappear();
  }

  async removeFilter(filterLabel: string) {
    const chip = this.activeFilters.locator('.filter-chip').filter({ hasText: filterLabel });
    await chip.locator('.chip-remove, button').click();
    await this.waitForSpinnerToDisappear();
  }

  async clearAllFilters() {
    await this.activeFilters.locator('.clear-filters, button:has-text("Clear")').click();
    await this.waitForSpinnerToDisappear();
  }

  async clickResult(index: number) {
    await this.resultItems.nth(index).click();
    await this.page.waitForLoadState('networkidle');
  }

  async clickResultById(id: string) {
    await this.resultItems.filter({ hasText: id }).click();
    await this.page.waitForLoadState('networkidle');
  }

  async getResultCount(): Promise<number> {
    return this.resultItems.count();
  }

  async expectResultCount(count: number) {
    await expect(this.resultItems).toHaveCount(count);
  }

  async expectResultContains(text: string) {
    await expect(this.resultItems.filter({ hasText: text })).toBeVisible();
  }

  async expectNoResults() {
    await expect(this.emptyState).toBeVisible();
  }

  async expectResultsCountText(text: string) {
    await expect(this.resultsCount).toContainText(text);
  }

  async expectFilterActive(filterLabel: string) {
    await expect(this.activeFilters.locator('.filter-chip').filter({ hasText: filterLabel })).toBeVisible();
  }

  // Keyboard navigation
  async pressKey(key: string) {
    await this.page.keyboard.press(key);
  }

  async navigateResultsWithKeyboard(direction: 'down' | 'up') {
    await this.page.keyboard.press(direction === 'down' ? 'ArrowDown' : 'ArrowUp');
  }

  async openSelectedResult() {
    await this.page.keyboard.press('Enter');
    await this.page.waitForLoadState('networkidle');
  }
}
