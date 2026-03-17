import { Page, Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Page object for List views.
 * Handles entity listing, filtering, and sorting.
 */
export class ListPage extends BasePage {
  private listName: string

  constructor(page: Page, listName: string) {
    super(page)
    this.listName = listName
  }

  // Selectors

  /** The list container */
  get listContainer(): Locator {
    return this.page.locator('.entity-list').first()
  }

  /** List header (shows when loaded successfully) */
  get listHeader(): Locator {
    return this.page.locator('.list-header')
  }

  /** Loading state indicator */
  get loadingState(): Locator {
    return this.page.locator('.entity-list .loading-state')
  }

  /** Empty state indicator */
  get emptyStateElement(): Locator {
    return this.page.locator('.entity-list .empty-state')
  }

  /** Table element (if list is a table) */
  get table(): Locator {
    return this.page.locator('.entity-table').first()
  }

  /** Table rows (data rows, not header) */
  get tableRows(): Locator {
    return this.page.locator('.entity-table .entity-row')
  }

  /** Table header row */
  get tableHeader(): Locator {
    return this.page.locator('.entity-table thead tr').first()
  }

  /** Column headers */
  get columnHeaders(): Locator {
    return this.page.locator('.entity-table th')
  }

  /** Pagination controls */
  get pagination(): Locator {
    return this.page.locator('.pagination, nav[aria-label*="pagination"], .page-controls')
  }

  /** Next page button */
  get nextPageButton(): Locator {
    return this.page.locator('.pagination-next, button:has-text("Next"), a:has-text("Next")').first()
  }

  /** Previous page button */
  get prevPageButton(): Locator {
    return this.page.locator('.pagination-prev, button:has-text("Previous"), a:has-text("Prev")').first()
  }

  /** Filter controls */
  get filterControls(): Locator {
    return this.page.locator('.filters, .filter-controls, [data-testid="filters"]')
  }

  /** Create new button */
  get createButton(): Locator {
    return this.page.locator('a[href*="/form/create"], button:has-text("Create"), button:has-text("New"), .btn-create').first()
  }

  /** Empty state when no items */
  get emptyState(): Locator {
    return this.page.locator('.empty-state, .no-items, [data-testid="empty-state"]')
  }

  // Page navigation

  async goto(): Promise<void> {
    await this.page.goto(`/list/${this.listName}`)
  }

  async waitForLoad(): Promise<void> {
    await this.listContainer.waitFor({ state: 'visible', timeout: 10000 })
    // Wait for loading to complete and either table, empty state, or error state to appear
    await this.page
      .locator('.entity-table, .empty-state, .error-state')
      .first()
      .waitFor({ state: 'visible', timeout: 10000 })
  }

  // Actions

  /**
   * Click on a row to view entity details
   */
  async clickRow(rowIndex: number): Promise<void> {
    await this.tableRows.nth(rowIndex).click()
  }

  /**
   * Click on a row containing specific text
   */
  async clickRowWithText(text: string): Promise<void> {
    await this.page.locator(`tr:has-text("${text}"), .list-item:has-text("${text}")`).first().click()
  }

  /**
   * Click the create button
   */
  async clickCreate(): Promise<void> {
    await this.createButton.click()
  }

  /**
   * Sort by clicking a column header
   */
  async sortByColumn(columnName: string): Promise<void> {
    await this.page.locator(`th:has-text("${columnName}"), .column-header:has-text("${columnName}")`).first().click()
  }

  /**
   * Go to next page
   */
  async nextPage(): Promise<void> {
    await this.nextPageButton.click()
  }

  /**
   * Go to previous page
   */
  async prevPage(): Promise<void> {
    await this.prevPageButton.click()
  }

  /**
   * Apply a filter (implementation depends on UI)
   */
  async filter(field: string, value: string): Promise<void> {
    const filterInput = this.page.locator(`[data-filter="${field}"], input[name="filter-${field}"]`).first()
    await filterInput.fill(value)
    await filterInput.press('Enter')
  }

  // State queries

  /**
   * Get the number of rows in the list
   */
  async getRowCount(): Promise<number> {
    return this.tableRows.count()
  }

  /**
   * Check if the list is empty
   */
  async isEmpty(): Promise<boolean> {
    const rowCount = await this.getRowCount()
    return rowCount === 0
  }

  /**
   * Get text content of a specific cell
   */
  async getCellText(rowIndex: number, columnIndex: number): Promise<string> {
    const cell = this.tableRows.nth(rowIndex).locator('td, .cell').nth(columnIndex)
    return (await cell.textContent()) || ''
  }

  /**
   * Check if list contains an entity with specific text
   */
  async containsEntity(text: string): Promise<boolean> {
    const content = await this.page.content()
    return content.includes(text)
  }

  /**
   * Check if pagination is visible
   */
  async hasPagination(): Promise<boolean> {
    return this.pagination.isVisible()
  }

  /**
   * Get all column header texts
   */
  async getColumnHeaders(): Promise<string[]> {
    return this.columnHeaders.allTextContents()
  }

  /**
   * Check if filter controls are visible
   */
  async hasFilters(): Promise<boolean> {
    const selects = this.page.locator('select, .filter, .ss-main, input[type="search"]')
    return (await selects.count()) > 0
  }

  /**
   * Click the new/create button
   */
  async clickNewButton(): Promise<void> {
    await this.createButton.click()
  }

  /**
   * Sort by column name
   */
  async sortBy(columnName: string): Promise<void> {
    await this.sortByColumn(columnName)
  }

  /**
   * Filter by status value
   */
  async filterByStatus(status: string): Promise<void> {
    const statusFilter = this.page.locator('select[name="status"], select[data-filter="status"], .ss-main').first()
    if (await statusFilter.isVisible()) {
      // Try SlimSelect first
      const ssMain = this.page.locator('.ss-main').first()
      if (await ssMain.isVisible()) {
        await ssMain.click()
        await this.page.locator(`.ss-option:has-text("${status}")`).first().click()
      } else {
        // Regular select
        await statusFilter.selectOption(status)
      }
    }
  }

  /**
   * Get full page content for assertions
   */
  async getPageContent(): Promise<string> {
    return this.page.content()
  }
}

/**
 * Factory function to create ListPage for different list views
 */
export function createListPage(page: Page, listName: string): ListPage {
  return new ListPage(page, listName)
}
