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

    if (await select.isVisible().catch(() => false)) {
      await select.selectOption(targetId);
      return;
    }
    // Default RelationPicker widget: a "Search <targetType>" text input that
    // surfaces matches as .dropdown-item. Type the id (which is the most
    // unambiguous search key) and click the first match.
    const search = relationSection.locator('input[placeholder^="Search "]').first();
    await search.fill(targetId);
    await this.page.locator(`.dropdown-item:has-text("${targetId}")`).first().click({ timeout: 5000 });
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

  async hasField(name: string): Promise<boolean> {
    return this.page.locator(`#field-${name}`).isVisible();
  }

  async hasSubmitButton(): Promise<boolean> {
    return this.submitButton.first().isVisible();
  }

  async hasValidationError(): Promise<boolean> {
    const error = this.page.locator('.error, .validation-error, [role="alert"]').first();
    return error.isVisible().catch(() => false);
  }

  async getSelectOptions(name: string): Promise<string[]> {
    const options = this.page.locator(`#field-${name} option`);
    const count = await options.count();
    const values: string[] = [];
    for (let i = 0; i < count; i++) {
      const v = await options.nth(i).getAttribute('value');
      if (v) values.push(v);
    }
    return values;
  }

  /** Submit and wait for the save PATCH on the given entity, returning the response. */
  async saveAndWaitForPatch(plural: string, entityId: string) {
    const [resp] = await Promise.all([
      this.page.waitForResponse(
        (r) => r.url().includes(`/api/v1/${plural}/${entityId}`) && r.request().method() === 'PATCH',
      ),
      this.submitButton.first().click(),
    ]);
    return resp;
  }

  /** True if the page shows an inline-create UI for related entities. */
  async hasInlineCreateButton(): Promise<boolean> {
    return this.page
      .locator('button:has-text("New"), .btn-inline-create, [data-create-inline]')
      .first()
      .isVisible()
      .catch(() => false);
  }

  async clickInlineCreateButton() {
    await this.page
      .locator('button:has-text("New"), .btn-inline-create, [data-create-inline]')
      .first()
      .click();
  }

  async expectInlineFormVisible() {
    await expect(this.page.locator('.inline-form, .modal, dialog')).toBeVisible({ timeout: 5000 });
  }

  // --- Markdown body editor (EasyMDE) ---

  get contentField(): Locator {
    return this.page.locator('.content-field');
  }

  get markdownEditorRoot(): Locator {
    return this.page.locator('.markdown-editor');
  }

  get markdownToolbar(): Locator {
    return this.page.locator('.editor-toolbar');
  }

  get codeMirror(): Locator {
    return this.page.locator('.CodeMirror');
  }

  async expectContentFieldVisible() {
    await expect(this.contentField).toBeVisible();
  }

  async expectContentLabelHasText(text: string) {
    await expect(this.contentField.locator('label')).toHaveText(text);
  }

  async expectMarkdownEditorReady() {
    await expect(this.markdownEditorRoot).toBeVisible();
    await expect(this.markdownToolbar).toBeVisible({ timeout: 10000 });
    await expect(this.codeMirror).toBeVisible({ timeout: 10000 });
  }

  async typeMarkdownBody(text: string) {
    await this.codeMirror.click();
    await this.page.keyboard.type(text);
  }

  // --- Template selector ---

  get templateSelector(): Locator {
    return this.page.locator('.template-selector');
  }

  get templatePills(): Locator {
    return this.page.locator('.template-pill');
  }

  async templatePillCount(): Promise<number> {
    return this.templatePills.count();
  }

  async templateSelectorVisible(): Promise<boolean> {
    return this.templateSelector.isVisible().catch(() => false);
  }

  async clickTemplatePill(index: number) {
    await this.templatePills.nth(index).click();
  }

  async expectTemplatePillActive(index: number) {
    await expect(this.templatePills.nth(index)).toHaveClass(/active/);
  }
}
