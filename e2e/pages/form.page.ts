import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class FormPage extends BasePage {
  readonly form: Locator;
  readonly submitButton: Locator;
  readonly cancelButton: Locator;
  readonly titleInput: Locator;
  readonly markdownEditor: Locator;

  constructor(page: Page) {
    super(page);
    this.form = page.locator('form');
    this.submitButton = page.locator('button[type="submit"], button:has-text("Save"), button:has-text("Create")');
    this.cancelButton = page.locator('button:has-text("Cancel")');
    this.titleInput = page.locator('input[name="title"], input#title, input[id*="title"]');
    this.markdownEditor = page.locator('.EasyMDEContainer, .markdown-editor, textarea[name="content"]');
  }

  async navigateToCreateForm(formId: string) {
    await this.navigateTo(`/form/${formId}`);
    await this.waitForSpinnerToDisappear();
  }

  async navigateToEditForm(formId: string, entityId: string) {
    await this.navigateTo(`/form/${formId}/${entityId}`);
    await this.waitForSpinnerToDisappear();
  }

  async fillField(name: string, value: string) {
    const field = this.page.locator(`input[name="${name}"], input#${name}, textarea[name="${name}"]`);
    await field.fill(value);
  }

  async selectField(name: string, value: string) {
    const field = this.page.locator(`select[name="${name}"], select#${name}`);
    await field.selectOption(value);
  }

  async fillTitle(value: string) {
    await this.titleInput.fill(value);
  }

  async fillMarkdown(value: string) {
    // EasyMDE creates a CodeMirror instance
    const codeMirror = this.page.locator('.CodeMirror');
    if (await codeMirror.isVisible({ timeout: 1000 }).catch(() => false)) {
      await codeMirror.click();
      // Clear existing content
      await this.page.keyboard.press('Meta+a');
      await this.page.keyboard.type(value);
    } else {
      // Fallback to textarea
      const textarea = this.page.locator('textarea[name="content"], textarea.markdown-input');
      await textarea.fill(value);
    }
  }

  async addRelation(relationName: string, targetId: string) {
    // Find the relation picker for this relation
    const relationSection = this.page.locator('.relation-field, .form-field').filter({ hasText: relationName });
    const select = relationSection.locator('select');

    if (await select.isVisible()) {
      await select.selectOption(targetId);
    } else {
      // Multi-select or tag picker
      const input = relationSection.locator('input');
      await input.fill(targetId);
      await this.page.locator(`[data-value="${targetId}"], .option:has-text("${targetId}")`).click();
    }
  }

  async submit() {
    await this.submitButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  async cancel() {
    await this.cancelButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  async expectValidationError(message: string) {
    await expect(this.page.locator('.error, .validation-error, [role="alert"]').filter({ hasText: message })).toBeVisible();
  }

  async expectFieldValue(name: string, value: string) {
    const field = this.page.locator(`input[name="${name}"], input#${name}, select[name="${name}"]`);
    await expect(field).toHaveValue(value);
  }

  async expectFormTitle(title: string) {
    await expect(this.page.locator('h1, h2, .form-title').filter({ hasText: title })).toBeVisible();
  }

  async getFieldValue(name: string): Promise<string> {
    const field = this.page.locator(`input[name="${name}"], input#${name}`);
    return field.inputValue();
  }
}
