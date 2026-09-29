import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

export class ListPage extends BasePage {
  readonly table: Locator;
  readonly createButton: Locator;
  readonly filterBar: Locator;
  readonly pagination: Locator;
  readonly emptyState: Locator;

  constructor(page: Page) {
    super(page);
    this.table = page.locator('.rl-table');
    this.createButton = page.locator('a, button').filter({ hasText: /new|create|add/i }).first();
    this.filterBar = page.locator('.filter-bar');
    this.pagination = page.locator('.pagination');
    // Scoped to the list: `.empty-state` is a generic class, and the detail
    // panel beside the list renders one of its own for an entity with no
    // document content. Unscoped, the two are one ambiguous locator.
    this.emptyState = page.locator('.entity-list .empty-state');
  }

  /**
   * A row located by its entity id.
   *
   * `data-entity-id` sits on the row element, supplied through RlTable's
   * `row-attrs`. Keyed on this rather than on the row link's href because a
   * row whose every column is locked (git-crypt) renders no link at all, and
   * that row still has to be addressable.
   */
  rowById(id: string): Locator {
    return this.page.locator(`.rl-table-row[data-entity-id="${id}"]`);
  }

  async navigateToList(listId: string, query?: string) {
    const path = query ? `/list/${listId}?${query}` : `/list/${listId}`;
    await this.navigateTo(path);
    await this.waitForSpinnerToDisappear();
    // Wait for the list to render — either rows or the empty-state message.
    const anyRow = this.page.locator('.rl-table-row').first();
    const empty = this.emptyState;
    await expect(anyRow.or(empty)).toBeVisible();
  }

  /** Wait for at least one row to render. Use when navigating to a list page
   *  directly (without `navigateToList`, which already waits). */
  async waitForRowsRendered(timeoutMs = 10_000) {
    // One selector for both layouts: RlTable stacks a row into a card at
    // narrow widths rather than rendering separate markup, so the row's name
    // cell is present either way.
    await this.page
      .locator('.rl-table-row__name')
      .first()
      .waitFor({ timeout: timeoutMs });
  }

  /** Assert no `+ New …` link is rendered on the page. Used to verify
   *  the AC10 read-only payoff: collection `_actions.create=false`
   *  hides the create affordance. */
  async expectNoCreateAffordance() {
    await expect(this.page.getByRole('link', { name: /^\+ New/ })).toHaveCount(0);
  }

  /** Assert the list offers no delete. Used to verify the AC10 read-only
   *  payoff: per-entity `_actions.delete=false` withholds the bulk bar's
   *  Delete. Rows are selectable only when a row may be deleted or the list
   *  has bulk actions, so with no checkbox there is nothing to delete with;
   *  otherwise a selected row must still not offer Delete. */
  async expectNoDeleteAffordance() {
    const boxes = this.page.locator('.rl-table-row__check input[type="checkbox"]');
    if ((await boxes.count()) > 0) {
      await boxes.first().check();
      await expect(this.page.locator('.rl-bulk-bar')).toBeVisible();
    }
    await expect(this.bulkDeleteButton).toHaveCount(0);
  }

  /** The Delete action in the bulk bar. Present only while a deletable row is selected. */
  get bulkDeleteButton(): Locator {
    return this.page.getByTestId('bulk-delete');
  }

  /** Tick a row's selection checkbox. */
  async selectRowById(id: string) {
    await this.rowById(id).locator('.rl-table-row__check input[type="checkbox"]').check();
  }

  async expectListHeading(title: string) {
    await expect(this.page.locator('h1', { hasText: title })).toBeVisible();
  }

  async getRowCount(): Promise<number> {
    const rows = this.page.locator('.rl-table-row');
    return rows.count();
  }

  /**
   * Plain-click a row, which opens its detail PANEL beside the list.
   *
   * This no longer leaves the list — see `openEntityPage` for the canonical
   * item view. Kept as the name for what a plain click does, so a spec that
   * means "click a row" gets the real behaviour.
   */
  async clickRow(index: number) {
    const rows = this.page.locator('.rl-table-row');
    await rows.nth(index).click();
  }

  async clickRowById(id: string) {
    await this.rowById(id).click();
  }

  /**
   * Go to a row's own entity page, the way a reader does deliberately:
   * open the panel, then expand it.
   *
   * Specs that want to BE on an entity page (to edit it, read its sections,
   * assert its header) should use this rather than clicking a row, which now
   * stops at the panel.
   */
  async openEntityPage(index: number) {
    await this.clickRow(index);
    await expect(this.detailPanel).toBeVisible();
    await this.expandPanel();
  }

