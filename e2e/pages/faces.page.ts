import { type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * Face and world affordances on list and detail views (TKT-WCMW47).
 *
 * Selectors mirror the components that render them: `WorldBanner.vue`
 * (`.world-banner`), `FaceMenu.vue` (`.face-single` for one other face,
 * `.face-menu` for several), `CopyMenu.vue` (`.copy-single` / `.copy-menu`),
 * `ExportMenu.vue` (`.export-menu`), the detail-page script actions
 * (header buttons named by their label) and relation cards (`article.entity-card`).
 */
export class FacesPage extends BasePage {
  /** Open `/entity/<type>/<address>`, optionally in a world, and wait for the
   *  page to settle on either the entity or the not-found state. */
  async openEntity(type: string, address: string, world?: string) {
    const query = world ? `?world=${encodeURIComponent(world)}` : '';
    await this.navigateTo(`/entity/${type}/${address}${query}`);
    await this.waitForSpinnerToDisappear();
    await expect(this.heading().or(this.notFound())).toBeVisible();
  }

  heading(): Locator {
    return this.page.locator('main h1').first();
  }

  notFound(): Locator {
    return this.page.getByTestId('page-state-error').filter({ hasText: /not found/i });
  }

  async expectHeading(text: string) {
    await expect(this.heading()).toHaveText(text);
  }

  async expectNotFound() {
    await expect(this.notFound()).toBeVisible();
  }

  async expectBodyContains(text: string) {
    await expect(this.page.locator('main')).toContainText(text);
  }

  async expectBodyNotContains(text: string) {
    await expect(this.page.locator('main')).not.toContainText(text);
  }

  // ── World banner ───────────────────────────────────────────────────

  worldBanner(): Locator {
    return this.page.locator('.world-banner').first();
  }

  async expectWorldBanner(label: string) {
    await expect(this.worldBanner().locator('.world-banner__label')).toHaveText(label);
  }

  /** The "absent" banner a detail view shows when the world has no face of
   *  this entity to present. */
  async expectAbsentBanner() {
    await expect(this.page.locator('.world-banner--absent')).toBeVisible();
  }

  // ── World switcher (WorldSwitcher.vue) ────────────────────────────

  worldSwitcher(): Locator {
    return this.page.getByTestId('world-switcher');
  }

  async selectWorld(name: string) {
    await this.worldSwitcher().selectOption(name);
    await this.waitForSpinnerToDisappear();
  }

  async expectSelectedWorld(name: string) {
    await expect(this.worldSwitcher()).toHaveValue(name);
  }

  /** The `?world=` the URL carries, or null when it carries none. */
  worldParam(): string | null {
    return new URL(this.page.url()).searchParams.get('world');
  }

  // ── Create-form face picker (DynamicForm.vue) ─────────────────────

  createFacePicker(): Locator {
    return this.page.getByTestId('create-face');
  }

  /** The labels of the faces the picker offers, without its placeholder. */
  async createFaceOptions(): Promise<string[]> {
    await expect(this.createFacePicker()).toBeVisible();
    return this.createFacePicker().locator('option:not([disabled])').allTextContents()
      .then((labels) => labels.map((l) => l.trim()));
  }

  async expectNoCreateFacePicker() {
    await expect(this.createFacePicker()).toHaveCount(0);
  }

  // ── Face switcher ──────────────────────────────────────────────────

  /** Switch to another face. One other face renders a single button
   *  ("View <Face>"); several render a menu. */
  async switchToFace(faceLabel: string) {
    const single = this.page.locator('button.face-single');
    const menu = this.page.locator('.face-menu > button');
    // Wait for either form before branching; isVisible() does not wait.
    await expect(single.or(menu)).toBeVisible();
    if (await single.isVisible()) {
      await expect(single).toContainText(faceLabel);
      await single.click();
    } else {
      await menu.click();
      await this.page.locator('.face-menu-item').filter({ hasText: faceLabel }).click();
    }
  }

  async expectFaceSwitcher(faceLabel: string) {
    await expect(
      this.page.locator('button.face-single, .face-menu').filter({ hasText: faceLabel }),
    ).toBeVisible();
  }

  // ── Copies ─────────────────────────────────────────────────────────

  copyButton(label: string): Locator {
    return this.page.locator('button.copy-single').filter({ hasText: label });
  }

  /** Invoke a single-offer copy and wait for the landing on the written face. */
  async invokeCopy(label: string, landingAddress: string) {
    await this.copyButton(label).click();
    await this.page.waitForURL((url) => decodeURIComponent(url.pathname).endsWith(landingAddress));
  }

  async expectNoCopy(label: string) {
    await this.waitForActionRow();
    await expect(this.copyButton(label)).toHaveCount(0);
  }

  // ── Edit / delete / script actions ─────────────────────────────────

  editLink(): Locator {
    // The desktop row; the label is followed by a keyboard hint (`Edit E`).
    return this.page.locator('.desktop-actions a, .desktop-actions button').filter({ hasText: /^\s*Edit\b/ });
  }

  async clickEdit() {
    await this.editLink().click();
    await this.page.waitForURL(/\/form\//);
  }

  async expectNoEdit() {
    await this.waitForActionRow();
    await expect(this.editLink()).toHaveCount(0);
  }

  /** Wait for the header action row to render, so an absence assertion on
   *  one of its affordances cannot pass before the row exists. The export
   *  menu is the anchor: every principal who can read the entity gets it,
   *  because the fixture registers a transform. */
  private async waitForActionRow() {
    await expect(this.page.locator('.desktop-actions .export-menu')).toBeVisible();
  }

  deleteButton(): Locator {
    return this.page.locator('.desktop-actions button').filter({ hasText: /^\s*Delete\b/ });
  }

  /** Delete the face on screen, confirm the modal (which names the face), and
   *  wait to leave the detail view. */
  async deleteShownFace(faceLabel: string) {
    await this.deleteButton().click();
    const modal = this.page.getByRole('alertdialog');
    await expect(modal).toContainText(`${faceLabel} face`);
    await modal.locator('button').filter({ hasText: /^Delete$/ }).click();
    await this.page.waitForURL((url) => !url.pathname.startsWith('/entity/'));
  }

  /** Wait until the browser shows an entity other than `previousId` and
   *  return its address (`ID` or `ID@face`). */
  async landedAddressOtherThan(type: string, previousId: string): Promise<string> {
    const prefix = `/entity/${type}/`;
    await this.page.waitForURL((url) => {
      const p = decodeURIComponent(url.pathname);
      return p.startsWith(prefix) && p.slice(prefix.length).split('@')[0] !== previousId;
    });
    return decodeURIComponent(new URL(this.page.url()).pathname).slice(prefix.length);
  }

  /** The script-error dialog a refused script action raises, naming the
   *  refusal. A failed script action reports there, not in a toast. */
  async expectActionRefused(reason = 'forbidden') {
    const dialog = this.page.getByRole('alertdialog', { name: 'Script error' });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText(reason);
  }

  /** Run a detail-page script action (an `actions:` entry with `available_on`). */
  async runAction(label: string) {
    await this.page.locator('.desktop-actions').getByRole('button', { name: label, exact: true }).click();
  }

  // ── Relations, documents, export, comments ─────────────────────────

  /** A relation card for `id` on the detail view. */
  relationCard(id: string): Locator {
    return this.page.locator(`article.entity-card[data-entity-id="${id}"]`);
  }

  /** Wait for a relation section to render, so an absence assertion on its
   *  cards cannot pass before the section exists. */
  async waitForSection(id: string) {
    await expect(this.page.locator(`section.view-section#${id}`)).toBeVisible();
  }

  documentsPanel(): Locator {
    return this.page.locator('.documents-panel');
  }

  async expectDocumentContains(text: string) {
    await expect(this.documentsPanel().locator('.document-body')).toContainText(text);
  }

  /** Open the export menu and pick a transform. Resolves to the download. */
  async exportAs(transform: string) {
    await this.page.locator('.export-menu > button').click();
    const download = this.page.waitForEvent('download');
    await this.page.locator('.export-menu-item').filter({ hasText: transform }).click();
    return download;
  }

  commentsPanelSummary(): Locator {
    return this.page.locator('.comments-panel .panel-summary');
  }
}
