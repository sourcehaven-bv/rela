import { type Locator, type Page, expect } from "@playwright/test";
import { BasePage } from "./base.page";

/** The panels a sidebar entry with `open: flyout` slides out over the page. */
export class FlyoutPage extends BasePage {
  constructor(page: Page) {
    super(page);
  }

  /** The sidebar row that opens the flyout. Still a link to the list page. */
  entry(label: string): Locator {
    return this.sidebar.getByRole("link", { name: label, exact: true });
  }

  /** A slide-out panel, by the title it is announced with. */
  panel(title: string): Locator {
    return this.page.getByRole("complementary", { name: title, exact: true });
  }

  get panels(): Locator {
    return this.page.locator(".rl-slide-panel");
  }

  row(listTitle: string, entityId: string): Locator {
    return this.panel(listTitle).locator(`[data-entity-id="${entityId}"]`);
  }

  async open(label: string) {
    await this.entry(label).click();
    await expect(this.panel(label)).toBeVisible();
  }

  async openRow(listTitle: string, entityId: string) {
    await this.row(listTitle, entityId).getByRole("button").click();
  }

  /** The expand control in a panel's header. */
  async expand(title: string) {
    await this.panel(title).getByRole("button", { name: /^Open the full/ }).click();
  }

  /** The expand control of the row detail, the second panel. */
  async expandDetail() {
    await this.panels.nth(1).getByRole("button", { name: "Open the full page" }).click();
  }

  async close(title: string) {
    await this.panel(title).getByRole("button", { name: "Close panel" }).first().click();
  }

  async expectClosed() {
    await expect(this.panels).toHaveCount(0);
  }
}
