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
    this.titleInput = page.locator('#field-title');
    this.markdownEditor = page.locator('.EasyMDEContainer, .markdown-editor, textarea[name="content"]');
  }

  async navigateToCreateForm(formId: string) {
    await this.navigateTo(`/form/${formId}`);
    await this.waitForSpinnerToDisappear();
    await expect(this.titleInput).toBeVisible();
  }

  /** Fill a set of property fields keyed by property name. */
  async fillFields(values: Record<string, string>) {
    for (const [name, value] of Object.entries(values)) {
      await this.fillField(name, value);
    }
  }

  /** Fill a set of select fields keyed by property name. */
  async selectFields(values: Record<string, string>) {
    for (const [name, value] of Object.entries(values)) {
      await this.selectField(name, value);
    }
  }

  /** Submit the form and wait for the create POST to succeed. Returns the entity response. */
  async submitAndExpectCreate(plural: string): Promise<{ id: string }> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        r => r.url().includes(`/api/v1/${plural}`) && r.request().method() === 'POST' && r.status() === 201,
      ),
      this.submitButton.click(),
    ]);
    expect(response.ok()).toBeTruthy();
    return response.json() as Promise<{ id: string }>;
  }

  async navigateToEditForm(formId: string, entityId: string) {
    await this.navigateTo(`/form/${formId}/${entityId}`);
    await this.waitForSpinnerToDisappear();
  }

  async fillField(name: string, value: string) {
    const field = this.page.locator(`#field-${name}`);
    await field.fill(value);
  }

  async selectField(name: string, value: string) {
    const field = this.page.locator(`#field-${name}`);
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
    const startUrl = this.page.url();
    await this.submitButton.click();
    // Submit is a client-side fetch followed by a router.push on success. Wait
    // briefly for the URL to change; if validation fails we stay on the form.
    await this.page.waitForURL((url) => url.toString() !== startUrl, { timeout: 2000 }).catch(() => {});
  }

  async cancel() {
    const startUrl = this.page.url();
    await this.cancelButton.click();
    await this.page.waitForURL((url) => url.toString() !== startUrl, { timeout: 2000 }).catch(() => {});
  }

  async expectValidationError(message: string) {
    await expect(this.page.locator('.error, .validation-error, [role="alert"]').filter({ hasText: message })).toBeVisible();
  }

  async expectFieldValue(name: string, value: string) {
    const field = this.page.locator(`#field-${name}`);
    await expect(field).toHaveValue(value);
  }

  async expectFormTitle(title: string) {
    await expect(this.page.locator('h1, h2, .form-title').filter({ hasText: title })).toBeVisible();
  }

  async getFieldValue(name: string): Promise<string> {
    const field = this.page.locator(`#field-${name}`);
    return field.inputValue();
  }
}
