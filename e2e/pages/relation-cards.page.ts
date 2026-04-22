import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/** Helpers for the `widget: cards` relation UI on an edit form. Each widget
 *  renders as `.relation-cards`; each linked target as `.relation-card` with
 *  `.entity-id`, `.card-properties`, and a `.remove-btn`. */
export class RelationCardsPage extends BasePage {
  constructor(page: Page) {
    super(page);
  }

  async navigateToEdit(formId: string, entityId: string) {
    await this.navigateTo(`/form/${formId}/${entityId}`);
    await this.waitForSpinnerToDisappear();
    await expect(this.page.locator('.relation-cards').first()).toBeVisible({ timeout: 10000 });
  }

  get widgets(): Locator {
    return this.page.locator('.relation-cards');
  }

  widgetByLabel(label: string): Locator {
    return this.page.locator('.relation-cards').filter({ has: this.page.locator(`.section-label:has-text("${label}")`) });
  }

  cardByTargetId(targetId: string): Locator {
    return this.page.locator('.relation-card', {
      has: this.page.locator(`.entity-id:has-text("${targetId}")`),
    }).first();
  }

  async widgetCount(): Promise<number> {
    return this.widgets.count();
  }

  async sectionLabels(): Promise<string[]> {
    return this.page.locator('.relation-cards .section-label').allTextContents();
  }

  async cardCount(widget: Locator): Promise<number> {
    return widget.locator('.relation-card').count();
  }

  async cardPropertyLabels(card: Locator): Promise<string[]> {
    return card.locator('.card-properties .prop-label').allTextContents();
  }

  async getTextInputValue(card: Locator): Promise<string> {
    return card.locator('.card-properties input.inline-edit').first().inputValue();
  }

  async editTextInput(card: Locator, value: string) {
    await card.locator('.card-properties input.inline-edit').first().fill(value);
  }

  /** True if at least one widget shows an unsaved badge. */
  async hasAnyUnsavedBadge(): Promise<boolean> {
    return this.page.locator('.pending-badge').first().isVisible();
  }

  async expectUnsavedBadgeOn(widget: Locator) {
    await expect(widget.locator('.pending-badge')).toBeVisible({ timeout: 3000 });
  }

  async expectNoUnsavedBadgeOn(widget: Locator) {
    await expect(widget.locator('.pending-badge')).toHaveCount(0);
  }

  async clickRemoveFirstCardIn(widget: Locator) {
    await widget.locator('.relation-card .remove-btn').first().click();
  }

  async getFirstCardEntityId(widget: Locator): Promise<string> {
    const text = await widget.locator('.relation-card .entity-id').first().textContent();
    return text?.trim() ?? '';
  }

  async expectCardHasClass(card: Locator, cls: string | RegExp) {
    const matcher = cls instanceof RegExp ? cls : new RegExp(cls);
    await expect(card).toHaveClass(matcher);
  }

  /** Click "+ Add" on a widget, search by `searchText` (bleve indexes titles,
   *  not IDs, so pass something that appears in the entity), click the match
   *  whose id is `targetId`, fill the reason text (required), click Link.
   *  Returns the newly-linked card locator. */
  async linkTargetByIdWithSearch(
    widget: Locator,
    targetId: string,
    searchText: string,
    reason: string,
  ): Promise<Locator> {
    await widget.locator('.add-btn').click();
    const search = widget.locator('.search-input');
    await expect(search).toBeVisible();
    await search.fill(searchText);
    const result = widget.locator('.search-result', { hasText: targetId });
    await expect(result).toBeVisible({ timeout: 10000 });
    await result.click();

    const meta = widget.locator('.new-meta-fields');
    await expect(meta).toBeVisible();
    await meta.locator('input[type="text"]').first().fill(reason);

    const link = widget.locator('.btn-primary', { hasText: 'Link' });
    await expect(link).toBeEnabled();
    await link.click();

    return widget
      .locator('.relation-card', { has: this.page.locator(`.entity-id:has-text("${targetId}")`) })
      .first();
  }

  get saveButton(): Locator {
    return this.page.locator('button[type="submit"], button:has-text("Save")').first();
  }

  async saveAndWaitForNavigation() {
    await Promise.all([
      this.page.waitForURL((url) => !url.pathname.includes('/form/'), { timeout: 10000 }),
      this.saveButton.click(),
    ]);
  }

  async save() {
    await this.saveButton.click();
  }

  async expectDateInputVisibleIn(widget: Locator) {
    await expect(widget.locator('input[type="date"]').first()).toBeVisible();
  }

  async expectNumberInputVisibleIn(widget: Locator) {
    await expect(widget.locator('input[type="number"]').first()).toBeVisible();
  }

  async expectCheckboxVisibleIn(widget: Locator) {
    await expect(widget.locator('input[type="checkbox"]').first()).toBeVisible();
  }

  async expectNoPendingBadges() {
    await expect(this.page.locator('.pending-badge')).toHaveCount(0, { timeout: 5000 });
  }
}
