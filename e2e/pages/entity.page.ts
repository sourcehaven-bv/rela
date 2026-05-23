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

  /** Wait for the page heading to render. Use after navigating to a
   *  detail URL directly without going through navigateToEntity. */
  async waitForHeading(timeoutMs = 10_000) {
    await this.heading.waitFor({ timeout: timeoutMs });
  }

  /** Assert no Edit button is rendered. Used to verify the AC10
   *  read-only payoff: per-entity `_actions.update=false` hides the
   *  Edit affordance. */
  async expectNoEditButton() {
    await expect(this.page.getByRole('button', { name: /^Edit/ })).toHaveCount(0);
  }

  /** Assert no Delete button is rendered. Used to verify the AC10
   *  read-only payoff: per-entity `_actions.delete=false` hides the
   *  Delete affordance. */
  async expectNoDeleteButton() {
    await expect(this.page.getByRole('button', { name: /^Delete/ })).toHaveCount(0);
  }

  async clickRelationLink(targetId: string) {
    // Detail screens render related entities as cards / list items with a
    // data-entity-id attribute on the row root and a clickable header
    // (cards) or anchor (list).
    const item = this.page.locator(`[data-entity-id="${targetId}"]`).first();
    await expect(item).toBeVisible();
    const trigger = item.locator('.card-header, .list-link').first();
    await trigger.click();
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
   *  The Vue handler calls preventDefault(), PATCHes the entity with the
   *  toggled content, and reactively splices the updated entity back into
   *  viewData — so the rendered checked state tracks the server source
   *  without a full-view refetch (and without the flicker that refetching
   *  the entity detail tree would cause). */
  async clickContentCheckbox(index: number): Promise<void> {
    await this.contentBody
      .locator(`input[type="checkbox"][data-cb-idx="${index}"]`)
      .click();
  }

  async contentCheckboxIsChecked(index: number): Promise<boolean> {
    return this.contentBody
      .locator(`input[type="checkbox"][data-cb-idx="${index}"]`)
      .isChecked();
  }

  // --- content entity-reference helpers ---

  /** Locator for an in-content link rewritten from a `\`<id>\`` code span
   *  to a navigable entity detail link (TKT-747O). Returns the <a> element
   *  whose href routes to the target's detail page. */
  contentEntityRefLink(entityType: string, id: string): Locator {
    return this.contentBody.locator(`a[href="/entity/${entityType}/${id}"]`).first();
  }

  /** Locator for an inline `<code>` element in the entity content with the
   *  given exact text. Used in negative tests to assert a code span was
   *  NOT rewritten into an anchor — e.g. unknown IDs or fenced blocks. */
  contentCodeSpan(text: string): Locator {
    return this.contentBody.locator('code', { hasText: text });
  }

  /** Click the in-content entity-reference link for `(entityType, id)` and
   *  wait for the SPA route to settle on the target page. */
  async clickContentEntityRef(entityType: string, id: string): Promise<void> {
    await this.contentEntityRefLink(entityType, id).click();
    await this.page.waitForURL(new RegExp(`/entity/${entityType}/${id}(\\?|$)`));
  }
}
