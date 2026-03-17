import { Page, Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Page object for the Search view.
 * Encapsulates all search-related UI interactions.
 */
export class SearchPage extends BasePage {
  constructor(page: Page) {
    super(page)
  }

  // Selectors - centralized for easy maintenance

  /** The search input field */
  get searchInput(): Locator {
    return this.page.locator('input[type="search"], input[name="q"], .search-input, input[placeholder*="search" i]').first()
  }

  /** The search submit button (if present) */
  get searchButton(): Locator {
    return this.page.locator('button[type="submit"], .search-button, button:has-text("Search")').first()
  }

  /** The search view container */
  get searchContainer(): Locator {
    return this.page.locator('.search-view').first()
  }

  /** Container for search results */
  get resultsContainer(): Locator {
    return this.page.locator('.search-results')
  }

  /** Individual search result items */
  get resultItems(): Locator {
    return this.page.locator('.result-item')
  }

  /** Result titles (text content) */
  get resultTitles(): Locator {
    return this.page.locator('.result-item .result-title')
  }

  /** Empty state / no results message */
  get emptyState(): Locator {
    return this.page.locator('.empty-state')
  }

  // Page navigation

  async goto(): Promise<void> {
    await this.page.goto('/search')
  }

  async waitForLoad(): Promise<void> {
    await this.searchContainer.waitFor({ state: 'visible', timeout: 10000 })
    // Wait for search input to be ready
    await this.searchInput.waitFor({ state: 'visible', timeout: 10000 })
  }

  // Actions - domain-specific methods that tests can use

  /**
   * Perform a search by entering a query and submitting
   */
  async search(query: string): Promise<void> {
    await this.searchInput.fill(query)
    await this.searchInput.press('Enter')
    // Wait for results or empty state to appear
    await this.page.locator('.search-results, .empty-state, .loading-state').first().waitFor({ state: 'visible', timeout: 10000 })
    // If loading, wait for it to complete
    const loadingState = this.page.locator('.loading-state')
    if (await loadingState.isVisible()) {
      await loadingState.waitFor({ state: 'hidden', timeout: 10000 })
    }
  }

  /**
   * Click on a search result by its visible text
   */
  async clickResult(text: string): Promise<void> {
    await this.page.locator(`.result-item:has-text("${text}")`).first().click()
  }

  /**
   * Click on the first search result
   */
  async clickFirstResult(): Promise<void> {
    await this.resultItems.first().click()
  }

  /**
   * Get the number of visible search results
   */
  async getResultCount(): Promise<number> {
    return this.resultItems.count()
  }

  // State queries - for assertions in tests

  /**
   * Check if search input is visible
   */
  async isSearchInputVisible(): Promise<boolean> {
    return this.searchInput.isVisible()
  }

  /**
   * Check if results contain specific text
   */
  async resultsContain(text: string): Promise<boolean> {
    const content = await this.page.content()
    return content.includes(text)
  }

  /**
   * Check if showing no results state
   */
  async isShowingNoResults(): Promise<boolean> {
    const content = await this.page.content()
    return /no results|not found|0 results|empty/i.test(content.toLowerCase())
  }

  /**
   * Check if empty state is visible
   */
  async isEmptyStateVisible(): Promise<boolean> {
    return this.emptyState.isVisible()
  }

  /**
   * Check if page contains specific text
   */
  async containsText(text: string): Promise<boolean> {
    const content = await this.page.content()
    return content.includes(text)
  }
}