  async openEntityPageById(id: string) {
    await this.openPanelForRow(id);
    await this.expandPanel();
  }

  /** Modifier/middle-click a row and return the tab it opens (TKT-3CSZRG).
   *  Rows are real links, so the browser — not the SPA — opens the tab; this
   *  waits on the resulting popup rather than on an in-page navigation. */
  async openRowInNewTab(index: number, how: { modifier?: 'ControlOrMeta'; middle?: boolean }) {
    const row = this.page.locator('.rl-table-row').nth(index);
    const popupPromise = this.page.context().waitForEvent('page');
    if (how.middle) {
      await row.click({ button: 'middle' });
    } else {
      await row.click({ modifiers: [how.modifier ?? 'ControlOrMeta'] });
    }
    return popupPromise;
  }

  /** The href of a row's link, for asserting the target and its scope query. */
  async rowLinkHref(index: number): Promise<string | null> {
    const row = this.page.locator('.rl-table-row').nth(index);
    return row.locator('.rl-table-row__primary a').first().getAttribute('href');
  }

  /** The detail panel beside the list. */
  get detailPanel(): Locator {
    return this.page.locator('.entity-detail-panel');
  }

  /** Open a row's panel with a plain click, as a reader would. */
  async openPanelForRow(id: string) {
    await this.rowById(id).locator('.rl-table-row__primary a').first().click();
    await expect(this.detailPanel).toBeVisible();
  }

  async closePanel() {
    await this.detailPanel.getByRole('button', { name: 'Close panel' }).click();
  }

  /** The panel's expand control, which leaves for the entity's own page. */
  async expandPanel() {
    await this.detailPanel.getByRole('button', { name: 'Expand' }).click();
  }

  /** The title the open panel is showing, for asserting which row it holds. */
  async panelHeading(): Promise<string | null> {
    return this.detailPanel.getByRole('heading').first().textContent();
  }

  /** The open panel's title heading. */
  get detailPanelHeading(): Locator {
    return this.detailPanel.getByRole('heading').first();
  }

  async expectPanelClosed() {
    await expect(this.detailPanel).toBeHidden();
  }

  async expectCursorOnRow(id: string) {
    await expect(this.rowById(id)).toHaveClass(/rl-table-row--cursor/);
  }

  async expectPanelOnRow(id: string) {
    await expect(this.rowById(id)).toHaveClass(/rl-table-row--selected/);
  }

  /**
   * The page header band, which the app shell paints ABOVE both the list and
   * the panel rather than inside the list (see usePageHeader). Locating it on
   * the shell's own element is the point: a header that had stayed in the
   * view would not match.
   */
  get header(): Locator {
    return this.page.locator('.rl-app-shell__header');
  }

  /** The list's title, as shown in the hoisted band. */
  get headerTitle(): Locator {
    return this.header.getByRole('heading').first();
  }

  /** The search box, which now lives in the header's lower bar. */
  get headerSearch(): Locator {
    return this.header.locator('.search-box input[type="search"]');
  }

  /** The create action, which now lives in the header's action cluster. */
  get headerCreateLink(): Locator {
    return this.header.getByRole('link', { name: /new/i });
  }

  /**
   * Right edge of an element, in viewport pixels.
   *
   * The geometry assertions below are the only way to tell a header that
   * spans both panes from one that stops at the panel's edge — happy-dom has
   * no layout engine and reports both identically.
   */
  async rightEdgeOf(locator: Locator): Promise<number> {
    const box = await locator.boundingBox();
    if (!box) throw new Error('element has no box');
    return box.x + box.width;
  }

  async widthOf(locator: Locator): Promise<number> {
    const box = await locator.boundingBox();
    if (!box) throw new Error('element has no box');
    return box.width;
  }

  /**
   * The panel's own box, for comparing edges against.
   *
   * Deliberately NOT `.rl-app-shell__panel`: that wrapper is
   * `display: contents`, so it is transparent to layout and has no box at
   * all. The panel element inside it is what actually occupies the row.
   */
  get panelPane(): Locator {
    return this.detailPanel;
  }

  /** The entity id of the row showing `title`. */
  async rowIdByTitle(title: string): Promise<string> {
    const row = this.page.locator('.rl-table-row').filter({ hasText: title });
    const id = await row.getAttribute('data-entity-id');
    if (!id) throw new Error(`No data-entity-id on row with title ${title}`);
    return id;
  }

  async deleteRowByTitle(title: string) {
    await this.deleteRowById(await this.rowIdByTitle(title));
  }

