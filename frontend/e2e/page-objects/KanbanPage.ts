import { Page, Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Page object for Kanban board views.
 * Handles kanban-specific interactions like drag-and-drop.
 */
export class KanbanPage extends BasePage {
  private boardName: string

  constructor(page: Page, boardName: string) {
    super(page)
    this.boardName = boardName
  }

  // Selectors

  /** The kanban view container */
  get kanbanContainer(): Locator {
    return this.page.locator('.kanban-view').first()
  }

  /** The kanban board container */
  get boardContainer(): Locator {
    return this.page.locator('.kanban-board').first()
  }

  /** Page header (shows when loaded successfully) */
  get pageHeader(): Locator {
    return this.page.locator('.page-header')
  }

  /** Loading state indicator */
  get loadingState(): Locator {
    return this.page.locator('.kanban-view .loading-state')
  }

  /** All column containers */
  get columns(): Locator {
    return this.page.locator('.kanban-column')
  }

  /** All card elements */
  get cards(): Locator {
    return this.page.locator('.kanban-card')
  }

  /** Column headers */
  get columnHeaders(): Locator {
    return this.page.locator('.column-header .column-title')
  }

  /** Add card button (if present) */
  get addCardButton(): Locator {
    return this.page.locator('.header-actions .btn-primary, button:has-text("New")').first()
  }

  // Page navigation

  async goto(): Promise<void> {
    await this.page.goto(`/kanban/${this.boardName}`)
  }

  async waitForLoad(): Promise<void> {
    await this.kanbanContainer.waitFor({ state: 'visible' })
    // Wait for loading indicator to disappear (if present)
    await this.loadingState.waitFor({ state: 'hidden' }).catch(() => {
      // Loading state may already be hidden or never shown
    })
    // Wait for the board to appear
    await this.boardContainer.waitFor({ state: 'visible' })
  }

  // Actions

  /**
   * Get a specific column by its header text
   */
  getColumn(headerText: string): Locator {
    return this.page.locator(`.kanban-column:has-text("${headerText}")`)
  }

  /**
   * Get cards within a specific column
   */
  getCardsInColumn(columnHeader: string): Locator {
    const column = this.getColumn(columnHeader)
    return column.locator('.kanban-card')
  }

  /**
   * Click on a card to view its details
   */
  async clickCard(cardText: string): Promise<void> {
    await this.page.locator(`.kanban-card:has-text("${cardText}")`).first().click()
  }

  /**
   * Drag a card from one column to another
   * Note: Playwright's drag-and-drop may need adjustments based on the actual implementation
   */
  async dragCard(cardText: string, toColumnHeader: string): Promise<void> {
    const card = this.page.locator(`.kanban-card:has-text("${cardText}")`).first()
    const targetColumn = this.getColumn(toColumnHeader)
    await card.dragTo(targetColumn)
  }

  /**
   * Click the add card button
   */
  async clickAddCard(): Promise<void> {
    await this.addCardButton.click()
  }

  // State queries

  /**
   * Get the number of columns
   */
  async getColumnCount(): Promise<number> {
    return this.columns.count()
  }

  /**
   * Get the number of cards in a column
   */
  async getCardCountInColumn(columnHeader: string): Promise<number> {
    return this.getCardsInColumn(columnHeader).count()
  }

  /**
   * Get all column header texts
   */
  async getColumnHeaders(): Promise<string[]> {
    return this.columnHeaders.allTextContents()
  }

  /**
   * Check if a card exists in a specific column
   */
  async isCardInColumn(cardText: string, columnHeader: string): Promise<boolean> {
    const cardsInColumn = this.getCardsInColumn(columnHeader)
    const count = await cardsInColumn.filter({ hasText: cardText }).count()
    return count > 0
  }

  /**
   * Get total card count across all columns
   */
  async getTotalCardCount(): Promise<number> {
    return this.cards.count()
  }

  /**
   * Alias for getTotalCardCount
   */
  async getCardCount(): Promise<number> {
    return this.getTotalCardCount()
  }

  /**
   * Check if new/create button is visible
   */
  async hasNewButton(): Promise<boolean> {
    const newButton = this.page.locator('a[href*="/form/create"], button:has-text("Create"), button:has-text("New"), .btn-create, .btn-new').first()
    return newButton.isVisible()
  }

  /**
   * Check if any card contains the specified text
   */
  async hasCardWithText(text: string): Promise<boolean> {
    const matchingCard = this.cards.filter({ hasText: text })
    return (await matchingCard.count()) > 0
  }

  /**
   * Get full page content for assertions
   */
  async getPageContent(): Promise<string> {
    return this.page.content()
  }
}

/**
 * Factory function to create KanbanPage for different boards
 */
export function createKanbanPage(page: Page, boardName: string): KanbanPage {
  return new KanbanPage(page, boardName)
}
