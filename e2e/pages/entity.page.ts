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

  /** The edit control SectionEditForm renders for a property. Its id is
   *  assigned by the form (`section-edit-<property>`), so this addresses the
   *  widget's own element rather than a wrapper. */
  private sectionFieldControl(heading: string, property: string) {
    return this.sectionByHeading(heading).locator(`#section-edit-${property}`);
  }

  /** Assert a property renders as a TEXTAREA — the load-bearing widget-override
   *  case (TKT-3R7RF3). A string property's type default is TextWidget's
   *  `<input>`, so a textarea here can only come from `widget: textarea`. */
  async expectSectionFieldIsTextarea(heading: string, property: string) {
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
    const patched = this.page.waitForResponse(
      (r) =>
        r.url().includes('/api/v1/') &&
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
   *  `render: input` reaches the edit arm rather than a disabled widget. */
  async expectListSectionRowControlEnabled() {
    const row = this.page.locator('.entity-list .list-item .section-edit-form').first();
    await expect(row).toBeVisible();
    const control = row.locator('select, input').first();
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
