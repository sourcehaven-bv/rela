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
  /**
   * The Edit control still reads as "Edit" to assistive tech.
   *
   * The shortcut hint sits INSIDE the button, so it contributes to the
   * accessible name. Were it to lead rather than trail, every `name: /^Edit/`
   * locator in this suite would start matching "E Edit" instead.
   */
  async expectEditNameStartsWithEdit() {
    const byRole = this.page
      .getByRole('button', { name: /^Edit/ })
      .or(this.page.getByRole('link', { name: /^Edit/ }))
      .first();
    await expect(byRole).toBeVisible();
  }

  async expectDocumentBodyContains(text: string) {
    await expect(this.documentBody).toContainText(text);
  }

  async navigateToEntity(entityType: string, id: string) {
    await this.navigateTo(`/entity/${entityType}/${id}`);
    await this.waitForSpinnerToDisappear();
    await expect(this.heading).toBeVisible();
  }

  /** Open an entity the way a list does, so it has a back target. */
  async navigateToEntityFrom(entityType: string, id: string, returnTo: string) {
    await this.navigateTo(`/entity/${entityType}/${id}?return_to=${encodeURIComponent(returnTo)}`);
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

  /** Navigate away from the current detail page via an in-SPA router link
   *  (NOT a full page load). The issue #997 crash fires during Vue's
   *  client-side unmount of the EntityDetail subtree, which only happens on
   *  a router-driven transition — a `page.goto()` reload tears down the whole
   *  document and never exercises the unmount path. Clicks the sidebar
   *  Dashboard link and waits for the route to settle. */
  async navigateAwayViaRouter() {
    await this.page.locator('a[href="/"]').first().click();
    await this.page.waitForURL(/\/(dashboard)?$/);
  }

  /** Assert a `display: list` relation section rendered at least one row
   *  with an inline-edit SectionEditForm mounted inside the list item.
   *  This is the DOM shape that triggered the issue #997 unmount crash —
   *  a guard so the regression test can't pass without exercising it. */
  async expectListSectionRowMounted() {
    await expect(this.page.locator('.entity-list .list-item .section-edit-form').first()).toBeVisible();
  }

  /** The entry (first) properties section of a detail page. Render-mode
   *  assertions below scope to it so an opted-in section further down the
   *  page can't satisfy a display-mode expectation (TKT-HOIX1). */
  private entryPropertiesSection() {
    return this.page.locator('.properties-list').first();
  }

  /** A detail-page section located by its heading, rather than by position.
   *  The entry-section helpers above use `.first()`, which cannot address a
   *  second properties section on the same page (TKT-3R7RF3 adds one). */
  private sectionByHeading(heading: string) {
    return this.page
      .locator('.view-section', { has: this.page.getByRole('heading', { name: heading }) })
      .first();
  }

  /** Assert the enum badges in a display section read exactly `texts`, in
   *  order. Badge text is the schema label when one is configured, else the
   *  raw value. */
  async expectSectionBadges(heading: string, texts: string[]) {
    await expect(this.sectionByHeading(heading).locator('.badge')).toHaveText(texts);
  }

  /** Press a section's create button for one target type (its label). */
  async clickSectionCreate(heading: string, targetLabel: string) {
    await this.sectionByHeading(heading)
      .getByRole('button', { name: targetLabel, exact: true })
      .click();
    await expect(this.createDialog).toBeVisible();
  }

  /** The create dialog a section create button opens. */
  get createDialog(): Locator {
    return this.page.getByRole('dialog');
  }

  async fillCreateDialogField(property: string, value: string) {
    await this.createDialog.locator(`#field-${property}`).fill(value);
  }

  /** Press the dialog's Create, not "Create & add another". */
  async submitCreateDialog() {
    await this.createDialog.getByRole('button', { name: /^Create(?! &)/ }).click();
  }

  async expectCreateDialogClosed() {
    await expect(this.createDialog).toHaveCount(0);
  }

  /** A row in a section, located by the text it shows. */
  async expectSectionRow(heading: string, text: string) {
    await expect(this.sectionByHeading(heading).getByText(text)).toBeVisible();
  }

  /** A property's row in a section, marked by SectionEditForm. */
  private sectionFieldRow(heading: string, property: string) {
    return this.sectionByHeading(heading).locator(`.property-row[data-property="${property}"]`);
  }

  /** The edit control for a property. Its id (`inline-<property>`) is set on
   *  the widget's own element rather than a wrapper. A toggle is live all the
   *  time; any other value shows its control only once opened. */
  private sectionFieldControl(heading: string, property: string) {
    return this.sectionFieldRow(heading, property).locator(`#inline-${property}`);
  }

  /** Assert every field in a section keeps its label and value inside its
   *  own grid cell: the value column has width, and neither runs past the
   *  cell edge into the neighbouring field (BUG-S67G88). Polled, so a layout
   *  that settles after the first paint is measured once it has settled.
   *  Geometry, so e2e-only. */
  async expectSectionFieldsFitTheirCells(heading: string) {
    const rows = this.sectionByHeading(heading).locator('.property-row');
    await expect(rows.first()).toBeVisible();
    await expect(async () => {
      const offenders = await rows.evaluateAll((els) =>
        els.flatMap((row) => {
          const name = row.getAttribute('data-property');
          const label = row.querySelector('.rl-detail-field__label');
          const value = row.querySelector('.rl-detail-field__value');
          if (!label || !value) return [`${name}: missing label or value`];
          const cell = row.getBoundingClientRect();
          const problems: string[] = [];
          if (value.getBoundingClientRect().width < 1) problems.push('value has no width');
          if (value.getBoundingClientRect().right > cell.right + 0.5) problems.push('value runs past its cell');
          if (value.scrollWidth > value.clientWidth + 1) problems.push('value content overflows');
          if (label.getBoundingClientRect().right > cell.right + 0.5) problems.push('label runs past its cell');
          return problems.map((p) => `${name}: ${p}`);
        })
      );
      expect(offenders).toEqual([]);
    }).toPass();
  }

  /** Width in px of a field's grid cell. Lets a layout test prove it is
   *  measuring the narrow case it means to, rather than passing because the
   *  cells happen to be wide. */
  async sectionFieldCellWidth(heading: string, property: string): Promise<number> {
    const box = await this.sectionFieldRow(heading, property).boundingBox();
    if (!box) throw new Error(`no row for ${property} in ${heading}`);
    return box.width;
  }

  /** Open a text field's inline editor and return the editor's box and its
   *  cell's, so a test can check the editor keeps its minimum width where
   *  there is room and stays inside a narrow cell (BUG-S67G88). Closes the
   *  editor again with Escape, which drops the edit. */
  async measureSectionFieldEditor(heading: string, property: string) {
    const row = this.sectionFieldRow(heading, property);
    await row.locator('.rl-inline-edit__trigger').click();
    const editor = row.locator('.inline-property-control');
    await expect(editor).toBeVisible();
    const [editorBox, cellBox] = await Promise.all([editor.boundingBox(), row.boundingBox()]);
    await this.page.keyboard.press('Escape');
    await expect(editor).toBeHidden();
    if (!editorBox || !cellBox) throw new Error(`no box for ${property} editor`);
    return { editorWidth: editorBox.width, editorRight: editorBox.x + editorBox.width, cellRight: cellBox.x + cellBox.width };
  }

  /** Assert a property edits as a TEXTAREA — the load-bearing widget-override
   *  case (TKT-3R7RF3). A string property's type default is TextWidget's
   *  `<input>`, so a textarea here can only come from `widget: textarea`. The
   *  value reads as text until clicked, so this opens it first. */
  async expectSectionFieldIsTextarea(heading: string, property: string) {
    await this.sectionFieldRow(heading, property).locator('.rl-inline-edit__trigger').click();
    const control = this.sectionFieldControl(heading, property);
    await expect(control).toBeVisible();
    await expect(control).toHaveJSProperty('tagName', 'TEXTAREA');
  }

  /** Assert a property renders as an ENABLED checkbox, i.e. the override
   *  reached the EDIT arm rather than a display span or a disabled control. */
  async expectSectionCheckboxEnabled(heading: string, property: string) {
    const box = this.sectionFieldControl(heading, property);
    await expect(box).toBeVisible();
    await expect(box).toHaveAttribute('type', 'checkbox');
    await expect(box).toBeEnabled();
  }

  /** Assert an overridden checkbox is checked (or not). Separate from the
   *  enabled-assertion so a spec can prove a toggle SURVIVED a reload, which
   *  is what distinguishes a real inline edit from a local DOM change. */
  async expectSectionCheckboxChecked(heading: string, property: string, checked = true) {
    const box = this.sectionFieldControl(heading, property);
    if (checked) {
      await expect(box).toBeChecked();
    } else {
      await expect(box).not.toBeChecked();
    }
  }

  /** Click a checkbox rendered by a widget override and return its new state.
   *
   *  Waits for the autosave PATCH to land before returning. SectionEditForm
   *  debounces saves by 800ms, so a caller that reloads immediately races the
   *  timer and sees the pre-edit value — which looks exactly like a dropped
   *  write rather than a test that asserted too early. */
  async toggleSectionCheckbox(heading: string, property: string): Promise<boolean> {
    const box = this.sectionFieldControl(heading, property);
    // Match the PATCH for THIS entity, not any PATCH: on a page with more
    // than one autosaving control a broad predicate resolves on the wrong
    // request, and the caller's reload then races the real save — the exact
    // flake this wait exists to remove.
    const entityId = this.page.url().split('/').pop() ?? '';
    const patched = this.page.waitForResponse(
      (r) =>
        r.url().includes('/api/v1/') &&
        r.url().includes(entityId) &&
        r.request().method() === 'PATCH' &&
        r.status() < 400,
      { timeout: 5000 },
    );
    await box.click();
    await patched;
    return box.isChecked();
  }

  /** Assert the entry properties section renders as DISPLAY values: visible,
   *  but with no inline-edit form mounted (TKT-HOIX1 — the `render: display`
   *  default). */
  async expectEntrySectionRendersDisplay() {
    const section = this.entryPropertiesSection();
    await expect(section).toBeVisible();
    await expect(section.locator('.section-edit-form')).toHaveCount(0);
  }

  /** Assert the entry properties section renders no form controls at all —
   *  no select, no input, no FieldShell wrapper. Stronger than
   *  expectEntrySectionRendersDisplay: it pins that display mode hides the
   *  CONTROL, not merely the autosave host. */
  async expectEntrySectionHasNoFormControls() {
    const section = this.entryPropertiesSection();
    await expect(section).toBeVisible();
    await expect(section.locator('select')).toHaveCount(0);
    await expect(section.locator('input')).toHaveCount(0);
    await expect(section.locator('.form-field')).toHaveCount(0);
  }

  /** Assert the entry properties section shows the given text — display mode
   *  hides the control, not the data. */
  async expectEntrySectionShowsValue(text: string) {
    await expect(this.entryPropertiesSection()).toContainText(text);
  }

  /** Assert the entry properties section does NOT show the given text. Used to
   *  prove a display-rendered value tracks the current server state rather than
   *  a stale string mirror (RR-GLK4UY). */
  async expectEntrySectionLacksValue(text: string) {
    await expect(this.entryPropertiesSection()).not.toContainText(text);
  }

  /** Assert the first inline-edit list row exposes an ENABLED control, proving
   *  `render: input` reaches the edit arm rather than a disabled widget. The
   *  row's fields are enums, which are live pickers. */
  async expectListSectionRowControlEnabled() {
    const row = this.page.locator('.entity-list .list-item .section-edit-form').first();
    await expect(row).toBeVisible();
    const control = row.locator('.rl-option-select__trigger').first();
    await expect(control).toBeVisible();
    await expect(control).toBeEnabled();
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
    // Inline-edit (TKT-IHC7B): properties may render as <input value="…">
    // or <textarea>…</textarea>, neither of which produce text nodes
    // matchable by getByText. Fall through to a form-control sweep so
    // tests against property values work in both display and edit mode.
    if (await this.page.getByText(text).first().isVisible().catch(() => false)) return true;
    return this.matchesFormControlValue(this.page.locator('body'), text);
  }

  /** Check that a property value is rendered inside the entity-detail container
   *  (scoped to avoid matching nav/sidebar elements). Matches either visible
   *  text (display mode: Badge / span) or form-control values (inline-edit
   *  mode: input / textarea / selected option) — see TKT-IHC7B. */
  async hasPropertyValue(value: string): Promise<boolean> {
    if (await this.detailContainer.getByText(value, { exact: true }).first().isVisible().catch(() => false)) {
      return true;
    }
    return this.matchesFormControlValue(this.detailContainer, value);
  }

  /** Internal: scan inputs / textareas / selected options under `scope` for
   *  a control whose value === text. Returns true on first match. */
  private async matchesFormControlValue(scope: Locator, text: string): Promise<boolean> {
    const inputs = await scope.locator('input, textarea').all();
    for (const ctrl of inputs) {
      const v = await ctrl.inputValue().catch(() => '');
      if (v === text) return true;
    }
    const selects = await scope.locator('select').all();
    for (const sel of selects) {
      const v = await sel.inputValue().catch(() => '');
      if (v === text) return true;
    }
    return false;
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

  // --- body inline-edit helpers ---

  /** The button that opens the body editor; the only way into it. */
  get bodyEditButton(): Locator {
    return this.page.getByRole('button', { name: 'Body, edit', exact: true });
  }

  /** The body editor's writing surface, present only while editing. */
  get bodyEditor(): Locator {
    return this.page.locator('.entity-body-editor .ProseMirror');
  }

  /** The body's read view, which the edit swaps out synchronously. */
  get bodyReadView(): Locator {
    return this.page.locator('.rl-inline-edit__read').filter({ has: this.contentBody });
  }

  /** The back/scope bar, which sticks to the top of the pane on a phone. */
  get mobileTopbar(): Locator {
    return this.page.locator('.scope-nav.mobile-topbar');
  }

  /** Whether the body's edit button lies below the mobile top bar. */
  async bodyEditButtonClearsTopbar(): Promise<boolean> {
    const button = await this.bodyEditButton.boundingBox();
    const bar = await this.mobileTopbar.boundingBox();
    if (!button || !bar) return false;
    return button.y >= bar.y + bar.height;
  }

  /**
   * Click a paragraph of the read view `clickCount` times, on its first word.
   * The paragraph is as wide as the body, so its centre can be empty space
   * past the end of the text, where a double-click selects nothing.
   */
  async clickBodyText(text: string, clickCount: 1 | 2 | 3) {
    const paragraph = this.contentBody.getByText(text);
    const box = await paragraph.boundingBox();
    if (!box) throw new Error(`body paragraph "${text}" is not rendered`);
    await paragraph.click({ clickCount, position: { x: 10, y: box.height / 2 } });
  }

  async selectedText(): Promise<string> {
    return this.page.evaluate(() => window.getSelection()?.toString() ?? '');
  }

  /** Scroll the main pane so the middle of the body is at the top of it. */
  async scrollBodyHalfway() {
    await this.contentBody.evaluate((el) => {
      const pane = el.closest('.rl-app-shell__main');
      if (!pane) throw new Error('no scrolling pane above the body');
      const body = el.getBoundingClientRect();
      pane.scrollTop += body.top - pane.getBoundingClientRect().top + body.height / 2;
    });
  }

  /** Whether the edit button lies inside both the viewport and the body. */
  async bodyEditButtonInView(): Promise<boolean> {
    const button = await this.bodyEditButton.boundingBox();
    const body = await this.contentBody.boundingBox();
    const viewport = this.page.viewportSize();
    if (!button || !body || !viewport) return false;
    return (
      button.y >= 0 &&
      button.y + button.height <= viewport.height &&
      button.y >= body.y &&
      button.y + button.height <= body.y + body.height
    );
  }

  // --- mermaid body-content helpers ---

  /** Rendered mermaid diagrams inside the content body. */
  get mermaidDiagrams(): Locator {
    return this.contentBody.locator('.mermaid-diagram');
  }

  /** Wait for the first mermaid diagram to finish rendering. Rendering is
   *  async (mermaid measures text in the live DOM before emitting SVG), so a
   *  bare count() races it. */
  async waitForMermaidDiagram() {
    await expect(this.mermaidDiagrams.first()).toBeVisible({ timeout: 15_000 });
  }

  /** Text of every node label in the first diagram. Flowchart labels are
   *  XHTML inside <foreignObject>; sequence labels are SVG <text>. */
  async mermaidLabelTexts(): Promise<string[]> {
    return this.mermaidDiagrams.first().locator('foreignObject, text').allTextContents();
  }

  /** Number of mermaid `.nodeLabel` wrappers inside the first diagram's
   *  labels. Zero means the label MARKUP was stripped even if text survived. */
  async mermaidLabelElementCount(): Promise<number> {
    return this.mermaidDiagrams.first().locator('foreignObject .nodeLabel').count();
  }

  /** Number of <br> elements inside the first diagram's labels — a
   *  multi-line node label that survived sanitization keeps its line break. */
  async mermaidLabelLineBreakCount(): Promise<number> {
    return this.mermaidDiagrams.first().locator('foreignObject br').count();
  }

  /** Names of every on* attribute anywhere inside the first diagram. */
  async mermaidEventHandlerAttributes(): Promise<string[]> {
    return this.mermaidDiagrams.first().evaluate((root) =>
      Array.from(root.querySelectorAll('*')).flatMap((el) =>
        Array.from(el.attributes)
          .map((a) => a.name)
          .filter((n) => /^on/i.test(n)),
      ),
    );
  }

  /** Embedded-media elements inside the first diagram. A label is text; any
   *  of these came from a hostile label and must have been stripped. */
  async mermaidEmbeddedMediaCount(): Promise<number> {
    return this.mermaidDiagrams.first().locator('img, iframe, object, embed, video, audio').count();
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

  // ── New-tab affordances (TKT-3CSZRG) ────────────────────────────────────
  // Navigable rows, cells and non-mutating nav controls are real <a href>
  // elements so the browser can open them in a tab. These helpers expose the
  // link elements and the modifier-click gesture.

  /** The first link inside a table-display section on the entity detail page. */
  get sectionTableLink(): Locator {
    return this.page.locator('.sections .data-table a[href]').first();
  }

  /** A header nav affordance rendered as a link (Prev/Next, Edit, History). */
  navLink(name: string | RegExp): Locator {
    return this.page.getByRole('link', { name });
  }

  /** Modifier-click a locator and return the tab the browser opens. Used to
   *  prove the BROWSER opened the tab, rather than the SPA emulating one.
   *  `ControlOrMeta` resolves per platform (Meta on macOS, Control elsewhere) —
   *  hardcoding either one passes on the author's machine and fails on CI. */
  async openInNewTab(target: Locator, modifier: 'ControlOrMeta' = 'ControlOrMeta') {
    const popupPromise = this.page.context().waitForEvent('page');
    await target.click({ modifiers: [modifier] });
    return popupPromise;
  }

  // ── Duplicate (TKT-Z8K2FS) ────────────────────────────────────────────────

  /** The Duplicate button in the desktop header action row. */
  get duplicateButton(): Locator {
    return this.page.locator('.desktop-actions button:has-text("Duplicate")');
  }

  get duplicateModal(): Locator {
    return this.page.locator('.duplicate-modal');
  }

  /**
   * One relation-type row in the picker, addressed by its visible label.
   * RlCheckbox puts the label and its count in one string, so the match is a
   * prefix rather than an exact one.
   */
  duplicateChoice(label: string): Locator {
    return this.duplicateModal
      .locator('label')
      .filter({ hasText: new RegExp(`^\\s*${label} \\(\\d+\\)`) });
  }

  /** The checkbox for a relation-type row. */
  duplicateChoiceCheckbox(label: string): Locator {
    return this.duplicateChoice(label).locator('input[type="checkbox"]');
  }

  /**
   * Edge count shown for a relation-type row. RlCheckbox renders the label
   * and count as one string, so the assertion reads the row's own text.
   */
  async expectDuplicateChoiceCount(label: string, count: number) {
    await expect(this.duplicateChoice(label)).toHaveText(
      new RegExp(`^\\s*${label} \\(${count}\\)`)
    );
  }

  async openDuplicate() {
    await this.duplicateButton.click();
    await expect(this.duplicateModal).toBeVisible();
  }

  async continueDuplicate() {
    await this.duplicateModal.locator('button:has-text("Continue")').click();
  }

  /** The create form hosted inside the duplicate dialog. */
  duplicateField(property: string): Locator {
    return this.duplicateModal.locator(`#field-${property}`);
  }

  async submitDuplicateForm() {
    await this.duplicateModal.locator('button[type="submit"]').first().click();
  }
}
