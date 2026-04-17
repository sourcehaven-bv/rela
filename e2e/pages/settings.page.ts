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
    this.saveButton = page.locator('button[type="submit"], button:has-text("Save")');
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
}
