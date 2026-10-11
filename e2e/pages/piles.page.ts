import { type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';
import { FlyoutPage } from './flyout.page';
import { SidebarPage } from './sidebar.page';

/**
 * Piles (TKT-K3RJLH): the "Add to pile" menu, the New pile dialog, the
 * sidebar's Piles group and the pile panel in the sidebar flyout.
 */
export class PilesPage extends BasePage {
  private get sidebarNav(): SidebarPage {
    return new SidebarPage(this.page);
  }

  private get flyout(): FlyoutPage {
    return new FlyoutPage(this.page);
  }

  // ── Add to pile ───────────────────────────────────────────────────────────

  /** The "Add to pile" trigger; the list's bulk bar and the entity page each render one. */
  get addToPileButton(): Locator {
    return this.page.getByTestId('add-to-pile').getByRole('button', { name: 'Add to pile' });
  }

  /** Open "Add to pile" and choose its last entry, which makes a new pile. */
  async chooseNewPile() {
    await this.addToPileButton.click();
    await this.page.getByTestId('add-to-new-pile').click();
    await expect(this.newPileDialog).toBeVisible();
  }

  // ── New pile dialog ───────────────────────────────────────────────────────

  get newPileDialog(): Locator {
    return this.page.getByRole('dialog', { name: 'New pile' });
  }

  async expectNewPileItemCount(count: number) {
    await expect(this.newPileDialog.getByTestId('new-pile-items')).toContainText(
      `The ${count} selected item`,
    );
  }

  /** Name the pile and create it; waits for the dialog to close. */
  async createPile(name: string) {
    await this.newPileDialog.getByLabel('Name').fill(name);
    await this.newPileDialog.getByRole('button', { name: 'Create pile' }).click();
    await expect(this.newPileDialog).toBeHidden();
  }

  // ── Sidebar Piles group ───────────────────────────────────────────────────

  /** A pile's row in the sidebar's Piles group. */
  sidebarRow(name: string): Locator {
    return this.sidebarNav
      .group('Piles')
      .locator('.rl-nav-item')
      .filter({ has: this.page.locator('.rl-nav-item__label', { hasText: name }) });
  }

  /** The row shows `count` readable items. */
  async expectSidebarCount(name: string, count: number) {
    await expect(this.sidebarRow(name).locator('.rl-nav-status__count')).toHaveText(String(count));
  }

  /** Open the pile in the flyout. */
  async openPile(name: string) {
    await this.sidebarRow(name).click();
    await expect(this.panel(name)).toBeVisible();
  }

  // ── Pile panel ────────────────────────────────────────────────────────────

  /** The flyout panel of a pile, announced by the pile's name. */
  panel(name: string): Locator {
    return this.flyout.panel(name).getByTestId('pile-panel');
  }

  /** A pile item's row, by its displayed title. */
  item(name: string, title: string): Locator {
    return this.panel(name).getByTestId('pile-row').filter({ hasText: title });
  }

  /** The panel lists exactly these titles, in any order. */
  async expectItems(name: string, titles: string[]) {
    const rows = this.panel(name).getByTestId('pile-row');
    await expect(rows).toHaveCount(titles.length);
    for (const title of titles) await expect(this.item(name, title)).toBeVisible();
  }

  async expectItemAbsent(name: string, title: string) {
    await expect(this.item(name, title)).toHaveCount(0);
  }

  /** The panel head's "N items" line. */
  async expectItemCount(name: string, count: number) {
    await expect(this.panel(name)).toContainText(`${count} item${count === 1 ? '' : 's'}`);
  }

  /** Tick an item's checkbox, which the row labels with the item's title. */
  async tickItem(name: string, title: string) {
    await this.item(name, title).getByRole('checkbox', { name: title }).check();
  }

  /** Remove the ticked items with the head's Remove button. */
  async removeTicked(name: string) {
    await this.panel(name).getByTestId('pile-remove').click();
  }

  /** Open the pile's first item in pile scope; the flyout closes as the route changes. */
  async stepThrough(name: string) {
    await this.panel(name).getByTestId('pile-step-through').click();
    await this.page.waitForURL((url) => url.searchParams.get('from') === 'pile');
  }

  // ── Toasts ────────────────────────────────────────────────────────────────

  /** A toast whose title contains `text`. */
  toast(text: string): Locator {
    return this.toastContainer.filter({ hasText: text });
  }

  /** Press Undo in the toast whose title contains `text`. */
  async undo(text: string) {
    await this.toast(text).getByRole('button', { name: 'Undo' }).click();
  }
}
