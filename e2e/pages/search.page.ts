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

  /** Wait for the first result to render, then enter keyboard-results mode by
   *  focusing the input and pressing ArrowDown. The first result will be
   *  selected when this returns. */
  async focusFirstResult() {
    await expect(this.resultItems.first()).toBeVisible();
    await this.searchInput.focus();
    await this.page.keyboard.press('ArrowDown');
    await expect(this.resultItems.first()).toHaveClass(/selected/);
  }

  /** Press Enter globally — used after focusFirstResult to open the selected one. */
  async openSelectedResult() {
    await this.page.keyboard.press('Enter');
  }

  /** Navigate to /search with an initial query param. */
  async navigateToSearchWithQuery(query: string) {
    await this.navigateTo(`/search?q=${encodeURIComponent(query)}`);
    await this.waitForSpinnerToDisappear();
  }

  /** Blur the search input (by clicking the page body) then press the F key. */
  async pressFilterHotkey() {
    await this.page.locator('body').click();
    await this.page.keyboard.press('f');
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
    await this.page.waitForLoadState('domcontentloaded');
  }

  async clickResultById(id: string) {
    await this.resultItems.filter({ hasText: id }).click();
    await this.page.waitForLoadState('domcontentloaded');
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

  async expectNoActiveFilters() {
    await expect(this.activeFilters.locator('.filter-chip')).toHaveCount(0);
  }

  async expectFirstResultHasTypeBadge() {
    await expect(this.resultItems.first().locator('.result-type')).toBeVisible();
  }

  async expectFirstResultIdContains(text: string) {
    await expect(this.resultItems.first().locator('.result-id')).toContainText(text);
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
    await this.page.waitForLoadState('domcontentloaded');
  }
}
