import { Page, Locator } from '@playwright/test'

/**
 * Base page object that all page objects extend from.
 * Provides common functionality and enforces the Page Object pattern.
 *
 * Following Martin Fowler's Page Object principles:
 * - Page objects encapsulate UI structure
 * - Page objects should not contain assertions
 * - Methods return page objects for navigation
 * - Public methods represent services the page offers
 */
export abstract class BasePage {
  constructor(protected readonly page: Page) {}

  /**
   * Navigate to this page's URL
   * Subclasses should implement this with their specific URL
   */
  abstract goto(): Promise<void>

  /**
   * Wait for the page to be fully loaded
   * Subclasses should implement this with their specific loading indicator
   */
  abstract waitForLoad(): Promise<void>

  /**
   * Navigate to this page and wait for it to be fully loaded.
   * Combines goto() + waitForLoad() - use this in tests for cleaner code.
   */
  async visit(): Promise<void> {
    await this.goto()
    await this.waitForLoad()
  }

  // Common UI elements present across all pages

  /** The main sidebar navigation */
  get sidebar(): Locator {
    return this.page.locator('.sidebar').first()
  }

  /** Sidebar navigation items */
  get sidebarNavItems(): Locator {
    return this.page.locator('.sidebar .nav-item')
  }

  /** The main content area */
  get mainContent(): Locator {
    return this.page.locator('main, .content, .main-content').first()
  }

  /** Loading indicator */
  get loadingIndicator(): Locator {
    return this.page.locator('.loading, .spinner, [data-loading]')
  }

  // Common actions available from any page

  /**
   * Navigate to the dashboard
   */
  async gotoDashboard(): Promise<void> {
    await this.page.goto('/dashboard')
  }

  /**
   * Navigate using the sidebar
   */
  async clickSidebarLink(text: string): Promise<void> {
    const navItem = this.page.locator(`.sidebar .nav-item:has-text("${text}")`).first()
    await navItem.waitFor({ state: 'visible', timeout: 10000 })
    await navItem.click()
  }

  /**
   * Navigate to a sidebar item by its label text
   * Waits for sidebar to be loaded before clicking
   */
  async navigateToSidebarItem(label: string): Promise<void> {
    // Wait for sidebar navigation items to load (API-driven)
    await this.page.locator('.sidebar .sidebar-nav .nav-item').first().waitFor({ state: 'visible', timeout: 10000 })
    // Find and click the item
    const navItem = this.page.locator(`.sidebar .nav-item:has-text("${label}")`).first()
    await navItem.waitFor({ state: 'visible', timeout: 10000 })
    await navItem.click()
  }

  /**
   * Get the current URL path
   */
  async getCurrentPath(): Promise<string> {
    return new URL(this.page.url()).pathname
  }

  /**
   * Wait for navigation to complete
   */
  async waitForNavigation(urlPattern: RegExp): Promise<void> {
    await this.page.waitForURL(urlPattern)
  }

  /**
   * Get page content as text (useful for broad content checks)
   */
  async getPageContent(): Promise<string> {
    return this.page.content()
  }

  /**
   * Check if text is visible on the page
   */
  async isTextVisible(text: string): Promise<boolean> {
    return this.page.getByText(text).isVisible()
  }

  /**
   * Wait for text to appear on the page
   */
  async waitForText(text: string, timeout = 10000): Promise<void> {
    await this.page.getByText(text).waitFor({ state: 'visible', timeout })
  }
}
