import { Page, Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Page object for the Dashboard view.
 * Handles dashboard-specific interactions.
 */
export class DashboardPage extends BasePage {
  constructor(page: Page) {
    super(page)
  }

  // Selectors

  /** The dashboard container */
  get dashboardContainer(): Locator {
    return this.page.locator('.dashboard-view').first()
  }

  /** Dashboard header (shows when loaded successfully) */
  get dashboardHeader(): Locator {
    return this.page.locator('.dashboard-header')
  }

  /** Loading state indicator */
  get loadingState(): Locator {
    return this.page.locator('.dashboard-view .loading-state')
  }

  /** Dashboard cards/widgets */
  get cards(): Locator {
    return this.page.locator('.dashboard-card')
  }

  /** Statistic values (count numbers) */
  get statValues(): Locator {
    return this.page.locator('.count-number')
  }

  /** Chart containers */
  get charts(): Locator {
    return this.page.locator('.chart, canvas, svg, [data-chart]')
  }

  /** Quick action buttons */
  get quickActions(): Locator {
    return this.page.locator('.quick-action, .action-button, [data-action]')
  }

  /** Recent activity section */
  get recentActivity(): Locator {
    return this.page.locator('.recent-activity, .activity-list, [data-testid="recent-activity"]')
  }

  // Page navigation

  async goto(): Promise<void> {
    await this.page.goto('/dashboard')
  }

  async waitForLoad(): Promise<void> {
    await this.dashboardContainer.waitFor({ state: 'visible', timeout: 10000 })
    // Wait for loading to complete and either dashboard-header or dashboard-grid to appear
    await this.page
      .locator('.dashboard-header, .dashboard-grid')
      .first()
      .waitFor({ state: 'visible', timeout: 10000 })
  }

  // Actions

  /**
   * Click on a dashboard card
   */
  async clickCard(cardTitle: string): Promise<void> {
    await this.page.locator(`.dashboard-card:has-text("${cardTitle}")`).first().click()
  }

  /**
   * Click a quick action button
   */
  async clickQuickAction(actionText: string): Promise<void> {
    await this.page.locator(`button:has-text("${actionText}"), a:has-text("${actionText}")`).first().click()
  }

  // State queries

  /**
   * Get the number of dashboard cards
   */
  async getCardCount(): Promise<number> {
    return this.cards.count()
  }

  /**
   * Get a statistic value by card title
   */
  async getStatValue(cardTitle: string): Promise<string> {
    const card = this.page.locator(`.dashboard-card:has-text("${cardTitle}")`).first()
    const value = card.locator('.count-number').first()
    return (await value.textContent()) || ''
  }

  /**
   * Check if dashboard has charts
   */
  async hasCharts(): Promise<boolean> {
    return this.charts.first().isVisible()
  }

  /**
   * Check if dashboard has recent activity section
   */
  async hasRecentActivity(): Promise<boolean> {
    return this.recentActivity.isVisible()
  }

  /**
   * Get all card titles
   */
  async getCardTitles(): Promise<string[]> {
    const titles = await this.cards.locator('.card-header h3').allTextContents()
    return titles.filter((t) => t.trim() !== '')
  }
}
