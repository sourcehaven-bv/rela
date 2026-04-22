import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class SettingsPage extends BasePage {
  readonly form: Locator;
  readonly saveButton: Locator;
  readonly resetButton: Locator;
  readonly propertyDefaultsCard: Locator;
  readonly relationDefaultsCard: Locator;
  readonly overridesCard: Locator;
  readonly appInfoCard: Locator;

  constructor(page: Page) {
    super(page);
    this.form = page.locator('.settings-form, form');
    this.saveButton = page.locator('.form-actions button[type="submit"]');
    this.resetButton = page.locator('.form-actions button:has-text("Reset")');
    this.propertyDefaultsCard = page.locator('.settings-card').filter({ hasText: 'Property Defaults' });
    this.relationDefaultsCard = page.locator('.settings-card').filter({ hasText: 'Relation Defaults' });
    this.overridesCard = page.locator('.settings-card').filter({ hasText: 'Overrides' });
    this.appInfoCard = page.locator('.settings-card').filter({ hasText: 'Application Info' });
  }

  async navigateToSettings() {
    await this.navigateTo('/settings');
    await this.waitForSpinnerToDisappear();
  }

  async addPropertyDefault(propertyName: string, value: string) {
    // Select property from dropdown
    const addSelect = this.propertyDefaultsCard.locator('select').filter({ hasText: 'Add property default' });
    await addSelect.selectOption(propertyName);

    // Fill the value
    const row = this.propertyDefaultsCard.locator('.settings-row').filter({ hasText: propertyName });
    const input = row.locator('input, select').first();

    if (await input.evaluate(el => el.tagName.toLowerCase()) === 'select') {
      await input.selectOption(value);
    } else {
      await input.fill(value);
    }
  }

  async removePropertyDefault(propertyName: string) {
    const row = this.propertyDefaultsCard.locator('.settings-row').filter({ hasText: propertyName });
    await row.locator('.remove-btn').click();
  }

  async getPropertyDefaultValue(propertyName: string): Promise<string> {
    const row = this.propertyDefaultsCard.locator('.settings-row').filter({ hasText: propertyName });
    const input = row.locator('input, select').first();
    return input.inputValue();
  }

  async addRelationDefault(relationName: string, targetId: string) {
    const addSelect = this.relationDefaultsCard.locator('select').filter({ hasText: 'Add relation default' });
    await addSelect.selectOption(relationName);

    const row = this.relationDefaultsCard.locator('.settings-row').filter({ hasText: relationName });
    await row.locator('select').first().selectOption(targetId);
  }

  async removeRelationDefault(relationName: string) {
    const row = this.relationDefaultsCard.locator('.settings-row').filter({ hasText: relationName });
    await row.locator('.remove-btn').click();
  }

  async addOverrideGroup() {
    await this.overridesCard.locator('button:has-text("+ Add override group")').click();
  }

  async removeOverrideGroup(index: number) {
    const groups = this.overridesCard.locator('.override-group');
    await groups.nth(index).locator('.remove-btn.large').click();
  }

  async setOverrideEntityTypes(groupIndex: number, types: string[]) {
    const group = this.overridesCard.locator('.override-group').nth(groupIndex);
    const tagSelect = group.locator('.tag-select, .override-types');

    for (const type of types) {
      await tagSelect.locator('input').fill(type);
      await this.page.locator(`.option:has-text("${type}")`).click();
    }
  }

  async save() {
    await this.saveButton.click();
    await this.waitForSpinnerToDisappear();
  }

  async reset() {
    await this.resetButton.click();
    await this.waitForSpinnerToDisappear();
  }

  async expectSaveSuccess() {
    await this.waitForToast('saved');
  }

  async expectPropertyDefaultExists(propertyName: string) {
    const row = this.propertyDefaultsCard.locator('.settings-row').filter({ hasText: propertyName });
    await expect(row).toBeVisible();
  }

  async expectPropertyDefaultNotExists(propertyName: string) {
    const row = this.propertyDefaultsCard.locator('.settings-row').filter({ hasText: propertyName });
    await expect(row).not.toBeVisible();
  }

  async expectAppInfo(field: string, value: string) {
    const row = this.appInfoCard.locator('.info-row').filter({ hasText: field });
    await expect(row.locator('.info-value')).toHaveText(value);
  }

  async getOverrideGroupCount(): Promise<number> {
    return this.overridesCard.locator('.override-group').count();
  }

  /** Select the first available option from the "Add property default" dropdown
   *  (index 0 is the placeholder, so we pick index 1) and wait for the new
   *  row to render. Returns the property name selected. */
  async addFirstAvailablePropertyDefault(): Promise<string> {
    const addSelect = this.propertyDefaultsCard.locator('select').last();
    const firstOption = addSelect.locator('option').nth(1);
    const value = await firstOption.getAttribute('value');
    if (!value) throw new Error('No available property to add as default');
    const beforeRows = await this.propertyDefaultsCard.locator('.settings-row').count();
    await addSelect.selectOption(value);
    // Wait for the settings-row to appear so follow-up interactions don't race
    // the re-render.
    await expect(async () => {
      const now = await this.propertyDefaultsCard.locator('.settings-row').count();
      expect(now).toBeGreaterThan(beforeRows);
    }).toPass({ timeout: 5000 });
    return value;
  }

  /** Set the value of the last-added property default row. If the input is a
   *  select, selects the first non-placeholder option (callers rarely care
   *  which value, just that the change round-trips). If a text field, fills
   *  the provided string. */
  async setLastPropertyDefaultValue(fallbackText: string): Promise<void> {
    const row = this.propertyDefaultsCard.locator('.settings-row').last();
    const input = row.locator('input, select').first();
    await expect(input).toBeVisible();
    const tag = await input.evaluate((el) => el.tagName.toLowerCase());
    if (tag === 'select') {
      await input.selectOption({ index: 1 });
    } else {
      await input.fill(fallbackText);
    }
  }

  async getPropertyDefaultRowCount(): Promise<number> {
    return this.propertyDefaultsCard.locator('.settings-row').count();
  }

  async removeFirstPropertyDefault(): Promise<void> {
    await this.propertyDefaultsCard.locator('.remove-btn').first().click();
  }
}