  /** Tick a row's selection checkbox by its "Select <title>" label. */
  async selectRowByTitle(title: string) {
    await this.page.getByRole('checkbox', { name: `Select ${title}` }).check();
  }

  /** The bar that appears while rows are selected. */
  get bulkBar(): Locator {
    return this.page.locator('.rl-bulk-bar');
  }

  /** The list's own actions in the bulk bar. */
  get bulkActions(): Locator {
    return this.bulkBar.getByTestId('bulk-action');
  }

  async clearSelection() {
    await this.bulkBar.getByRole('button', { name: 'Clear selection' }).click();
  }

  /** The confirm dialog a delete would raise, if it asked. */
  get alertDialog(): Locator {
    return this.page.getByRole('alertdialog');
  }

  /** Press the action button inside a toast, e.g. Undo. */
  async clickToastAction(toast: Locator, name: string) {
    await toast.getByRole('button', { name }).click();
  }

  /** Looks in the name and the cells only: the row's selection checkbox
   *  carries a hidden "Select <title>" label that would also match. */
  async expectCellInRow(id: string, cellText: string) {
    await expect(
      this.rowById(id)
        .locator('.rl-table-row__primary, .rl-table-row__cell')
        .locator(`text=${cellText}`)
        .first(),
    ).toBeVisible();
  }

  /** Count of cells in the row marked as inaccessible (rendered with
   *  the lock indicator). Used by git-crypt.spec.ts to assert that
   *  encrypted entities lock every visible column. */
  async lockedCellsInRow(id: string): Promise<number> {
    return this.page
      .locator(`.rl-table-row[data-entity-id="${id}"] .inaccessible-cell`)
      .count();
  }

  async expectRowNotVisible(text: string) {
    await expect(
      this.page.locator('.rl-table-row').filter({ hasText: text }),
    ).not.toBeVisible();
  }

  /**
   * Go to the create form's own page. A plain click on New opens the form in
   * a dialog instead (see openCreateDialog); specs that exercise the form
   * page follow the button's link, which is what a new-tab click does.
   */
  async clickCreateButton() {
    const href = await this.createButton.first().getAttribute('href');
    if (!href) throw new Error('New button has no link');
    await this.page.goto(new URL(href, this.page.url()).toString());
    await this.page.waitForLoadState('domcontentloaded');
  }

  /** The create dialog New opens over the list. */
  get createDialog(): Locator {
    return this.page.getByRole('dialog');
  }

  async openCreateDialog() {
    await this.createButton.first().click();
    await expect(this.createDialog).toBeVisible();
  }

  /** Open the create dialog through the header's "New …" link. */
  async openCreateDialogFromNewLink() {
    await this.page.getByRole('link', { name: /^New/ }).first().click();
    await expect(this.createDialog).toBeVisible();
  }

  /** Open the create dialog through the Add button below the rows. */
  async clickAddBelowRows() {
    await this.page.locator('.rl-table-section__add').click();
  }

  /** Press the dialog's Create, not "Create & add another". */
  async submitCreateDialog() {
    await this.createDialog.getByRole('button', { name: /^Create(?! &)/ }).click();
  }

  /** Select the row and press the bulk bar's Delete. There is no confirm;
   *  the toast it raises offers Undo instead. */
  async deleteRowById(id: string) {
    await this.selectRowById(id);
    await this.bulkDeleteButton.click();
  }

  /**
   * Press a column's sort control.
   *
   * The control is a real button inside the header, not the header itself:
   * rela's sort used to be a click handler on a bare `<th>`, which meant no
   * keyboard route to sorting at all. Clicking the BUTTON is what a keyboard
   * user can also reach, so the test exercises the same affordance they do.
   */
  async sortByColumn(columnName: string) {
    const header = this.columnHeader(columnName);
    await header.getByRole('button').click();
    await this.waitForSpinnerToDisappear();
  }

  /**
   * Assert a column's sort state through `aria-sort`.
   *
   * Replaces an assertion on the ▲/▼ glyphs. The glyph is decoration the
   * library may restyle; `aria-sort` is the contract a screen reader reads,
   * so asserting it pins the guarantee that actually matters and does not
   * break on a visual change.
   */
  async expectSortIndicator(columnName: string, direction: 'asc' | 'desc') {
    const header = this.columnHeader(columnName);
    await expect(header).toHaveAttribute(
      'aria-sort',
      direction === 'asc' ? 'ascending' : 'descending',
    );
  }

