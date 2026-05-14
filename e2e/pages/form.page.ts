import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class FormPage extends BasePage {
  readonly form: Locator;
  readonly submitButton: Locator;
  readonly cancelButton: Locator;
  readonly titleInput: Locator;

  constructor(page: Page) {
    super(page);
    this.form = page.locator('form');
    this.submitButton = page.locator('button[type="submit"], button:has-text("Save"), button:has-text("Create")');
    this.cancelButton = page.locator('button:has-text("Cancel")');
    this.titleInput = page.locator('#field-title');
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

  /** Wait for the URL to be the edit form for the given form/entity. Used
   *  when navigation is triggered by another page (e.g. Edit button on the
   *  document view). */
  async expectAtFormUrl(formId: string, entityId: string) {
    await this.page.waitForURL(new RegExp(`/form/${formId}/${entityId}`), { timeout: 10000 });
    await this.waitForSpinnerToDisappear();
  }

  /** Decoded value of the `return_to` query param on the current URL, or
   *  null if absent. Useful for asserting on the form's return target
   *  without coupling to URL-encoding details (RR-BS0O4). */
  readReturnTo(): string | null {
    return new URL(this.page.url()).searchParams.get('return_to');
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
    // Legacy <select> path (kept for any form that still renders one) —
    // for the default RelationPicker widget we delegate to
    // `pickInRelationPicker` so both helpers share the same combobox-
    // scoped selector strategy.
    const relationSection = this.page.locator('.relation-field, .form-field').filter({ hasText: relationName });
    const select = relationSection.locator('select');
    if (await select.isVisible().catch(() => false)) {
      await select.selectOption(targetId);
      return;
    }
    await this.pickInRelationPicker(relationSection, targetId, targetId);
  }

  async submit() {
    const startUrl = this.page.url();
    // Wait briefly for the submit button to appear; in create mode the
    // form always has it. In autosave edit mode it never appears.
    const submitVisible = await this.submitButton
      .first()
      .waitFor({ state: 'visible', timeout: 2000 })
      .then(() => true)
      .catch(() => false);
    if (submitVisible) {
      await this.submitButton.first().click();
      // Submit is a client-side fetch followed by a router.push on success.
      // Wait briefly for the URL to change; if validation fails we stay on the form.
      await this.page.waitForURL((url) => url.toString() !== startUrl, { timeout: 2000 }).catch(() => {});
      return;
    }
    // TKT-E6094 autosave: no explicit Submit. Blur, wait for the autosave
    // PATCH(es) to drain, then navigate back; the route guard flushes any
    // pending edits before the URL changes.
    await this.page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await this.page.waitForTimeout(1000);
    await this.page.goBack();
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

  /** Submit and wait for the save PATCH on the given entity, returning
   *  the response. After TKT-E6094 (autosave) edit forms no longer have
   *  an explicit Save button: the PATCH fires automatically after a
   *  debounce. We detect either mode and wait for the same PATCH.
   *
   *  For autosave forms, the caller should have just performed an edit
   *  (fillField/selectField etc.) — this helper triggers a blur to flush
   *  the debounce and then waits for the PATCH. */
  async saveAndWaitForPatch(plural: string, entityId: string) {
    const submitVisible = await this.submitButton
      .first()
      .waitFor({ state: 'visible', timeout: 2000 })
      .then(() => true)
      .catch(() => false);
    if (submitVisible) {
      const [resp] = await Promise.all([
        this.page.waitForResponse(
          (r) => r.url().includes(`/api/v1/${plural}/${entityId}`) && r.request().method() === 'PATCH',
        ),
        this.submitButton.first().click(),
      ]);
      return resp;
    }
    // Autosave path: blur to flush pending input, then wait for the
    // PATCH chain to drain. Multiple edits in quick succession serialize
    // through the FIFO queue, so we wait for an idle window of no PATCH
    // activity. Bounded so a no-edit form doesn't hang.
    await this.page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    let lastResp: Awaited<ReturnType<typeof this.page.waitForResponse>> | undefined;
    // Poll for PATCH responses until 1s elapses without one.
    for (;;) {
      const resp = await this.page
        .waitForResponse(
          (r) => r.url().includes(`/api/v1/${plural}/${entityId}`) && r.request().method() === 'PATCH',
          { timeout: 1500 },
        )
        .catch(() => undefined);
      if (!resp) break;
      lastResp = resp;
    }
    return lastResp;
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
    await expect(this.page.locator('.inline-form, .modal, dialog')).toBeVisible();
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
    await expect(this.markdownToolbar).toBeVisible();
    await expect(this.codeMirror).toBeVisible();
  }

  async typeMarkdownBody(text: string) {
    await this.codeMirror.click();
    await this.page.keyboard.type(text);
  }

  // --- Entity-reference picker (TKT-I5NO) ---

  /** Toolbar button that opens the EntityPickerModal. Located by its
   *  title attribute rather than the icon class so the test survives
   *  icon-swap refactors. */
  get insertEntityRefButton(): Locator {
    return this.markdownToolbar.locator('button[title="Insert entity reference"]')
  }

  /** Picker modal overlay. Teleported to <body>, so we query the page
   *  rather than the form root. */
  get entityPickerOverlay(): Locator {
    return this.page.locator('.entity-picker-overlay')
  }

  get entityPickerInput(): Locator {
    return this.entityPickerOverlay.locator('.entity-picker-input')
  }

  get entityPickerOptions(): Locator {
    return this.entityPickerOverlay.locator('.entity-picker-option')
  }

  /** Click the toolbar button and wait for the modal to render. */
  async openEntityPicker(): Promise<void> {
    await this.insertEntityRefButton.click()
    await expect(this.entityPickerInput).toBeFocused()
  }

  /** Type a query into the picker and wait for results to render. The
   *  150ms client debounce plus Bleve commit latency means we wait up to
   *  five seconds before failing — generous enough that a freshly-created
   *  entity surfaces, tight enough that a real bug still fails fast. */
  async searchEntityPicker(query: string): Promise<void> {
    await this.entityPickerInput.fill(query)
    await expect(this.entityPickerOptions.first()).toBeVisible({ timeout: 5_000 })
  }

  /** Read the current CodeMirror buffer back as a single string. Used
   *  to assert the picker inserted the expected `<id>` code span. */
  async getMarkdownBody(): Promise<string> {
    return await this.codeMirror.evaluate((el) => {
      // CodeMirror v5 exposes the instance on the .CodeMirror node via the
      // global CodeMirror constructor. EasyMDE preserves the same shape;
      // every line in the document is concatenated with '\n'.
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cm = (el as any).CodeMirror
      return typeof cm?.getValue === 'function' ? (cm.getValue() as string) : ''
    })
  }

  // --- Inline backtick autocomplete (TKT-2RCP) ---

  /** Inline popup the markdown editor pops when the user types a
   *  backtick in prose context. Rendered next to the editor textarea
   *  (NOT teleported), so we scope to the editor container. */
  get backtickPopup(): Locator {
    return this.markdownEditorRoot.locator('.backtick-popup')
  }

  get backtickPopupOptions(): Locator {
    return this.backtickPopup.locator('.backtick-popup-option')
  }

  get backtickPopupHint(): Locator {
    return this.backtickPopup.locator('.backtick-popup-hint')
  }

  /** Type a single character into the editor at the current cursor.
   *  Uses execCommand insertText so EasyMDE's CodeMirror sees a real
   *  user-input event (mirrors the inputRead flow). Focuses the
   *  CodeMirror inputField explicitly because Playwright's click on
   *  `.CodeMirror` lands on the gutter wrapper, not the hidden
   *  textarea that captures input events. */
  async typeIntoEditor(text: string): Promise<void> {
    await this.codeMirror.click()
    await this.page.evaluate(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cm = (document.querySelector('.CodeMirror') as any)?.CodeMirror
      cm?.focus()
    })
    for (const ch of text) {
      await this.page.evaluate((c) => document.execCommand('insertText', false, c), ch)
    }
  }

  /** Shrink the inline-autocomplete open-delay to a small value for
   *  e2e timing. Without this, the test depends on Playwright's
   *  per-character `execCommand` latency to spread typing across the
   *  default 600 ms grace window — fragile on fast runners. Call
   *  before the editor mounts (i.e. before navigating to the form). */
  async useFastAutocompleteDelay(delayMs = 30): Promise<void> {
    await this.page.addInitScript((d) => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      ;(window as any).__BACKTICK_AUTOCOMPLETE_DELAY_MS__ = d
    }, delayMs)
  }

  /** Clear the editor's buffer entirely. Useful when a form preloads a
   *  template body and the test wants a known starting state. */
  async clearEditorBuffer(): Promise<void> {
    await this.codeMirror.click()
    await this.page.evaluate(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cm = (document.querySelector('.CodeMirror') as any)?.CodeMirror
      if (cm) {
        cm.setValue('')
        cm.focus()
        cm.setCursor({ line: 0, ch: 0 })
      }
    })
  }

  /** Wait for the inline autocomplete popup to appear. Default 1500 ms
   *  (well above the 600 ms open delay). */
  async waitForBacktickPopup(): Promise<void> {
    await this.backtickPopup.waitFor({ state: 'visible', timeout: 1_500 })
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

  // --- Relation picker (non-cards widget) ---

  /** Locate a non-cards relation picker by its rendered section label. The
   *  picker renders as `.form-field.relation-picker` with the label at the
   *  top; we filter on the label text. */
  relationPickerByLabel(label: string): Locator {
    return this.page
      .locator('.form-field.relation-picker', { hasText: label })
      .first();
  }

  /** A selected-entity tile inside a relation picker. The tile renders the
   *  entity's title (via `getEntityLabel`), so callers pass the title text. */
  pickerTileByText(picker: Locator, text: string): Locator {
    return picker.locator('.selected-entity', { hasText: text });
  }

  /** Type into the picker's combobox input and click the dropdown item
   *  that contains `optionText`. Picker-scoped on both sides — the input
   *  and the option are looked up inside `picker` — so it stays correct
   *  when the form has multiple pickers open. Cards-widget callers want
   *  `RelationCardsPage.linkTargetByIdWithSearch` instead; that path
   *  surfaces a different (search-then-meta-form) UI. */
  async pickInRelationPicker(picker: Locator, query: string, optionText: string) {
    const search = picker.locator('input[role="combobox"]');
    await expect(search).toBeVisible();
    await search.fill(query);
    const option = picker.locator('.dropdown-item', { hasText: optionText }).first();
    await expect(option).toBeVisible();
    await option.click();
  }

  /** Remove a selected entity tile from a relation picker. */
  async removePickerTile(picker: Locator, tileText: string) {
    await this.pickerTileByText(picker, tileText).locator('.remove-btn').click();
  }

  async saveAndWaitForNavigation() {
    await this.submitFormAndWaitForNavigation(this.submitButton.first());
  }

  // --- Manual ID / prefix picker (TKT-E7NNM) ---

  get idInput(): Locator {
    return this.page.locator('.id-field input[type="text"]');
  }

  get prefixSelect(): Locator {
    return this.page.locator('.id-field select');
  }

  async expectIdInputVisible() {
    await expect(this.idInput).toBeVisible();
  }

  async expectPrefixSelectVisible() {
    await expect(this.prefixSelect).toBeVisible();
  }

  async expectPrefixSelectHidden() {
    await expect(this.prefixSelect).toHaveCount(0);
  }

  async fillId(value: string) {
    await this.idInput.fill(value);
  }

  async selectPrefix(value: string) {
    await this.prefixSelect.selectOption(value);
  }

  async getPrefixOptions(): Promise<string[]> {
    return this.prefixSelect.locator('option').allTextContents();
  }
}
