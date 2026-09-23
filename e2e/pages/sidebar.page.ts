import { type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/** The sidebar's navigation groups, including `entities:` entries (TKT-PEKL8L). */
export class SidebarPage extends BasePage {
  /** The navigation group whose heading is `title`. */
  group(title: string): Locator {
    return this.page
      .locator('.sidebar-nav .nav-section')
      .filter({ has: this.page.locator('.nav-section-title', { hasText: title }) });
  }

  /** Assert the group's links read exactly `labels`, in order. */
  async expectGroupLinks(title: string, labels: string[]) {
    await expect(this.group(title).locator('a.nav-item .nav-label')).toHaveText(labels);
  }

  /** Assert the group is not shown at all. */
  async expectGroupAbsent(title: string) {
    await expect(this.group(title)).toBeHidden();
  }

  /** Click a link in the group and wait for the URL to name `id`. */
  async openGroupLink(title: string, label: string, id: string) {
    await this.group(title).locator('a.nav-item', { hasText: label }).click();
    await this.page.waitForURL((url) => url.pathname.includes(id));
  }
}
