import { Page, Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Page object for the Entity Detail view.
 * Handles displaying individual entity information.
 */
export class EntityDetailPage extends BasePage {
  private entityType: string
  private entityId: string

  constructor(page: Page, entityType: string, entityId: string) {
    super(page)
    this.entityType = entityType
    this.entityId = entityId
  }

  // Selectors

  /** The main detail view container */
  get detailContainer(): Locator {
    return this.page.locator('.entity-detail').first()
  }

  /** Loading state indicator */
  get loadingState(): Locator {
    return this.page.locator('.entity-detail .loading-state')
  }

  /** Error state (entity not found) */
  get errorState(): Locator {
    return this.page.locator('.entity-detail .error-state')
  }

  /** Detail header (shows when entity loaded successfully) */
  get detailHeader(): Locator {
    return this.page.locator('.entity-detail .detail-header')
  }

  /** The entity title/header */
  get titleElement(): Locator {
    return this.page.locator('h1, .entity-title, .detail-title').first()
  }

  /** The edit button */
  get editButton(): Locator {
    return this.page.locator('a[href*="/form/edit"], button:has-text("Edit"), .btn-edit').first()
  }

  /** The delete button */
  get deleteButton(): Locator {
    return this.page.locator('button:has-text("Delete"), .btn-delete, [data-action="delete"]').first()
  }

  /** Property sections */
  get propertySections(): Locator {
    return this.page.locator('.property-section, .detail-section, section')
  }

  /** Relation sections */
  get relationSections(): Locator {
    return this.page.locator('.relation-section, .relations, [data-testid*="relation"]')
  }

  // Page navigation

  async goto(): Promise<void> {
    await this.page.goto(`/entity/${this.entityType}/${this.entityId}`)
  }

  async waitForLoad(): Promise<void> {
    // Wait for the entity-detail container to appear
    await this.detailContainer.waitFor({ state: 'visible', timeout: 10000 })
    // Then wait for either the content header or error state (loading complete)
    await this.page
      .locator('.entity-detail .detail-header, .entity-detail .error-state')
      .waitFor({ state: 'visible', timeout: 10000 })
  }

  // Actions

  /**
   * Click the edit button to navigate to edit form
   */
  async clickEdit(): Promise<void> {
    await this.editButton.click()
  }

  /**
   * Alias for clickEdit
   */
  async clickEditButton(): Promise<void> {
    await this.clickEdit()
  }

  /**
   * Click the delete button
   */
  async clickDelete(): Promise<void> {
    await this.deleteButton.click()
  }

  /**
   * Navigate to a related entity by clicking its link
   */
  async clickRelatedEntity(entityId: string): Promise<void> {
    await this.page.locator(`a[href*="${entityId}"]`).first().click()
  }

  // State queries

  /**
   * Get the displayed title text
   */
  async getTitle(): Promise<string> {
    return (await this.titleElement.textContent()) || ''
  }

  /**
   * Check if a specific property value is displayed in the detail container
   */
  async hasPropertyValue(value: string): Promise<boolean> {
    // Scope to the entity-detail container to avoid matching nav elements
    return this.detailContainer.getByText(value, { exact: true }).first().isVisible()
  }

  /**
   * Check if page contains specific text
   */
  async containsText(text: string): Promise<boolean> {
    const content = await this.page.content()
    return content.includes(text)
  }

  /**
   * Check if a specific property is shown (alias for hasPropertyValue)
   */
  async hasProperty(value: string): Promise<boolean> {
    return this.hasPropertyValue(value)
  }

  /**
   * Wait for specific text to appear (useful for Vue SPA rendering)
   */
  async waitForContent(text: string, timeout = 10000): Promise<void> {
    await this.page.getByText(text).waitFor({ state: 'visible', timeout })
  }

  /**
   * Check if edit button is visible
   */
  async hasEditButton(): Promise<boolean> {
    return this.editButton.isVisible()
  }

  /**
   * Check if content contains blocking relations
   */
  async hasBlockingRelations(): Promise<boolean> {
    const content = await this.page.content()
    return /blocked|blocking/i.test(content.toLowerCase())
  }

  /**
   * Get page content for broader assertions
   */
  async getContent(): Promise<string> {
    return this.page.content()
  }
}

/**
 * Factory function to create EntityDetailPage for different entity types
 */
export function createEntityDetailPage(page: Page, entityType: string, entityId: string): EntityDetailPage {
  return new EntityDetailPage(page, entityType, entityId)
}