  /**
   * Whether the first row is the topmost element at its own centre.
   *
   * Guards a bug a DOM assertion cannot see: the sticky column header is
   * positioned to sit above the rows, and when its offset does not match the
   * section header's actual height it floats down and COVERS the first row.
   * The row stays present, visible and correctly marked up while being
   * impossible to click. Only a hit test finds it.
   */
  async firstRowIsClickable(): Promise<boolean> {
    return this.page
      .locator('.rl-table-row')
      .first()
      .evaluate((el) => {
        const r = el.getBoundingClientRect();
        const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
        return !!(top && el.contains(top));
      });
  }

  /** How many sort controls a column's header offers (0 when unsortable). */
  async sortControlCount(columnName: string): Promise<number> {
    return this.columnHeader(columnName).getByRole('button').count();
  }

  /** A column header cell, by its visible label. */
  columnHeader(name: string | RegExp): Locator {
    const matcher = name instanceof RegExp ? name : new RegExp(name, 'i');
    return this.page.getByRole('columnheader').filter({ hasText: matcher }).first();
  }

  async setFilter(property: string, value: string) {
    const control = this.filterBar.locator('.filter-item').filter({ hasText: new RegExp(property, 'i') });
    await this.pickFilterOption(control, value);
    await this.waitForSpinnerToDisappear();
  }

  async goToPage(pageNumber: number) {
    await this.pagination.locator(`button:has-text("${pageNumber}")`).click();
    await this.waitForSpinnerToDisappear();
  }

  async nextPage() {
    await this.pagination.locator('button:has-text("Next"), button:has-text("→")').click();
    await this.waitForSpinnerToDisappear();
  }

  async prevPage() {
    await this.pagination.locator('button:has-text("Prev"), button:has-text("←")').click();
    await this.waitForSpinnerToDisappear();
  }

  async expectRowCount(count: number) {
    const rows = this.page.locator('.rl-table-row');
    await expect(rows).toHaveCount(count);
  }

  async expectRowContains(text: string) {
    const rowById = this.rowById(text);
    const rowByText = this.page.locator('.rl-table-row').filter({ hasText: text });
    await expect(rowById.or(rowByText).first()).toBeVisible();
  }

  async expectColumnHeader(name: string | RegExp) {
    await expect(this.columnHeader(name)).toBeVisible();
  }

  /** Set the Nth filter in the filter bar to the given option value and wait
   *  for any resulting list refetch to settle. */
  async setFilterByIndex(index: number, value: string | string[] | { index: number }) {
    await this.pickFilterOption(this.filterBar.locator('.filter-item').nth(index), value);
    await this.waitForSpinnerToDisappear();
  }

  async filterControlCount(): Promise<number> {
    return this.filterBar.locator('.filter-item').count();
  }

  /** The Nth filter-bar <select>. Only relation filters in select mode
   *  render one; enum filters draw their own option list. */
  private filterSelect(index = 0): Locator {
    return this.filterBar.locator('select').nth(index);
  }

  /** Count how many <option>s in the Nth filter select carry the given text. */
  async filterOptionCount(text: string, index = 0): Promise<number> {
    return this.filterSelect(index).locator('option', { hasText: text }).count();
  }

  /** Select an option (by its value) in the Nth filter select and wait for the
   *  resulting list refetch. Pass '' to clear. */
  async selectFilterOption(value: string, index = 0) {
    await this.filterSelect(index).selectOption(value);
    await this.waitForSpinnerToDisappear();
  }

  /** Wait for rows to be present before we try to issue keyboard commands
   *  against them. The ListView's keydown handler is attached to `document`,
   *  so we don't need to focus the table itself. */
  async focusTable() {
    const firstRow = this.page.locator('.rl-table-row').first();
    await expect(firstRow).toBeVisible();
  }

  async pressKey(key: string) {
    await this.page.keyboard.press(key);
  }

  /**
   * The row the j/k cursor is on. Distinct from `panelRow` below: the cursor
   * moves with j/k, the panel stays on the row that was opened.
   */
  get cursorRow(): Locator {
    return this.page.locator('.rl-table-row--cursor');
  }

  /** The row whose detail panel is open, marked by RlTable's `selected-id`. */
  get panelRow(): Locator {
    return this.page.locator('.rl-table-row--selected');
  }

  async expectEmpty() {
    await expect(this.emptyState).toBeVisible();
  }

  async expectTotal(total: number) {
    // Check pagination or header for total count
    const totalText = this.page.locator('.results-count, .total-count, .pagination');
    await expect(totalText).toContainText(String(total));
  }
}
