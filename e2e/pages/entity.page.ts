import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class EntityPage extends BasePage {
  readonly detailContainer: Locator;
  readonly heading: Locator;
  readonly editButton: Locator;
  readonly deleteButton: Locator;
  readonly typeBadge: Locator;
  /** Rendered document body inside the documents panel (if any). */
  readonly documentBody: Locator;
  /** Document panel tab selector (only rendered when >1 doc applies). */
  readonly documentSelector: Locator;

  constructor(page: Page) {
    super(page);
    this.detailContainer = page.locator('.entity-detail').first();
    this.heading = page.locator('main h1').first();
    this.editButton = page.locator('a:has-text("Edit"), button:has-text("Edit")').first();
    this.deleteButton = page.locator('button:has-text("Delete")').first();
    this.typeBadge = page.locator('.entity-type-badge');
    this.documentBody = page.locator('.document-body').first();
    this.documentSelector = page.locator('.documents-panel .doc-select');
  }

  /** Wait for the documents panel to render (the body becomes visible
   *  once the server responds with HTML). If the entity has no
   *  applicable docs, this waits in vain — callers should only invoke
   *  when the fixture configures a doc for this entity type. */
  async waitForDocumentBody() {
    await expect(this.documentBody).toBeVisible({ timeout: 10_000 });
  }

  /** Force the documents-panel tab selection when the entity has
   *  multiple applicable docs. No-op when the selector isn't rendered. */
  async selectDocument(name: string) {
    if (await this.documentSelector.isVisible({ timeout: 500 }).catch(() => false)) {
      await this.documentSelector.selectOption(name);
    }
  }

  /** Click a link inside the rendered document body by its visible text. */
  async clickDocumentLink(text: string) {
    await this.documentBody.locator(`a:has-text("${text}")`).first().click();
  }

  /** Assert the document body contains the given text. */
  async expectDocumentBodyContains(text: string) {
    await expect(this.documentBody).toContainText(text);
  }

  async navigateToEntity(entityType: string, id: string) {
    await this.navigateTo(`/entity/${entityType}/${id}`);
    await this.waitForSpinnerToDisappear();
    await expect(this.heading).toBeVisible();
  }

  async expectHeadingText(text: string | RegExp) {
    const matcher = text instanceof RegExp ? text : new RegExp(text);
    await expect(this.heading.filter({ hasText: matcher })).toBeVisible();
  }

  async expectNoErrorState() {
    await expect(this.page.getByText('Entity not found')).not.toBeVisible();
    await expect(this.page.getByText('Failed to load')).not.toBeVisible();
  }

  async expectPropertyValue(value: string | RegExp) {
    const matcher = value instanceof RegExp ? value : new RegExp(value);
    await expect(this.page.locator('main').getByText(matcher).first()).toBeVisible();
  }

  /** Click the Edit action on the entity view to navigate to the edit form. */
  async clickEdit() {
    await expect(this.editButton).toBeVisible();
    await this.editButton.click();
    await this.page.waitForURL(/\/form\//);
  }

  async clickRelationLink(targetId: string) {
    const link = this.page.locator('button.relation-link').filter({ hasText: targetId });
    await expect(link).toBeVisible();
    await link.click();
  }

  async expectTypeBadge(type: string | RegExp) {
    const matcher = type instanceof RegExp ? type : new RegExp(type, 'i');
    await expect(this.typeBadge.filter({ hasText: matcher }).first()).toBeVisible();
  }

  async hasEditButton(): Promise<boolean> {
    return this.editButton.isVisible();
  }

  /** True when the inaccessible (encrypted) banner is rendered. */
  get inaccessibleBanner(): Locator {
    return this.page.locator('.inaccessible-banner');
  }

  async expectInaccessibleBanner() {
    await expect(this.inaccessibleBanner).toBeVisible();
    await expect(this.inaccessibleBanner).toContainText(/git-crypt/i);
  }

  /** Count of property values rendered as locked placeholders in the
   *  detail view. Each schema property of a fully-encrypted entity
   *  produces one such marker. */
  async lockedPropertyCount(): Promise<number> {
    return this.detailContainer.locator('.property-inaccessible').count();
  }

  async containsText(text: string): Promise<boolean> {
    return this.page.getByText(text).first().isVisible();
  }

  /** Check that a property value is rendered inside the entity-detail container
   *  (scoped to avoid matching nav/sidebar elements). */
  async hasPropertyValue(value: string): Promise<boolean> {
    return this.detailContainer.getByText(value, { exact: true }).first().isVisible();
  }

  /** True if the entity-detail body contains any blocking-relation text. */
  async hasBlockingRelationsSection(): Promise<boolean> {
    const text = (await this.detailContainer.textContent()) ?? '';
    return /block/i.test(text);
  }

  async detailTextContains(pattern: RegExp | string): Promise<boolean> {
    const text = (await this.detailContainer.textContent()) ?? '';
    if (typeof pattern === 'string') return text.toLowerCase().includes(pattern.toLowerCase());
    return pattern.test(text);
  }

  // --- checkbox body-content helpers ---

  get contentBody(): Locator {
    return this.page.locator('.content-body');
  }

  get checkboxStats(): Locator {
    return this.page.locator('.cb-stats');
  }

  async hasCheckboxStats(): Promise<boolean> {
    return this.checkboxStats.isVisible().catch(() => false);
  }

  async getCheckboxStatsText(): Promise<string> {
    return (await this.checkboxStats.textContent()) ?? '';
  }

  async contentCheckboxCount(): Promise<number> {
    return this.contentBody.locator('input[type="checkbox"]').count();
  }

  /** Click the checkbox with data-cb-idx="index" in the content body.
   *  `force: true` because GFM-rendered checkboxes are disabled by default —
   *  Vue installs a click handler that toggles via the API regardless. */
  async clickContentCheckbox(index: number): Promise<void> {
    await this.contentBody
      .locator(`input[type="checkbox"][data-cb-idx="${index}"]`)
      .click({ force: true });
  }

  async contentCheckboxIsChecked(index: number): Promise<boolean> {
    return this.contentBody
      .locator(`input[type="checkbox"][data-cb-idx="${index}"]`)
      .isChecked();
  }
}
