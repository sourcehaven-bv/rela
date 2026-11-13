import { type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * The remotes page and a remote's conflict list (TKT-0FMYPEZ).
 *
 * Selectors mirror `RemotesView.vue` (`remote-row`, `remote-conflict-total`),
 * `RemoteConflictsView.vue` (`remote-entry`, `remote-keep-local`,
 * `remote-dismiss`, `remote-link-open`, ...) and the footer link in
 * `SidebarFooter.vue` (`footer-remotes`).
 */
export class RemotesPage extends BasePage {
  rows(): Locator {
    return this.page.getByTestId('remote-row');
  }

  entries(): Locator {
    return this.page.getByTestId('remote-entry');
  }

  private entry(local: string): Locator {
    return this.entries().filter({ hasText: local });
  }

  /** Open the remotes page through the sidebar footer link. */
  async openFromFooter() {
    await this.page.getByTestId('footer-remotes').click();
    await expect(this.page.getByTestId('remotes-loaded')).toBeVisible({ timeout: 15_000 });
  }

  async expectRemote(name: string, total: number) {
    const row = this.rows().filter({ hasText: name });
    await expect(row).toHaveCount(1, { timeout: 15_000 });
    await expect(row.getByTestId('remote-conflict-total')).toContainText(String(total));
  }

  async openConflicts(name: string) {
    await this.rows().filter({ hasText: name }).getByRole('link', { name, exact: true }).click();
    await expect(this.page.getByTestId('remote-conflicts-loaded')).toBeVisible({ timeout: 15_000 });
  }

  async expectEntries(locals: string[]) {
    await expect(this.entries()).toHaveCount(locals.length, { timeout: 15_000 });
    for (const l of locals) await expect(this.entry(l)).toHaveCount(1);
  }

  async expectNoText(text: string) {
    await expect(this.page.getByText(text)).toHaveCount(0);
  }

  async expectEntryText(local: string, text: string) {
    await expect(this.entry(local)).toContainText(text);
  }

  async expectNotKept(local: string) {
    await expect(this.entry(local).getByTestId('remote-entry-not-kept')).toBeVisible();
  }

  async expectActions(local: string, actions: { keep: boolean; take: boolean; dismiss: boolean; link: boolean }) {
    const e = this.entry(local);
    await expect(e.getByTestId('remote-keep-local')).toHaveCount(actions.keep ? 1 : 0);
    await expect(e.getByTestId('remote-take-remote')).toHaveCount(actions.take ? 1 : 0);
    await expect(e.getByTestId('remote-dismiss')).toHaveCount(actions.dismiss ? 1 : 0);
    await expect(e.getByTestId('remote-link-open')).toHaveCount(actions.link ? 1 : 0);
  }

  async keepLocal(local: string) {
    await this.entry(local).getByTestId('remote-keep-local').click();
  }

  async dismiss(local: string) {
    await this.entry(local).getByTestId('remote-dismiss').click();
  }

  async link(local: string, address: string) {
    const e = this.entry(local);
    await e.getByTestId('remote-link-open').click();
    await e.getByTestId('remote-link-address').locator('input').fill(address);
    await e.getByTestId('remote-link-submit').click();
  }

  /** Answer the in-app confirm dialog with the button `label`. */
  async answerDialog(label: string) {
    const dialog = this.page.locator('[role=dialog], [role=alertdialog]').last();
    await expect(dialog).toBeVisible({ timeout: 15_000 });
    await dialog.getByRole('button', { name: label, exact: true }).click();
    await expect(dialog).toBeHidden();
  }
}
